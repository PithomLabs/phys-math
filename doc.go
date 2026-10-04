// Package physmath is the pure mathematical common denominator of the
// PhysMath/phys-* architecture (plan9.3 §§5, 11; spec §§2.1, 6-13, 17).
//
// It owns theory-neutral mathematics: expressions, functions, parameters,
// tensors, transformations, derivations, theorems, canonical identity,
// and the seven closed executable rules. It owns NO physical interpretation,
// theory names, assumptions, conventions, or trust semantics.
//
// Every exported API must be explainable without naming a physical theory
// (the mathematician test). See purity_test.go.
package physmath
