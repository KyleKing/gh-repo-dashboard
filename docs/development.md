# Development

Setup, the task table, the live integration test, and the release process live in
[CONTRIBUTING.md](../CONTRIBUTING.md), which the copier template re-renders on
every update. This page holds the few things that file does not cover yet, and
they belong in the template once it has a section for them.

## Tests

`mise run test`, `mise run test:golden`, and `mise run ci` cover the usual paths.
Two extras have no task:

```bash
go test -race ./...              # race detector, part of the release checklist
go test -v ./internal/filters/... # one package, verbose
```

Golden-file tests run under a build tag. `mise run test:golden-update`
regenerates the snapshots.

## Recording the demo

`mise run demo` builds the binary and replays every `.tape` file through
[VHS](https://github.com/charmbracelet/vhs). To record just this one:

```bash
vhs < .github/assets/demo.tape
```

The tape writes `.github/assets/demo.gif`, which the README embeds. It needs
`vhs` on `PATH`, the "Hack Nerd Font Mono" font installed, and it drives a real
`~/Developer/kyleking` checkout, so the recorded repo list reflects whatever is
on disk when you run it.

## Regenerating usage docs

Fixtures in `internal/app/testdata/fixtures/` generate `docs/USAGE.md`. Run
`mise run docs:usage` after changing a fixture, and never edit that file by hand.
