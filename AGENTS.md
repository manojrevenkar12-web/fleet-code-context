# Working in this repository

Fast feedback first: `make test` runs in seconds. `go run ./cmd/fcc eval` takes about a minute and rebuilds the
crowded fleet in a temporary directory.

## Invariants

- **Nothing that serves reads the truth.** `eval/truth/` is read by `internal/eval` and tests only. The index,
  drift checker, ranker, packer and MCP server never open it (a test enforces this).
- **The held-out tickets are run once.** `eval/tasks/heldout.json` is hashed in `config/heldout.lock`;
  `fcc eval -heldout` refuses to run if it changed, and keeps `reports/heldout/first_run.json` as written. Tune
  on `eval/tasks/dev.json` only.
- **Settings in config.** Ranker weights live in `config/rank.json`, pack defaults in `config/pack.json`; a test
  keeps the checked-in ranker config equal to the code defaults.
- **Stale instructions are labelled, never passed on as instructions.** Anything the drift checker flags goes in
  the pack's out-of-date section with its reason.
- **Read-only tools, no ticket text in telemetry.** Every MCP tool is read-only; telemetry records tool, latency,
  tokens and errors.
- **Generated reports.** `reports/*`, `reports/REPORT.md` and the README dashboard come from `make eval bench
  report`. Do not edit them by hand.
