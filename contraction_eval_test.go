package physmath

import (
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// spaceRef mints a collection/1 space designator shared by paired indices.
func spaceRef(t *testing.T, s Store, name string) artifact.ArtifactRef {
	t.Helper()
	designator := mustStoreExpr(t, s, NewEphemeralSymbol(name))
	raw, err := Collection{ElementRefs: []artifact.ArtifactRef{designator}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyCollection, raw)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// idxRef mints an index/1 artifact with explicit space.
func idxRef(t *testing.T, s Store, label string, v IndexVariance, space artifact.ArtifactRef) artifact.ArtifactRef {
	t.Helper()
	payload, err := Index{Label: label, Variance: v, Space: &space}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyIndex, payload)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// tensorRef mints a tensor/1 artifact from component expressions in
// canonical row-major order.
func tensorRef(t *testing.T, s Store, rank int, dim int, idx []artifact.ArtifactRef, comps []Expr) artifact.ArtifactRef {
	t.Helper()
	crefs := make([]artifact.ArtifactRef, len(comps))
	for i, e := range comps {
		crefs[i] = mustStoreExpr(t, s, e)
	}
	tn := Tensor{Rank: uint16(rank), Dimension: uint16(dim), IndexRefs: idx, ComponentRefs: crefs}
	if err := tn.Validate(); err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyTensor, tn.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// evalRecord builds, stores, and checks a ContractionEvaluation for operands.
func evalRecord(t *testing.T, s Store, tensor, vector artifact.ArtifactRef, slot uint32) (ContractionEvaluation, artifact.ArtifactRef, Derivation) {
	t.Helper()
	result, d, err := EvaluateContraction(s, tensor, vector, slot)
	if err != nil {
		t.Fatal(err)
	}
	dref, err := s.Put(FamilyDerivation, mustEncodeDerivation(t, d))
	if err != nil {
		t.Fatal(err)
	}
	c := ContractionEvaluation{TensorRef: tensor, VectorRef: vector,
		ContractedTensorSlot: slot, ResultTensorRef: result, DerivationRef: dref}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := CheckContractionEvaluation(s, c); err != nil {
		t.Fatalf("fresh evaluation record rejected: %v", err)
	}
	return c, result, d
}

func mustEncodeDerivation(t *testing.T, d Derivation) []byte {
	t.Helper()
	raw, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// rationalMatrix builds d*d rational expressions row-major from ints.
func rationalMatrix(vals ...int64) []Expr {
	out := make([]Expr, len(vals))
	for i, v := range vals {
		out[i] = MustRational(v, 1)
	}
	return out
}

func TestEvaluateContractionRational(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "lab_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(5, 6))
	result, d, err := EvaluateContraction(s, tref, vref, 1)
	if err != nil {
		t.Fatalf("rational contraction failed: %v", err)
	}
	// Hand-derived golden: w = [1*5+2*6, 3*5+4*6] = [17, 39].
	want0, err := MustRational(17, 1).Encode()
	if err != nil {
		t.Fatal(err)
	}
	want1, err := MustRational(39, 1).Encode()
	if err != nil {
		t.Fatal(err)
	}
	rfam, rpayload, err := s.Lookup(result)
	if err != nil {
		t.Fatal(err)
	}
	if rfam != FamilyTensor {
		t.Fatalf("result family = %v, want tensor/1", rfam)
	}
	rt, err := DecodeTensor(rpayload)
	if err != nil {
		t.Fatal(err)
	}
	if rt.Rank != 1 || int(rt.Dimension) != 2 {
		t.Fatalf("result shape rank=%d dim=%d, want 1x2", rt.Rank, rt.Dimension)
	}
	if len(rt.IndexRefs) != 1 || rt.IndexRefs[0] != ia {
		t.Fatal("result must carry the free-slot index artifact")
	}
	got0fam, got0, err := s.Lookup(rt.ComponentRefs[0])
	if err != nil {
		t.Fatal(err)
	}
	_ = got0fam
	got1fam, got1, err := s.Lookup(rt.ComponentRefs[1])
	if err != nil {
		t.Fatal(err)
	}
	_ = got1fam
	if string(got0) != string(want0) || string(got1) != string(want1) {
		t.Fatal("result components are not the hand-derived [17, 39]")
	}
	// Exactly 4 steps with exact step-to-component correspondence.
	if len(d.Steps) != 4 {
		t.Fatalf("derivation has %d steps, want 4", len(d.Steps))
	}
	for i := 0; i < 2; i++ {
		s0, s1 := d.Steps[2*i], d.Steps[2*i+1]
		if s0.RuleID != RuleExprCanon || s1.RuleID != RuleExprSimplify {
			t.Fatalf("component %d steps are not CANON+SIMPLIFY", i)
		}
		if s1.OutputRef != rt.ComponentRefs[i] {
			t.Fatalf("step output for component %d does not match tensor component", i)
		}
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("derivation does not replay: %v", err)
	}
	// Determinism: recomputation mints identical references.
	result2, _, err := EvaluateContraction(s, tref, vref, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result2 != result {
		t.Fatal("contraction recomputation nondeterministic")
	}
	// Certificate round-trip + provenance check.
	c, _, _ := evalRecord(t, s, tref, vref, 1)
	raw := c.Encode()
	dec, err := DecodeContractionEvaluation(raw)
	if err != nil {
		t.Fatalf("evaluation record does not decode: %v", err)
	}
	if err := CheckContractionEvaluation(s, dec); err != nil {
		t.Fatalf("decoded record rejected: %v", err)
	}
}

func TestEvaluateContractionSlotZero(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "space0")
	ic := idxRef(t, s, "c", VarianceCovariant, sp)
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ix := idxRef(t, s, "c", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ic, ia}, rationalMatrix(1, 2, 3, 4))
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(5, 6))
	result, _, err := EvaluateContraction(s, tref, vref, 0)
	if err != nil {
		t.Fatalf("slot-0 contraction failed: %v", err)
	}
	// Hand-derived: w_i = T[0,i]*5 + T[1,i]*6 = [1*5+3*6, 2*5+4*6] = [23, 34].
	_, rpayload, err := s.Lookup(result)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(rpayload)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.IndexRefs) != 1 || rt.IndexRefs[0] != ia {
		t.Fatal("slot-0 result must carry the free slot-1 index")
	}
	for i, want := range []int64{23, 34} {
		_, got, err := s.Lookup(rt.ComponentRefs[i])
		if err != nil {
			t.Fatal(err)
		}
		w, err := MustRational(want, 1).Encode()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(w) {
			t.Fatalf("component %d is not hand-derived %d", i, want)
		}
	}
	if _, _, _ = evalRecord(t, s, tref, vref, 0); true {
	}
}

func TestEvaluateContractionSigned(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "lorentz_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(-1, 0, 0, 1))
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(1, 1))
	result, _, err := EvaluateContraction(s, tref, vref, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, rpayload, err := s.Lookup(result)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(rpayload)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int64{-1, 1} {
		_, got, err := s.Lookup(rt.ComponentRefs[i])
		if err != nil {
			t.Fatal(err)
		}
		w, err := MustRational(want, 1).Encode()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(w) {
			t.Fatalf("signed component %d mismatch", i)
		}
	}
	evalRecord(t, s, tref, vref, 1)
}

func TestEvaluateContractionSymbolic(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "sym_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	xbar := NewEphemeralSymbol("xbar")
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib},
		[]Expr{xbar, MustRational(1, 1), MustRational(0, 1), MustRational(1, 1)})
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix},
		[]Expr{MustRational(1, 1), MustRational(1, 1)})
	result, d, err := EvaluateContraction(s, tref, vref, 1)
	if err != nil {
		t.Fatalf("symbolic contraction failed: %v", err)
	}
	// Shape only: Add/Mul canonical form mentioning xbar, no numeric claim.
	_, rpayload, err := s.Lookup(result)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(rpayload)
	if err != nil {
		t.Fatal(err)
	}
	_, c0, err := s.Lookup(rt.ComponentRefs[0])
	if err != nil {
		t.Fatal(err)
	}
	e, err := DecodeExpr(artifact.NewDecoder(c0))
	if err != nil {
		t.Fatal(err)
	}
	if e.Kind != TagAdd && e.Kind != TagMul && e.Kind != TagSymbol {
		t.Fatalf("unexpected symbolic component shape: %v", e.Kind)
	}
	if len(d.Steps) != 4 {
		t.Fatalf("symbolic derivation has %d steps, want 4", len(d.Steps))
	}
	evalRecord(t, s, tref, vref, 1)
}

func TestEvaluateContractionDimOne(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "one_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 1, []artifact.ArtifactRef{ia, ib}, rationalMatrix(2))
	vref := tensorRef(t, s, 1, 1, []artifact.ArtifactRef{ix}, rationalMatrix(3))
	result, d, err := EvaluateContraction(s, tref, vref, 1)
	if err != nil {
		t.Fatalf("dim-1 contraction failed: %v", err)
	}
	_, rpayload, err := s.Lookup(result)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(rpayload)
	if err != nil {
		t.Fatal(err)
	}
	_, got, err := s.Lookup(rt.ComponentRefs[0])
	if err != nil {
		t.Fatal(err)
	}
	w, err := MustRational(6, 1).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(w) {
		t.Fatal("dim-1 result is not hand-derived 6")
	}
	if len(d.Steps) != 2 {
		t.Fatalf("dim-1 derivation has %d steps, want 2", len(d.Steps))
	}
	evalRecord(t, s, tref, vref, 1)
}

func TestEvaluateContractionBoundEdge(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "bound_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	zero := MustRational(0, 1)
	tcomps := make([]Expr, 64*64)
	for i := range tcomps {
		tcomps[i] = zero
	}
	vcomps := make([]Expr, 64)
	for i := range vcomps {
		vcomps[i] = zero
	}
	tref := tensorRef(t, s, 2, 64, []artifact.ArtifactRef{ia, ib}, tcomps)
	vref := tensorRef(t, s, 1, 64, []artifact.ArtifactRef{ix}, vcomps)
	result, d, err := EvaluateContraction(s, tref, vref, 1)
	if err != nil {
		t.Fatalf("d=64 zero contraction failed: %v", err)
	}
	// SET-dedup: all premises and results identical → single entries.
	if len(d.PremiseRefs) != 1 || len(d.ResultRefs) != 1 {
		t.Fatalf("dedup failed: %d premises, %d results; want 1 and 1",
			len(d.PremiseRefs), len(d.ResultRefs))
	}
	if len(d.Steps) != 128 {
		t.Fatalf("d=64 derivation has %d steps, want 128", len(d.Steps))
	}
	c, _, _ := evalRecord(t, s, tref, vref, 1)
	_ = c
	_ = result
}

func TestEvaluateContractionBoundExceeded(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "big_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	zero := MustRational(0, 1)
	tcomps := make([]Expr, 65*65)
	for i := range tcomps {
		tcomps[i] = zero
	}
	vcomps := make([]Expr, 65)
	for i := range vcomps {
		vcomps[i] = zero
	}
	tref := tensorRef(t, s, 2, 65, []artifact.ArtifactRef{ia, ib}, tcomps)
	vref := tensorRef(t, s, 1, 65, []artifact.ArtifactRef{ix}, vcomps)
	got, _, err := EvaluateContraction(s, tref, vref, 1)
	if err == nil {
		t.Fatal("d=65 contraction accepted despite bound")
	}
	if got != (artifact.ArtifactRef{}) {
		t.Fatal("bound failure returned a usable result reference")
	}
}

func TestEvaluateContractionNegatives(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "neg_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	good := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	goodV := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(5, 6))

	// Wrong tensor rank.
	r3 := tensorRef(t, s, 3, 2, []artifact.ArtifactRef{ia, ib, ia},
		rationalMatrix(1, 2, 3, 4, 5, 6, 7, 8))
	if _, _, err := EvaluateContraction(s, r3, goodV, 1); err == nil {
		t.Fatal("rank-3 tensor accepted")
	}
	// Wrong vector rank.
	rv := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	if _, _, err := EvaluateContraction(s, good, rv, 1); err == nil {
		t.Fatal("rank-2 vector accepted")
	}
	// Slot out of range (structural Validate also rejects).
	c := ContractionEvaluation{TensorRef: good, VectorRef: goodV, ContractedTensorSlot: 2,
		ResultTensorRef: good, DerivationRef: good}
	if err := c.Validate(); err == nil {
		t.Fatal("slot 2 accepted by Validate")
	}
	if _, _, err := EvaluateContraction(s, good, goodV, 2); err == nil {
		t.Fatal("slot 2 accepted by evaluation")
	}
	// UNSPECIFIED variance.
	iu := idxRef(t, s, "b", VarianceUnspecified, sp)
	tu := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, iu}, rationalMatrix(1, 2, 3, 4))
	if _, _, err := EvaluateContraction(s, tu, goodV, 1); err == nil {
		t.Fatal("UNSPECIFIED tensor variance accepted")
	}
	// Equal variance.
	ibCo := idxRef(t, s, "b", VarianceCovariant, sp)
	tco := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ibCo}, rationalMatrix(1, 2, 3, 4))
	vcCo := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ibCo}, rationalMatrix(5, 6))
	if _, _, err := EvaluateContraction(s, tco, vcCo, 1); err == nil {
		t.Fatal("equal variance accepted")
	}
	// Label mismatch.
	ic := idxRef(t, s, "c", VarianceContravariant, sp)
	vc := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ic}, rationalMatrix(5, 6))
	if _, _, err := EvaluateContraction(s, good, vc, 1); err == nil {
		t.Fatal("label mismatch accepted")
	}
	// Missing space (nil on vector index).
	ixNilPayload, err := Index{Label: "b", Variance: VarianceContravariant}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ixNil, err := s.Put(FamilyIndex, ixNilPayload)
	if err != nil {
		t.Fatal(err)
	}
	vNil := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ixNil}, rationalMatrix(5, 6))
	if _, _, err := EvaluateContraction(s, good, vNil, 1); err == nil {
		t.Fatal("missing vector space accepted")
	}
	// Dimension mismatch.
	ix3 := idxRef(t, s, "b", VarianceContravariant, sp)
	ia3 := idxRef(t, s, "a", VarianceCovariant, sp)
	ib3 := idxRef(t, s, "b", VarianceCovariant, sp)
	v3 := tensorRef(t, s, 1, 3, []artifact.ArtifactRef{ix3}, rationalMatrix(1, 2, 3))
	_ = ia3
	_ = ib3
	if _, _, err := EvaluateContraction(s, good, v3, 1); err == nil {
		t.Fatal("dimension mismatch accepted")
	}
	// Unresolvable operand.
	ghost := ComputeTestRef(t, "physmath", "tensor/1")
	if _, _, err := EvaluateContraction(s, ghost, goodV, 1); err == nil {
		t.Fatal("unresolvable tensor accepted")
	}
	// Wrong-family operand (expression as tensor).
	one := mustStoreExpr(t, s, MustRational(1, 1))
	if _, _, err := EvaluateContraction(s, one, goodV, 1); err == nil {
		t.Fatal("expression-family tensor accepted")
	}
	// Metric-supplied invocation: symmetric_bilinear/1 is not tensor/1.
	mcomps := []artifact.ArtifactRef{one, one, one, one}
	m := SymmetricBilinear{Dimension: 2, ComponentRefs: mcomps}
	mref, err := s.Put(FamilySymmetricBilinear, m.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := EvaluateContraction(s, mref, goodV, 1); err == nil {
		t.Fatal("metric artifact accepted as contraction tensor")
	}
}

func TestEvaluateContractionSpaceMismatch(t *testing.T) {
	s := NewMemStore()
	spA := spaceRef(t, s, "space_A")
	spB := spaceRef(t, s, "space_B")
	ia := idxRef(t, s, "a", VarianceCovariant, spA)
	ib := idxRef(t, s, "b", VarianceCovariant, spA)
	ix := idxRef(t, s, "b", VarianceContravariant, spB)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(5, 6))
	// Dimensions, labels, and variance all agree; spaces differ.
	if _, _, err := EvaluateContraction(s, tref, vref, 1); err == nil {
		t.Fatal("mismatched index spaces accepted")
	}
	// Space pointing at the wrong family.
	badSpace := mustStoreExpr(t, s, MustRational(9, 1))
	ixBadPayload, err := Index{Label: "b", Variance: VarianceContravariant, Space: &badSpace}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ixBad, err := s.Put(FamilyIndex, ixBadPayload)
	if err != nil {
		t.Fatal(err)
	}
	vBad := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ixBad}, rationalMatrix(5, 6))
	if _, _, err := EvaluateContraction(s, tref, vBad, 1); err == nil {
		t.Fatal("wrong-family space reference accepted")
	}
}

func TestContractionEvaluationSubstituted(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "sub_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(5, 6))
	c, _, _ := evalRecord(t, s, tref, vref, 1)
	// Substitute an honestly computed but unrelated result tensor.
	other := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(9, 9, 9, 9))
	otherV := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(1, 1))
	otherRes, _, err := EvaluateContraction(s, other, otherV, 1)
	if err != nil {
		t.Fatal(err)
	}
	sub := c
	sub.ResultTensorRef = otherRes
	if err := sub.Validate(); err != nil {
		t.Fatalf("substituted record should stay structurally valid: %v", err)
	}
	if err := CheckContractionEvaluation(s, sub); err == nil {
		t.Fatal("substituted result tensor accepted by evaluation check")
	}
	// Negated result (j claimed as w): membership failure, w-only boundary.
	_, rpayload, err := s.Lookup(c.ResultTensorRef)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(rpayload)
	if err != nil {
		t.Fatal(err)
	}
	negRefs := make([]artifact.ArtifactRef, len(rt.ComponentRefs))
	for i, r := range rt.ComponentRefs {
		_, payload, err := s.Lookup(r)
		if err != nil {
			t.Fatal(err)
		}
		e, err := DecodeExpr(artifact.NewDecoder(payload))
		if err != nil {
			t.Fatal(err)
		}
		negRefs[i] = mustStoreExpr(t, s, Neg(e))
	}
	negT := Tensor{Rank: 1, Dimension: rt.Dimension, IndexRefs: rt.IndexRefs, ComponentRefs: negRefs}
	if err := negT.Validate(); err != nil {
		t.Fatal(err)
	}
	negRef, err := s.Put(FamilyTensor, negT.Encode())
	if err != nil {
		t.Fatal(err)
	}
	negClaim := c
	negClaim.ResultTensorRef = negRef
	if err := CheckContractionEvaluation(s, negClaim); err == nil {
		t.Fatal("negated result accepted as evaluation output")
	}
}

func TestContractionEvaluationStaleOperands(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "stale_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	t1 := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	v := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(5, 6))
	c, _, _ := evalRecord(t, s, t1, v, 1)
	// Edited operand: same labels/space/dim, different diagonal content.
	t2 := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 5))
	stale := c
	stale.TensorRef = t2
	if err := CheckContractionEvaluation(s, stale); err == nil {
		t.Fatal("stale-operand evidence accepted by evaluation check")
	}
}

func TestContractionEvaluationSwappedCorrespondence(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "swap_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(5, 6))
	_, result, d := func() (ContractionEvaluation, artifact.ArtifactRef, Derivation) {
		res, dd, err := EvaluateContraction(s, tref, vref, 1)
		if err != nil {
			t.Fatal(err)
		}
		return ContractionEvaluation{}, res, dd
	}()
	_ = result
	// Swap the per-component step pairs (keeping ordinals contiguous): the
	// derivation still replays, but component correspondence is broken.
	d.Steps[0], d.Steps[1], d.Steps[2], d.Steps[3] = d.Steps[2], d.Steps[3], d.Steps[0], d.Steps[1]
	for i := range d.Steps {
		d.Steps[i].Ordinal = uint32(i)
	}
	raw, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dref, err := s.Put(FamilyDerivation, raw)
	if err != nil {
		t.Fatal(err)
	}
	c := ContractionEvaluation{TensorRef: tref, VectorRef: vref, ContractedTensorSlot: 1,
		ResultTensorRef: result, DerivationRef: dref}
	if err := CheckContractionEvaluation(s, c); err == nil {
		t.Fatal("swapped step correspondence accepted by evaluation check")
	}
}

func TestContractionEvaluationCorrupted(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "corr_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(2, 0, 0, 3))
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(1, 1))
	// det-style choice: diagonals {2,3} avoid payload collisions below.
	c, result, d := evalRecord(t, s, tref, vref, 1)
	_ = result
	// Partial store: copy everything except one tensor component payload.
	partial := NewMemStore()
	_, tpayload, err := s.Lookup(tref)
	if err != nil {
		t.Fatal(err)
	}
	tt, err := DecodeTensor(tpayload)
	if err != nil {
		t.Fatal(err)
	}
	skip := tt.ComponentRefs[0]
	for _, r := range tt.ComponentRefs[1:] {
		fam, payload, err := s.Lookup(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := partial.Put(fam, payload); err != nil {
			t.Fatal(err)
		}
	}
	// Copy remaining reachable payloads except the skipped component.
	var walk func(r artifact.ArtifactRef)
	seen := map[artifact.ArtifactRef]bool{}
	walk = func(r artifact.ArtifactRef) {
		if seen[r] || r == skip {
			return
		}
		seen[r] = true
		fam, payload, err := s.Lookup(r)
		if err != nil {
			return
		}
		if _, err := partial.Put(fam, payload); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range append(append([]artifact.ArtifactRef{tref, vref, c.DerivationRef, c.ResultTensorRef},
		d.PremiseRefs...), d.ResultRefs...) {
		walk(r)
	}
	for _, st := range d.Steps {
		for _, r := range append(append([]artifact.ArtifactRef{}, st.InputRefs...), st.OutputRef) {
			walk(r)
		}
	}
	// Index and space refs needed for operand resolution.
	for _, r := range append(append([]artifact.ArtifactRef{}, tt.IndexRefs...), sp) {
		walk(r)
	}
	_, vpayload, err := s.Lookup(vref)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := DecodeTensor(vpayload)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range append(append([]artifact.ArtifactRef{}, vt.IndexRefs...), vt.ComponentRefs...) {
		walk(r)
	}
	if _, err := partial.Put(FamilyTensor, tpayload); err != nil {
		t.Fatal(err)
	}
	if _, err := partial.Put(FamilyTensor, vpayload); err != nil {
		t.Fatal(err)
	}
	if err := CheckContractionEvaluation(partial, c); err == nil {
		t.Fatal("corrupted operand dependency accepted by evaluation check")
	}
}

func TestContractionEvaluationDroppedPremise(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "drop_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(5, 6))
	_, result, d := func() (ContractionEvaluation, artifact.ArtifactRef, Derivation) {
		res, dd, err := EvaluateContraction(s, tref, vref, 1)
		if err != nil {
			t.Fatal(err)
		}
		return ContractionEvaluation{}, res, dd
	}()
	_ = result
	d.PremiseRefs = nil
	raw, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dref, err := s.Put(FamilyDerivation, raw)
	if err != nil {
		t.Fatal(err)
	}
	c := ContractionEvaluation{TensorRef: tref, VectorRef: vref, ContractedTensorSlot: 1,
		ResultTensorRef: result, DerivationRef: dref}
	if err := CheckContractionEvaluation(s, c); err == nil {
		t.Fatal("derivation with dropped premises accepted by evaluation check")
	}
}

func TestContractionEvaluationUnresolvableEvidence(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "ghost_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(5, 6))
	c, _, _ := evalRecord(t, s, tref, vref, 1)
	// Grammatically valid but absent derivation evidence.
	ghost := ComputeTestRef(t, "physmath", "derivation/1")
	missing := c
	missing.DerivationRef = ghost
	if err := missing.Validate(); err != nil {
		t.Fatalf("unresolvable-but-grammatical ref should pass Validate: %v", err)
	}
	if err := CheckContractionEvaluation(s, missing); err == nil {
		t.Fatal("unresolvable derivation evidence accepted by evaluation check")
	}
	// Wrong-family derivation evidence (expression instead of derivation).
	one := mustStoreExpr(t, s, MustRational(1, 1))
	wrongFam := c
	wrongFam.DerivationRef = one
	if err := CheckContractionEvaluation(s, wrongFam); err == nil {
		t.Fatal("wrong-family derivation evidence accepted by evaluation check")
	}
	// Zero derivation reference fails structural presence.
	zero := c
	zero.DerivationRef = artifact.ArtifactRef{}
	if err := zero.Validate(); err == nil {
		t.Fatal("missing derivation reference accepted by Validate")
	}
}

func TestDecodeTensorMalformed(t *testing.T) {
	s := NewMemStore()
	sp := spaceRef(t, s, "dec_space")
	ia := idxRef(t, s, "a", VarianceCovariant, sp)
	ib := idxRef(t, s, "b", VarianceCovariant, sp)
	ix := idxRef(t, s, "b", VarianceContravariant, sp)
	tref := tensorRef(t, s, 2, 2, []artifact.ArtifactRef{ia, ib}, rationalMatrix(1, 2, 3, 4))
	vref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(5, 6))
	c, _, _ := evalRecord(t, s, tref, vref, 1)
	raw := c.Encode()
	if _, err := DecodeContractionEvaluation(raw[:len(raw)/2]); err == nil {
		t.Fatal("truncated evaluation record accepted")
	}
	if _, err := DecodeContractionEvaluation(append(append([]byte{}, raw...), 0xFF)); err == nil {
		t.Fatal("trailing bytes accepted in evaluation record")
	}
	badSlot := c
	badSlot.ContractedTensorSlot = 7
	if err := badSlot.Validate(); err == nil {
		t.Fatal("out-of-range slot accepted by Validate")
	}
	// Malformed tensor bytes: truncated payload.
	_, tpayload, err := s.Lookup(tref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeTensor(tpayload[:len(tpayload)/2]); err == nil {
		t.Fatal("truncated tensor accepted")
	}
	if _, err := DecodeTensor(append(append([]byte{}, tpayload...), 0x00)); err == nil {
		t.Fatal("trailing bytes accepted in tensor")
	}
}
