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
