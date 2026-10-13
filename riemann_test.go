package physmath

import (
	"errors"
	"strings"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// curvedSetup builds the 2D polynomial curved chart (x,y) with metric
// diag(1,(1+x^2)^2), mapping, declaring function, and determinant condition.
func curvedSetup(t *testing.T, s Store) (chart, metric, fn artifact.ArtifactRef, mapping CoordParamMap, x, y Expr, cond artifact.ArtifactRef) {
	t.Helper()
	px := DeriveParameterID(0)
	py := DeriveParameterID(1)
	x = NewBoundSymbol("x", px)
	y = NewBoundSymbol("y", py)
	desig := mustStoreExpr(t, s, NewEphemeralSymbol("curved_space"))
	spaceRaw, err := Collection{ElementRefs: []artifact.ArtifactRef{desig}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	space, err := s.Put(FamilyCollection, spaceRaw)
	if err != nil {
		t.Fatal(err)
	}
	cx := mustStoreExpr(t, s, x)
	cy := mustStoreExpr(t, s, y)
	cs := CoordinateSystem{Dimension: 2, CoordinateRefs: []artifact.ArtifactRef{cx, cy}, SpaceRef: space}
	if err := cs.Validate(); err != nil {
		t.Fatal(err)
	}
	chart, err = s.Put(FamilyCoordinateSystem, cs.Encode())
	if err != nil {
		t.Fatal(err)
	}
	one := mustStoreExpr(t, s, MustRational(1, 1))
	zero := mustStoreExpr(t, s, MustRational(0, 1))
	f := Add(MustRational(1, 1), Pow(x, MustRational(2, 1)))
	gyy := mustStoreExpr(t, s, Pow(f, MustRational(2, 1)))
	m := SymmetricBilinear{Dimension: 2, ComponentRefs: []artifact.ArtifactRef{one, zero, zero, gyy}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	metric, err = s.Put(FamilySymmetricBilinear, m.Encode())
	if err != nil {
		t.Fatal(err)
	}
	fnObj := Function{ParameterIDs: []ParameterID{px, py},
		ParamNames: []string{"x", "y"},
		BodyRef:    mustStoreExpr(t, s, x)}
	fnBytes, err := fnObj.Encode()
	if err != nil {
		t.Fatal(err)
	}
	fn, err = s.Put(FamilyFunction, fnBytes)
	if err != nil {
		t.Fatal(err)
	}
	mapping = CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: fn, ParameterID: px},
		{DeclaringArtifactRef: fn, ParameterID: py},
	}}
	detRef, _, err := GeneralDeterminant(s, metric)
	if err != nil {
		t.Fatal(err)
	}
	cond = detCond(t, s, detRef)
	return chart, metric, fn, mapping, x, y, cond
}

// oneDimSetup builds a 1D chart (x) with metric [(1+x^2)].
func oneDimSetup(t *testing.T, s Store) (chart, metric, fn artifact.ArtifactRef, mapping CoordParamMap, x Expr, cond artifact.ArtifactRef) {
	t.Helper()
	px := DeriveParameterID(0)
	x = NewBoundSymbol("x", px)
	desig := mustStoreExpr(t, s, NewEphemeralSymbol("oned_space"))
	spaceRaw, err := Collection{ElementRefs: []artifact.ArtifactRef{desig}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	space, err := s.Put(FamilyCollection, spaceRaw)
	if err != nil {
		t.Fatal(err)
	}
	cx := mustStoreExpr(t, s, x)
	cs := CoordinateSystem{Dimension: 1, CoordinateRefs: []artifact.ArtifactRef{cx}, SpaceRef: space}
	if err := cs.Validate(); err != nil {
		t.Fatal(err)
	}
	chart, err = s.Put(FamilyCoordinateSystem, cs.Encode())
	if err != nil {
		t.Fatal(err)
	}
	gxx := mustStoreExpr(t, s, Add(MustRational(1, 1), Pow(x, MustRational(2, 1))))
	m := SymmetricBilinear{Dimension: 1, ComponentRefs: []artifact.ArtifactRef{gxx}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	metric, err = s.Put(FamilySymmetricBilinear, m.Encode())
	if err != nil {
		t.Fatal(err)
	}
	fnObj := Function{ParameterIDs: []ParameterID{px},
		ParamNames: []string{"x"},
		BodyRef:    mustStoreExpr(t, s, x)}
	fnBytes, err := fnObj.Encode()
	if err != nil {
		t.Fatal(err)
	}
	fn, err = s.Put(FamilyFunction, fnBytes)
	if err != nil {
		t.Fatal(err)
	}
	mapping = CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: fn, ParameterID: px},
	}}
	detRef, _, err := GeneralDeterminant(s, metric)
	if err != nil {
		t.Fatal(err)
	}
	cond = detCond(t, s, detRef)
	return chart, metric, fn, mapping, x, cond
}

// mustConnRef derives the Levi-Civita connection for coefficient inspection.
func mustConnRef(t *testing.T, s Store, metric, chart artifact.ArtifactRef, mapping CoordParamMap, fn, cond artifact.ArtifactRef) artifact.ArtifactRef {
	t.Helper()
	conn, _, err := LeviCivita(s, metric, chart, mapping, fn, []artifact.ArtifactRef{cond})
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

// riemannComps resolves the ordered result components of an evaluation.
func riemannComps(t *testing.T, s Store, resultField artifact.ArtifactRef) []Expr {
	t.Helper()
	_, fpayload, err := s.Lookup(resultField)
	if err != nil {
		t.Fatal(err)
	}
	rf, err := DecodeTensorField(fpayload)
	if err != nil {
		t.Fatal(err)
	}
	_, tpayload, err := s.Lookup(rf.TensorRef)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(tpayload)
	if err != nil {
		t.Fatal(err)
	}
	comps := make([]Expr, len(rt.ComponentRefs))
	for i, r := range rt.ComponentRefs {
		comps[i] = mustResolve(t, s, r)
	}
	return comps
}

// showExpr renders an expression as a readable S-expression (test aid).
func showExpr(e Expr) string {
	switch e.Kind {
	case TagRational:
		return e.Numerator.ToBig().String() + "/" + e.Denominator.ToBig().String()
	case TagSymbol:
		return e.DisplayName
	case TagAdd:
		s := "(+"
		for _, c := range e.Children {
			s += " " + showExpr(c)
		}
		return s + ")"
	case TagMul:
		s := "(*"
		for _, c := range e.Children {
			s += " " + showExpr(c)
		}
		return s + ")"
	case TagNeg:
		return "(- " + showExpr(*e.Operand) + ")"
	case TagPow:
		return "(^ " + showExpr(*e.Base) + " " + showExpr(*e.Exp) + ")"
	case TagSqrt:
		return "(sqrt " + showExpr(*e.Radicand) + ")"
	default:
		return "(?)"
	}
}

// polarAtoms builds the shared factored blocks of the polar Riemann golden.
// r2 = r^2, r2i = (r^2)^-1, r2i2 = (r^2)^-2, n2r = -(2r).
func polarAtoms(r Expr) (r2, r2i, r2i2, n2r Expr) {
	r2 = Pow(r, MustRational(2, 1))
	r2i = Pow(Pow(r, MustRational(2, 1)), MustRational(-1, 1))
	r2i2 = Pow(Pow(r, MustRational(2, 1)), MustRational(-2, 1))
	n2r = Neg(Mul(r, MustRational(2, 1)))
	return r2, r2i, r2i2, n2r
}

// TestRiemannPolarFlat pins the flat-space cancellation golden: the polar
// connection is nonzero, but every R^rho_sigma_mu_nu vanishes. Components
// whose formula terms all carry exact-zero factors collapse to literal 0/1;
// the rest take X-X factored forms whose byte-identical mirror pairs sum to
// zero mathematically. Point checks (r=1, r=2) on each factored form below
// confirm the encoded mathematics; derivations replay and Check passes.
func TestRiemannPolarFlat(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, r, _, cond := polarSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	result, recRef, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatalf("polar riemann failed: %v", err)
	}
	comps := riemannComps(t, s, result)
	if len(comps) != 16 {
		t.Fatalf("polar riemann has %d components, want 16", len(comps))
	}
	zero := MustRational(0, 1)
	for _, i := range []int{0, 1, 2, 4, 7, 8, 11, 13, 14} {
		if !comps[i].Equal(zero) {
			t.Fatalf("polar R[%d] is not exactly zero:\n%s", i, showExpr(comps[i]))
		}
	}
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne, negTwo := MustRational(-1, 1), MustRational(-2, 1)
	r2, r2i, r2i2, n2r := polarAtoms(r)
	// W = Gamma^r_thth * Gamma^th_rth, canon-flattened: the surviving
	// Gamma-Gamma product (zero-coefficient products strip).
	w := Mul(r, half, half, two, n2r, r2, r2i, r2i)
	// R[3] = R^r_rthth: d_th Gamma^r_thr(0) - d_th Gamma^r_rth(0) + W - W.
	want3, err := Canonicalize(Add(cloneExpr(w), Neg(cloneExpr(w))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[3]); err != nil || !got.Equal(want3) {
		t.Fatalf("polar R[3] is not the hand-derived W-W form:\n%s", showExpr(comps[3]))
	}
	// R[5] = R^r_thrth: d_r Gamma^r_thth - 0 + 0 - W, with the Leibniz legs
	// T1 = d(Neg(2r)) leg, T2 = d(r^2) leg, T3 = d((r^2)^-1) leg.
	// Point values: T1 = -1, T2 = -2, T3 = +2, W = -1 (r-independent).
	t1 := Mul(half, negTwo, r2, r2i)
	t2 := Mul(r, half, two, n2r, r2i)
	t3 := Mul(r, half, two, negOne, n2r, r2, r2i2)
	want5, err := Canonicalize(Add(t1, t2, t3, Neg(w)))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[5]); err != nil || !got.Equal(want5) {
		t.Fatalf("polar R[5] is not the hand-derived form:\n%s", showExpr(comps[5]))
	}
	// R[6] = R^r_ththr: antisymmetric partner, W - (T1+T2+T3).
	want6, err := Canonicalize(Add(cloneExpr(w), Neg(Add(cloneExpr(t1), cloneExpr(t2), cloneExpr(t3)))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[6]); err != nil || !got.Equal(want6) {
		t.Fatalf("polar R[6] is not the hand-derived form:\n%s", showExpr(comps[6]))
	}
	// R[9] = R^th_rrth: d_r Gamma^th_thr + (Gamma^th_rth)^2, with
	// U1 = d(2r) leg, U2 = d((r^2)^-1) leg, U3 = squared connection.
	// Point values at r=1: 1, -2, 1; at r=2: 1/4, -1/2, 1/4.
	u1 := Mul(half, two, r2i)
	u2 := Mul(r, r, half, two, two, negOne, r2i2)
	u3 := Mul(r, r, half, half, two, two, r2i, r2i)
	want9, err := Canonicalize(Add(u1, u2, u3))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[9]); err != nil || !got.Equal(want9) {
		t.Fatalf("polar R[9] is not the hand-derived form:\n%s", showExpr(comps[9]))
	}
	// R[10] = R^th_rrth: antisymmetric partner, -(U1+U2) - U3.
	want10, err := Canonicalize(Add(Neg(Add(cloneExpr(u1), cloneExpr(u2))), Neg(cloneExpr(u3))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[10]); err != nil || !got.Equal(want10) {
		t.Fatalf("polar R[10] is not the hand-derived form:\n%s", showExpr(comps[10]))
	}
	// R[12] = R^th_thrr: mu=nu identical pairs, (U1+U2)-(U1+U2)+U3-U3.
	want12, err := Canonicalize(Add(cloneExpr(u1), cloneExpr(u2), cloneExpr(u3),
		Neg(Add(cloneExpr(u1), cloneExpr(u2))), Neg(cloneExpr(u3))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[12]); err != nil || !got.Equal(want12) {
		t.Fatalf("polar R[12] is not the hand-derived form:\n%s", showExpr(comps[12]))
	}
	// R[15] = R^th_ththth: W - W with the same product bytes as R[3].
	want15, err := Canonicalize(Add(cloneExpr(w), Neg(cloneExpr(w))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[15]); err != nil || !got.Equal(want15) {
		t.Fatalf("polar R[15] is not the hand-derived form:\n%s", showExpr(comps[15]))
	}
	if !comps[15].Equal(comps[3]) {
		t.Fatal("polar R[15] does not share R[3] content-addressed bytes")
	}
	// Provenance: every embedded differentiation record revalidates, and the
	// composite record passes Check and artifact Replay.
	assertRiemannProvenance(t, s, recRef, 6)
	if err := CheckRiemannEvaluation(s, recRef); err != nil {
		t.Fatalf("polar record rejected by Check: %v", err)
	}
	if err := ReplayRiemannEvaluation(s, recRef); err != nil {
		t.Fatalf("polar record rejected by artifact replay: %v", err)
	}
}

// assertRiemannProvenance verifies that a composite derivation carries
// exactly want unique coordinate_differentiation/1 records, each passing
// CheckCoordinateDifferentiation.
func assertRiemannProvenance(t *testing.T, s Store, recRef artifact.ArtifactRef, want int) {
	t.Helper()
	rec := riemannRecord(t, s, recRef)
	seen := map[artifact.ArtifactRef]bool{}
	for _, r := range rec.PartialRecordRefs {
		seen[r] = true
	}
	if len(seen) != want {
		t.Fatalf("evaluation carries %d unique differentiation records, want %d", len(seen), want)
	}
	for r := range seen {
		if err := CheckCoordinateDifferentiation(s, r); err != nil {
			t.Fatalf("embedded differentiation record rejected: %v", err)
		}
	}
}

// riemannRecord resolves a riemann_evaluation/1 record for inspection.
func riemannRecord(t *testing.T, s Store, ref artifact.ArtifactRef) RiemannEvaluation {
	t.Helper()
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyRiemannEvaluation {
		t.Fatalf("record is %s/%s, not riemann_evaluation/1", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeRiemannEvaluation(payload)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// TestRiemannCurvedPoly pins the decisive nonzero golden for
// ds^2 = dx^2 + (1+x^2)^2 dy^2 with f = 1+x^2, F = f^2.
// Hand derivation: Gamma^x_yy = -2xf.(F/F), Gamma^y_xy = 2xf/F, rest vanish;
// R^x_yxy = d_x Gamma^x_yy - 0 + 0 - Gamma^x_yy Gamma^y_xy = -2f = -2(1+x^2).
// Point checks on the pinned factored form below: x=0 gives -2 (only T1
// survives); x=1 gives -8-8+8+4 = -4; x=2 gives -26-32+32+16 = -10.
// Fully covariant R_xyxy equals the mixed R^x_yxy here ONLY because g_xx = 1
// for this golden metric — documented as a property of the example, never a
// general equivalence. Gaussian curvature K = R_xyxy/det = -2f/f^2 = -2/f is
// computed independently by hand; no Gaussian evaluator is implemented.
func TestRiemannCurvedPoly(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, x, _, cond := curvedSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	result, recRef, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatalf("curved riemann failed: %v", err)
	}
	comps := riemannComps(t, s, result)
	if len(comps) != 16 {
		t.Fatalf("curved riemann has %d components, want 16", len(comps))
	}
	// Index contract: flat position 5 is (rho,sigma,mu,nu) = (x,y,x,y), so
	// R[5] is the mixed component R^x_yxy asserted below.
	if got := riemannIndex(0, 1, 0, 1, 2); got != 5 {
		t.Fatalf("riemann index ordering broken: (0,1,0,1) maps to %d, want 5", got)
	}
	zero := MustRational(0, 1)
	for _, i := range []int{0, 1, 2, 4, 7, 8, 11, 13, 14} {
		if !comps[i].Equal(zero) {
			t.Fatalf("curved R[%d] is not exactly zero:\n%s", i, showExpr(comps[i]))
		}
	}
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne := MustRational(-1, 1)
	twoX := Mul(x, two)
	f := Add(MustRational(1, 1), Pow(x, MustRational(2, 1)))
	bigF := Pow(cloneExpr(f), MustRational(2, 1))
	bigFi := Pow(cloneExpr(bigF), MustRational(-1, 1))
	bigFi2 := Pow(cloneExpr(bigF), MustRational(-2, 1))
	// dNegAdd: the differentiated Neg factor of Gamma^x_yy (zero leg strips).
	dNegAdd := Add(Mul(two, cloneExpr(f), two), Mul(x, two, two, cloneExpr(twoX)))
	// T1 = d(Neg) Leibniz leg, T2 = d(F) leg, T3 = d(F^-1) leg of d_x Gamma^x_yy.
	t1 := Mul(half, Neg(cloneExpr(dNegAdd)), cloneExpr(bigF), cloneExpr(bigFi))
	negInner := Neg(Mul(x, two, two, cloneExpr(f)))
	t2 := Mul(half, two, cloneExpr(twoX), cloneExpr(negInner), cloneExpr(f), cloneExpr(bigFi))
	t3 := Mul(half, two, negOne, cloneExpr(twoX), cloneExpr(negInner), cloneExpr(f), cloneExpr(bigF), cloneExpr(bigFi2))
	// V: the surviving Gamma-Gamma product -Gamma^x_yy Gamma^y_xy.
	prod := Mul(x, half, half, two, two, cloneExpr(f), cloneExpr(negInner), cloneExpr(bigF), cloneExpr(bigFi), cloneExpr(bigFi))
	// R[5] = R^x_yxy = T1+T2+T3 - V = -2(1+x^2) mathematically.
	want5, err := Canonicalize(Add(t1, t2, t3, Neg(prod)))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[5]); err != nil || !got.Equal(want5) {
		t.Fatalf("curved R[5] is not the hand-derived -2(1+x^2) form:\n%s", showExpr(comps[5]))
	}
	// Explicit anti-always-zero tripwire: the golden component is not zero.
	if comps[5].Equal(zero) {
		t.Fatal("curved R[5] collapsed to zero: nonzero golden lost")
	}
	// R[3] = R^x_xyy and R[15] = R^y_yyy: mu=nu identical pairs, V - V.
	want3, err := Canonicalize(Add(cloneExpr(prod), Neg(cloneExpr(prod))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[3]); err != nil || !got.Equal(want3) {
		t.Fatalf("curved R[3] is not the hand-derived form:\n%s", showExpr(comps[3]))
	}
	if got, err := Canonicalize(comps[15]); err != nil || !got.Equal(want3) {
		t.Fatalf("curved R[15] is not the hand-derived form:\n%s", showExpr(comps[15]))
	}
	if !comps[15].Equal(comps[3]) {
		t.Fatal("curved R[15] does not share R[3] content-addressed bytes")
	}
	// R[6] = R^x_yxy with mu/nu swapped: antisymmetric partner V - (T1+T2+T3).
	want6, err := Canonicalize(Add(cloneExpr(prod), Neg(Add(cloneExpr(t1), cloneExpr(t2), cloneExpr(t3)))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[6]); err != nil || !got.Equal(want6) {
		t.Fatalf("curved R[6] is not the hand-derived form:\n%s", showExpr(comps[6]))
	}
	// U1 = d(f)-form leg, U2 = d(F^-1)-form leg of d_x Gamma^y_yx, U3 = square.
	uInner := Add(Mul(two, cloneExpr(f), two), Mul(x, two, two, cloneExpr(twoX)))
	u1 := Mul(half, cloneExpr(uInner), cloneExpr(bigFi))
	u2 := Mul(x, half, two, two, two, negOne, cloneExpr(twoX), cloneExpr(f), cloneExpr(f), cloneExpr(bigFi2))
	u3 := Mul(x, x, half, half, two, two, two, two, cloneExpr(f), cloneExpr(f), cloneExpr(bigFi), cloneExpr(bigFi))
	// R[9] = R^y_xxy = U1+U2+U3 = 2/f mathematically (point: x=0 -> 2, x=1 -> 1).
	want9, err := Canonicalize(Add(u1, u2, u3))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[9]); err != nil || !got.Equal(want9) {
		t.Fatalf("curved R[9] is not the hand-derived form:\n%s", showExpr(comps[9]))
	}
	// R[10] = R^y_xyx: antisymmetric partner, -(U1+U2) - U3.
	want10, err := Canonicalize(Add(Neg(Add(cloneExpr(u1), cloneExpr(u2))), Neg(cloneExpr(u3))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[10]); err != nil || !got.Equal(want10) {
		t.Fatalf("curved R[10] is not the hand-derived form:\n%s", showExpr(comps[10]))
	}
	// R[12] = R^y_yxx: mu=nu identical pairs, (U1+U2)-(U1+U2)+U3-U3.
	want12, err := Canonicalize(Add(cloneExpr(u1), cloneExpr(u2), cloneExpr(u3),
		Neg(Add(cloneExpr(u1), cloneExpr(u2))), Neg(cloneExpr(u3))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[12]); err != nil || !got.Equal(want12) {
		t.Fatalf("curved R[12] is not the hand-derived form:\n%s", showExpr(comps[12]))
	}
	// Provenance: embedded records revalidate; composite passes Check and replay.
	assertRiemannProvenance(t, s, recRef, 6)
	if err := CheckRiemannEvaluation(s, recRef); err != nil {
		t.Fatalf("curved record rejected by Check: %v", err)
	}
	if err := ReplayRiemannEvaluation(s, recRef); err != nil {
		t.Fatalf("curved record rejected by artifact replay: %v", err)
	}
}

// TestRiemannOneDim pins the structural zero: in one dimension the single
// component is d_x Gamma - d_x Gamma + Gamma^2 - Gamma^2, whose premise terms
// pair into byte-identical Neg mirrors (A,B,C below). Point check at x=0:
// A=1, B=C=0; at x=1: A=1/2, B=-1/2, C=1/16. Total zero in both cases.
func TestRiemannOneDim(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, x, cond := oneDimSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	result, recRef, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatalf("one-dim riemann failed: %v", err)
	}
	comps := riemannComps(t, s, result)
	if len(comps) != 1 {
		t.Fatalf("one-dim riemann has %d components, want 1", len(comps))
	}
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne := MustRational(-1, 1)
	twoX := Mul(x, two)
	fp := Add(MustRational(1, 1), Pow(x, MustRational(2, 1)))
	fpi := Pow(cloneExpr(fp), MustRational(-1, 1))
	fpi2 := Pow(cloneExpr(fp), MustRational(-2, 1))
	// Dd: the differentiated (1+x^2) factor as produced by the engine.
	dd := Add(cloneExpr(twoX), cloneExpr(twoX), Neg(cloneExpr(twoX)))
	a := Mul(half, two, cloneExpr(fpi))
	b := Mul(half, negOne, cloneExpr(twoX), cloneExpr(dd), cloneExpr(fpi2))
	c := Mul(half, half, cloneExpr(dd), cloneExpr(dd), cloneExpr(fpi), cloneExpr(fpi))
	want, err := Canonicalize(Add(a, b, c, Neg(Add(cloneExpr(a), cloneExpr(b))), Neg(cloneExpr(c))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[0]); err != nil || !got.Equal(want) {
		t.Fatalf("one-dim R[0] is not the hand-derived mirror form:\n%s", showExpr(comps[0]))
	}
	// Every non-Neg child has its byte-identical Neg mirror: structural zero.
	assertNegMirrored(t, comps[0])
	assertRiemannProvenance(t, s, recRef, 1)
	if err := CheckRiemannEvaluation(s, recRef); err != nil {
		t.Fatalf("one-dim record rejected by Check: %v", err)
	}
	if err := ReplayRiemannEvaluation(s, recRef); err != nil {
		t.Fatalf("one-dim record rejected by artifact replay: %v", err)
	}
}

// assertNegMirrored requires an Add whose every non-Neg child appears Negated
// among its children, either directly or inside a Neg(Add(...)) group: a
// structural P-P+Q-Q zero without engine cancellation.
func assertNegMirrored(t *testing.T, e Expr) {
	t.Helper()
	if e.Kind != TagAdd {
		t.Fatalf("structural-zero target is not an Add: %s", showExpr(e))
	}
	negated := map[string]bool{}
	plain := []Expr{}
	for _, c := range e.Children {
		if c.Kind == TagNeg {
			if c.Operand.Kind == TagAdd {
				for _, g := range c.Operand.Children {
					raw, err := g.Encode()
					if err != nil {
						t.Fatal(err)
					}
					negated[string(raw)] = true
				}
				continue
			}
			raw, err := c.Operand.Encode()
			if err != nil {
				t.Fatal(err)
			}
			negated[string(raw)] = true
			continue
		}
		plain = append(plain, c)
	}
	for _, c := range plain {
		raw, err := c.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if !negated[string(raw)] {
			t.Fatalf("child without Neg mirror: %s", showExpr(c))
		}
	}
}

// TestRiemannSingularMetric: a singular source metric is rejected in the
// inverse path before any Riemann result is accepted. No result artifact is
// reported as verified: construction returns zero references with error.
func TestRiemannSingularMetric(t *testing.T) {
	s := NewMemStore()
	chart, _, fn, mapping, _, _, _ := polarSetup(t, s)
	one := mustStoreExpr(t, s, MustRational(1, 1))
	zero := mustStoreExpr(t, s, MustRational(0, 1))
	m := SymmetricBilinear{Dimension: 2, ComponentRefs: []artifact.ArtifactRef{one, zero, zero, zero}}
	sing, err := s.Put(FamilySymmetricBilinear, m.Encode())
	if err != nil {
		t.Fatal(err)
	}
	rf, rr, _, err := EvaluateRiemann(s, sing, chart, mapping, fn, nil)
	if err == nil {
		t.Fatal("singular metric accepted")
	}
	if !strings.Contains(err.Error(), "not invertible") {
		t.Fatalf("singular metric wrong boundary: %v", err)
	}
	if rf != (artifact.ArtifactRef{}) || rr != (artifact.ArtifactRef{}) {
		t.Fatal("singular metric returned usable references")
	}
}

// TestRiemannConditions pins condition licensing at its actual boundaries.
// The determinant condition licenses exactly the canonical NEQ(base,0) bases
// it names byte-for-byte; it is not a generic nonzero license. A merely
// related condition (NEQ(r,0) where the engine needs NEQ(r^2,0)) fails
// closed at the inverse stage, and a mismatched condition fails a
// reciprocal-coefficient partial even though inversion is not involved.
func TestRiemannConditions(t *testing.T) {
	t.Run("no-conditions-fail-closed", func(t *testing.T) {
		s := NewMemStore()
		chart, metric, fn, mapping, _, _, _ := polarSetup(t, s)
		if _, _, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, nil); err == nil {
			t.Fatal("condition-free polar evaluation accepted")
		} else if !errors.Is(err, ErrMissingCondition) {
			t.Fatalf("condition-free wrong boundary: %v", err)
		}
	})

	t.Run("related-condition-insufficient", func(t *testing.T) {
		s := NewMemStore()
		chart, metric, fn, mapping, r, _, _ := polarSetup(t, s)
		body, err := EncodeRelationBody(OpNEQ, r, MustRational(0, 1))
		if err != nil {
			t.Fatal(err)
		}
		rel, err := s.Put(FamilyRelation, body)
		if err != nil {
			t.Fatal(err)
		}
		rCond, err := s.Put(FamilyCondition, rel.Encode())
		if err != nil {
			t.Fatal(err)
		}
		// NEQ(r,0) does not byte-match the required NEQ(det,0): the inverse
		// stage fails closed rather than treating related evidence as license.
		if _, _, _, err := EvaluateRiemann(s, metric, chart, mapping, fn,
			[]artifact.ArtifactRef{rCond}); err == nil {
			t.Fatal("related-condition evaluation accepted")
		} else if !errors.Is(err, ErrMissingCondition) {
			t.Fatalf("related-condition wrong boundary: %v", err)
		}
	})

	t.Run("partial-exact-base-boundary", func(t *testing.T) {
		s := NewMemStore()
		chart, metric, fn, mapping, r, _, cond := polarSetup(t, s)
		_ = chart
		conn, _, err := LeviCivita(s, metric, chart, mapping, fn, []artifact.ArtifactRef{cond})
		if err != nil {
			t.Fatal(err)
		}
		_, cpayload, err := s.Lookup(conn)
		if err != nil {
			t.Fatal(err)
		}
		cc, err := DecodeAffineConnection(cpayload)
		if err != nil {
			t.Fatal(err)
		}
		// Slot 5 holds Gamma^th_rth with a (r^2)^-1 factor: differentiating
		// it under NEQ(r,0) must fail exact-base matching, while the
		// determinant condition licenses it.
		body, err := EncodeRelationBody(OpNEQ, r, MustRational(0, 1))
		if err != nil {
			t.Fatal(err)
		}
		rel, err := s.Put(FamilyRelation, body)
		if err != nil {
			t.Fatal(err)
		}
		rCond, err := s.Put(FamilyCondition, rel.Encode())
		if err != nil {
			t.Fatal(err)
		}
		_, cpayload2, err := s.Lookup(chart)
		if err != nil {
			t.Fatal(err)
		}
		_ = cpayload2
		if _, _, _, err := DifferentiateCoordinate(s, cc.CoefficientRefs[5], chart, 0, mapping, fn, 1,
			[]artifact.ArtifactRef{rCond}); err == nil {
			t.Fatal("mismatched-condition partial accepted")
		} else if !errors.Is(err, ErrMissingCondition) {
			t.Fatalf("mismatched-condition partial wrong boundary: %v", err)
		}
		if _, _, _, err := DifferentiateCoordinate(s, cc.CoefficientRefs[5], chart, 0, mapping, fn, 1,
			[]artifact.ArtifactRef{cond}); err != nil {
			t.Fatalf("licensed partial rejected: %v", err)
		}
	})
}

// tamperRiemannRecord decodes a record, applies mutate, and re-mints it.
func tamperRiemannRecord(t *testing.T, s Store, ref artifact.ArtifactRef, mutate func(*RiemannEvaluation)) artifact.ArtifactRef {
	t.Helper()
	rec := riemannRecord(t, s, ref)
	mutate(&rec)
	raw, err := rec.Encode()
	if err != nil {
		t.Fatalf("tampered record does not encode: %v", err)
	}
	bad, err := s.Put(FamilyRiemannEvaluation, raw)
	if err != nil {
		t.Fatal(err)
	}
	return bad
}

// TestRiemannAdversarial proves corrupted curvature evidence fails Check and
// artifact Replay at the intended boundary.
func TestRiemannAdversarial(t *testing.T) {
	mk := func(t *testing.T) (*MemStore, artifact.ArtifactRef, RiemannEvaluation) {
		s := NewMemStore()
		chart, metric, fn, mapping, _, _, cond := polarSetup(t, s)
		_, recRef, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, []artifact.ArtifactRef{cond})
		if err != nil {
			t.Fatal(err)
		}
		return s, recRef, riemannRecord(t, s, recRef)
	}
	bothFail := func(t *testing.T, s *MemStore, bad artifact.ArtifactRef, reason string) {
		t.Helper()
		if err := CheckRiemannEvaluation(s, bad); err == nil {
			t.Fatal("corrupted record accepted by Check")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("Check wrong boundary (want %q): %v", reason, err)
		}
		if err := ReplayRiemannEvaluation(s, bad); err == nil {
			t.Fatal("corrupted record accepted by artifact replay")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("replay wrong boundary (want %q): %v", reason, err)
		}
	}

	t.Run("swapped-mapping-construction", func(t *testing.T) {
		s := NewMemStore()
		chart, metric, fn, _, _, _, cond := polarSetup(t, s)
		swapped := CoordParamMap{Entries: []BoundParameterRef{
			{DeclaringArtifactRef: fn, ParameterID: DeriveParameterID(1)},
			{DeclaringArtifactRef: fn, ParameterID: DeriveParameterID(0)},
		}}
		if _, _, _, err := EvaluateRiemann(s, metric, chart, swapped, fn,
			[]artifact.ArtifactRef{cond}); err == nil {
			t.Fatal("swapped mapping accepted")
		} else if !strings.Contains(err.Error(), "PID mismatch") {
			t.Fatalf("swapped mapping wrong boundary: %v", err)
		}
	})

	t.Run("substituted-metric", func(t *testing.T) {
		s, recRef, rec := mk(t)
		// Wrong family in the metric position fails closed.
		badFam := tamperRiemannRecord(t, s, recRef, func(r *RiemannEvaluation) {
			r.MetricRef = rec.ConnectionRef
		})
		bothFail(t, s, badFam, "not in closed target set")
		// A different valid metric: the recorded connection no longer
		// matches recomputation from it, so metric verification fails.
		one := mustStoreExpr(t, s, MustRational(1, 1))
		zero := mustStoreExpr(t, s, MustRational(0, 1))
		flat, err := s.Put(FamilySymmetricBilinear, SymmetricBilinear{
			Dimension: 2, ComponentRefs: []artifact.ArtifactRef{one, zero, zero, one},
		}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		// License the flat metric's own inverse with its exact determinant
		// condition, so recomputation reaches the connection gate instead of
		// failing on unrelated exact-base conditions.
		flatDet, _, err := GeneralDeterminant(s, flat)
		if err != nil {
			t.Fatal(err)
		}
		flatCond := detCond(t, s, flatDet)
		bad := tamperRiemannRecord(t, s, recRef, func(r *RiemannEvaluation) {
			r.MetricRef = flat
			r.ConditionRefs = []artifact.ArtifactRef{flatCond}
		})
		bothFail(t, s, bad, "does not match")
	})

	t.Run("substituted-connection", func(t *testing.T) {
		s, recRef, rec := mk(t)
		zero := mustStoreExpr(t, s, MustRational(0, 1))
		coeffs := make([]artifact.ArtifactRef, 8)
		for i := range coeffs {
			coeffs[i] = zero
		}
		fake, err := s.Put(FamilyAffineConnection, AffineConnection{
			CoordRef: rec.ChartRef, Dimension: 2, CoefficientRefs: coeffs,
		}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperRiemannRecord(t, s, recRef, func(r *RiemannEvaluation) { r.ConnectionRef = fake })
		bothFail(t, s, bad, "does not match")
	})

	t.Run("substituted-result-chart-declaring", func(t *testing.T) {
		s, recRef, _ := mk(t)
		other := mustStoreExpr(t, s, MustRational(9, 1))
		bad := tamperRiemannRecord(t, s, recRef, func(r *RiemannEvaluation) { r.ResultRef = other })
		bothFail(t, s, bad, "not in closed target set")
		ghostChart := ComputeTestRef(t, "physmath", "coordinate_system/1")
		badChart := tamperRiemannRecord(t, s, recRef, func(r *RiemannEvaluation) { r.ChartRef = ghostChart })
		bothFail(t, s, badChart, "unresolvable")
		ghostFn := ComputeTestRef(t, "physmath", "function/1")
		badFn := tamperRiemannRecord(t, s, recRef, func(r *RiemannEvaluation) { r.DeclaringFnRef = ghostFn })
		bothFail(t, s, badFn, "unresolvable")
	})

	t.Run("altered-mapping-entry", func(t *testing.T) {
		s, recRef, rec := mk(t)
		swapped := CoordParamMap{Entries: []BoundParameterRef{
			rec.Mapping.Entries[1], rec.Mapping.Entries[0],
		}}
		bad := tamperRiemannRecord(t, s, recRef, func(r *RiemannEvaluation) { r.Mapping = swapped })
		bothFail(t, s, bad, "PID mismatch")
	})

	t.Run("dropped-record-and-tampered-step", func(t *testing.T) {
		s, recRef, rec := mk(t)
		dropped := append([]artifact.ArtifactRef{}, rec.PartialRecordRefs[:len(rec.PartialRecordRefs)-1]...)
		bad := tamperRiemannRecord(t, s, recRef, func(r *RiemannEvaluation) { r.PartialRecordRefs = dropped })
		bothFail(t, s, bad, "record count")
		// Tamper one assembly output inside the stored derivation.
		_, dpayload, err := s.Lookup(rec.DerivationRef)
		if err != nil {
			t.Fatal(err)
		}
		d, err := DecodeDerivation(dpayload)
		if err != nil {
			t.Fatal(err)
		}
		other := mustStoreExpr(t, s, MustRational(77, 1))
		d.Steps[len(d.Steps)-1].OutputRef = other
		draw, err := d.Encode()
		if err != nil {
			t.Fatal(err)
		}
		altDeriv, err := s.Put(FamilyDerivation, draw)
		if err != nil {
			t.Fatal(err)
		}
		badDeriv := tamperRiemannRecord(t, s, recRef, func(r *RiemannEvaluation) { r.DerivationRef = altDeriv })
		bothFail(t, s, badDeriv, "output mismatch")
	})

	t.Run("malformed-inputs", func(t *testing.T) {
		s := NewMemStore()
		chart, metric, fn, mapping, _, _, cond := polarSetup(t, s)
		conds := []artifact.ArtifactRef{cond}
		exprRef := mustStoreExpr(t, s, MustRational(1, 1))
		if _, _, _, err := EvaluateRiemann(s, exprRef, chart, mapping, fn, conds); err == nil {
			t.Fatal("wrong-family metric accepted")
		}
		ghost := ComputeTestRef(t, "physmath", "symmetric_bilinear/1")
		if _, _, _, err := EvaluateRiemann(s, ghost, chart, mapping, fn, conds); err == nil {
			t.Fatal("unresolvable metric accepted")
		}
		chart3, _, _ := chartFixture(t, s, "x", "y", "z")
		if _, _, _, err := EvaluateRiemann(s, metric, chart3, mapping, fn, conds); err == nil {
			t.Fatal("chart/metric dimension mismatch accepted")
		}
		// Dimension bound enforced before minting.
		px := []ParameterID{DeriveParameterID(0), DeriveParameterID(1), DeriveParameterID(2),
			DeriveParameterID(3), DeriveParameterID(4)}
		coords := []Expr{
			NewBoundSymbol("a", px[0]), NewBoundSymbol("b", px[1]), NewBoundSymbol("c", px[2]),
			NewBoundSymbol("d", px[3]), NewBoundSymbol("e", px[4]),
		}
		bigChart, bigFn, bigMap := bindFixture(t, s, coords, px,
			[]string{"a", "b", "c", "d", "e"},
			[]ParameterID{px[0], px[1], px[2], px[3], px[4]})
		zero := mustStoreExpr(t, s, MustRational(0, 1))
		zrefs := make([]artifact.ArtifactRef, 25)
		for i := range zrefs {
			zrefs[i] = zero
		}
		bigMetric, err := s.Put(FamilySymmetricBilinear, SymmetricBilinear{Dimension: 5, ComponentRefs: zrefs}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := EvaluateRiemann(s, bigMetric, bigChart, bigMap, bigFn, nil); err == nil {
			t.Fatal("dimension-5 metric accepted")
		} else if !strings.Contains(err.Error(), "exceeds bound") {
			t.Fatalf("dimension bound wrong boundary: %v", err)
		}
	})

	t.Run("record-codec-gates", func(t *testing.T) {
		s, recRef, rec := mk(t)
		_ = s
		raw, err := rec.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeRiemannEvaluation(raw); err != nil {
			t.Fatalf("record does not round-trip: %v", err)
		}
		if _, err := DecodeRiemannEvaluation(raw[:len(raw)/2]); err == nil {
			t.Fatal("truncated record accepted")
		}
		if _, err := DecodeRiemannEvaluation(append(append([]byte{}, raw...), 0x00)); err == nil {
			t.Fatal("trailing bytes accepted")
		}
		empty := rec
		empty.Mapping = CoordParamMap{}
		if err := empty.Validate(); err == nil {
			t.Fatal("empty-mapping record validated")
		}
		dup := rec
		dup.Mapping = CoordParamMap{Entries: []BoundParameterRef{
			rec.Mapping.Entries[0], rec.Mapping.Entries[0],
		}}
		if err := dup.Validate(); err == nil {
			t.Fatal("duplicate-PID mapping record validated")
		}
		dupConds := rec
		dupConds.ConditionRefs = []artifact.ArtifactRef{rec.ConditionRefs[0], rec.ConditionRefs[0]}
		if err := dupConds.Validate(); err == nil {
			t.Fatal("duplicate-condition record validated")
		}
		if err := CheckRiemannEvaluation(s, recRef); err != nil {
			t.Fatalf("valid record rejected: %v", err)
		}
		if err := ReplayRiemannEvaluation(s, recRef); err != nil {
			t.Fatalf("valid record replay rejected: %v", err)
		}
		if err := CheckRiemannEvaluation(s, exprRefOf(t, s)); err == nil {
			t.Fatal("expression target accepted as riemann record")
		}
	})
}

// exprRefOf mints a bare expression reference for wrong-family tests.
func exprRefOf(t *testing.T, s Store) artifact.ArtifactRef {
	t.Helper()
	return mustStoreExpr(t, s, MustRational(1, 1))
}

// TestRiemannContractionPolarFlat pins the polar Ricci golden: R_rr and
// R_thth reuse the Riemann factored blocks (program-verified child
// multisets: Q0 matches R[10], Q3 matches R[5]+R[15]), R_rth = R_thr = 0/1.
// Point values: every component 0 at r=1 and r=2.
func TestRiemannContractionPolarFlat(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, r, _, cond := polarSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	result, recRef, _, err := EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatalf("polar contraction failed: %v", err)
	}
	comps := riemannContractionComps(t, s, result)
	if len(comps) != 4 {
		t.Fatalf("polar contraction has %d components, want 4", len(comps))
	}
	zero := MustRational(0, 1)
	for _, i := range []int{1, 2} {
		if !comps[i].Equal(zero) {
			t.Fatalf("polar Ricci[%d] is not exactly zero:\n%s", i, showExpr(comps[i]))
		}
	}
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne := MustRational(-1, 1)
	r2, r2i, r2i2, n2r := polarAtoms(r)
	// Q0 = R_rr = R[10]-form: -(U1+U2) - U3 (single surviving summand).
	u1 := Mul(half, two, cloneExpr(r2i))
	u2 := Mul(r, r, half, two, two, negOne, cloneExpr(r2i2))
	u3 := Mul(r, r, half, half, two, two, cloneExpr(r2i), cloneExpr(r2i))
	want0, err := Canonicalize(Add(Neg(Add(cloneExpr(u1), cloneExpr(u2))), Neg(cloneExpr(u3))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[0]); err != nil || !got.Equal(want0) {
		t.Fatalf("polar Ricci[0] is not the hand-derived form:\n%s", showExpr(comps[0]))
	}
	// Q3 = R_thth = R[5]-form + R[15]-form: T1+T2+T3 - W + W - W.
	t1 := Mul(half, MustRational(-2, 1), cloneExpr(r2), cloneExpr(r2i))
	t2 := Mul(r, half, two, cloneExpr(n2r), cloneExpr(r2i))
	t3 := Mul(r, half, two, negOne, cloneExpr(n2r), cloneExpr(r2), cloneExpr(r2i2))
	w := Mul(r, half, half, two, cloneExpr(n2r), cloneExpr(r2), cloneExpr(r2i), cloneExpr(r2i))
	want3, err := Canonicalize(Add(t1, t2, t3, cloneExpr(w), Neg(cloneExpr(w)), Neg(cloneExpr(w))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[3]); err != nil || !got.Equal(want3) {
		t.Fatalf("polar Ricci[3] is not the hand-derived form:\n%s", showExpr(comps[3]))
	}
	// Result contract: rank 2, both slots covariant over the chart space.
	assertContractionShape(t, s, result, chart, 2)
	if err := CheckRiemannContraction(s, recRef); err != nil {
		t.Fatalf("polar contraction rejected by Check: %v", err)
	}
	if err := ReplayRiemannContraction(s, recRef); err != nil {
		t.Fatalf("polar contraction rejected by artifact replay: %v", err)
	}
}

// riemannContractionComps resolves the ordered Ricci components.
func riemannContractionComps(t *testing.T, s Store, resultField artifact.ArtifactRef) []Expr {
	t.Helper()
	_, fpayload, err := s.Lookup(resultField)
	if err != nil {
		t.Fatal(err)
	}
	rf, err := DecodeTensorField(fpayload)
	if err != nil {
		t.Fatal(err)
	}
	_, tpayload, err := s.Lookup(rf.TensorRef)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(tpayload)
	if err != nil {
		t.Fatal(err)
	}
	comps := make([]Expr, len(rt.ComponentRefs))
	for i, r := range rt.ComponentRefs {
		comps[i] = mustResolve(t, s, r)
	}
	return comps
}

// assertContractionShape verifies rank, dimension, chart, slot variance and
// space of a Ricci result field.
func assertContractionShape(t *testing.T, s Store, resultField, chart artifact.ArtifactRef, dim int) {
	t.Helper()
	_, fpayload, err := s.Lookup(resultField)
	if err != nil {
		t.Fatal(err)
	}
	rf, err := DecodeTensorField(fpayload)
	if err != nil {
		t.Fatal(err)
	}
	if rf.CoordinateSystemRef != chart {
		t.Fatal("contraction result chart not preserved")
	}
	_, tpayload, err := s.Lookup(rf.TensorRef)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeTensor(tpayload)
	if err != nil {
		t.Fatal(err)
	}
	if int(rt.Rank) != 2 || int(rt.Dimension) != dim || len(rt.ComponentRefs) != dim*dim {
		t.Fatalf("contraction result shape wrong: rank %d dim %d comps %d", rt.Rank, rt.Dimension, len(rt.ComponentRefs))
	}
	_, cpayload, err := s.Lookup(chart)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.IndexRefs) != 2 {
		t.Fatal("contraction result index count wrong")
	}
	for i, r := range rt.IndexRefs {
		idx, err := resolveIndex(s, "test slot", r)
		if err != nil {
			t.Fatal(err)
		}
		if idx.Variance != VarianceCovariant {
			t.Fatalf("contraction slot %d variance wrong", i)
		}
		if idx.Space == nil || *idx.Space != cs.SpaceRef {
			t.Fatalf("contraction slot %d space mismatch", i)
		}
	}
}

// TestRiemannScalarPolarFlat pins the polar scalar golden: R = g^rr R_rr +
// g^thth R_thth with g^rr = r^2.(r^2)^-1 (adjugate form, never collapsed to
// 1) and g^thth = (r^2)^-1. Both Ricci factors are zero-valued factored
// forms (point: r=1 and r=2 give 0), so the scalar is a factored zero. The
// pairing of inverse components with Ricci components in (sigma,nu) order is
// what this golden locks; the curved golden proves nonzero closure.
func TestRiemannScalarPolarFlat(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, r, _, cond := polarSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	_, crec, _, err := EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatal(err)
	}
	scalar, srec, _, err := EvaluateRiemannScalar(s, crec)
	if err != nil {
		t.Fatalf("polar scalar failed: %v", err)
	}
	got := mustResolve(t, s, scalar)
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne := MustRational(-1, 1)
	r2, r2i, r2i2, n2r := polarAtoms(r)
	u1 := Mul(half, two, cloneExpr(r2i))
	u2 := Mul(r, r, half, two, two, negOne, cloneExpr(r2i2))
	u3 := Mul(r, r, half, half, two, two, cloneExpr(r2i), cloneExpr(r2i))
	q0 := Add(Neg(Add(cloneExpr(u1), cloneExpr(u2))), Neg(cloneExpr(u3)))
	t1 := Mul(half, MustRational(-2, 1), cloneExpr(r2), cloneExpr(r2i))
	t2 := Mul(r, half, two, cloneExpr(n2r), cloneExpr(r2i))
	t3 := Mul(r, half, two, negOne, cloneExpr(n2r), cloneExpr(r2), cloneExpr(r2i2))
	w := Mul(r, half, half, two, cloneExpr(n2r), cloneExpr(r2), cloneExpr(r2i), cloneExpr(r2i))
	q3 := Add(t1, t2, t3, cloneExpr(w), Neg(cloneExpr(w)), Neg(cloneExpr(w)))
	grr := Mul(cloneExpr(r2), cloneExpr(r2i))
	gtt := cloneExpr(r2i)
	want, err := Canonicalize(Add(Mul(cloneExpr(grr), q0), Mul(cloneExpr(gtt), q3)))
	if err != nil {
		t.Fatal(err)
	}
	if gotCanon, err := Canonicalize(got); err != nil || !gotCanon.Equal(want) {
		t.Fatalf("polar scalar is not the hand-derived form:\n%s", showExpr(got))
	}
	// Determinant evidence pins to (r^2)-form: anchors the K-style quotient.
	rec := scalarRecord(t, s, srec)
	detWant, err := Canonicalize(Pow(r, MustRational(2, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if detGot, err := Canonicalize(mustResolve(t, s, rec.DetRef)); err != nil || !detGot.Equal(detWant) {
		t.Fatalf("polar determinant is not (r^2)-form:\n%s", showExpr(mustResolve(t, s, rec.DetRef)))
	}
	if err := CheckRiemannScalar(s, srec); err != nil {
		t.Fatalf("polar scalar rejected by Check: %v", err)
	}
	if err := ReplayRiemannScalar(s, srec); err != nil {
		t.Fatalf("polar scalar rejected by artifact replay: %v", err)
	}
}

// scalarRecord resolves a riemann_scalar/1 record for inspection.
func scalarRecord(t *testing.T, s Store, ref artifact.ArtifactRef) RiemannScalarEvaluation {
	t.Helper()
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyRiemannScalar {
		t.Fatalf("record is %s/%s, not riemann_scalar/1", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeRiemannScalarEvaluation(payload)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// TestRiemannContractionCurvedPoly pins the decisive nonzero Ricci golden for
// ds^2 = dx^2 + (1+x^2)^2 dy^2 with f = 1+x^2, F = f^2. Hand derivation:
// R_xx = -2/f (point: x=0 -> -2, x=1 -> -1, x=2 -> -2/5),
// R_yy = -2f (point: x=0 -> -2, x=1 -> -4, x=2 -> -10),
// R_xy = R_yx = 0/1 exactly. An always-zero implementation fails these pins.
func TestRiemannContractionCurvedPoly(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, x, _, cond := curvedSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	result, recRef, _, err := EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatalf("curved contraction failed: %v", err)
	}
	comps := riemannContractionComps(t, s, result)
	if len(comps) != 4 {
		t.Fatalf("curved contraction has %d components, want 4", len(comps))
	}
	zero := MustRational(0, 1)
	for _, i := range []int{1, 2} {
		if !comps[i].Equal(zero) {
			t.Fatalf("curved Ricci[%d] is not exactly zero:\n%s", i, showExpr(comps[i]))
		}
	}
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne := MustRational(-1, 1)
	f := Add(MustRational(1, 1), Pow(x, MustRational(2, 1)))
	bigF := Pow(cloneExpr(f), MustRational(2, 1))
	bigFi := Pow(cloneExpr(bigF), MustRational(-1, 1))
	bigFi2 := Pow(cloneExpr(bigF), MustRational(-2, 1))
	negFlat := Neg(Mul(x, two, two, cloneExpr(f)))
	// Q0 = R_xx: -(A+B) - C with A = d(f)-form leg, B = d(F^-1)-form leg,
	// C = squared-connection leg. Point: x=0 -> -(2+0)-0 = -2.
	aInner := Add(Mul(two, two, cloneExpr(f)), Mul(x, x, two, two, two))
	a := Mul(half, cloneExpr(aInner), cloneExpr(bigFi))
	b := Mul(x, x, half, two, two, two, two, negOne, cloneExpr(f), cloneExpr(f), cloneExpr(bigFi2))
	c := Mul(x, x, half, half, two, two, two, two, cloneExpr(f), cloneExpr(f), cloneExpr(bigFi), cloneExpr(bigFi))
	_ = negFlat
	want0, err := Canonicalize(Add(Neg(Add(cloneExpr(a), cloneExpr(b))), Neg(cloneExpr(c))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[0]); err != nil || !got.Equal(want0) {
		t.Fatalf("curved Ricci[0] is not the hand-derived -2/(1+x^2) form:\n%s", showExpr(comps[0]))
	}
	if comps[0].Equal(zero) {
		t.Fatal("curved Ricci[0] collapsed to zero: nonzero golden lost")
	}
	// Q3 = R_yy: N1+N2+N3 + V - V - V = -2f mathematically.
	// Point: x=0 -> -2; x=1 -> -8-8+8-4+4+4 = -4; x=2 -> -26-32+32-16+16+16 = -10.
	n1 := Mul(half, Neg(Add(Mul(two, two, cloneExpr(f)), Mul(x, x, two, two, two))), cloneExpr(bigF), cloneExpr(bigFi))
	n2 := Mul(x, half, two, two, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigFi))
	n3 := Mul(x, half, two, two, negOne, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigF), cloneExpr(bigFi2))
	v := Mul(x, half, half, two, two, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigF), cloneExpr(bigFi), cloneExpr(bigFi))
	want3, err := Canonicalize(Add(n1, n2, n3, cloneExpr(v), Neg(cloneExpr(v)), Neg(cloneExpr(v))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[3]); err != nil || !got.Equal(want3) {
		t.Fatalf("curved Ricci[3] is not the hand-derived -2(1+x^2) form:\n%s", showExpr(comps[3]))
	}
	if comps[3].Equal(zero) {
		t.Fatal("curved Ricci[3] collapsed to zero: nonzero golden lost")
	}
	assertContractionShape(t, s, result, chart, 2)
	if err := CheckRiemannContraction(s, recRef); err != nil {
		t.Fatalf("curved contraction rejected by Check: %v", err)
	}
	if err := ReplayRiemannContraction(s, recRef); err != nil {
		t.Fatalf("curved contraction rejected by artifact replay: %v", err)
	}
}

// TestRiemannScalarCurvedPoly pins R = g^xx R_xx + g^yy R_yy = -4/(1+x^2):
// point x=0 -> -2 + 1.(-2) = -4; x=1 -> -1 + (1/4)(-4) = -2; x=2 ->
// -2/5 + (1/25)(-10) = -4/5. The inverse blocks g^xx = F.(F^-1) (adjugate
// form, never collapsed to 1) and g^yy = (F^-1) are pinned as stored.
func TestRiemannScalarCurvedPoly(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, x, _, cond := curvedSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	_, crec, _, err := EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatal(err)
	}
	scalar, srec, _, err := EvaluateRiemannScalar(s, crec)
	if err != nil {
		t.Fatalf("curved scalar failed: %v", err)
	}
	got := mustResolve(t, s, scalar)
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne := MustRational(-1, 1)
	f := Add(MustRational(1, 1), Pow(x, MustRational(2, 1)))
	bigF := Pow(cloneExpr(f), MustRational(2, 1))
	bigFi := Pow(cloneExpr(bigF), MustRational(-1, 1))
	bigFi2 := Pow(cloneExpr(bigF), MustRational(-2, 1))
	// Ricci blocks, identical math to the contraction golden.
	aInner := Add(Mul(two, two, cloneExpr(f)), Mul(x, x, two, two, two))
	a := Mul(half, cloneExpr(aInner), cloneExpr(bigFi))
	b := Mul(x, x, half, two, two, two, two, negOne, cloneExpr(f), cloneExpr(f), cloneExpr(bigFi2))
	c := Mul(x, x, half, half, two, two, two, two, cloneExpr(f), cloneExpr(f), cloneExpr(bigFi), cloneExpr(bigFi))
	q0 := Add(Neg(Add(cloneExpr(a), cloneExpr(b))), Neg(cloneExpr(c)))
	n1 := Mul(half, Neg(Add(Mul(two, two, cloneExpr(f)), Mul(x, x, two, two, two))), cloneExpr(bigF), cloneExpr(bigFi))
	n2 := Mul(x, half, two, two, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigFi))
	n3 := Mul(x, half, two, two, negOne, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigF), cloneExpr(bigFi2))
	v := Mul(x, half, half, two, two, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigF), cloneExpr(bigFi), cloneExpr(bigFi))
	q3 := Add(n1, n2, n3, cloneExpr(v), Neg(cloneExpr(v)), Neg(cloneExpr(v)))
	gxx := Mul(cloneExpr(bigF), cloneExpr(bigFi))
	gyy := cloneExpr(bigFi)
	want, err := Canonicalize(Add(Mul(cloneExpr(gxx), q0), Mul(cloneExpr(gyy), q3)))
	if err != nil {
		t.Fatal(err)
	}
	if gotCanon, err := Canonicalize(got); err != nil || !gotCanon.Equal(want) {
		t.Fatalf("curved scalar is not the hand-derived -4/(1+x^2) form:\n%s", showExpr(got))
	}
	if got.Equal(MustRational(0, 1)) {
		t.Fatal("curved scalar collapsed to zero: nonzero golden lost")
	}
	// Determinant evidence pins to F = (1+x^2)^2: anchors K = R/det by hand.
	rec := scalarRecord(t, s, srec)
	detWant, err := Canonicalize(Pow(cloneExpr(f), MustRational(2, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if detGot, err := Canonicalize(mustResolve(t, s, rec.DetRef)); err != nil || !detGot.Equal(detWant) {
		t.Fatalf("curved determinant is not (1+x^2)^2-form:\n%s", showExpr(mustResolve(t, s, rec.DetRef)))
	}
	if err := CheckRiemannScalar(s, srec); err != nil {
		t.Fatalf("curved scalar rejected by Check: %v", err)
	}
	if err := ReplayRiemannScalar(s, srec); err != nil {
		t.Fatalf("curved scalar rejected by artifact replay: %v", err)
	}
}

// TestRiemannContractionOneDim pins the 1D Ricci golden: the single component
// is the Riemann P-P+Q-Q mirror form with A = d(1)-form leg, B = d(f')-form
// leg, C = squared-connection leg, Dd = the differentiated (1+x^2) factor.
// Point: x=0 -> (1+0)-(1+0)+0-0 = 0; x=1 -> (1/2-1/2)-(1/2-1/2)+1/16-1/16 = 0.
func TestRiemannContractionOneDim(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, x, cond := oneDimSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	result, recRef, _, err := EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatalf("one-dim contraction failed: %v", err)
	}
	comps := riemannContractionComps(t, s, result)
	if len(comps) != 1 {
		t.Fatalf("one-dim contraction has %d components, want 1", len(comps))
	}
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne := MustRational(-1, 1)
	twoX := Mul(x, two)
	fp := Add(MustRational(1, 1), Pow(x, MustRational(2, 1)))
	fpi := Pow(cloneExpr(fp), MustRational(-1, 1))
	fpi2 := Pow(cloneExpr(fp), MustRational(-2, 1))
	dd := Add(cloneExpr(twoX), cloneExpr(twoX), Neg(cloneExpr(twoX)))
	a := Mul(half, two, cloneExpr(fpi))
	b := Mul(half, negOne, cloneExpr(twoX), cloneExpr(dd), cloneExpr(fpi2))
	c := Mul(half, half, cloneExpr(dd), cloneExpr(dd), cloneExpr(fpi), cloneExpr(fpi))
	want, err := Canonicalize(Add(a, b, c, Neg(Add(cloneExpr(a), cloneExpr(b))), Neg(cloneExpr(c))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[0]); err != nil || !got.Equal(want) {
		t.Fatalf("one-dim Ricci[0] is not the hand-derived form:\n%s", showExpr(comps[0]))
	}
	assertNegMirrored(t, comps[0])
	assertContractionShape(t, s, result, chart, 1)
	if err := CheckRiemannContraction(s, recRef); err != nil {
		t.Fatalf("one-dim contraction rejected by Check: %v", err)
	}
	if err := ReplayRiemannContraction(s, recRef); err != nil {
		t.Fatalf("one-dim contraction rejected by artifact replay: %v", err)
	}
}

// TestRiemannScalarOneDim pins the 1D scalar golden: R = g^xx R_xx with
// g^xx = (1+x^2)^-1 in adjugate form. Point: x=0 -> 0.1 = 0; x=1 -> 0.1/2 = 0.
func TestRiemannScalarOneDim(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, x, cond := oneDimSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	_, crec, _, err := EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatal(err)
	}
	scalar, srec, _, err := EvaluateRiemannScalar(s, crec)
	if err != nil {
		t.Fatalf("one-dim scalar failed: %v", err)
	}
	got := mustResolve(t, s, scalar)
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne := MustRational(-1, 1)
	twoX := Mul(x, two)
	fp := Add(MustRational(1, 1), Pow(x, MustRational(2, 1)))
	fpi := Pow(cloneExpr(fp), MustRational(-1, 1))
	fpi2 := Pow(cloneExpr(fp), MustRational(-2, 1))
	dd := Add(cloneExpr(twoX), cloneExpr(twoX), Neg(cloneExpr(twoX)))
	a := Mul(half, two, cloneExpr(fpi))
	b := Mul(half, negOne, cloneExpr(twoX), cloneExpr(dd), cloneExpr(fpi2))
	c := Mul(half, half, cloneExpr(dd), cloneExpr(dd), cloneExpr(fpi), cloneExpr(fpi))
	q0 := Add(a, b, c, Neg(Add(cloneExpr(a), cloneExpr(b))), Neg(cloneExpr(c)))
	want, err := Canonicalize(Mul(q0, cloneExpr(fpi)))
	if err != nil {
		t.Fatal(err)
	}
	if gotCanon, err := Canonicalize(got); err != nil || !gotCanon.Equal(want) {
		t.Fatalf("one-dim scalar is not the hand-derived form:\n%s", showExpr(got))
	}
	if err := CheckRiemannScalar(s, srec); err != nil {
		t.Fatalf("one-dim scalar rejected by Check: %v", err)
	}
	if err := ReplayRiemannScalar(s, srec); err != nil {
		t.Fatalf("one-dim scalar rejected by artifact replay: %v", err)
	}
}

// tamperContractionRecord decodes a contraction record, applies mutate, and
// re-mints it as a new artifact.
func tamperContractionRecord(t *testing.T, s Store, ref artifact.ArtifactRef, mutate func(*RiemannContractionEvaluation)) artifact.ArtifactRef {
	t.Helper()
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyRiemannContraction {
		t.Fatalf("record is %s/%s, not riemann_contraction/1", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeRiemannContractionEvaluation(payload)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&rec)
	raw, err := rec.Encode()
	if err != nil {
		t.Fatalf("tampered record does not encode: %v", err)
	}
	bad, err := s.Put(FamilyRiemannContraction, raw)
	if err != nil {
		t.Fatal(err)
	}
	return bad
}

// tamperScalarRecord decodes a scalar record, applies mutate, and re-mints it.
func tamperScalarRecord(t *testing.T, s Store, ref artifact.ArtifactRef, mutate func(*RiemannScalarEvaluation)) artifact.ArtifactRef {
	t.Helper()
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyRiemannScalar {
		t.Fatalf("record is %s/%s, not riemann_scalar/1", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeRiemannScalarEvaluation(payload)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&rec)
	raw, err := rec.Encode()
	if err != nil {
		t.Fatalf("tampered record does not encode: %v", err)
	}
	bad, err := s.Put(FamilyRiemannScalar, raw)
	if err != nil {
		t.Fatal(err)
	}
	return bad
}

// TestRiemannContractionAdversarial proves corrupted contraction evidence
// fails Check and artifact Replay at the intended boundary.
func TestRiemannContractionAdversarial(t *testing.T) {
	mk := func(t *testing.T) (*MemStore, artifact.ArtifactRef, RiemannContractionEvaluation) {
		s := NewMemStore()
		chart, metric, fn, mapping, _, _, cond := polarSetup(t, s)
		_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, []artifact.ArtifactRef{cond})
		if err != nil {
			t.Fatal(err)
		}
		_, recRef, _, err := EvaluateRiemannContraction(s, riemRec)
		if err != nil {
			t.Fatal(err)
		}
		return s, recRef, riemannContractionRecord(t, s, recRef)
	}
	bothFail := func(t *testing.T, s *MemStore, bad artifact.ArtifactRef, reason string) {
		t.Helper()
		if err := CheckRiemannContraction(s, bad); err == nil {
			t.Fatal("corrupted record accepted by Check")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("Check wrong boundary (want %q): %v", reason, err)
		}
		if err := ReplayRiemannContraction(s, bad); err == nil {
			t.Fatal("corrupted record accepted by artifact replay")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("replay wrong boundary (want %q): %v", reason, err)
		}
	}

	t.Run("substituted-riemann-source", func(t *testing.T) {
		s, recRef, _ := mk(t)
		// A foreign Riemann record whose dependencies do not resolve here
		// fails closed at the availability boundary.
		s2 := NewMemStore()
		c2, m2, f2, mp2, _, _, cd2 := curvedSetup(t, s2)
		_, rr2, _, err := EvaluateRiemann(s2, m2, c2, mp2, f2, []artifact.ArtifactRef{cd2})
		if err != nil {
			t.Fatal(err)
		}
		_, rp, err := s2.Lookup(rr2)
		if err != nil {
			t.Fatal(err)
		}
		rrCopy, err := s.Put(FamilyRiemannEvaluation, rp)
		if err != nil {
			t.Fatal(err)
		}
		ghost := tamperContractionRecord(t, s, recRef, func(r *RiemannContractionEvaluation) { r.RiemannRef = rrCopy })
		bothFail(t, s, ghost, "unresolvable")
		// A fully resolvable Riemann record for a DIFFERENT metric on the
		// same chart reaches the component gate: recomputation from the
		// substituted result no longer matches the stored contraction.
		_, cpayload, err := s.Lookup(recRef)
		if err != nil {
			t.Fatal(err)
		}
		crec, err := DecodeRiemannContractionEvaluation(cpayload)
		if err != nil {
			t.Fatal(err)
		}
		_, rpayload, err := s.Lookup(crec.RiemannRef)
		if err != nil {
			t.Fatal(err)
		}
		rrec, err := DecodeRiemannEvaluation(rpayload)
		if err != nil {
			t.Fatal(err)
		}
		one := mustStoreExpr(t, s, MustRational(1, 1))
		zero := mustStoreExpr(t, s, MustRational(0, 1))
		flat, err := s.Put(FamilySymmetricBilinear, SymmetricBilinear{
			Dimension: 2, ComponentRefs: []artifact.ArtifactRef{one, zero, zero, one},
		}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		flatCond := detCond(t, s, mustFlatDet(t, s, flat))
		_, flatRiem, _, err := EvaluateRiemann(s, flat, rrec.ChartRef, rrec.Mapping, rrec.DeclaringFnRef, []artifact.ArtifactRef{flatCond})
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperContractionRecord(t, s, recRef, func(r *RiemannContractionEvaluation) { r.RiemannRef = flatRiem })
		bothFail(t, s, bad, "mismatch")
	})

	t.Run("substituted-result-reordered", func(t *testing.T) {
		s, recRef, rec := mk(t)
		_, fpayload, err := s.Lookup(rec.ResultRef)
		if err != nil {
			t.Fatal(err)
		}
		rf, err := DecodeTensorField(fpayload)
		if err != nil {
			t.Fatal(err)
		}
		_, tpayload, err := s.Lookup(rf.TensorRef)
		if err != nil {
			t.Fatal(err)
		}
		rt, err := DecodeTensor(tpayload)
		if err != nil {
			t.Fatal(err)
		}
		// Swap two component references: reordered result evidence.
		swapped := append([]artifact.ArtifactRef{}, rt.ComponentRefs...)
		swapped[0], swapped[3] = swapped[3], swapped[0]
		badTensor, err := s.Put(FamilyTensor, Tensor{Rank: 2, Dimension: 2, IndexRefs: rt.IndexRefs, ComponentRefs: swapped}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		badField, err := s.Put(FamilyTensorField, TensorField{TensorRef: badTensor, CoordinateSystemRef: rf.CoordinateSystemRef}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperContractionRecord(t, s, recRef, func(r *RiemannContractionEvaluation) { r.ResultRef = badField })
		bothFail(t, s, bad, "mismatch")
	})

	t.Run("substituted-derivation-chart", func(t *testing.T) {
		s, recRef, _ := mk(t)
		other := mustStoreExpr(t, s, MustRational(5, 1))
		_ = other
		ghostDeriv := ComputeTestRef(t, "physmath", "derivation/1")
		bad := tamperContractionRecord(t, s, recRef, func(r *RiemannContractionEvaluation) { r.DerivationRef = ghostDeriv })
		bothFail(t, s, bad, "unresolvable")
		ghostChart := ComputeTestRef(t, "physmath", "coordinate_system/1")
		badChart := tamperContractionRecord(t, s, recRef, func(r *RiemannContractionEvaluation) { r.ChartRef = ghostChart })
		bothFail(t, s, badChart, "unresolvable")
	})

	t.Run("wrong-family-and-ghost", func(t *testing.T) {
		s, _, _ := mk(t)
		exprRef := mustStoreExpr(t, s, MustRational(1, 1))
		if err := CheckRiemannContraction(s, exprRef); err == nil {
			t.Fatal("expression target accepted as contraction record")
		}
		if err := ReplayRiemannContraction(s, exprRef); err == nil {
			t.Fatal("expression target replayed as contraction record")
		}
		ghost := ComputeTestRef(t, "physmath", "riemann_contraction/1")
		if err := CheckRiemannContraction(s, ghost); err == nil {
			t.Fatal("unresolvable record accepted")
		}
	})

	t.Run("tampered-assembly-output", func(t *testing.T) {
		s, recRef, rec := mk(t)
		_, dpayload, err := s.Lookup(rec.DerivationRef)
		if err != nil {
			t.Fatal(err)
		}
		d, err := DecodeDerivation(dpayload)
		if err != nil {
			t.Fatal(err)
		}
		other := mustStoreExpr(t, s, MustRational(77, 1))
		d.Steps[len(d.Steps)-1].OutputRef = other
		draw, err := d.Encode()
		if err != nil {
			t.Fatal(err)
		}
		altDeriv, err := s.Put(FamilyDerivation, draw)
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperContractionRecord(t, s, recRef, func(r *RiemannContractionEvaluation) { r.DerivationRef = altDeriv })
		bothFail(t, s, bad, "mismatch")
	})

	t.Run("record-codec-gates", func(t *testing.T) {
		s, recRef, rec := mk(t)
		raw, err := rec.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeRiemannContractionEvaluation(raw); err != nil {
			t.Fatalf("record does not round-trip: %v", err)
		}
		if _, err := DecodeRiemannContractionEvaluation(raw[:len(raw)/2]); err == nil {
			t.Fatal("truncated record accepted")
		}
		if _, err := DecodeRiemannContractionEvaluation(append(append([]byte{}, raw...), 0x00)); err == nil {
			t.Fatal("trailing bytes accepted")
		}
		if err := CheckRiemannContraction(s, recRef); err != nil {
			t.Fatalf("valid record rejected: %v", err)
		}
	})
}

// riemannContractionRecord resolves a riemann_contraction/1 record.
func riemannContractionRecord(t *testing.T, s Store, ref artifact.ArtifactRef) RiemannContractionEvaluation {
	t.Helper()
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyRiemannContraction {
		t.Fatalf("record is %s/%s, not riemann_contraction/1", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeRiemannContractionEvaluation(payload)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// mustFlatDet returns the determinant reference of a stored metric.
func mustFlatDet(t *testing.T, s Store, metric artifact.ArtifactRef) artifact.ArtifactRef {
	t.Helper()
	detRef, _, err := GeneralDeterminant(s, metric)
	if err != nil {
		t.Fatal(err)
	}
	return detRef
}

// TestRiemannScalarAdversarial proves corrupted scalar evidence fails Check
// and artifact Replay at the intended boundary, including the exact
// condition-chain binding.
func TestRiemannScalarAdversarial(t *testing.T) {
	mk := func(t *testing.T) (*MemStore, artifact.ArtifactRef, RiemannScalarEvaluation) {
		s := NewMemStore()
		chart, metric, fn, mapping, _, _, cond := polarSetup(t, s)
		_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, []artifact.ArtifactRef{cond})
		if err != nil {
			t.Fatal(err)
		}
		_, crec, _, err := EvaluateRiemannContraction(s, riemRec)
		if err != nil {
			t.Fatal(err)
		}
		_, recRef, _, err := EvaluateRiemannScalar(s, crec)
		if err != nil {
			t.Fatal(err)
		}
		return s, recRef, scalarRecord(t, s, recRef)
	}
	bothFail := func(t *testing.T, s *MemStore, bad artifact.ArtifactRef, reason string) {
		t.Helper()
		if err := CheckRiemannScalar(s, bad); err == nil {
			t.Fatal("corrupted record accepted by Check")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("Check wrong boundary (want %q): %v", reason, err)
		}
		if err := ReplayRiemannScalar(s, bad); err == nil {
			t.Fatal("corrupted record accepted by artifact replay")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("replay wrong boundary (want %q): %v", reason, err)
		}
	}

	t.Run("substituted-contraction", func(t *testing.T) {
		s, recRef, _ := mk(t)
		s2 := NewMemStore()
		c2, m2, f2, mp2, _, _, cd2 := curvedSetup(t, s2)
		_, rr2, _, err := EvaluateRiemann(s2, m2, c2, mp2, f2, []artifact.ArtifactRef{cd2})
		if err != nil {
			t.Fatal(err)
		}
		_, cr2, _, err := EvaluateRiemannContraction(s2, rr2)
		if err != nil {
			t.Fatal(err)
		}
		_, rp, err := s2.Lookup(cr2)
		if err != nil {
			t.Fatal(err)
		}
		crCopy, err := s.Put(FamilyRiemannContraction, rp)
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperScalarRecord(t, s, recRef, func(r *RiemannScalarEvaluation) { r.ContractionRef = crCopy })
		bothFail(t, s, bad, "unresolvable")
	})

	t.Run("substituted-metric-chain", func(t *testing.T) {
		s, recRef, rec := mk(t)
		one := mustStoreExpr(t, s, MustRational(1, 1))
		zero := mustStoreExpr(t, s, MustRational(0, 1))
		flat, err := s.Put(FamilySymmetricBilinear, SymmetricBilinear{
			Dimension: 2, ComponentRefs: []artifact.ArtifactRef{one, zero, zero, one},
		}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		flatDet, _, err := GeneralDeterminant(s, flat)
		if err != nil {
			t.Fatal(err)
		}
		flatCond := detCond(t, s, flatDet)
		bad := tamperScalarRecord(t, s, recRef, func(r *RiemannScalarEvaluation) {
			r.MetricRef = flat
			r.ConditionRefs = []artifact.ArtifactRef{flatCond}
		})
		_ = rec
		// The chain-consistency gate fires first: the substituted metric
		// is not the metric of the bound Riemann record.
		bothFail(t, s, bad, "differs from riemann metric")
	})

	t.Run("substituted-det-inverse-evidence", func(t *testing.T) {
		s, recRef, rec := mk(t)
		other := mustStoreExpr(t, s, MustRational(11, 1))
		badDet := tamperScalarRecord(t, s, recRef, func(r *RiemannScalarEvaluation) { r.DetRef = other })
		bothFail(t, s, badDet, "mismatch")
		// Inverse tensor with swapped components.
		_, ipayload, err := s.Lookup(rec.InvTensorRef)
		if err != nil {
			t.Fatal(err)
		}
		gt, err := DecodeTensor(ipayload)
		if err != nil {
			t.Fatal(err)
		}
		swapped := append([]artifact.ArtifactRef{}, gt.ComponentRefs...)
		swapped[0], swapped[len(swapped)-1] = swapped[len(swapped)-1], swapped[0]
		badInv, err := s.Put(FamilyTensor, Tensor{Rank: 2, Dimension: 2, IndexRefs: gt.IndexRefs, ComponentRefs: swapped}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperScalarRecord(t, s, recRef, func(r *RiemannScalarEvaluation) { r.InvTensorRef = badInv })
		bothFail(t, s, bad, "mismatch")
		// Inverse derivation pointing at determinant evidence.
		badDeriv := tamperScalarRecord(t, s, recRef, func(r *RiemannScalarEvaluation) {
			r.InvDerivationRef = rec.DetDerivationRef
		})
		bothFail(t, s, badDeriv, "mismatch")
	})

	t.Run("substituted-conditions", func(t *testing.T) {
		s, recRef, _ := mk(t)
		empty := tamperScalarRecord(t, s, recRef, func(r *RiemannScalarEvaluation) { r.ConditionRefs = nil })
		bothFail(t, s, empty, "condition")
	})

	t.Run("singular-metric-at-scalar", func(t *testing.T) {
		s, recRef, _ := mk(t)
		one := mustStoreExpr(t, s, MustRational(1, 1))
		zero := mustStoreExpr(t, s, MustRational(0, 1))
		sing, err := s.Put(FamilySymmetricBilinear, SymmetricBilinear{
			Dimension: 2, ComponentRefs: []artifact.ArtifactRef{one, zero, zero, zero},
		}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperScalarRecord(t, s, recRef, func(r *RiemannScalarEvaluation) { r.MetricRef = sing })
		bothFail(t, s, bad, "differs from riemann metric")
	})

	t.Run("substituted-result-tampered-step", func(t *testing.T) {
		s, recRef, rec := mk(t)
		other := mustStoreExpr(t, s, MustRational(13, 1))
		bad := tamperScalarRecord(t, s, recRef, func(r *RiemannScalarEvaluation) { r.ResultRef = other })
		bothFail(t, s, bad, "does not match")
		_, dpayload, err := s.Lookup(rec.DerivationRef)
		if err != nil {
			t.Fatal(err)
		}
		d, err := DecodeDerivation(dpayload)
		if err != nil {
			t.Fatal(err)
		}
		d.Steps[len(d.Steps)-1].OutputRef = other
		draw, err := d.Encode()
		if err != nil {
			t.Fatal(err)
		}
		altDeriv, err := s.Put(FamilyDerivation, draw)
		if err != nil {
			t.Fatal(err)
		}
		badDeriv := tamperScalarRecord(t, s, recRef, func(r *RiemannScalarEvaluation) { r.DerivationRef = altDeriv })
		bothFail(t, s, badDeriv, "does not match")
	})

	t.Run("wrong-family-ghost-codec", func(t *testing.T) {
		s, _, rec := mk(t)
		exprRef := mustStoreExpr(t, s, MustRational(1, 1))
		if err := CheckRiemannScalar(s, exprRef); err == nil {
			t.Fatal("expression target accepted as scalar record")
		}
		if err := ReplayRiemannScalar(s, exprRef); err == nil {
			t.Fatal("expression target replayed as scalar record")
		}
		ghost := ComputeTestRef(t, "physmath", "riemann_scalar/1")
		if err := CheckRiemannScalar(s, ghost); err == nil {
			t.Fatal("unresolvable record accepted")
		}
		raw, err := rec.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeRiemannScalarEvaluation(raw); err != nil {
			t.Fatalf("record does not round-trip: %v", err)
		}
		if _, err := DecodeRiemannScalarEvaluation(raw[:len(raw)/2]); err == nil {
			t.Fatal("truncated record accepted")
		}
		if _, err := DecodeRiemannScalarEvaluation(append(append([]byte{}, raw...), 0x00)); err == nil {
			t.Fatal("trailing bytes accepted")
		}
		dup := rec
		dup.ConditionRefs = []artifact.ArtifactRef{rec.ConditionRefs[0], rec.ConditionRefs[0]}
		if err := dup.Validate(); err == nil {
			t.Fatal("duplicate-condition record validated")
		}
	})
}
