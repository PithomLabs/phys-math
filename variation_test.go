package physmath

import (
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// varTensor mints a rank-2 tensor/1 over fresh Space-bearing indices.
func varTensor(t *testing.T, s Store, dim int, idx []artifact.ArtifactRef, comps []Expr) artifact.ArtifactRef {
	t.Helper()
	crefs := make([]artifact.ArtifactRef, len(comps))
	for i, e := range comps {
		crefs[i] = mustStoreExpr(t, s, e)
	}
	tn := Tensor{Rank: 2, Dimension: uint16(dim), IndexRefs: idx, ComponentRefs: crefs}
	if err := tn.Validate(); err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyTensor, tn.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func varSpace(t *testing.T, s Store) (artifact.ArtifactRef, artifact.ArtifactRef, artifact.ArtifactRef) {
	t.Helper()
	sp := spaceRef(t, s, "var_space")
	ia := idxRef(t, s, "i", VarianceCovariant, sp)
	ib := idxRef(t, s, "j", VarianceCovariant, sp)
	return sp, ia, ib
}

// reduceRecord runs ReduceVariation, stores the derivation, and checks the record.
func reduceRecord(t *testing.T, s Store, gamma, variation artifact.ArtifactRef, a Expr) (VariationReduction, artifact.ArtifactRef, Derivation) {
	t.Helper()
	res, d, err := ReduceVariation(s, gamma, variation, a)
	if err != nil {
		t.Fatal(err)
	}
	dref, err := s.Put(FamilyDerivation, mustEncodeDerivation(t, d))
	if err != nil {
		t.Fatal(err)
	}
	c := VariationReduction{AnsatzRef: gamma, VariationRef: variation, ReducedRef: res, DerivationRef: dref}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := CheckVariationReduction(s, c, a); err != nil {
		t.Fatalf("fresh reduction record rejected: %v", err)
	}
	return c, res, d
}

// TestReduceVariationPositive: γ = I₂, V = [[1,2],[3,4]], S = Σ 2·a·γ·V.
// Hand-derived: 2a·1·1 + 0 + 0 + 2a·1·4 → Add(Mul(2,a), Mul(2,a,4)).
func TestReduceVariationPositive(t *testing.T) {
	s := NewMemStore()
	_, ia, ib := varSpace(t, s)
	a := NewEphemeralSymbol("a")
	gamma := varTensor(t, s, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 0, 0, 1))
	vary := varTensor(t, s, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	res, d, err := ReduceVariation(s, gamma, vary, a)
	if err != nil {
		t.Fatalf("reduction failed: %v", err)
	}
	want, err := Canonicalize(Add(
		Mul(MustRational(2, 1), a),
		Mul(MustRational(2, 1), a, MustRational(4, 1)),
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t, s, res); !got.Equal(want) {
		t.Fatal("reduced integrand is not hand-derived 2a + 2a·4")
	}
	if len(d.Steps) != 2 || d.Steps[0].RuleID != RuleExprCanon || d.Steps[1].RuleID != RuleExprSimplify {
		t.Fatal("reduction derivation is not CANON+SIMPLIFY")
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("reduction derivation does not replay: %v", err)
	}
	res2, _, err := ReduceVariation(s, gamma, vary, a)
	if err != nil {
		t.Fatal(err)
	}
	if res2 != res {
		t.Fatal("reduction recomputation nondeterministic")
	}
	c, _, _ := reduceRecord(t, s, gamma, vary, a)
	raw := c.Encode()
	dec, err := DecodeVariationReduction(raw)
	if err != nil {
		t.Fatalf("reduction record does not decode: %v", err)
	}
	if err := CheckVariationReduction(s, dec, a); err != nil {
		t.Fatalf("decoded record rejected: %v", err)
	}
}

// TestReduceVariationDimOne: degenerate scalar-counterpart path.
func TestReduceVariationDimOne(t *testing.T) {
	s := NewMemStore()
	_, ia, ib := varSpace(t, s)
	a := NewEphemeralSymbol("a")
	v := NewEphemeralSymbol("v")
	gamma := varTensor(t, s, 1, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1))
	vary := varTensor(t, s, 1, []artifact.ArtifactRef{ia, ib}, []Expr{v})
	res, _, err := ReduceVariation(s, gamma, vary, a)
	if err != nil {
		t.Fatalf("dim-1 reduction failed: %v", err)
	}
	want, err := Canonicalize(Mul(MustRational(2, 1), a, v))
	if err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t, s, res); !got.Equal(want) {
		t.Fatal("dim-1 reduced integrand is not hand-derived 2·a·v")
	}
	reduceRecord(t, s, gamma, vary, a)
}

func TestReduceVariationNegatives(t *testing.T) {
	s := NewMemStore()
	_, ia, ib := varSpace(t, s)
	a := NewEphemeralSymbol("a")
	good := varTensor(t, s, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 0, 0, 1))
	goodV := varTensor(t, s, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	// Rank mismatch.
	r1 := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ia}, rationalMatrix(1, 2))
	if _, _, err := ReduceVariation(s, r1, goodV, a); err == nil {
		t.Fatal("rank-1 ansatz accepted")
	}
	// Dimension mismatch.
	d3 := varTensor(t, s, 3, []artifact.ArtifactRef{ia, ib},
		rationalMatrix(1, 0, 0, 0, 1, 0, 0, 0, 1))
	if _, _, err := ReduceVariation(s, d3, goodV, a); err == nil {
		t.Fatal("dimension mismatch accepted")
	}
	// Bound exceeded: shared zero components keep the mint cheap.
	zero := mustStoreExpr(t, s, MustRational(0, 1))
	zcrefs := make([]artifact.ArtifactRef, 65*65)
	for i := range zcrefs {
		zcrefs[i] = zero
	}
	big := Tensor{Rank: 2, Dimension: 65,
		IndexRefs: []artifact.ArtifactRef{ia, ib}, ComponentRefs: zcrefs}
	if err := big.Validate(); err != nil {
		t.Fatal(err)
	}
	bigRef, err := s.Put(FamilyTensor, big.Encode())
	if err != nil {
		t.Fatal(err)
	}
	zv := make([]artifact.ArtifactRef, 65*65)
	for i := range zv {
		zv[i] = zero
	}
	bigV := Tensor{Rank: 2, Dimension: 65,
		IndexRefs: []artifact.ArtifactRef{ia, ib}, ComponentRefs: zv}
	if err := bigV.Validate(); err != nil {
		t.Fatal(err)
	}
	bigVRef, err := s.Put(FamilyTensor, bigV.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if got, _, err := ReduceVariation(s, bigRef, bigVRef, a); err == nil || got != (artifact.ArtifactRef{}) {
		t.Fatal("dimension 65 accepted or returned a usable ref")
	}
	// Unresolvable / wrong-family operands.
	ghost := ComputeTestRef(t, "physmath", "tensor/1")
	if _, _, err := ReduceVariation(s, ghost, goodV, a); err == nil {
		t.Fatal("unresolvable ansatz accepted")
	}
	one := mustStoreExpr(t, s, MustRational(1, 1))
	if _, _, err := ReduceVariation(s, one, goodV, a); err == nil {
		t.Fatal("expression ansatz accepted")
	}
	// Non-expression component.
	badComps := append(append([]artifact.ArtifactRef{}, goodVComps(t, s)...), one)
	_ = badComps
	symIdx := tensorRef(t, s, 1, 1, []artifact.ArtifactRef{ia}, []Expr{NewEphemeralSymbol("q")})
	_ = symIdx
	// Record-level negatives.
	c, _, d := reduceRecord(t, s, good, goodV, a)
	_ = d
	sub := c
	sub.ReducedRef = one
	if err := CheckVariationReduction(s, sub, a); err == nil {
		t.Fatal("substituted reduced integrand accepted")
	}
	edited := varTensor(t, s, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 5))
	stale := c
	stale.VariationRef = edited
	if err := CheckVariationReduction(s, stale, a); err == nil {
		t.Fatal("stale variation evidence accepted")
	}
	ghostD := ComputeTestRef(t, "physmath", "derivation/1")
	missing := c
	missing.DerivationRef = ghostD
	if err := missing.Validate(); err != nil {
		t.Fatalf("grammatical ghost ref should pass Validate: %v", err)
	}
	if err := CheckVariationReduction(s, missing, a); err == nil {
		t.Fatal("unresolvable derivation accepted")
	}
	zero2 := c
	zero2.DerivationRef = artifact.ArtifactRef{}
	if err := zero2.Validate(); err == nil {
		t.Fatal("zero derivation ref accepted by Validate")
	}
	raw := c.Encode()
	if _, err := DecodeVariationReduction(raw[:len(raw)/2]); err == nil {
		t.Fatal("truncated record accepted")
	}
	if _, err := DecodeVariationReduction(append(append([]byte{}, raw...), 0xFF)); err == nil {
		t.Fatal("trailing bytes accepted")
	}
	// Dropped premise.
	d.PremiseRefs = nil
	rawD, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dropped, err := s.Put(FamilyDerivation, rawD)
	if err != nil {
		t.Fatal(err)
	}
	dc := c
	dc.DerivationRef = dropped
	if err := CheckVariationReduction(s, dc, a); err == nil {
		t.Fatal("dropped-premise derivation accepted")
	}
}

func goodVComps(t *testing.T, s Store) []artifact.ArtifactRef {
	t.Helper()
	out := make([]artifact.ArtifactRef, 4)
	for i, v := range []int64{1, 2, 3, 4} {
		out[i] = mustStoreExpr(t, s, MustRational(v, 1))
	}
	return out
}
