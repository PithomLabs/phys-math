# PhysMath — theory-neutral mathematical representation

`github.com/PithomLabs/phys-math` is the pure mathematical common
denominator of the Phys architecture. It represents mathematical
structures and executes a closed set of explicitly specified
operations — without knowing general relativity, quantum mechanics,
Bohmian mechanics, or any other physical interpretation.

Every exported API must be explainable without naming a physical
theory. `purity_test.go` and the `TestNoPhysicalVocabulary` /
`TestImportDiscipline` gates enforce that boundary.

## What it owns

- **Structured expressions**: symbols (ephemeral, bound-parameter,
  constant), rationals, arithmetic nodes, powers, function calls,
  relations (`ast.go`).
- **Functions, parameters, and identity**: declaring functions with
  explicit parameter vocabularies, so differentiation targets are
  never inferred from display names (`identity.go`).
- **Tensors, geometry, and coordinate structures**, with rank, slot,
  variance, and space discipline.
- **Derivations, theorems, equations, conditions, and obligations**
  as durable, content-addressed artifacts (`families.go`,
  `store.go`).
- **Nine closed executable rules** — and no others:
  `EXPR-CANON-001`, `EXPR-SIMPLIFY-001`, `EXPR-SUBSTITUTE-001`,
  `RAT-NORMALIZE-001`, `DIFFERENTIATE-001`,
  `TOTAL-DERIVATIVE-001`, `EXPR-COMBINE-001`, `SELECT-BRANCH-001`,
  `EXPR-APPLY-001` (`rules.go`).
- **Replay** (`replay.go`): re-executing a recorded derivation from
  its declared premises, inputs, parameters, and conditions.

## What it explicitly does not own

Physical meaning of any kind: no theories, assumptions,
conventions, interpretations, or trust semantics. It also does not
provide unlimited symbolic mathematics — there is no general
distributivity, term collection, or rational-function normalization
beyond the nine rules, and an equivalence the engine cannot derive
is not engine-proved (it may still be checked by independent,
hand-derived evidence in the consuming fixture).

## How it fits into Phys

```text
phys-lib/<framework>   physical meaning (assumptions, equations, theorems)
        ↓ references pure mathematics
phys-math              this repository: representation + bounded execution
        ↓ encodes through
phys-artifact          canonical bytes + identity
```

[PhysArtifact](https://github.com/PithomLabs/phys-artifact) supplies
identity and encoding; [PhysLib](https://github.com/PithomLabs/phys-lib)
adds framework semantics on top of these representations;
[PhysTrace](https://github.com/PithomLabs/phys-trace) coordinates
paper-derivation workflows. Related repositories:
[phys-artifact](https://github.com/PithomLabs/phys-artifact),
[phys-lib](https://github.com/PithomLabs/phys-lib).

## How do I start?

```bash
go build ./...
```

Read `doc.go`, then [`AGENTS.md`](./AGENTS.md) (operating notes and
the test-shard protocol), `ast.go`, `rules.go`, and `replay.go`.
Fast feedback loop (registry, purity, codec — seconds):

```bash
go test -count=1 -run 'TestRegistryCompleteness|TestNoPhysicalVocabulary|TestImportDiscipline' .
```

The full suite is sharded by cost; see `AGENTS.md` for the exact
shard commands and timeouts before running it.

## How does derivation formalization work?

A derivation is an ordered list of steps, each naming its producer,
rule, exact input references, conditions, parameters, and output
reference. Intermediate results are minted artifacts, so provenance
is a chain of references rather than prose. `Replay` re-executes
each step and fails closed on missing, wrong-family, or corrupted
evidence. Structural validation checks grammar without executing;
replay checks execution without judging physical meaning.

## Where are the rules for representing derivations?

In the normative
[Scientific Derivation Specification Guidelines](https://github.com/PithomLabs/phys-trace/blob/main/DERIVATION_SPEC_GUIDELINES.md),
which govern faithful, auditable scientific derivation
formalization. This README is orientation only and does not replace
them. A replayed derivation establishes that the recorded
computation reproduces from its premises — not that a theorem is
true, and not that any physical claim holds.

## License 

MIT

