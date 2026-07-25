#!/usr/bin/env bash
#
# live-test.sh - end-to-end integration test for the DESTRUCTIVE `:cleanup` path.
#
# `:cleanup` runs `git branch -d` on merged branches and `git branch -D` on
# branches whose tip OID matches a merged pull request's head (the squash-merge
# path). Unit tests cover that logic with mocks; this script proves it against
# real git and a real GitHub repo.
#
# It creates its own throwaway GitHub repo, clones it into a temp dir, builds
# four branch classes, and asserts which ones survive. It never touches a
# pre-existing repo and never runs outside the temp dir.
#
# Run it with `mise run test:live`. See CONTRIBUTING.md ("Live integration test").

set -euo pipefail

readonly SCRATCH_PREFIX="gh-rd-livetest-"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
readonly REPO_ROOT

# State the trap needs. Set before anything can fail so cleanup is always safe.
TMP_ROOT=""
SCRATCH_REPO=""
SCRATCH_FULL=""
PROBE_REPO=""
FAILURES=0
CHECKS=0

KEEP="${KEEP:-0}"
OPTED_IN="${GH_RD_LIVE_TEST:-0}"

for arg in "$@"; do
    case "$arg" in
    --yes) OPTED_IN=1 ;;
    --keep) KEEP=1 ;;
    -h | --help)
        awk 'NR>1 && /^#/ {sub(/^# ?/, ""); print; next} NR>1 {exit}' "${BASH_SOURCE[0]}"
        exit 0
        ;;
    *)
        echo "unknown argument: $arg (expected --yes, --keep, or --help)" >&2
        exit 1
        ;;
    esac
done

# ---------------------------------------------------------------------------
# Output helpers
# ---------------------------------------------------------------------------

step() { printf '\n=== %s\n' "$*"; }
info() { printf '    %s\n' "$*"; }

pass() {
    CHECKS=$((CHECKS + 1))
    printf '  PASS  %s\n' "$*"
}

fail() {
    CHECKS=$((CHECKS + 1))
    FAILURES=$((FAILURES + 1))
    printf '  FAIL  %s\n' "$*" >&2
}

die() {
    printf 'ERROR: %s\n' "$*" >&2
    exit 1
}

assert_contains() {
    local haystack="$1" needle="$2" label="$3"
    if [[ "$haystack" == *"$needle"* ]]; then
        pass "$label"
    else
        fail "$label (expected to find '$needle')"
    fi
}

assert_eq() {
    local got="$1" want="$2" label="$3"
    if [[ "$got" == "$want" ]]; then
        pass "$label"
    else
        fail "$label (got '$got', want '$want')"
    fi
}

# ---------------------------------------------------------------------------
# Safety guards. Every destructive call goes through these.
# ---------------------------------------------------------------------------

# assert_scratch_name refuses any repo name that is not one this script minted.
assert_scratch_name() {
    local name="${1##*/}"
    case "$name" in
    "$SCRATCH_PREFIX"?*) return 0 ;;
    *) die "refusing to delete '$1': name does not start with ${SCRATCH_PREFIX}" ;;
    esac
}

# assert_in_tmp refuses to operate anywhere outside the mktemp working dir.
assert_in_tmp() {
    local here
    here="$(pwd -P)"
    [[ -n "$TMP_ROOT" ]] || die "refusing to run: temp dir is unset"
    case "$here" in
    "$TMP_ROOT" | "$TMP_ROOT"/*) return 0 ;;
    *) die "refusing to run git in '$here': not under $TMP_ROOT" ;;
    esac
}

delete_scratch_repo() {
    local name="$1"
    [[ -n "$name" ]] || return 0
    assert_scratch_name "$name"
    gh repo delete "$name" --yes >/dev/null 2>&1 ||
        printf 'WARNING: could not delete %s - delete it manually\n' "$name" >&2
}

on_exit() {
    local code=$?
    if [[ "$KEEP" == "1" ]]; then
        printf '\nKEEP=1: leaving %s and %s in place\n' "${SCRATCH_FULL:-<no repo>}" "${TMP_ROOT:-<no dir>}"
        return "$code"
    fi

    step "Cleaning up"
    delete_scratch_repo "${PROBE_REPO:-}"
    delete_scratch_repo "${SCRATCH_FULL:-}"
    if [[ -n "$TMP_ROOT" && -d "$TMP_ROOT" ]]; then
        case "$TMP_ROOT" in
        /* ) rm -rf "$TMP_ROOT" && info "removed $TMP_ROOT" ;;
        *) printf 'WARNING: temp dir %s is not absolute, not removing\n' "$TMP_ROOT" >&2 ;;
        esac
    fi
    return "$code"
}

# ---------------------------------------------------------------------------
# Opt-in gate
# ---------------------------------------------------------------------------

if [[ "$OPTED_IN" != "1" ]]; then
    cat >&2 <<EOF
This is a LIVE integration test. It talks to real GitHub and deletes real branches.

It will:
  CREATE  a private GitHub repo named ${SCRATCH_PREFIX}<epoch>-<rand> under your account
  CREATE  a pull request in that repo and squash-merge it
  CREATE  a clone in a fresh mktemp directory
  DELETE  local branches in that clone (the behavior under test)
  DELETE  the scratch GitHub repo and the temp directory when it finishes

It will NOT touch any existing repo, your ~/Developer tree, or any branch outside
the temp clone.

Cost: a few GitHub API calls and roughly 30 seconds. Requires the delete_repo scope.

Re-run with --yes, or set GH_RD_LIVE_TEST=1, to opt in.
EOF
    exit 1
fi

trap on_exit EXIT

# ---------------------------------------------------------------------------
# Preflight
# ---------------------------------------------------------------------------

step "Preflight"

for tool in gh git go; do
    command -v "$tool" >/dev/null 2>&1 || die "$tool is not installed"
done
info "gh, git, go present"

gh auth status >/dev/null 2>&1 || die "gh is not authenticated - run 'gh auth login'"
GH_USER="$(gh api user --jq .login)"
info "authenticated as $GH_USER"

# A missing delete_repo scope leaks the scratch repo, so prove deletion works
# before creating anything that matters. Classic tokens advertise their scopes;
# fine-grained tokens do not, so fall back to a create/delete probe on a repo
# that is itself disposable and named with the scratch prefix.
AUTH_STATUS="$(gh auth status 2>&1 || true)"
if [[ "$AUTH_STATUS" == *"Token scopes:"* && "$AUTH_STATUS" == *"delete_repo"* ]]; then
    info "token advertises the delete_repo scope"
else
    probe="${SCRATCH_PREFIX}probe-$(date +%s)-$$"
    info "token scopes unknown (fine-grained PAT?) - probing with $probe"
    # PROBE_REPO is assigned only after the repo exists, so a create failure
    # doesn't make the trap warn about a repo that was never made.
    gh repo create "$probe" --private >/dev/null || die "cannot create repositories with this token.
A fine-grained PAT needs 'Administration: Read and write' on the target account;
a classic token needs the 'repo' and 'delete_repo' scopes."
    PROBE_REPO="$probe"
    assert_scratch_name "$PROBE_REPO"
    if ! gh repo delete "$PROBE_REPO" --yes >/dev/null 2>&1; then
        die "cannot delete repos with this token (needs the delete_repo scope).
The probe repo $GH_USER/$PROBE_REPO has LEAKED - delete it manually:
  gh repo delete $GH_USER/$PROBE_REPO --yes"
    fi
    PROBE_REPO=""
    info "create and delete both work"
fi

# ---------------------------------------------------------------------------
# Build the binary under test from source
# ---------------------------------------------------------------------------

TMP_ROOT="$(cd "$(mktemp -d)" && pwd -P)"
readonly TMP_ROOT
info "temp dir: $TMP_ROOT"

step "Building gh-repo-dashboard from source"
BIN="$TMP_ROOT/gh-repo-dashboard"
(cd "$REPO_ROOT" && go build -o "$BIN" ./cmd/gh-repo-dashboard)
info "built $BIN"

# ---------------------------------------------------------------------------
# Create the scratch repo
# ---------------------------------------------------------------------------

step "Creating scratch GitHub repo"
# od reads a fixed number of bytes, so nothing downstream closes the pipe early;
# `tr ... | head -c` would raise SIGPIPE and trip `set -o pipefail`.
SCRATCH_REPO="${SCRATCH_PREFIX}$(date +%s)-$(od -An -tx1 -N3 /dev/urandom | tr -d ' \n')"
readonly SCRATCH_REPO
assert_scratch_name "$SCRATCH_REPO"
SCRATCH_FULL="$GH_USER/$SCRATCH_REPO"
readonly SCRATCH_FULL

gh repo create "$SCRATCH_REPO" --private --add-readme >/dev/null
info "created $SCRATCH_FULL"

WORK="$TMP_ROOT/clone"
gh repo clone "$SCRATCH_FULL" "$WORK" -- --quiet
cd "$WORK"
assert_in_tmp

git config user.name "gh-repo-dashboard live test"
git config user.email "live-test@example.invalid"
git config commit.gpgsign false

MAIN="$(git symbolic-ref --short refs/remotes/origin/HEAD)"
MAIN="${MAIN#origin/}"
readonly MAIN
info "default branch: $MAIN"

commit_file() {
    local name="$1"
    assert_in_tmp
    echo "$name $(date +%s%N)" >"$name.txt"
    git add "$name.txt"
    git commit --quiet -m "add $name"
}

# ---------------------------------------------------------------------------
# Build the four branch classes
# ---------------------------------------------------------------------------

step "Setting up branch fixtures"

# (a) merged-branch: a real merge into the default branch. `git branch --merged`
#     sees it, so cleanup deletes it with `git branch -d`.
git checkout --quiet -b merged-branch
commit_file merged
git checkout --quiet "$MAIN"
git merge --quiet --no-ff -m "merge merged-branch" merged-branch
git push --quiet origin "$MAIN"
info "merged-branch: merged into $MAIN"

# (b) squashed-branch: squash-merged through a PR. Its tip is NOT an ancestor of
#     the default branch, so `git branch --merged` misses it. Cleanup must
#     recognize it by matching the branch tip against the merged PR's head OID
#     and delete it with `git branch -D`. This is the path most worth covering.
git checkout --quiet -b squashed-branch
commit_file squashed
git push --quiet -u origin squashed-branch
gh pr create --title "squashed-branch" --body "live test" --base "$MAIN" --head squashed-branch >/dev/null
PR_NUMBER="$(gh pr list --head squashed-branch --json number --jq '.[0].number')"
[[ -n "$PR_NUMBER" ]] || die "could not find the PR for squashed-branch"

# GitHub needs a moment to compute mergeability on a brand-new PR.
merged_ok=0
for _ in 1 2 3 4 5 6 7 8 9 10; do
    if gh pr merge "$PR_NUMBER" --squash --subject "squash squashed-branch" --body "live test" >/dev/null 2>&1; then
        merged_ok=1
        break
    fi
    sleep 2
done
[[ "$merged_ok" == "1" ]] || die "could not squash-merge PR #$PR_NUMBER"

SQUASH_TIP="$(git rev-parse squashed-branch)"
PR_HEAD_OID="$(gh pr view "$PR_NUMBER" --json headRefOid --jq .headRefOid)"
assert_eq "$SQUASH_TIP" "$PR_HEAD_OID" "squashed-branch tip matches the merged PR head OID"
info "squashed-branch: squash-merged as PR #$PR_NUMBER"

git fetch --quiet origin
git checkout --quiet "$MAIN"
git merge --quiet --ff-only "origin/$MAIN"

# (c) unmerged-branch: a unique commit, no PR, nothing merged. Deleting this is
#     data loss. It is the single most important assertion in this file.
git checkout --quiet -b unmerged-branch
commit_file unmerged
info "unmerged-branch: unique commit, no PR"

# (d) current-branch: checked out, and deliberately positioned at the default
#     branch tip so `git branch --merged` reports it. `git branch -d` refuses to
#     delete a checked-out branch, so it must survive. This is also the fixture
#     that exposes the dry-run over-reporting bug documented below.
git checkout --quiet "$MAIN"
git checkout --quiet -b current-branch
info "current-branch: checked out at the $MAIN tip"

info "branches before cleanup: $(git branch --format='%(refname:short)' | tr '\n' ' ')"

# ---------------------------------------------------------------------------
# Dry run
# ---------------------------------------------------------------------------

step "Running :cleanup --dry-run"

DRY_OUT="$(printf ':cleanup --dry-run\n' | "$BIN" --script - "$WORK" 2>&1)"
printf '%s\n' "$DRY_OUT" | sed 's/^/    | /'

# The dry run's own claim, captured so the real run can be diffed against it.
DRY_LIST="$(printf '%s\n' "$DRY_OUT" | sed -n 's/.*Would delete [0-9]* branches: //p' | head -1)"
info "dry run would delete: ${DRY_LIST:-<nothing>}"
[[ -n "$DRY_LIST" ]] || fail "dry run printed no 'Would delete' line"

# split_list turns a "a, b, c" summary list into one name per line. The trailing
# newline matters: without it `read` returns non-zero on the final name and every
# caller silently skips the last branch in the list.
split_list() { printf '%s\n' "$1" | tr ',' '\n' | tr -d ' '; }

# in_list matches a branch name exactly. A substring test would be wrong here:
# "unmerged-branch" contains "merged-branch", so a plain grep would report the
# most dangerous false positive in this file as a pass.
in_list() {
    local want="$1" line
    while IFS= read -r line; do
        [[ "$line" == "$want" ]] && return 0
    done < <(split_list "$2")

    return 1
}

assert_listed() {
    local branch="$1" list="$2" label="$3"
    if in_list "$branch" "$list"; then
        pass "$label"
    else
        fail "$label (not named in '$list')"
    fi
}

assert_not_listed() {
    local branch="$1" list="$2" label="$3"
    if in_list "$branch" "$list"; then
        fail "$label (named in '$list')"
    else
        pass "$label"
    fi
}

assert_listed merged-branch "$DRY_LIST" "dry run reports merged-branch"
assert_listed squashed-branch "$DRY_LIST" "dry run reports squashed-branch (OID-verified squash merge)"
assert_not_listed unmerged-branch "$DRY_LIST" "dry run does NOT report unmerged-branch"
assert_not_listed "$MAIN" "$DRY_LIST" "dry run does NOT report the default branch $MAIN"

# KNOWN ISSUE (harness-confirmed, do not "fix" here): `:cleanup --dry-run`
# lists the checked-out branch. batch.PreviewCleanup reports everything
# `git branch --merged` returns, but the real run's `git branch -d` refuses to
# delete a checked-out branch, so the branch is reported and then survives.
# The dry run over-reports; it never under-reports, so no branch is deleted
# without warning. The assertion below pins the CURRENT behavior on purpose: if
# the source is fixed, this check flips to FAIL and tells you to update it.
if in_list current-branch "$DRY_LIST"; then
    pass "KNOWN ISSUE reproduced: dry run over-reports the checked-out branch (current-branch)"
else
    fail "KNOWN ISSUE appears fixed: dry run no longer reports current-branch - update this script"
fi

BEFORE="$(git branch --format='%(refname:short)' | sort)"

# ---------------------------------------------------------------------------
# Real run
# ---------------------------------------------------------------------------

step "Running :cleanup for real"

assert_in_tmp
REAL_OUT="$(printf ':cleanup\n' | "$BIN" --script - "$WORK" 2>&1)"
printf '%s\n' "$REAL_OUT" | sed 's/^/    | /'

AFTER="$(git branch --format='%(refname:short)' | sort)"
info "branches after cleanup: $(printf '%s' "$AFTER" | tr '\n' ' ')"

# ---------------------------------------------------------------------------
# Post-state assertions
# ---------------------------------------------------------------------------

step "Asserting post-state"

has_branch() { git branch --list "$1" --format='%(refname:short)' | grep -qx "$1"; }

if has_branch merged-branch; then
    fail "merged-branch MUST be deleted, but it survived"
else
    pass "merged-branch was deleted"
fi

if has_branch squashed-branch; then
    fail "squashed-branch MUST be deleted (squash-merge OID path), but it survived"
else
    pass "squashed-branch was deleted"
fi

if has_branch unmerged-branch; then
    pass "unmerged-branch SURVIVED (a deletion here is data loss)"
else
    fail "DATA LOSS: unmerged-branch was deleted"
fi

if has_branch current-branch; then
    pass "current-branch (checked out) SURVIVED"
else
    fail "DATA LOSS: the checked-out branch current-branch was deleted"
fi

if has_branch "$MAIN"; then
    pass "$MAIN SURVIVED"
else
    fail "DATA LOSS: the default branch $MAIN was deleted"
fi

assert_eq "$(git rev-parse --abbrev-ref HEAD)" "current-branch" "HEAD is still on current-branch"

# ---------------------------------------------------------------------------
# Dry run vs. real run
# ---------------------------------------------------------------------------

step "Comparing dry run against the real run"

DELETED="$(comm -23 <(printf '%s\n' "$BEFORE") <(printf '%s\n' "$AFTER") | tr '\n' ',')"
DELETED="${DELETED%,}"
info "actually deleted: ${DELETED:-<nothing>}"

# Every branch that was actually deleted must have been named in the dry run.
# The reverse is not asserted, because of the known over-reporting issue above.
while IFS= read -r branch; do
    [[ -n "$branch" ]] || continue
    assert_listed "$branch" "$DRY_LIST" "dry run predicted the deletion of $branch"
done < <(split_list "$DELETED")

# And the dry run's extra name is exactly the one known issue, nothing more.
while IFS= read -r branch; do
    [[ -n "$branch" ]] || continue
    if in_list "$branch" "$DELETED"; then
        continue
    fi
    assert_eq "$branch" "current-branch" "dry run's only unfulfilled prediction is the known issue"
done < <(split_list "$DRY_LIST")

# The real run's own summary must agree with the branches that actually vanished.
REAL_LIST="$(printf '%s\n' "$REAL_OUT" | sed -n 's/.*Deleted [0-9]* branches: \([^;]*\).*/\1/p' | head -1)"
info "real run claims it deleted: ${REAL_LIST:-<nothing>}"
assert_listed merged-branch "$REAL_LIST" "real run names merged-branch in its summary"
assert_listed squashed-branch "$REAL_LIST" "real run names squashed-branch in its summary"
assert_not_listed unmerged-branch "$REAL_LIST" "real run does NOT name unmerged-branch in its summary"
assert_contains "$REAL_OUT" "failed: current-branch" "real run reports the checked-out branch as a failed delete"

# ---------------------------------------------------------------------------
# Result
# ---------------------------------------------------------------------------

step "Result"
printf '  %d checks, %d failures\n' "$CHECKS" "$FAILURES"

if [[ "$FAILURES" -gt 0 ]]; then
    exit 1
fi
printf '  live cleanup test passed\n'
