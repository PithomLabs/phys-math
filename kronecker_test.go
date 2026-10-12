package physmath

import (
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// selectorIndex mints a CONTRAVARIANT index/1 over the given space.
func selectorIndex(t *testing.T, s Store, label string, space artifact.ArtifactRef) artifact.ArtifactRef {
	t.Helper()
	payload, err := Index{Label: label, Variance: VarianceContravariant, Space: &space}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyIndex, payload)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestKroneckerSelectorValid(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "comoving_space")
	ix := selectorIndex(t, s, "mu", sp)
	sel, err := KroneckerSelector(s, 4, 0, ix)
	if err != nil {
		t.Fatalf("selector mint failed: %v", err)
	}
	if err := ValidateKroneckerSelector(s, sel, 0); err != nil {
		t.Fatalf("valid selector rejected: %v", err)
	}
	// Components are exactly (1,0,0,0) by canonical bytes.
	_, payload, err := s.Lookup(sel)
	if err != nil {
		t.Fatal(err)
	}
	tn, err := DecodeTensor(payload)
	if err != nil {
		t.Fatal(err)
	}
	if tn.Rank != 1 || tn.Dimension != 4 || len(tn.ComponentRefs) != 4 {
		t.Fatal("selector shape wrong")
	}
	one, _ := MustRational(1, 1).Encode()
	zero, _ := MustRational(0, 1).Encode()
	for i, r := range tn.ComponentRefs {
		_, cp, err := s.Lookup(r)
		if err != nil {
			t.Fatal(err)
		}
		want := zero
		if i == 0 {
			want = one
		}
		if string(cp) != string(want) {
			t.Fatalf("selector component %d wrong", i)
		}
	}
	// Position variants: the 1 follows the declared position.
	sel2, err := KroneckerSelector(s, 3, 2, ix)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateKroneckerSelector(s, sel2, 2); err != nil {
		t.Fatalf("position-2 selector rejected: %v", err)
	}
	if err := ValidateKroneckerSelector(s, sel2, 0); err == nil {
		t.Fatal("wrong position accepted")
	}
}

func TestKroneckerSelectorNegatives(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "neg_space")
	ix := selectorIndex(t, s, "mu", sp)
	// Dimension 0 and out-of-range position fail at construction.
	if _, err := KroneckerSelector(s, 0, 0, ix); err == nil {
		t.Fatal("dimension 0 accepted")
	}
	if _, err := KroneckerSelector(s, 4, 4, ix); err == nil {
		t.Fatal("position 4 of dim 4 accepted")
	}
	if _, err := KroneckerSelector(s, 65, 0, ix); err == nil {
		t.Fatal("dimension 65 accepted despite bound")
	}
	// Unresolvable index fails at construction.
	ghost := ComputeTestRef(t, "physmath", "index/1")
	if _, err := KroneckerSelector(s, 2, 0, ghost); err == nil {
		t.Fatal("unresolvable index accepted")
	}
	// Wrong-family index fails at construction.
	one := mustStoreExpr(t, s, MustRational(1, 1))
	if _, err := KroneckerSelector(s, 2, 0, one); err == nil {
		t.Fatal("expression index accepted")
	}
	// A rank-2 tensor is not a selector.
	rank2 := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ix, ix}, rationalMatrix(1, 0, 0, 1))
	if err := ValidateKroneckerSelector(s, rank2, 0); err == nil {
		t.Fatal("rank-2 tensor accepted as selector")
	}
	// Two ones is not the selector pattern.
	two := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(1, 1))
	if err := ValidateKroneckerSelector(s, two, 0); err == nil {
		t.Fatal("double-one pattern accepted")
	}
	// Non-rational component fails the pattern check.
	symT := tensorRef(t, s, 1, 1, []artifact.ArtifactRef{ix}, []Expr{NewEphemeralSymbol("z")})
	if err := ValidateKroneckerSelector(s, symT, 0); err == nil {
		t.Fatal("symbolic component accepted as selector")
	}
	// Unresolvable tensor reference fails.
	if err := ValidateKroneckerSelector(s, ghost, 0); err == nil {
		t.Fatal("unresolvable tensor accepted")
	}
	// Missing derivation-style evidence: zero reference fails lookup.
	if err := ValidateKroneckerSelector(s, artifact.ArtifactRef{}, 0); err == nil {
		t.Fatal("zero reference accepted")
	}
}

// TestKroneckerSelectorContraction wires the selector as the vector operand
// of a contraction: with T = diag-like rows, contracting slot 1 against
// e_0 selects the first column by hand-derived arithmetic.
func TestKroneckerSelectorContraction(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "select_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(5, 6, 7, 8))
	sel, err := KroneckerSelector(s, 2, 0, ix)
	if err != nil {
		t.Fatal(err)
	}
	result, d, err := EvaluateContraction(s, tref, sel, 1)
	if err != nil {
		t.Fatalf("selector contraction failed: %v", err)
	}
	// Hand-derived: w = [5*1+6*0, 7*1+8*0] = [5, 7].
	_, payload, err := s.Lookup(result)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(payload)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int64{5, 7} {
		_, got, err := s.Lookup(rt.ComponentRefs[i])
		if err != nil {
			t.Fatal(err)
		}
		w, err := MustRational(want, 1).Encode()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(w) {
			t.Fatalf("selector-selected component %d wrong", i)
		}
	}
	if len(d.Steps) != 4 {
		t.Fatalf("selector derivation has %d steps, want 4", len(d.Steps))
	}
	c, _, _ := evalRecord(t, s, tref, sel, 1)
	_ = c
	_ = result
}
