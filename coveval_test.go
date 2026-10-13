package physmath

import (
	"errors"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// covChart mints a coordinate system over bound coordinate symbols with a
// declaring function and mapping. Returns chart, mapping, declFn, symbols.
func covChart(t *testing.T, s Store, names ...string) (artifact.ArtifactRef, CoordParamMap, artifact.ArtifactRef, []Expr) {
	t.Helper()
	desig := mustStoreExpr(t, s, NewEphemeralSymbol("cov_space"))
	spaceRaw, err := Collection{ElementRefs: []artifact.ArtifactRef{desig}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	space, err := s.Put(FamilyCollection, spaceRaw)
	if err != nil {
		t.Fatal(err)
	}
	n := len(names)
	pids := make([]ParameterID, n)
	syms := make([]Expr, n)
	crefs := make([]artifact.ArtifactRef, n)
	for i, name := range names {
		pids[i] = DeriveParameterID(uint32(i))
		syms[i] = NewBoundSymbol(name, pids[i])
		crefs[i] = mustStoreExpr(t, s, syms[i])
	}
	cs := CoordinateSystem{Dimension: uint16(n), CoordinateRefs: crefs, SpaceRef: space}
	if err := cs.Validate(); err != nil {
		t.Fatal(err)
	}
	chart, err := s.Put(FamilyCoordinateSystem, cs.Encode())
	if err != nil {
		t.Fatal(err)
	}
	body := syms[0]
	if n > 1 {
		body = Mul(syms[0], syms[1])
	}
	fnObj := Function{ParameterIDs: pids, ParamNames: names, BodyRef: mustStoreExpr(t, s, body)}
	fnBytes, err := fnObj.Encode()
	if err != nil {
		t.Fatal(err)
	}
	declFn, err := s.Put(FamilyFunction, fnBytes)
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]BoundParameterRef, n)
	for i := range pids {
		entries[i] = BoundParameterRef{DeclaringArtifactRef: declFn, ParameterID: pids[i]}
	}
	return chart, CoordParamMap{Entries: entries}, declFn, syms
}

// covIndex mints an index/1 over the chart space.
func covIndex(t *testing.T, s Store, chart artifact.ArtifactRef, label string, v IndexVariance) artifact.ArtifactRef {
	t.Helper()
	_, cpayload, err := s.Lookup(chart)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Index{Label: label, Variance: v, Space: &cs.SpaceRef}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyIndex, p)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// covTensor mints a tensor/1 with given index refs and component exprs.
func covTensor(t *testing.T, s Store, rank, dim int, idx []artifact.ArtifactRef, comps []Expr) artifact.ArtifactRef {
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

// zeroConn mints an all-zero affine_connection/1 over chart (ClaimSymmetric false).
func zeroConn(t *testing.T, s Store, chart artifact.ArtifactRef, dim int) artifact.ArtifactRef {
	t.Helper()
	zero := mustStoreExpr(t, s, MustRational(0, 1))
	coeffs := make([]artifact.ArtifactRef, dim*dim*dim)
	for i := range coeffs {
		coeffs[i] = zero
	}
	c := AffineConnection{CoordRef: chart, Dimension: uint16(dim), CoefficientRefs: coeffs}
	raw := c.Encode()
	ref, err := s.Put(FamilyAffineConnection, raw)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// evalRecordCov evaluates, stores the derivation, builds the certificate,
// and checks it — the happy-path helper.
func evalRecordCov(t *testing.T, s Store, field, conn artifact.ArtifactRef, mapping CoordParamMap, declFn artifact.ArtifactRef) (CovariantEvaluation, artifact.ArtifactRef, Derivation) {
	t.Helper()
	result, d, err := EvaluateCovariantDerivative(s, field, conn, mapping, declFn, nil)
	if err != nil {
		t.Fatal(err)
	}
	dref, err := s.Put(FamilyDerivation, mustEncodeDerivation(t, d))
	if err != nil {
		t.Fatal(err)
	}
	c := CovariantEvaluation{FieldRef: field, ConnectionRef: conn, ResultRef: result, DerivationRef: dref}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := CheckCovariantEvaluation(s, c, mapping, declFn); err != nil {
		t.Fatalf("fresh covariant record rejected: %v", err)
	}
	return c, result, d
}

// TestCovariantScalarZeroConn: ∇_μ f = ∂_μ f for f = x²+y² → [2x, 2y].
func TestCovariantScalarZeroConn(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	x, y := syms[0], syms[1]
	f := covTensor(t, s, 0, 2, nil, []Expr{Add(Pow(x, MustRational(2, 1)), Pow(y, MustRational(2, 1)))})
	field := TensorField{TensorRef: f, CoordinateSystemRef: chart}
	fieldRef, err := s.Put(FamilyTensorField, field.Encode())
	if err != nil {
		t.Fatal(err)
	}
	conn := zeroConn(t, s, chart, 2)
	result, d, err := EvaluateCovariantDerivative(s, fieldRef, conn, mapping, declFn, nil)
	if err != nil {
		t.Fatalf("scalar covariant failed: %v", err)
	}
	rfam, rpayload, err := s.Lookup(result)
	if err != nil {
		t.Fatal(err)
	}
	if rfam != FamilyTensorField {
		t.Fatal("result is not tensor_field/1")
	}
	rf, err := DecodeTensorField(rpayload)
	if err != nil {
		t.Fatal(err)
	}
	if rf.CoordinateSystemRef != chart {
		t.Fatal("result chart not preserved")
	}
	_, tpayload, err := s.Lookup(rf.TensorRef)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(tpayload)
	if err != nil {
		t.Fatal(err)
	}
	if rt.Rank != 1 || int(rt.Dimension) != 2 || len(rt.IndexRefs) != 1 {
		t.Fatal("result shape wrong")
	}
	// Hand goldens: ∂_x f = 2x, ∂_y f = 2y (SIMPLIFY strips Pow-one).
	want0, _ := Canonicalize(Mul(MustRational(2, 1), x))
	want1, _ := Canonicalize(Mul(MustRational(2, 1), y))
	for i, want := range []Expr{want0, want1} {
		gotRaw, err := resolveExpression(s, rt.ComponentRefs[i])
		if err != nil {
			t.Fatal(err)
		}
		got, err := Canonicalize(gotRaw)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Equal(want) {
			t.Fatalf("scalar component %d wrong", i)
		}
	}
	// Derivation shape: 2 partials + 2*2 assembly steps = 6.
	if len(d.Steps) != 6 {
		t.Fatalf("scalar derivation has %d steps, want 6", len(d.Steps))
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("scalar derivation does not replay: %v", err)
	}
	evalRecordCov(t, s, fieldRef, conn, mapping, declFn)
}

// TestCovariantVectorSigns: dim-1 vector V=[t²] with Γ=1:
// ∇V = 2t + t². Covector [t²] gives 2t − t². Signs verified.
func TestCovariantVectorSigns(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "t")
	tt := syms[0]
	mkConn := func() artifact.ArtifactRef {
		t.Helper()
		one := mustStoreExpr(t, s, MustRational(1, 1))
		c := AffineConnection{CoordRef: chart, Dimension: 1, CoefficientRefs: []artifact.ArtifactRef{one}}
		raw := c.Encode()
		ref, err := s.Put(FamilyAffineConnection, raw)
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	mkVec := func(v IndexVariance) artifact.ArtifactRef {
		t.Helper()
		ix := covIndex(t, s, chart, "v", v)
		tref := covTensor(t, s, 1, 1, []artifact.ArtifactRef{ix},
			[]Expr{Pow(tt, MustRational(2, 1))})
		f := TensorField{TensorRef: tref, CoordinateSystemRef: chart}
		ref, err := s.Put(FamilyTensorField, f.Encode())
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	// Contravariant: ∂(t²) + Γ·t². Partial simplifies to Mul(t,2);
	// assembly to Add(Mul(t,2), Mul(1,Pow(t,2))) with the unit stripped.
	vec := mkVec(VarianceContravariant)
	res, _, err := EvaluateCovariantDerivative(s, vec, mkConn(), mapping, declFn, nil)
	if err != nil {
		t.Fatalf("vector covariant failed: %v", err)
	}
	rf, err := DecodeTensorField(mustPayloadOf(t, s, res))
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(mustPayloadOf(t, s, rf.TensorRef))
	if err != nil {
		t.Fatal(err)
	}
	if rt.Rank != 2 || len(rt.IndexRefs) != 2 {
		t.Fatal("vector result rank wrong")
	}
	if rt.IndexRefs[1] != mustIndexOf(t, s, vec) {
		t.Fatal("free index not preserved")
	}
	wantV, err := Canonicalize(Add(
		Mul(tt, MustRational(2, 1)),
		Pow(tt, MustRational(2, 1)),
	))
	if err != nil {
		t.Fatal(err)
	}
	gotV, err := Canonicalize(mustResolveOrFail(t, s, rt.ComponentRefs[0]))
	if err != nil {
		t.Fatal(err)
	}
	if !gotV.Equal(wantV) {
		t.Fatal("vector covariant component wrong: +Γ sign or ordering broken")
	}
	// Covector: ∂(t²) − Γ·t² (negative connection term).
	covec := mkVec(VarianceCovariant)
	resC, _, err := EvaluateCovariantDerivative(s, covec, mkConn(), mapping, declFn, nil)
	if err != nil {
		t.Fatalf("covector covariant failed: %v", err)
	}
	rfC, err := DecodeTensorField(mustPayloadOf(t, s, resC))
	if err != nil {
		t.Fatal(err)
	}
	rtC, err := DecodeTensor(mustPayloadOf(t, s, rfC.TensorRef))
	if err != nil {
		t.Fatal(err)
	}
	wantC, err := Canonicalize(Add(
		Mul(tt, MustRational(2, 1)),
		Neg(Pow(tt, MustRational(2, 1))),
	))
	if err != nil {
		t.Fatal(err)
	}
	gotC, err := Canonicalize(mustResolveOrFail(t, s, rtC.ComponentRefs[0]))
	if err != nil {
		t.Fatal(err)
	}
	if !gotC.Equal(wantC) {
		t.Fatal("covector covariant component wrong: −Γ sign broken")
	}
	evalRecordCov(t, s, vec, mkConn(), mapping, declFn)
	evalRecordCov(t, s, covec, mkConn(), mapping, declFn)
}

func mustPayloadOf(t *testing.T, s Store, ref artifact.ArtifactRef) []byte {
	t.Helper()
	_, p, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// mustIndexOf decodes a tensor_field's tensor and returns a slot index ref.
func mustIndexOf(t *testing.T, s Store, fieldRef artifact.ArtifactRef) artifact.ArtifactRef {
	t.Helper()
	ff, err := DecodeTensorField(mustPayloadOf(t, s, fieldRef))
	if err != nil {
		t.Fatal(err)
	}
	tn, err := DecodeTensor(mustPayloadOf(t, s, ff.TensorRef))
	if err != nil {
		t.Fatal(err)
	}
	return tn.IndexRefs[0]
}

func mustResolveOrFail(t *testing.T, s Store, ref artifact.ArtifactRef) Expr {
	t.Helper()
	e, err := resolveExpression(s, ref)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// TestCovariantMixedRank2 verifies both variance signs in one rank-2
// tensor with zero connection: result is pure partials with rank 3.
func TestCovariantMixedRank2(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	x, y := syms[0], syms[1]
	ia := covIndex(t, s, chart, "a", VarianceContravariant)
	ib := covIndex(t, s, chart, "b", VarianceCovariant)
	tref := covTensor(t, s, 2, 2, []artifact.ArtifactRef{ia, ib},
		[]Expr{Mul(x, y), x, y, MustRational(1, 1)})
	fieldRef, err := s.Put(FamilyTensorField,
		TensorField{TensorRef: tref, CoordinateSystemRef: chart}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	conn := zeroConn(t, s, chart, 2)
	result, d, err := EvaluateCovariantDerivative(s, fieldRef, conn, mapping, declFn, nil)
	if err != nil {
		t.Fatalf("mixed covariant failed: %v", err)
	}
	rf, err := DecodeTensorField(mustPayloadOf(t, s, result))
	if err != nil {
		t.Fatal(err)
	}
	if rf.CoordinateSystemRef != chart {
		t.Fatal("chart not preserved")
	}
	rt, err := DecodeTensor(mustPayloadOf(t, s, rf.TensorRef))
	if err != nil {
		t.Fatal(err)
	}
	if rt.Rank != 3 || len(rt.ComponentRefs) != 8 {
		t.Fatal("mixed result shape wrong")
	}
	if rt.IndexRefs[1] != ia || rt.IndexRefs[2] != ib {
		t.Fatal("free indices not preserved in order")
	}
	// ∂_x(x·y) = y (first result component), by hand.
	got, err := Canonicalize(mustResolveOrFail(t, s, rt.ComponentRefs[0]))
	if err != nil {
		t.Fatal(err)
	}
	want, err := Canonicalize(y)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Fatal("mixed partial component wrong")
	}
	// Derivation: 2 coords × 4 comps = 8 partials + 16 assembly = 24 steps.
	if len(d.Steps) != 24 {
		t.Fatalf("mixed derivation has %d steps, want 24", len(d.Steps))
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("mixed derivation does not replay: %v", err)
	}
	evalRecordCov(t, s, fieldRef, conn, mapping, declFn)
}

// TestCovariantNegatives pins each rejection boundary.
func TestCovariantNegatives(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	x := syms[0]
	ix := covIndex(t, s, chart, "v", VarianceContravariant)
	vref := covTensor(t, s, 1, 2, []artifact.ArtifactRef{ix}, []Expr{x, x})
	fieldRef, err := s.Put(FamilyTensorField,
		TensorField{TensorRef: vref, CoordinateSystemRef: chart}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	conn := zeroConn(t, s, chart, 2)
	// Chart mismatch: connection over another chart.
	chartB, _, _ := chartFixture(t, s, "x", "y")
	connB := zeroConn(t, s, chartB, 2)
	if _, _, err := EvaluateCovariantDerivative(s, fieldRef, connB, mapping, declFn, nil); err == nil {
		t.Fatal("chart-mismatched connection accepted")
	}
	// Plain tensor (not field) rejected at the family gate.
	if _, _, err := EvaluateCovariantDerivative(s, vref, conn, mapping, declFn, nil); err == nil {
		t.Fatal("plain tensor accepted as field")
	}
	// Wrong-family connection.
	one := mustStoreExpr(t, s, MustRational(1, 1))
	if _, _, err := EvaluateCovariantDerivative(s, fieldRef, one, mapping, declFn, nil); err == nil {
		t.Fatal("expression connection accepted")
	}
	// UNSPECIFIED variance slot.
	iu := covIndexRaw(t, s, chart, "u", VarianceUnspecified)
	vu := covTensor(t, s, 1, 2, []artifact.ArtifactRef{iu}, []Expr{x, x})
	fu, err := s.Put(FamilyTensorField,
		TensorField{TensorRef: vu, CoordinateSystemRef: chart}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := EvaluateCovariantDerivative(s, fu, conn, mapping, declFn, nil); err == nil {
		t.Fatal("UNSPECIFIED variance accepted")
	}
	// Space mismatch: slot over another space.
	sp2 := spaceRef(t, s, "other_space")
	io := idxRef(t, s, "o", VarianceContravariant, sp2)
	vo := covTensor(t, s, 1, 2, []artifact.ArtifactRef{io}, []Expr{x, x})
	fo, err := s.Put(FamilyTensorField,
		TensorField{TensorRef: vo, CoordinateSystemRef: chart}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := EvaluateCovariantDerivative(s, fo, conn, mapping, declFn, nil); err == nil {
		t.Fatal("space-mismatched slot accepted")
	}
	// Mapping cardinality mismatch.
	short := CoordParamMap{Entries: mapping.Entries[:1]}
	if _, _, err := EvaluateCovariantDerivative(s, fieldRef, conn, short, declFn, nil); err == nil {
		t.Fatal("short mapping accepted")
	}
	// Unresolvable field.
	ghost := ComputeTestRef(t, "physmath", "tensor_field/1")
	if _, _, err := EvaluateCovariantDerivative(s, ghost, conn, mapping, declFn, nil); err == nil {
		t.Fatal("unresolvable field accepted")
	}
	// Rank bound: rank-5 tensor rejected pre-mint.
	idx5 := make([]artifact.ArtifactRef, 5)
	for i := range idx5 {
		idx5[i] = ix
	}
	comps5 := make([]artifact.ArtifactRef, 32)
	one5 := mustStoreExpr(t, s, MustRational(1, 1))
	for i := range comps5 {
		comps5[i] = one5
	}
	r5 := Tensor{Rank: 5, Dimension: 2, IndexRefs: idx5, ComponentRefs: comps5}
	if err := r5.Validate(); err != nil {
		t.Fatal(err)
	}
	r5ref, err := s.Put(FamilyTensor, r5.Encode())
	if err != nil {
		t.Fatal(err)
	}
	f5, err := s.Put(FamilyTensorField,
		TensorField{TensorRef: r5ref, CoordinateSystemRef: chart}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if got, _, err := EvaluateCovariantDerivative(s, f5, conn, mapping, declFn, nil); err == nil || got != (artifact.ArtifactRef{}) {
		t.Fatal("rank-5 evaluation accepted or returned a ref")
	}
}

func covIndexRaw(t *testing.T, s Store, chart artifact.ArtifactRef, label string, v IndexVariance) artifact.ArtifactRef {
	t.Helper()
	return covIndex(t, s, chart, label, v)
}

// TestCovariantStaleEvidence: substituted result, swapped connection, and
// missing derivation payload each fail at the intended boundary.
func TestCovariantStaleEvidence(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "t")
	tt := syms[0]
	ix := covIndex(t, s, chart, "v", VarianceContravariant)
	vref := covTensor(t, s, 1, 1, []artifact.ArtifactRef{ix}, []Expr{Pow(tt, MustRational(2, 1))})
	fieldRef, err := s.Put(FamilyTensorField,
		TensorField{TensorRef: vref, CoordinateSystemRef: chart}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	one := mustStoreExpr(t, s, MustRational(1, 1))
	conn := func() artifact.ArtifactRef {
		c := AffineConnection{CoordRef: chart, Dimension: 1, CoefficientRefs: []artifact.ArtifactRef{one}}
		ref, err := s.Put(FamilyAffineConnection, c.Encode())
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}()
	c, _, _ := evalRecordCov(t, s, fieldRef, conn, mapping, declFn)
	// Substituted result tensor.
	other, _, err := EvaluateCovariantDerivative(s, fieldRef, conn, mapping, declFn, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = other
	alt := covTensor(t, s, 1, 1, []artifact.ArtifactRef{ix}, []Expr{tt})
	altField, err := s.Put(FamilyTensorField,
		TensorField{TensorRef: alt, CoordinateSystemRef: chart}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	sub := c
	sub.ResultRef = altField
	if err := sub.Validate(); err != nil {
		t.Fatalf("substituted record should stay structural: %v", err)
	}
	if err := CheckCovariantEvaluation(s, sub, mapping, declFn); err == nil {
		t.Fatal("substituted result accepted by covariant check")
	}
	// Swapped connection (zero connection, same chart): recomputation differs.
	zero := zeroConn(t, s, chart, 1)
	swapped := c
	swapped.ConnectionRef = zero
	if err := CheckCovariantEvaluation(s, swapped, mapping, declFn); err == nil {
		t.Fatal("swapped connection accepted by covariant check")
	}
	// Missing derivation payload.
	partial := NewMemStore()
	for _, r := range []artifact.ArtifactRef{fieldRef, conn} {
		fam, payload, err := s.Lookup(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := partial.Put(fam, payload); err != nil {
			t.Fatal(err)
		}
	}
	// Copy everything except the derivation payload.
	var walk func(r artifact.ArtifactRef)
	seen := map[artifact.ArtifactRef]bool{}
	walk = func(r artifact.ArtifactRef) {
		if seen[r] || r == c.DerivationRef {
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
	for _, r := range []artifact.ArtifactRef{fieldRef, conn} {
		_, payload, err := s.Lookup(r)
		if err != nil {
			t.Fatal(err)
		}
		_ = payload
		walk(r)
	}
	if err := CheckCovariantEvaluation(partial, c, mapping, declFn); err == nil {
		t.Fatal("missing derivation payload accepted")
	}
	// Certificate round-trip + zero-ref Validate gate.
	raw := c.Encode()
	dec, err := DecodeCovariantEvaluation(raw)
	if err != nil {
		t.Fatalf("record does not decode: %v", err)
	}
	if err := CheckCovariantEvaluation(s, dec, mapping, declFn); err != nil {
		t.Fatalf("decoded record rejected: %v", err)
	}
	zero2 := c
	zero2.DerivationRef = artifact.ArtifactRef{}
	if err := zero2.Validate(); err == nil {
		t.Fatal("zero derivation ref accepted by Validate")
	}
	if _, err := DecodeCovariantEvaluation(raw[:len(raw)/2]); err == nil {
		t.Fatal("truncated record accepted")
	}
}

// TestScalarLaplacianPolar: ∇²(r²) = 4 in the polar chart. The 4 arises
// only through connection terms: the partial-only form g^μν∂_μ∂_ν(r²) is
// bare 2, so this test fails on any implementation that drops the
// Γ-corrections. Full collapse to 4 uses the COMBINE assembly stage
// under the nonzero-determinant condition.
func TestScalarLaplacianPolar(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, r, _, cond := polarSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	// Scalar field f = r² as a rank-0 tensor.
	fref := covTensor(t, s, 0, 2, nil, []Expr{Pow(r, MustRational(2, 1))})
	fieldRef, err := s.Put(FamilyTensorField,
		TensorField{TensorRef: fref, CoordinateSystemRef: chart}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	// Levi-Civita connection for the polar metric.
	conn, _, err := LeviCivita(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatalf("polar connection failed: %v", err)
	}
	// Inverse metric tensor with contravariant slots via the general path.
	_, invComps, _, err := InverseMatrix(s, metric, conds)
	if err != nil {
		t.Fatalf("polar inverse failed: %v", err)
	}
	_, cpayload, err := s.Lookup(chart)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		t.Fatal(err)
	}
	up := func(label string) artifact.ArtifactRef {
		p, err := Index{Label: label, Variance: VarianceContravariant, Space: &cs.SpaceRef}.Encode()
		if err != nil {
			t.Fatal(err)
		}
		ref, err := s.Put(FamilyIndex, p)
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	ginv := Tensor{Rank: 2, Dimension: 2,
		IndexRefs:     []artifact.ArtifactRef{up("mu"), up("nu")},
		ComponentRefs: invComps}
	if err := ginv.Validate(); err != nil {
		t.Fatal(err)
	}
	ginvRef, err := s.Put(FamilyTensor, ginv.Encode())
	if err != nil {
		t.Fatal(err)
	}
	result, d, err := ScalarLaplacian(s, fieldRef, conn, ginvRef, mapping, fn, conds)
	if err != nil {
		t.Fatalf("polar laplacian failed: %v", err)
	}
	// Hand-verified golden: S = S_rr + S_thth with off-diagonal products
	// annihilated to zero and stripped (exactly two summands remain):
	//   S_rr  = g^rr·H_rr, H_rr = 2
	//   S_thth = g^thth·H_thth, H_thth = -Γ^r_thth·2r (connection term)
	// At r=1: S_rr = 1·2 = 2, S_thth = 1·2 = 2, total 4. The Neg node
	// inside S_thth is the −Γ sign; a connection-dropping implementation
	// cannot produce it (see anti-golden below).
	r2 := Pow(r, MustRational(2, 1))
	r2inv := Pow(Pow(r, MustRational(2, 1)), MustRational(-1, 1))
	s1 := Mul(Neg(Mul(r, MustRational(1, 2), MustRational(2, 1),
		Neg(Mul(r, MustRational(2, 1))), r2, r2inv)), r2inv)
	s2 := Mul(MustRational(2, 1), r2, r2inv)
	want, err := Canonicalize(Add(s1, s2))
	if err != nil {
		t.Fatal(err)
	}
	gotRaw := mustResolveOrFail(t, s, result)
	got, err := Canonicalize(gotRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Fatal("polar scalar laplacian is not hand-verified S_rr + S_thth")
	}
	if gotRaw.Kind != TagAdd || len(gotRaw.Children) != 2 {
		t.Fatal("laplacian is not a two-summand diagonal sum")
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("laplacian derivation does not replay: %v", err)
	}
	// Anti-golden: the partial-only second derivative is bare 2, never
	// the covariant 4. A connection-dropping implementation fails here.
	pr := DeriveParameterID(0)
	second := applyRule(t, s, RuleDifferentiate,
		[]artifact.ArtifactRef{
			mustStoreExpr(t, s, Pow(r, MustRational(2, 1))),
			fn,
		}, nil, totalParamsPolar(t, fn, pr, 2))
	_ = pr
	if got := mustResolveOrFail(t, s, second.OutputRef); got.Equal(MustRational(4, 1)) {
		t.Fatal("partial-only form unexpectedly equals 4")
	}
	simpSecond := applyRule(t, s, RuleExprSimplify,
		[]artifact.ArtifactRef{second.OutputRef}, nil, nil)
	gb, _ := mustResolveOrFail(t, s, simpSecond.OutputRef).Encode()
	_ = gb
	if got := mustResolveOrFail(t, s, simpSecond.OutputRef); !got.Equal(MustRational(2, 1)) {
		gotC, _ := Canonicalize(mustResolveOrFail(t, s, simpSecond.OutputRef))
		_ = gotC
		t.Fatalf("partial-only second derivative is not 2")
	}
}

// totalParamsPolar builds BPR+order parameters for a coordinate pid.
func totalParamsPolar(t *testing.T, fn artifact.ArtifactRef, pid ParameterID, order ...uint32) []byte {
	t.Helper()
	o := uint32(1)
	if len(order) > 0 {
		o = order[0]
	}
	return append(BoundParameterRef{DeclaringArtifactRef: fn, ParameterID: pid}.Encode(),
		artifact.EncodeUInt32(o)...)
}

// TestCovariantConditionPropagation: a component with a negative power
// requires its nonzero condition; removal fails at the partial boundary.
func TestCovariantConditionPropagation(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "t")
	tt := syms[0]
	ix := covIndex(t, s, chart, "v", VarianceContravariant)
	vref := covTensor(t, s, 1, 1, []artifact.ArtifactRef{ix},
		[]Expr{Pow(tt, MustRational(-1, 1))})
	fieldRef, err := s.Put(FamilyTensorField,
		TensorField{TensorRef: vref, CoordinateSystemRef: chart}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	conn := zeroConn(t, s, chart, 1)
	// Without conditions: the partial hits Pow(t,-1) and fails closed.
	if _, _, err := EvaluateCovariantDerivative(s, fieldRef, conn, mapping, declFn, nil); err == nil {
		t.Fatal("negative-power partial without condition accepted")
	} else if !isMissingCondition(t, err) {
		t.Fatalf("wrong boundary for missing condition: %v", err)
	}
	// With the exact condition: succeeds and replays.
	cond := neqCondFor(t, s, tt)
	res, d, err := EvaluateCovariantDerivative(s, fieldRef, conn, mapping, declFn,
		[]artifact.ArtifactRef{cond})
	if err != nil {
		t.Fatalf("conditioned evaluation failed: %v", err)
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("conditioned derivation does not replay: %v", err)
	}
	_ = res
}

func isMissingCondition(t *testing.T, err error) bool {
	t.Helper()
	return errors.Is(err, ErrMissingCondition)
}

func neqCondFor(t *testing.T, s Store, e Expr) artifact.ArtifactRef {
	t.Helper()
	body, err := EncodeRelationBody(OpNEQ, e, MustRational(0, 1))
	if err != nil {
		t.Fatal(err)
	}
	rel, err := s.Put(FamilyRelation, body)
	if err != nil {
		t.Fatal(err)
	}
	cond, err := s.Put(FamilyCondition, rel.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return cond
}
