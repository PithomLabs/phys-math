# AGENTS.md — `phys-math` Agent Notes

Package-local operating notes for coding agents working in `phys-math`.
The collective workflow contract lives in `phys-system/AGENTS.md`; this file
records package-specific lessons. Do not put physical (framework) meaning
into this package — pure mathematics only.

## Test suite: use parallel shards, not one serial run

The full suite (`go test ./...`) is dominated by a few heavy
geometry-building tests and runs serially inside the package:

| Shard | Selector | Wall time (16-core reference) |
|---|---|---|
| A — obligation tests | `-run 'TestObligation\|TestWitness'` | ~14 min |
| B — divergence tests | `-run 'TestDivergence'` | ~5 min |
| C — everything else | `-skip 'TestObligation\|TestDivergence\|TestWitness'` | seconds |

Serial total is ~30 min; sharded wall time is bounded by shard A.

### Rules

1. **Always pass an explicit timeout.** The default 10-minute `go test`
   timeout kills the suite mid-run (it looks like a hang; it is not).
   Minimums: `-timeout 115m` for shard A / full runs, `-timeout 20m` for
   shard B and focused goldens.
2. **Always pass `-count=1`.** No cached results for verification runs.
3. **Never run two suites concurrently** against this package. Geometry
   builds are CPU-bound; concurrent runs slow each other and invalidate
   timing expectations. One exception: the three shards A/B/C, which are
   disjoint by construction (separate binaries, in-memory stores only).
4. **The A/B/C partition covers every test exactly once.** Jointly green
   shards count as a green full suite; say so explicitly in the report
   (list the three commands and exits) rather than re-running serially.
5. **Shard C is the fast feedback loop.** Registry, purity, codec, and all
   pre-1.18 goldens finish in seconds — run it first after edits, then the
   heavy shard(s) covering what changed.
6. **Do not retrofit `t.Parallel()`** to speed things up. Shared test
   helpers were written for serial execution; parallelizing them is
   out-of-scope churn with race risk. Process-level sharding is the
   approved mechanism.
7. **Deduplicate builds within a run, never verdicts across runs.**
   Expensive geometry chains are built once per parent test and cloned
   per subtest (`cloneMemStore`); a green verdict from a previous run or
   an unchanged input digest never excuses re-execution, because the
   suite guards the engine and any edit can change what identical inputs
   produce. Review must reject content-address-equality skip proposals.
8. **Report actual commands, exit codes, and timings** for every
   verification run. Never substitute a prior run's results.

### Tiers (Plan 1.20)

- **Tier 0** — shard C + registry/purity gate + `go vet` + `gofmt`.
  Always runs. Seconds.
- **Tier 1** — the affected heavy shard (A and/or B). Runs when
  `Evaluate*`, checker, fixture, or test files in its area change.
- **Tier 2** — all shards. Pre-commit gate only.

Each milestone plan declares its tiers up front; the commit report lists
which tiers ran and why the others were excluded. Milestone checklists
carry the line: "no verdicts reused across runs."

### Canonical verification sequence (Plan 1.19 and later)

```bash
# from the phys-math root, shards in parallel background jobs:
go test -count=1 -timeout 115m -run 'TestObligation|TestWitness' -v .
go test -count=1 -timeout 115m -run 'TestDivergence' -v .
go test -count=1 -timeout 115m -skip 'TestObligation|TestDivergence|TestWitness' .

# registry / purity / import gate (fast, explicit):
go test -count=1 -run 'TestRegistryCompleteness|TestNoPhysicalVocabulary|TestImportDiscipline' -v .

# static checks:
go vet ./...
gofmt -l .
```
