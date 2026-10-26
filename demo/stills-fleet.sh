#!/usr/bin/env bash
# Builds a deterministic fleet of throwaway repos for demo/stills.tape and prints
# the exports that point the dashboard at it, so the real HOME and config are never read.
# The TUI scans where it is launched, so start it from $STILLS_FLEET.
# Usage: eval "$(demo/stills-fleet.sh [root])"
set -euo pipefail

root=${1:-$(mktemp -d)}
root=$(cd "$root" && pwd -P)
if [ -n "$(ls -A "$root")" ]; then
  echo "refusing to build into a non-empty $root" >&2
  exit 1
fi
fleet=$root/fleet
home=$root/home
mkdir -p "$fleet" "$home/.config/gh-repo-dashboard" "$home/.cache"

# A mise shim cannot resolve its tool once HOME moves, so take the real binary.
jj_bin=
real_path=$(printf '%s' "$PATH" | tr ':' '\n' | grep -v '/shims$' | paste -sd: -)
if jj_path=$(PATH=$real_path command -v jj); then
  jj_bin=$(dirname "$jj_path")
fi

export GIT_CONFIG_GLOBAL=$home/.gitconfig GIT_CONFIG_NOSYSTEM=1
git config --global user.name "Demo Dev"
git config --global user.email "demo@example.com"
git config --global init.defaultBranch main
git config --global advice.detachedHead false

now=$(date +%s)
commit() { # name message hours-ago
  local when=$(( now - $3 * 3600 ))
  GIT_AUTHOR_DATE="@$when +0000" GIT_COMMITTER_DATE="@$when +0000" git commit -q -m "$2"
}

mkrepo() { # name hours-ago
  local dir=$fleet/$1
  mkdir -p "$dir"
  cd "$dir"
  git init -q
  printf '# %s\n' "$1" > README.md
  printf 'package main\n\nfunc main() {}\n' > main.go
  git add -A
  commit "$1" "feat: initial commit" "$(( $2 + 48 ))"
  printf 'v2\n' >> README.md
  git add -A
  commit "$1" "docs: expand readme" "$(( $2 + 24 ))"
  git remote add origin "https://github.com/acme/$1.git"
  git update-ref refs/remotes/origin/main HEAD
  git branch -q --set-upstream-to=origin/main main
}

mkrepo alpha 2
printf 'func helper() {}\n' >> main.go
printf 'scratch\n' > notes.tmp

mkrepo beta 5
printf 'x\n' > feature.txt
git add feature.txt
commit beta "feat: add feature file" 1

mkrepo gamma 30

mkrepo delta 52
printf -- '- finish the migration\n- ask about the cache TTL\n' > .doing
git add .doing
commit delta "chore: add notes" 50
git update-ref refs/remotes/origin/main HEAD

mkrepo epsilon 70
printf 'wip\n' >> README.md
git stash push -q -m "wip: readme experiment"

mkrepo eta 96

mkrepo theta 8
git switch -q -c feature/login
printf 'func login() {}\n' >> main.go
git add -A
commit theta "feat: start login" 3
printf 'half done\n' >> main.go

cd "$fleet/beta"
git worktree add -q "$fleet/beta-review" -b review/spike

jj_run() { PATH=$jj_bin:$PATH jj "$@"; }

if [ -n "$jj_bin" ]; then
  export JJ_USER="Demo Dev" JJ_EMAIL=demo@example.com
  mkdir "$fleet/zeta"
  cd "$fleet/zeta"
  jj_run git init --colocate >/dev/null 2>&1
  printf '# zeta\n' > README.md
  jj_run describe -m "feat: bootstrap zeta" >/dev/null 2>&1
  jj_run bookmark create main -r @ >/dev/null 2>&1
  jj_run git remote add origin https://github.com/acme/zeta.git >/dev/null 2>&1
  jj_run new >/dev/null 2>&1
  printf 'more\n' >> README.md
fi

cat > "$home/.config/gh-repo-dashboard/config.toml" <<TOML
scan_paths = ["$fleet"]
depth = 1
cache_to_disk = false

[[tasks]]
name = "gc"
desc = "repack and prune loose objects"
run = "git gc --quiet"
every = "30d"
where = "has_notes"

[[tasks]]
name = "dirty-status"
desc = "short status of repos with local changes"
run = "git status --short"
where = "dirty"
TOML

printf 'export STILLS_FLEET=%q\n' "$fleet"
printf 'export HOME=%q\n' "$home"
printf 'export XDG_CONFIG_HOME=%q\n' "$home/.config"
printf 'export XDG_CACHE_HOME=%q\n' "$home/.cache"
printf 'export GIT_CONFIG_GLOBAL=%q GIT_CONFIG_NOSYSTEM=1\n' "$home/.gitconfig"
printf 'export JJ_USER=%q JJ_EMAIL=%q\n' "Demo Dev" demo@example.com
[ -z "$jj_bin" ] || printf 'export PATH=%q:"$PATH"\n' "$jj_bin"
printf "export PS1='\$ '\n"
