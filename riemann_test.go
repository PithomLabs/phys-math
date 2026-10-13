package physmath

import (
	"errors"
	"math/big"
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

// prodSetup builds the 3D product chart (x,y,z) with metric
// diag(1,(1+x^2)^2,1), mapping, declaring function, and determinant
// condition. Block-diagonal decoupling makes z-index connection and
// curvature vanish while the x-y block reproduces the 2D curved values.
func prodSetup(t *testing.T, s Store) (chart, metric, fn artifact.ArtifactRef, mapping CoordParamMap, x, y, z Expr, cond artifact.ArtifactRef) {
	t.Helper()
	px := DeriveParameterID(0)
	py := DeriveParameterID(1)
	pz := DeriveParameterID(2)
	x = NewBoundSymbol("x", px)
	y = NewBoundSymbol("y", py)
	z = NewBoundSymbol("z", pz)
	desig := mustStoreExpr(t, s, NewEphemeralSymbol("prod_space"))
	spaceRaw, err := Collection{ElementRefs: []artifact.ArtifactRef{desig}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	space, err := s.Put(FamilyCollection, spaceRaw)
	if err != nil {
		t.Fatal(err)
	}
	cs := CoordinateSystem{Dimension: 3, CoordinateRefs: []artifact.ArtifactRef{
		mustStoreExpr(t, s, x), mustStoreExpr(t, s, y), mustStoreExpr(t, s, z),
	}, SpaceRef: space}
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
	m := SymmetricBilinear{Dimension: 3, ComponentRefs: []artifact.ArtifactRef{
		one, zero, zero,
		zero, gyy, zero,
		zero, zero, one,
	}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	metric, err = s.Put(FamilySymmetricBilinear, m.Encode())
	if err != nil {
		t.Fatal(err)
	}
	fnObj := Function{ParameterIDs: []ParameterID{px, py, pz},
		ParamNames: []string{"x", "y", "z"},
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
		{DeclaringArtifactRef: fn, ParameterID: pz},
	}}
	detRef, _, err := GeneralDeterminant(s, metric)
	if err != nil {
		t.Fatal(err)
	}
	cond = detCond(t, s, detRef)
	return chart, metric, fn, mapping, x, y, z, cond
}

// traceReverseComps resolves the ordered trace-reversed components.
func traceReverseComps(t *testing.T, s Store, resultField artifact.ArtifactRef) []Expr {
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

// dumpStructured renders an expression as an indented tree for failure
// messages and golden transcription. Test aid only.
func dumpStructured(e Expr, ind string) string {
	s := ""
	switch e.Kind {
	case TagRational:
		return ind + "Q(" + e.Numerator.ToBig().String() + "/" + e.Denominator.ToBig().String() + ")"
	case TagSymbol:
		return ind + "S(" + e.DisplayName + ")"
	case TagAdd:
		s = ind + "Add[\n"
	case TagMul:
		s = ind + "Mul[\n"
	case TagNeg:
		return ind + "Neg(\n" + dumpStructured(*e.Operand, ind+"  ") + "\n" + ind + ")"
	case TagPow:
		return ind + "Pow(\n" + dumpStructured(*e.Base, ind+"  ") + "\n" + dumpStructured(*e.Exp, ind+"  ") + "\n" + ind + ")"
	default:
		return ind + "?()"
	}
	for _, c := range e.Children {
		s += dumpStructured(c, ind+"  ") + "\n"
	}
	return s + ind + "]"
}

// TestRiemannTraceReverseTwoDim pins the 2D consistency golden: the curved
// 2D Einstein tensor vanishes identically (trace identity), with factored
// forms for the diagonal components and exact zeros off-diagonal. Point
// values: every component 0 at x=0, 1, 2.
func TestRiemannTraceReverseTwoDim(t *testing.T) {
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
	_, srec, _, err := EvaluateRiemannScalar(s, crec)
	if err != nil {
		t.Fatal(err)
	}
	result, recRef, _, err := EvaluateRiemannTraceReverse(s, crec, srec)
	if err != nil {
		t.Fatalf("2d trace-reverse failed: %v", err)
	}
	comps := traceReverseComps(t, s, result)
	if len(comps) != 4 {
		t.Fatalf("2d trace-reverse has %d components, want 4", len(comps))
	}
	zero := MustRational(0, 1)
	for _, i := range []int{1, 2} {
		if !comps[i].Equal(zero) {
			t.Fatalf("2d G[%d] is not exactly zero:\n%s", i, showExpr(comps[i]))
		}
	}
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne := MustRational(-1, 1)
	f := Add(MustRational(1, 1), Pow(x, MustRational(2, 1)))
	bigF := Pow(cloneExpr(f), MustRational(2, 1))
	bigFi := Pow(cloneExpr(bigF), MustRational(-1, 1))
	bigFi2 := Pow(cloneExpr(bigF), MustRational(-2, 1))
	// Ricci blocks (same math as the contraction golden).
	aInner := Add(Mul(two, two, cloneExpr(f)), Mul(x, x, two, two, two))
	a := Mul(half, cloneExpr(aInner), cloneExpr(bigFi))
	b := Mul(x, x, half, two, two, two, two, negOne, cloneExpr(f), cloneExpr(f), cloneExpr(bigFi2))
	c := Mul(x, x, half, half, two, two, two, two, cloneExpr(f), cloneExpr(f), cloneExpr(bigFi), cloneExpr(bigFi))
	q0 := Add(Neg(Add(cloneExpr(a), cloneExpr(b))), Neg(cloneExpr(c)))
	nInner := Add(Mul(two, two, cloneExpr(f)), Mul(x, x, two, two, two))
	n1 := Mul(half, Neg(cloneExpr(nInner)), cloneExpr(bigF), cloneExpr(bigFi))
	n2 := Mul(x, half, two, two, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigFi))
	n3 := Mul(x, half, two, two, negOne, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigF), cloneExpr(bigFi2))
	v := Mul(x, half, half, two, two, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigF), cloneExpr(bigFi), cloneExpr(bigFi))
	q3 := Add(n1, n2, n3, cloneExpr(v), Neg(cloneExpr(v)), Neg(cloneExpr(v)))
	// Scalar blocks: R = g^xx R_xx + g^yy R_yy with adjugate-form inverses.
	rForm := Add(Mul(Mul(cloneExpr(bigF), cloneExpr(bigFi)), cloneExpr(q0)), Mul(cloneExpr(bigFi), cloneExpr(q3)))
	// G_xx = R_xx - (1/2) R . 1 with the unit diagonal stripped by arity.
	// Point: x=0 -> -2+2 = 0; x=1 -> -1+1 = 0; x=2 -> -2/5+2/5 = 0.
	want0, err := Canonicalize(Add(cloneExpr(q0), Neg(Mul(half, cloneExpr(rForm)))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[0]); err != nil || !got.Equal(want0) {
		t.Fatalf("2d G[0] is not the hand-derived form:\n%s", showExpr(comps[0]))
	}
	// G_yy = R_yy - (1/2) R F. Point: x=0 -> -2+2 = 0; x=1 -> -4+4 = 0.
	want3, err := Canonicalize(Add(cloneExpr(n1), cloneExpr(n2), cloneExpr(n3),
		cloneExpr(v), Neg(cloneExpr(v)), Neg(cloneExpr(v)),
		Neg(Mul(half, cloneExpr(rForm), cloneExpr(bigF)))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[3]); err != nil || !got.Equal(want3) {
		t.Fatalf("2d G[3] is not the hand-derived form:\n%s", showExpr(comps[3]))
	}
	assertTraceReverseShape(t, s, result, chart, 2)
	if err := CheckRiemannTraceReverse(s, recRef); err != nil {
		t.Fatalf("2d trace-reverse rejected by Check: %v", err)
	}
	if err := ReplayRiemannTraceReverse(s, recRef); err != nil {
		t.Fatalf("2d trace-reverse rejected by artifact replay: %v", err)
	}
}

// assertTraceReverseShape verifies rank, dimension, chart, slot variance and
// space of a trace-reversed result field.
func assertTraceReverseShape(t *testing.T, s Store, resultField, chart artifact.ArtifactRef, dim int) {
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
		t.Fatal("trace-reverse result chart not preserved")
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
		t.Fatalf("trace-reverse result shape wrong: rank %d dim %d comps %d", rt.Rank, rt.Dimension, len(rt.ComponentRefs))
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
		t.Fatal("trace-reverse result index count wrong")
	}
	for i, r := range rt.IndexRefs {
		idx, err := resolveIndex(s, "test slot", r)
		if err != nil {
			t.Fatal(err)
		}
		if idx.Variance != VarianceCovariant {
			t.Fatalf("trace-reverse slot %d variance wrong", i)
		}
		if idx.Space == nil || *idx.Space != cs.SpaceRef {
			t.Fatalf("trace-reverse slot %d space mismatch", i)
		}
	}
}

// TestRiemannTraceReverseThreeDim pins the decisive nonzero golden for
// ds^2 = dx^2 + (1+x^2)^2 dy^2 + dz^2 with f = 1+x^2, F = f^2.
// Hand derivation: z-index connection and curvature vanish by block-diagonal
// decoupling, so the x-y block reproduces the 2D values while
// G_zz = 0 - (1/2)(-4/f)(1) = 2/(1+x^2):
// point x=0 -> 2, x=1 -> 1, x=2 -> 2/5. All other components vanish
// (G_xx, G_yy by the 2D trace identity; off-diagonals exactly).
func TestRiemannTraceReverseThreeDim(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, x, _, _, cond := prodSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	_, crec, _, err := EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatal(err)
	}
	_, srec, _, err := EvaluateRiemannScalar(s, crec)
	if err != nil {
		t.Fatal(err)
	}
	result, recRef, _, err := EvaluateRiemannTraceReverse(s, crec, srec)
	if err != nil {
		t.Fatalf("3d trace-reverse failed: %v", err)
	}
	comps := traceReverseComps(t, s, result)
	if len(comps) != 9 {
		t.Fatalf("3d trace-reverse has %d components, want 9", len(comps))
	}
	// Flat position (mu,nu) = (2,2) is index 8: the G_zz slot.
	if got := 2*3 + 2; got != 8 {
		t.Fatalf("index ordering broken: (2,2) maps to %d, want 8", got)
	}
	zero := MustRational(0, 1)
	for _, i := range []int{1, 2, 3, 5, 6, 7} {
		if !comps[i].Equal(zero) {
			t.Fatalf("3d G[%d] is not exactly zero:\n%s", i, showExpr(comps[i]))
		}
	}
	half, two := MustRational(1, 2), MustRational(2, 1)
	negOne := MustRational(-1, 1)
	f := Add(MustRational(1, 1), Pow(x, MustRational(2, 1)))
	bigF := Pow(cloneExpr(f), MustRational(2, 1))
	bigFi := Pow(cloneExpr(bigF), MustRational(-1, 1))
	bigFi2 := Pow(cloneExpr(bigF), MustRational(-2, 1))
	// Ricci x-y blocks (same math as the 2D contraction golden).
	aInner := Add(Mul(two, two, cloneExpr(f)), Mul(x, x, two, two, two))
	a := Mul(half, cloneExpr(aInner), cloneExpr(bigFi))
	b := Mul(x, x, half, two, two, two, two, negOne, cloneExpr(f), cloneExpr(f), cloneExpr(bigFi2))
	c := Mul(x, x, half, half, two, two, two, two, cloneExpr(f), cloneExpr(f), cloneExpr(bigFi), cloneExpr(bigFi))
	q0 := Add(Neg(Add(cloneExpr(a), cloneExpr(b))), Neg(cloneExpr(c)))
	nInner := Add(Mul(two, two, cloneExpr(f)), Mul(x, x, two, two, two))
	n1 := Mul(half, Neg(cloneExpr(nInner)), cloneExpr(bigF), cloneExpr(bigFi))
	n2 := Mul(x, half, two, two, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigFi))
	n3 := Mul(x, half, two, two, negOne, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigF), cloneExpr(bigFi2))
	v := Mul(x, half, half, two, two, cloneExpr(f), Neg(Mul(x, two, two, cloneExpr(f))), cloneExpr(bigF), cloneExpr(bigFi), cloneExpr(bigFi))
	q3 := Add(n1, n2, n3, cloneExpr(v), Neg(cloneExpr(v)), Neg(cloneExpr(v)))
	// Scalar block: R = g^xx R_xx + g^yy R_yy with adjugate-form inverses.
	rForm := Add(Mul(Mul(cloneExpr(bigF), cloneExpr(bigFi)), cloneExpr(q0)), Mul(cloneExpr(bigFi), cloneExpr(q3)))
	// G_xx = R_xx - (1/2) R . 1 (unit diagonal stripped by arity).
	// Point: x=0 -> -2+2 = 0; x=1 -> -1+1 = 0; x=2 -> -2/5+2/5 = 0.
	want0, err := Canonicalize(Add(cloneExpr(q0), Neg(Mul(half, cloneExpr(rForm)))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[0]); err != nil || !got.Equal(want0) {
		t.Fatalf("3d G[0] is not the hand-derived form:\n%s", showExpr(comps[0]))
	}
	// G_yy = R_yy - (1/2) R F. Point: x=0 -> -2+2 = 0; x=1 -> -4+4 = 0.
	want4, err := Canonicalize(Add(cloneExpr(n1), cloneExpr(n2), cloneExpr(n3),
		cloneExpr(v), Neg(cloneExpr(v)), Neg(cloneExpr(v)),
		Neg(Mul(half, cloneExpr(rForm), cloneExpr(bigF)))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[4]); err != nil || !got.Equal(want4) {
		t.Fatalf("3d G[4] is not the hand-derived form:\n%s", showExpr(comps[4]))
	}
	// G_zz = 0 - (1/2) R . 1 = 2/(1+x^2): the decisive nonzero component.
	// Point: x=0 -> 2, x=1 -> 1, x=2 -> 2/5.
	want8, err := Canonicalize(Neg(Mul(half, cloneExpr(rForm))))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(comps[8]); err != nil || !got.Equal(want8) {
		t.Fatalf("3d G[8] is not the hand-derived 2/(1+x^2) form:\n%s", showExpr(comps[8]))
	}
	if comps[8].Equal(zero) {
		t.Fatal("3d G[8] collapsed to zero: nonzero golden lost")
	}
	assertTraceReverseShape(t, s, result, chart, 3)
	if err := CheckRiemannTraceReverse(s, recRef); err != nil {
		t.Fatalf("3d trace-reverse rejected by Check: %v", err)
	}
	if err := ReplayRiemannTraceReverse(s, recRef); err != nil {
		t.Fatalf("3d trace-reverse rejected by artifact replay: %v", err)
	}
}

// TestTraceReverseDecoupling proves block-diagonal decoupling byte-for-byte:
// the 3D connection x-y block reproduces the 2D curved coefficients, every
// z-index coefficient is exactly zero, and every z-containing Riemann
// component is exactly zero.
func TestTraceReverseDecoupling(t *testing.T) {
	s := NewMemStore()
	chart3, metric3, fn3, mapping3, _, _, _, cond3 := prodSetup(t, s)
	conds3 := []artifact.ArtifactRef{cond3}
	conn3, _, err := LeviCivita(s, metric3, chart3, mapping3, fn3, conds3)
	if err != nil {
		t.Fatal(err)
	}
	_, cpayload3, err := s.Lookup(conn3)
	if err != nil {
		t.Fatal(err)
	}
	cc3, err := DecodeAffineConnection(cpayload3)
	if err != nil {
		t.Fatal(err)
	}
	chart2, metric2, fn2, mapping2, _, _, cond2 := curvedSetup(t, s)
	conds2 := []artifact.ArtifactRef{cond2}
	conn2, _, err := LeviCivita(s, metric2, chart2, mapping2, fn2, conds2)
	if err != nil {
		t.Fatal(err)
	}
	_, cpayload2, err := s.Lookup(conn2)
	if err != nil {
		t.Fatal(err)
	}
	cc2, err := DecodeAffineConnection(cpayload2)
	if err != nil {
		t.Fatal(err)
	}
	// x-y block coefficients match the 2D run exactly; all z-index slots
	// are exactly zero.
	zero := MustRational(0, 1)
	for rho := 0; rho < 3; rho++ {
		for mu := 0; mu < 3; mu++ {
			for nu := 0; nu < 3; nu++ {
				got := mustResolve(t, s, cc3.CoefficientRefs[(rho*3+mu)*3+nu])
				if rho < 2 && mu < 2 && nu < 2 {
					want := mustResolve(t, s, cc2.CoefficientRefs[(rho*2+mu)*2+nu])
					if !got.Equal(want) {
						t.Fatalf("3d Gamma^%d_%d%d differs from 2D:\n%s", rho, mu, nu, showExpr(got))
					}
					continue
				}
				if !got.Equal(zero) {
					t.Fatalf("3d Gamma^%d_%d%d is not exactly zero:\n%s", rho, mu, nu, showExpr(got))
				}
			}
		}
	}
	// Every z-containing Riemann component is exactly zero.
	res3, _, _, err := EvaluateRiemann(s, metric3, chart3, mapping3, fn3, conds3)
	if err != nil {
		t.Fatal(err)
	}
	rcomps := riemannComps(t, s, res3)
	for _, i := range riemannZIndices(3) {
		if got := rcomps[i]; !got.Equal(zero) {
			rho, rem := i/27, i%27
			sigma, rem := rem/9, rem%9
			mu, nu := rem/3, rem%3
			t.Fatalf("3d R[%d] (%d,%d,%d,%d) is not exactly zero:\n%s", i, rho, sigma, mu, nu, showExpr(got))
		}
	}
}

// riemannZIndices returns the flat indices of rank-4 components with at
// least one slot equal to dim-1 (the decoupled direction).
func riemannZIndices(dim int) []int {
	var out []int
	for rho := 0; rho < dim; rho++ {
		for sigma := 0; sigma < dim; sigma++ {
			for mu := 0; mu < dim; mu++ {
				for nu := 0; nu < dim; nu++ {
					if rho == dim-1 || sigma == dim-1 || mu == dim-1 || nu == dim-1 {
						out = append(out, ((rho*dim+sigma)*dim+mu)*dim+nu)
					}
				}
			}
		}
	}
	return out
}

// tamperTraceReverseRecord decodes a trace-reverse record, applies mutate,
// and re-mints it as a new artifact.
func tamperTraceReverseRecord(t *testing.T, s Store, ref artifact.ArtifactRef, mutate func(*RiemannTraceReverseEvaluation)) artifact.ArtifactRef {
	t.Helper()
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyRiemannTraceReverse {
		t.Fatalf("record is %s/%s, not riemann_trace_reverse/1", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeRiemannTraceReverseEvaluation(payload)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&rec)
	raw, err := rec.Encode()
	if err != nil {
		t.Fatalf("tampered record does not encode: %v", err)
	}
	bad, err := s.Put(FamilyRiemannTraceReverse, raw)
	if err != nil {
		t.Fatal(err)
	}
	return bad
}

// traceReverseFixture builds the 2D curved chain through the scalar stage.
func traceReverseFixture(t *testing.T, s Store) (cf, sf artifact.ArtifactRef, crec RiemannContractionEvaluation, srec RiemannScalarEvaluation) {
	t.Helper()
	chart, metric, fn, mapping, _, _, cond := curvedSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	_, crecRef, _, err := EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatal(err)
	}
	_, srecRef, _, err := EvaluateRiemannScalar(s, crecRef)
	if err != nil {
		t.Fatal(err)
	}
	return crecRef, srecRef, riemannContractionRecord(t, s, crecRef), scalarRecord(t, s, srecRef)
}

// TestRiemannTraceReverseAdversarial proves corrupted trace-reversed evidence
// fails Check and artifact Replay at the intended boundary.
func TestRiemannTraceReverseAdversarial(t *testing.T) {
	mk := func(t *testing.T) (*MemStore, artifact.ArtifactRef, RiemannTraceReverseEvaluation) {
		s := NewMemStore()
		cf, sf, _, _ := traceReverseFixture(t, s)
		res, recRef, _, err := EvaluateRiemannTraceReverse(s, cf, sf)
		if err != nil {
			t.Fatal(err)
		}
		_ = res
		return s, recRef, traceReverseRecord(t, s, recRef)
	}
	bothFail := func(t *testing.T, s *MemStore, bad artifact.ArtifactRef, reason string) {
		t.Helper()
		if err := CheckRiemannTraceReverse(s, bad); err == nil {
			t.Fatal("corrupted record accepted by Check")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("Check wrong boundary (want %q): %v", reason, err)
		}
		if err := ReplayRiemannTraceReverse(s, bad); err == nil {
			t.Fatal("corrupted record accepted by artifact replay")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("replay wrong boundary (want %q): %v", reason, err)
		}
	}

	t.Run("mismatched-scalar-pair", func(t *testing.T) {
		s, recRef, rec := mk(t)
		// A second scalar chain over another metric in the same store.
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
		_, fpayload, err := s.Lookup(rec.ChartRef)
		if err != nil {
			t.Fatal(err)
		}
		_ = fpayload
		chart, metric, fn, mapping, _, _, _ := polarSetup(t, s)
		_ = chart
		_ = metric
		_ = fn
		_ = mapping
		// Build an independent flat-metric chain on the polar chart.
		pchart, _, pfn, pmap, _, _, _ := polarSetup(t, s)
		_ = pchart
		_ = pfn
		_ = pmap
		_, flatRiem, _, err := EvaluateRiemann(s, flat, pchart, pmap, pfn, []artifact.ArtifactRef{flatCond})
		if err != nil {
			t.Fatal(err)
		}
		_, flatCont, _, err := EvaluateRiemannContraction(s, flatRiem)
		if err != nil {
			t.Fatal(err)
		}
		_, flatScalar, _, err := EvaluateRiemannScalar(s, flatCont)
		if err != nil {
			t.Fatal(err)
		}
		// Scalar bound to another contraction: pair linkage rejects it.
		bad := tamperTraceReverseRecord(t, s, recRef, func(r *RiemannTraceReverseEvaluation) {
			r.ScalarRef = flatScalar
		})
		bothFail(t, s, bad, "another contraction")
	})

	t.Run("substituted-sources", func(t *testing.T) {
		s, recRef, rec := mk(t)
		ghostCont := ComputeTestRef(t, "physmath", "riemann_contraction/1")
		bad := tamperTraceReverseRecord(t, s, recRef, func(r *RiemannTraceReverseEvaluation) { r.ContractionRef = ghostCont })
		bothFail(t, s, bad, "unresolvable")
		one := mustStoreExpr(t, s, MustRational(1, 1))
		zero := mustStoreExpr(t, s, MustRational(0, 1))
		flat, err := s.Put(FamilySymmetricBilinear, SymmetricBilinear{
			Dimension: 2, ComponentRefs: []artifact.ArtifactRef{one, zero, zero, one},
		}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		badMetric := tamperTraceReverseRecord(t, s, recRef, func(r *RiemannTraceReverseEvaluation) { r.MetricRef = flat })
		bothFail(t, s, badMetric, "differs from")
		_ = rec
		ghostChart := ComputeTestRef(t, "physmath", "coordinate_system/1")
		badChart := tamperTraceReverseRecord(t, s, recRef, func(r *RiemannTraceReverseEvaluation) { r.ChartRef = ghostChart })
		bothFail(t, s, badChart, "unresolvable")
	})

	t.Run("reordered-components-wrong-slots", func(t *testing.T) {
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
		bad := tamperTraceReverseRecord(t, s, recRef, func(r *RiemannTraceReverseEvaluation) { r.ResultRef = badField })
		bothFail(t, s, bad, "mismatch")
		// Wrong slot variance: contravariant first slot.
		_, cpayload, err := s.Lookup(rec.ChartRef)
		if err != nil {
			t.Fatal(err)
		}
		cs, err := DecodeCoordinateSystem(cpayload)
		if err != nil {
			t.Fatal(err)
		}
		upPayload, err := Index{Label: "rt0", Variance: VarianceContravariant, Space: &cs.SpaceRef}.Encode()
		if err != nil {
			t.Fatal(err)
		}
		upRef, err := s.Put(FamilyIndex, upPayload)
		if err != nil {
			t.Fatal(err)
		}
		wrongIdx := []artifact.ArtifactRef{upRef, rt.IndexRefs[1]}
		wrongTensor, err := s.Put(FamilyTensor, Tensor{Rank: 2, Dimension: 2, IndexRefs: wrongIdx, ComponentRefs: rt.ComponentRefs}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		wrongField, err := s.Put(FamilyTensorField, TensorField{TensorRef: wrongTensor, CoordinateSystemRef: rf.CoordinateSystemRef}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		badSlots := tamperTraceReverseRecord(t, s, recRef, func(r *RiemannTraceReverseEvaluation) { r.ResultRef = wrongField })
		bothFail(t, s, badSlots, "identity broken")
	})

	t.Run("tampered-step-ghost-codec-dim", func(t *testing.T) {
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
		bad := tamperTraceReverseRecord(t, s, recRef, func(r *RiemannTraceReverseEvaluation) { r.DerivationRef = altDeriv })
		bothFail(t, s, bad, "output mismatch")
		exprRef := mustStoreExpr(t, s, MustRational(1, 1))
		if err := CheckRiemannTraceReverse(s, exprRef); err == nil {
			t.Fatal("expression target accepted as trace-reverse record")
		}
		if err := ReplayRiemannTraceReverse(s, exprRef); err == nil {
			t.Fatal("expression target replayed as trace-reverse record")
		}
		ghost := ComputeTestRef(t, "physmath", "riemann_trace_reverse/1")
		if err := CheckRiemannTraceReverse(s, ghost); err == nil {
			t.Fatal("unresolvable record accepted")
		}
		raw, err := rec.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeRiemannTraceReverseEvaluation(raw); err != nil {
			t.Fatalf("record does not round-trip: %v", err)
		}
		if _, err := DecodeRiemannTraceReverseEvaluation(raw[:len(raw)/2]); err == nil {
			t.Fatal("truncated record accepted")
		}
		if _, err := DecodeRiemannTraceReverseEvaluation(append(append([]byte{}, raw...), 0x00)); err == nil {
			t.Fatal("trailing bytes accepted")
		}
	})

	t.Run("dim-mismatch-3d-scalar", func(t *testing.T) {
		s, recRef, _ := mk(t)
		// A 3D scalar record cannot back this 2D result: substitute its
		// reference and require rejection at the dimension boundary.
		s3 := NewMemStore()
		c3, m3, f3, mp3, _, _, _, cd3 := prodSetup(t, s3)
		_, rr3, _, err := EvaluateRiemann(s3, m3, c3, mp3, f3, []artifact.ArtifactRef{cd3})
		if err != nil {
			t.Fatal(err)
		}
		_, cr3, _, err := EvaluateRiemannContraction(s3, rr3)
		if err != nil {
			t.Fatal(err)
		}
		_, sc3, _, err := EvaluateRiemannScalar(s3, cr3)
		if err != nil {
			t.Fatal(err)
		}
		_, sp, err := s3.Lookup(sc3)
		if err != nil {
			t.Fatal(err)
		}
		scCopy, err := s.Put(FamilyRiemannScalar, sp)
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperTraceReverseRecord(t, s, recRef, func(r *RiemannTraceReverseEvaluation) { r.ScalarRef = scCopy })
		// Dependencies of the foreign record do not resolve here.
		if err := CheckRiemannTraceReverse(s, bad); err == nil {
			t.Fatal("foreign-dimension scalar accepted")
		}
		if err := ReplayRiemannTraceReverse(s, bad); err == nil {
			t.Fatal("foreign-dimension scalar replayed")
		}
	})
}

// traceReverseRecord resolves a riemann_trace_reverse/1 record.
func traceReverseRecord(t *testing.T, s Store, ref artifact.ArtifactRef) RiemannTraceReverseEvaluation {
	t.Helper()
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyRiemannTraceReverse {
		t.Fatalf("record is %s/%s, not riemann_trace_reverse/1", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeRiemannTraceReverseEvaluation(payload)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// warpedSetup builds the nontrivial warped 3D chart (x,y,z) with metric
// diag(1,f^2,f^2), f = 1+x^2, plus mapping, declaring function, and
// determinant condition. Unlike the block-diagonal product metric, the y-z
// plane carries genuine warping curvature (R^y_zyz != 0), so the divergence
// cancellation is nontrivial.
func warpedSetup(t *testing.T, s Store) (chart, metric, fn artifact.ArtifactRef, mapping CoordParamMap, x, y, z Expr, cond artifact.ArtifactRef) {
	t.Helper()
	px := DeriveParameterID(0)
	py := DeriveParameterID(1)
	pz := DeriveParameterID(2)
	x = NewBoundSymbol("x", px)
	y = NewBoundSymbol("y", py)
	z = NewBoundSymbol("z", pz)
	desig := mustStoreExpr(t, s, NewEphemeralSymbol("warped_space"))
	spaceRaw, err := Collection{ElementRefs: []artifact.ArtifactRef{desig}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	space, err := s.Put(FamilyCollection, spaceRaw)
	if err != nil {
		t.Fatal(err)
	}
	cs := CoordinateSystem{Dimension: 3, CoordinateRefs: []artifact.ArtifactRef{
		mustStoreExpr(t, s, x), mustStoreExpr(t, s, y), mustStoreExpr(t, s, z),
	}, SpaceRef: space}
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
	m := SymmetricBilinear{Dimension: 3, ComponentRefs: []artifact.ArtifactRef{
		one, zero, zero,
		zero, gyy, zero,
		zero, zero, gyy,
	}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	metric, err = s.Put(FamilySymmetricBilinear, m.Encode())
	if err != nil {
		t.Fatal(err)
	}
	fnObj := Function{ParameterIDs: []ParameterID{px, py, pz},
		ParamNames: []string{"x", "y", "z"},
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
		{DeclaringArtifactRef: fn, ParameterID: pz},
	}}
	detRef, _, err := GeneralDeterminant(s, metric)
	if err != nil {
		t.Fatal(err)
	}
	cond = detCond(t, s, detRef)
	return chart, metric, fn, mapping, x, y, z, cond
}

func TestDivergenceWarped(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, x, _, _, cond := warpedSetup(t, s)
	px := DeriveParameterID(0)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	_, crec, _, err := EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatal(err)
	}
	_, srec, _, err := EvaluateRiemannScalar(s, crec)
	if err != nil {
		t.Fatal(err)
	}
	_, trec, _, err := EvaluateRiemannTraceReverse(s, crec, srec)
	if err != nil {
		t.Fatal(err)
	}
	result, recRef, _, err := EvaluateRiemannTraceReverseDivergence(s, trec)
	if err != nil {
		t.Fatalf("warped divergence failed: %v", err)
	}
	comps := divergenceComps(t, s, result)
	if len(comps) != 3 {
		t.Fatalf("warped divergence has %d components, want 3", len(comps))
	}
	zero := MustRational(0, 1)
	// y/z routes vanish: every route carries a zero partial, a zero
	// connection coefficient, or an exact-zero inverse factor.
	for _, i := range []int{1, 2} {
		if !comps[i].Equal(zero) {
			t.Fatalf("warped D[%d] is not exactly zero:\n%s", i, showExpr(comps[i]))
		}
	}
	rec := divergenceRecord(t, s, recRef)
	// Small byte pins (connection, inverse diagonal, determinant) plus
	// mirror-slot sharing and exact-zero gates. Values below are
	// machine-checked in pinWarpedValues.
	pinWarpedAtoms(t, s, rec, x, px)
	// W[0]: Add of exactly three diagonal Mul pairings (off-diagonal inverse
	// factors are exact zeros and strip). Canon sorts children, so pairings
	// are matched by content multiset, never position.
	if comps[0].Kind != TagAdd || len(comps[0].Children) != 3 {
		t.Fatalf("warped D[0] is not a three-term sum:\n%s", showExpr(comps[0]))
	}
	invComps := tensorComps(t, s, rec.InvTensorRef)
	hessComps := tensorComps(t, s, mustTensorOf(t, s, rec.CovariantResultRef))
	assertPairMultiset(t, "warped D[0] pairing", comps[0].Children,
		[][2]Expr{{invComps[0], hessComps[0]}, {invComps[4], hessComps[12]}, {invComps[8], hessComps[24]}})
	// Nontrivial intermediates: routes and the source G_xx are genuine
	// nonzero forms (rules out a constant-zero pipeline).
	for _, k := range []int{0, 12, 24} {
		if hessComps[k].Equal(zero) {
			t.Fatalf("warped H-route %d is literal zero: nontrivial computation lost", k)
		}
	}
	gComps := tensorComps(t, s, mustTensorOf(t, s, mustFieldTensor(t, s, trec)))
	if gComps[0].Equal(zero) {
		t.Fatal("warped G_xx input is literal zero: golden vacuous")
	}
	// Machine-checked values for every hand-arithmetic input (see the table
	// in pinWarpedValues): connection, Ricci, inverse, determinant, scalar,
	// trace-reverse, routes, and the divergence itself.
	pinWarpedValues(t, s, rec, trec, px)
	// Independent task-formula recomputation: dG_xx/dx via the guarded
	// API on the recorded G_xx plus hand-built 2(f'/f)G_xx - 2(f'/f^3)G_yy
	// terms, assembled and normalized through the same rule pipeline. The
	// formula comes from the task statement, not the implementation; only
	// resolved recorded references enter as operands.
	taskRecomputeX(t, s, rec, trec, x)
	// Machine-checked zero at x=0: substitution + canon + simplify closes
	// the factored sum to literal zero through sound rules only.
	if got := substituteZero(t, s, comps[0], fn); !got.Equal(zero) {
		t.Fatalf("warped D[0] at x=0 is not zero:\n%s", showExpr(got))
	}
	// Assembly wiring: the nu=0 premise carries 9 products in (lambda,mu)
	// order with exact inverse/covariant operand references.
	assertDivergenceWiring(t, s, recRef, 0, invComps, hessComps)
	if err := CheckRiemannTraceReverseDivergence(s, recRef); err != nil {
		t.Fatalf("warped divergence rejected by Check: %v", err)
	}
	if err := ReplayRiemannTraceReverseDivergence(s, recRef); err != nil {
		t.Fatalf("warped divergence rejected by artifact replay: %v", err)
	}
}

// divergenceComps resolves the ordered divergence components.
func divergenceComps(t *testing.T, s Store, resultField artifact.ArtifactRef) []Expr {
	t.Helper()
	return tensorComps(t, s, mustTensorOf(t, s, resultField))
}

// tensorComps resolves ordered tensor components.
func tensorComps(t *testing.T, s Store, tensorRef artifact.ArtifactRef) []Expr {
	t.Helper()
	_, tpayload, err := s.Lookup(tensorRef)
	if err != nil {
		t.Fatal(err)
	}
	tn, err := DecodeTensor(tpayload)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]Expr, len(tn.ComponentRefs))
	for i, r := range tn.ComponentRefs {
		out[i] = mustResolve(t, s, r)
	}
	return out
}

// mustTensorOf resolves a tensor field to its tensor reference.
func mustTensorOf(t *testing.T, s Store, fieldRef artifact.ArtifactRef) artifact.ArtifactRef {
	t.Helper()
	_, fpayload, err := s.Lookup(fieldRef)
	if err != nil {
		t.Fatal(err)
	}
	tf, err := DecodeTensorField(fpayload)
	if err != nil {
		t.Fatal(err)
	}
	return tf.TensorRef
}

// mustFieldTensor resolves a trace-reverse record's result field.
func mustFieldTensor(t *testing.T, s Store, trecRef artifact.ArtifactRef) artifact.ArtifactRef {
	t.Helper()
	fam, payload, err := s.Lookup(trecRef)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyRiemannTraceReverse {
		t.Fatalf("not a trace-reverse record: %s/%s", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeRiemannTraceReverseEvaluation(payload)
	if err != nil {
		t.Fatal(err)
	}
	return rec.ResultRef
}

// mustCompRef resolves one trace-reverse result component reference.
func mustCompRef(t *testing.T, s Store, trecRef artifact.ArtifactRef, idx int) artifact.ArtifactRef {
	t.Helper()
	_, fpayload, err := s.Lookup(mustFieldTensor(t, s, trecRef))
	if err != nil {
		t.Fatal(err)
	}
	tf, err := DecodeTensorField(fpayload)
	if err != nil {
		t.Fatal(err)
	}
	_, tpayload, err := s.Lookup(tf.TensorRef)
	if err != nil {
		t.Fatal(err)
	}
	tn, err := DecodeTensor(tpayload)
	if err != nil {
		t.Fatal(err)
	}
	return tn.ComponentRefs[idx]
}

// substituteZero substitutes x := 0 into an expression and simplifies: the
// machine-checked point evaluation used where factored forms cannot close
// by canonicalization alone.
func substituteZero(t *testing.T, s Store, e Expr, fn artifact.ArtifactRef) Expr {
	t.Helper()
	target := mustStoreExpr(t, s, e)
	zeroRef := mustStoreExpr(t, s, MustRational(0, 1))
	return substituteZeroVia(t, s, target, fn, zeroRef)
}

func substituteZeroVia(t *testing.T, s Store, target, fn, zeroRef artifact.ArtifactRef) Expr {
	t.Helper()
	entry := append(BoundParameterRef{fn, DeriveParameterID(0)}.Encode(), zeroRef.Encode()...)
	params := append(artifact.EncodeUInt32(1), entry...)
	sub, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprSubstitute, InputRefs: []artifact.ArtifactRef{target, fn}, Parameters: params})
	if err != nil {
		t.Fatalf("substitute: %v", err)
	}
	canon, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprCanon, InputRefs: []artifact.ArtifactRef{sub.OutputRef}})
	if err != nil {
		t.Fatalf("canon: %v", err)
	}
	simp, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprSimplify, InputRefs: []artifact.ArtifactRef{canon.OutputRef}})
	if err != nil {
		t.Fatalf("simplify: %v", err)
	}
	return mustResolve(t, s, simp.OutputRef)
}

// assertDivergenceWiring verifies the nu-th assembly premise carries 9
// products in (lambda,mu) order with exact inverse/covariant operand refs.
func assertDivergenceWiring(t *testing.T, s Store, recRef artifact.ArtifactRef, nu int, invComps, hessComps []Expr) {
	t.Helper()
	rec := divergenceRecord(t, s, recRef)
	_, dpayload, err := s.Lookup(rec.DerivationRef)
	if err != nil {
		t.Fatal(err)
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		t.Fatal(err)
	}
	// Assembly pairs are the final 2*dim steps in nu order.
	dim := 3
	if len(d.Steps) < 2*dim {
		t.Fatal("derivation too short for assembly pairs")
	}
	base := len(d.Steps) - 2*dim + 2*nu
	canonStep := d.Steps[base]
	if canonStep.RuleID != RuleExprCanon || len(canonStep.InputRefs) != 1 {
		t.Fatalf("nu=%d assembly canon step malformed", nu)
	}
	premise := mustResolve(t, s, canonStep.InputRefs[0])
	if premise.Kind != TagAdd || len(premise.Children) != dim*dim {
		t.Fatalf("nu=%d premise has %d children, want %d", nu, len(premise.Children), dim*dim)
	}
	k := 0
	for lam := 0; lam < dim; lam++ {
		for mu := 0; mu < dim; mu++ {
			term := premise.Children[k]
			k++
			if term.Kind != TagMul || len(term.Children) != 2 {
				t.Fatalf("nu=%d term (%d,%d) is not a binary product", nu, lam, mu)
			}
			gotG, err := term.Children[0].Encode()
			if err != nil {
				t.Fatal(err)
			}
			wantG, err := invComps[lam*dim+mu].Encode()
			if err != nil {
				t.Fatal(err)
			}
			if string(gotG) != string(wantG) {
				t.Fatalf("nu=%d term (%d,%d) pairs the wrong inverse component", nu, lam, mu)
			}
			gotH, err := term.Children[1].Encode()
			if err != nil {
				t.Fatal(err)
			}
			wantH, err := hessComps[(lam*dim+mu)*dim+nu].Encode()
			if err != nil {
				t.Fatal(err)
			}
			if string(gotH) != string(wantH) {
				t.Fatalf("nu=%d term (%d,%d) pairs the wrong derivative route", nu, lam, mu)
			}
		}
	}
}

// divergenceRecord resolves a divergence record for inspection.
func divergenceRecord(t *testing.T, s Store, ref artifact.ArtifactRef) RiemannTraceReverseDivergenceEvaluation {
	t.Helper()
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyRiemannTraceReverseDivergence {
		t.Fatalf("record is %s/%s, not divergence", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeRiemannTraceReverseDivergenceEvaluation(payload)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// evalAtRational evaluates a closed single-variable expression at an integer
// point using exact big-rational arithmetic. It substitutes the target PID,
// folds Add/Mul/Neg exactly, and evaluates integer-exponent powers (negative
// via reciprocal, failing closed on zero base or zero denominator). Unknown
// symbols and non-executable nodes fail closed: a supplemental oracle per
// Lesson 2, never a replacement for structural and provenance tests.
func evalAtRational(t *testing.T, e Expr, pid ParameterID, value int64) *big.Rat {
	t.Helper()
	v := big.NewRat(value, 1)
	var walk func(x Expr) *big.Rat
	walk = func(x Expr) *big.Rat {
		switch x.Kind {
		case TagRational:
			return new(big.Rat).SetFrac(x.Numerator.ToBig(), x.Denominator.ToBig())
		case TagSymbol:
			if x.ParamID == nil || *x.ParamID != pid {
				t.Fatalf("eval oracle hit foreign symbol %q", x.DisplayName)
			}
			return new(big.Rat).Set(v)
		case TagAdd:
			acc := new(big.Rat)
			for _, c := range x.Children {
				acc.Add(acc, walk(c))
			}
			return acc
		case TagMul:
			acc := big.NewRat(1, 1)
			for _, c := range x.Children {
				acc.Mul(acc, walk(c))
			}
			return acc
		case TagNeg:
			return new(big.Rat).Neg(walk(*x.Operand))
		case TagPow:
			base, exp := walk(*x.Base), walk(*x.Exp)
			if !exp.IsInt() {
				t.Fatalf("eval oracle hit non-integer exponent %v", exp)
			}
			n := exp.Num().Int64()
			if n < 0 && base.Sign() == 0 {
				t.Fatalf("eval oracle hit zero base with negative exponent")
			}
			pow := func(z *big.Int, m int64) *big.Int {
				if m < 0 {
					m = -m
				}
				return new(big.Int).Exp(z, big.NewInt(m), nil)
			}
			numPow, denPow := pow(base.Num(), n), pow(base.Denom(), n)
			if n >= 0 {
				return new(big.Rat).SetFrac(numPow, denPow)
			}
			return new(big.Rat).SetFrac(denPow, numPow)
		default:
			t.Fatalf("eval oracle hit unsupported node kind %d", x.Kind)
			return nil
		}
	}
	return walk(e)
}

// assertEvalValue requires an expression to evaluate to want at x=value.
func assertEvalValue(t *testing.T, e Expr, pid ParameterID, x, num, den int64) {
	t.Helper()
	got := evalAtRational(t, e, pid, x)
	want := big.NewRat(num, den)
	if got.Cmp(want) != 0 {
		t.Fatalf("eval at x=%d: got %v, want %v", x, got, want)
	}
}

// pinWarpedAtoms pins small byte-exact forms (connection, inverse diagonal,
// determinant) and byte-identical mirror slots. Larger forms are verified by
// value (pinWarpedValues), structure, wiring, and recomputation instead.
func pinWarpedAtoms(t *testing.T, s Store, rec RiemannTraceReverseDivergenceEvaluation, x Expr, px ParameterID) {
	t.Helper()
	half, two := MustRational(1, 2), MustRational(2, 1)
	f := Add(MustRational(1, 1), Pow(x, MustRational(2, 1)))
	bigF := Pow(cloneExpr(f), MustRational(2, 1))
	twoX := Mul(x, two)
	// Gamma^x_yy = (1/2)(-(2(2x)f)) F F (F F)^-1. Point: 0, -4, -20.
	gxxy, err := Canonicalize(Mul(half, Neg(Mul(two, cloneExpr(twoX), cloneExpr(f))), cloneExpr(bigF), cloneExpr(bigF), Pow(Mul(cloneExpr(bigF), cloneExpr(bigF)), MustRational(-1, 1))))
	if err != nil {
		t.Fatal(err)
	}
	// Gamma^y_xy = (1/2)(2 f 2x) F (F F)^-1. Point: 0, 1, 4/5.
	gyxy, err := Canonicalize(Mul(half, Mul(two, cloneExpr(f), cloneExpr(twoX)), cloneExpr(bigF), Pow(Mul(cloneExpr(bigF), cloneExpr(bigF)), MustRational(-1, 1))))
	if err != nil {
		t.Fatal(err)
	}
	_, connP, err := s.Lookup(rec.ConnectionRef)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := DecodeAffineConnection(connP)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(mustResolve(t, s, cc.CoefficientRefs[4])); err != nil || !got.Equal(gxxy) {
		t.Fatalf("warped Gamma^x_yy mismatch:\n%s", showExpr(mustResolve(t, s, cc.CoefficientRefs[4])))
	}
	if got, err := Canonicalize(mustResolve(t, s, cc.CoefficientRefs[10])); err != nil || !got.Equal(gyxy) {
		t.Fatalf("warped Gamma^y_xy mismatch:\n%s", showExpr(mustResolve(t, s, cc.CoefficientRefs[10])))
	}
	// Mirror slots share byte-identical references (x-only content makes
	// y<->z symmetric pairs coincide, not merely agree in value).
	for _, pair := range [][2]int{{12, 10}, {24, 20}, {8, 4}} {
		if cc.CoefficientRefs[pair[0]] != cc.CoefficientRefs[pair[1]] {
			t.Fatalf("warped Gamma slots %d and %d do not share bytes", pair[0], pair[1])
		}
	}
	// Gamma^. _xx slots are exactly zero (used by the hand arithmetic).
	zero := MustRational(0, 1)
	for _, k := range []int{0, 3, 6} {
		if got := mustResolve(t, s, cc.CoefficientRefs[k]); !got.Equal(zero) {
			t.Fatalf("warped Gamma slot %d is not exactly zero:\n%s", k, showExpr(got))
		}
	}
	// Inverse diagonal pins: g^xx = F.(F F)^-1 (value 1), g^yy = F.(F F)^-1
	// with the cofactor form (value 1/f^2). Off-diagonals are exactly zero.
	_, ginvP, err := s.Lookup(rec.InvTensorRef)
	if err != nil {
		t.Fatal(err)
	}
	ginv, err := DecodeTensor(ginvP)
	if err != nil {
		t.Fatal(err)
	}
	// Adjugate forms with the 3D determinant: g^xx = F.F.(F.F)^-1 (value
	// 1 wherever defined), g^yy = F.(F.F)^-1 (value 1/f^2).
	detForm := Mul(cloneExpr(bigF), cloneExpr(bigF))
	detInv := Pow(cloneExpr(detForm), MustRational(-1, 1))
	gxxWant, err := Canonicalize(Mul(cloneExpr(detForm), cloneExpr(detInv)))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(mustResolve(t, s, ginv.ComponentRefs[0])); err != nil || !got.Equal(gxxWant) {
		t.Fatalf("warped g^xx mismatch:\n%s", showExpr(mustResolve(t, s, ginv.ComponentRefs[0])))
	}
	gyyWant, err := Canonicalize(Mul(cloneExpr(bigF), cloneExpr(detInv)))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Canonicalize(mustResolve(t, s, ginv.ComponentRefs[4])); err != nil || !got.Equal(gyyWant) {
		t.Fatalf("warped g^yy mismatch:\n%s", showExpr(mustResolve(t, s, ginv.ComponentRefs[4])))
	}
	if got, err := Canonicalize(mustResolve(t, s, ginv.ComponentRefs[8])); err != nil || !got.Equal(gyyWant) {
		t.Fatalf("warped g^zz mismatch:\n%s", showExpr(mustResolve(t, s, ginv.ComponentRefs[8])))
	}
	for _, k := range []int{1, 2, 3, 5, 6, 7} {
		if got := mustResolve(t, s, ginv.ComponentRefs[k]); !got.Equal(zero) {
			t.Fatalf("warped inverse [%d] is not exactly zero:\n%s", k, showExpr(got))
		}
	}
}

// pinWarpedValues asserts machine-checked exact values at x = 0, 1, 2 for
// every quantity entering the hand arithmetic, via the independent rational
// oracle. Hand values: f 1,2,5; Gamma^x_yy 0,-4,-20; Gamma^y_xy 0,1,4/5;
// R_xx -4,-2,-4/5; R_yy -2,-8,-26; det 1,16,625; g^xx 1,1,1; g^yy 1,1/4,1/25;
// R -8,-6,-72/25; G_xx 0,1,16/25; G_yy 2,4,10; H_xx 0,0,-48/125;
// H_yy 0,0,24/5; W 0,0,0.
func pinWarpedValues(t *testing.T, s Store, rec RiemannTraceReverseDivergenceEvaluation, trecRef artifact.ArtifactRef, px ParameterID) {
	t.Helper()
	trec := traceReverseRecord(t, s, trecRef)
	crec := riemannContractionRecord(t, s, trec.ContractionRef)
	srec := scalarRecord(t, s, trec.ScalarRef)
	val := func(e Expr, x, num, den int64) {
		t.Helper()
		assertEvalValue(t, e, px, x, num, den)
	}
	// Connection values from the recorded connection.
	_, connP, err := s.Lookup(rec.ConnectionRef)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := DecodeAffineConnection(connP)
	if err != nil {
		t.Fatal(err)
	}
	gxxy := mustResolve(t, s, cc.CoefficientRefs[4])
	gyxy := mustResolve(t, s, cc.CoefficientRefs[10])
	for _, x := range []int64{0, 1, 2} {
		_ = x
	}
	val(gxxy, 0, 0, 1)
	val(gxxy, 1, -4, 1)
	val(gxxy, 2, -20, 1)
	val(gyxy, 0, 0, 1)
	val(gyxy, 1, 1, 1)
	val(gyxy, 2, 4, 5)
	// Ricci values from the recorded contraction result.
	_, cfp, err := s.Lookup(crec.ResultRef)
	if err != nil {
		t.Fatal(err)
	}
	ctf, err := DecodeTensorField(cfp)
	if err != nil {
		t.Fatal(err)
	}
	_, ctp, err := s.Lookup(ctf.TensorRef)
	if err != nil {
		t.Fatal(err)
	}
	ctn, err := DecodeTensor(ctp)
	if err != nil {
		t.Fatal(err)
	}
	rxx := mustResolve(t, s, ctn.ComponentRefs[0])
	ryy := mustResolve(t, s, ctn.ComponentRefs[4])
	rzz := mustResolve(t, s, ctn.ComponentRefs[8])
	if ctn.ComponentRefs[8] != ctn.ComponentRefs[4] {
		t.Fatal("warped R_zz does not share R_yy bytes")
	}
	val(rxx, 0, -4, 1)
	val(rxx, 1, -2, 1)
	val(rxx, 2, -4, 5)
	val(ryy, 0, -2, 1)
	val(ryy, 1, -8, 1)
	val(ryy, 2, -26, 1)
	val(rzz, 0, -2, 1)
	val(rzz, 1, -8, 1)
	val(rzz, 2, -26, 1)
	// Inverse + determinant values.
	_, ginvP, err := s.Lookup(rec.InvTensorRef)
	if err != nil {
		t.Fatal(err)
	}
	ginv, err := DecodeTensor(ginvP)
	if err != nil {
		t.Fatal(err)
	}
	gxx := mustResolve(t, s, ginv.ComponentRefs[0])
	gyy := mustResolve(t, s, ginv.ComponentRefs[4])
	val(gxx, 0, 1, 1)
	val(gxx, 1, 1, 1)
	val(gxx, 2, 1, 1)
	val(gyy, 0, 1, 1)
	val(gyy, 1, 1, 4)
	val(gyy, 2, 1, 25)
	// Scalar value.
	scalar := mustResolve(t, s, srec.ResultRef)
	val(scalar, 0, -8, 1)
	val(scalar, 1, -6, 1)
	val(scalar, 2, -72, 25)
	// G values from the trace-reverse result.
	_, gfp, err := s.Lookup(trec.ResultRef)
	if err != nil {
		t.Fatal(err)
	}
	gtf, err := DecodeTensorField(gfp)
	if err != nil {
		t.Fatal(err)
	}
	_, gtp, err := s.Lookup(gtf.TensorRef)
	if err != nil {
		t.Fatal(err)
	}
	gtn, err := DecodeTensor(gtp)
	if err != nil {
		t.Fatal(err)
	}
	gxxq := mustResolve(t, s, gtn.ComponentRefs[0])
	gyyq := mustResolve(t, s, gtn.ComponentRefs[4])
	val(gxxq, 0, 0, 1)
	val(gxxq, 1, 1, 1)
	val(gxxq, 2, 16, 25)
	val(gyyq, 0, 2, 1)
	val(gyyq, 1, 4, 1)
	val(gyyq, 2, 10, 1)
	// H-route values from the recorded covariant result.
	_, covP, err := s.Lookup(rec.CovariantResultRef)
	if err != nil {
		t.Fatal(err)
	}
	covTF, err := DecodeTensorField(covP)
	if err != nil {
		t.Fatal(err)
	}
	_, covTP, err := s.Lookup(covTF.TensorRef)
	if err != nil {
		t.Fatal(err)
	}
	covT, err := DecodeTensor(covTP)
	if err != nil {
		t.Fatal(err)
	}
	hxx := mustResolve(t, s, covT.ComponentRefs[0])
	hyy := mustResolve(t, s, covT.ComponentRefs[12])
	hzz := mustResolve(t, s, covT.ComponentRefs[24])
	for _, h := range []Expr{hxx, hyy, hzz} {
		if h.Equal(MustRational(0, 1)) {
			t.Fatal("warped H-route is literal zero: nontrivial computation lost")
		}
	}
	val(hxx, 0, 0, 1)
	val(hxx, 1, 0, 1)
	val(hxx, 2, -48, 125)
	val(hyy, 0, 0, 1)
	val(hyy, 1, 0, 1)
	val(hyy, 2, 24, 5)
	val(hzz, 0, 0, 1)
	val(hzz, 1, 0, 1)
	val(hzz, 2, 24, 5)
}

// assertPairMultiset requires a bijective canon-normalized match between
// stored children and expected (ginv, route) pairs. Canon sorts and flattens
// same-kind nesting on both sides, so association churn and child order
// never matter; atom multisets and operator-kind structure do.
func assertPairMultiset(t *testing.T, what string, stored []Expr, pairs [][2]Expr) {
	t.Helper()
	if len(stored) != len(pairs) {
		t.Fatalf("%s has %d children, want %d", what, len(stored), len(pairs))
	}
	used := make([]bool, len(stored))
	for _, pr := range pairs {
		want, err := Canonicalize(Mul(cloneExpr(pr[0]), cloneExpr(pr[1])))
		if err != nil {
			t.Fatal(err)
		}
		found := -1
		for j, s := range stored {
			if used[j] {
				continue
			}
			got, err := Canonicalize(s)
			if err != nil {
				t.Fatal(err)
			}
			if got.Equal(want) {
				found = j
				used[j] = true
				break
			}
		}
		if found < 0 {
			t.Fatalf("%s has no child matching pair:\n%s", what, showExpr(want))
		}
	}
}

// taskRecomputeX independently re-derives the x-divergence value pieces: the
// x partial of G_xx through the guarded API plus hand-built 2(f'/f)G_xx and
// -2(f'/f^3)G_yy terms from the task formula (not from the implementation's
// assembly). Each piece's exact values are asserted below; the exact rational
// total is checked here. Hand values: dG (0,0,-48/125); t1 (0,2,128/125);
// t2 (0,-2,-80/125); totals 0, 0, 0.
func taskRecomputeX(t *testing.T, s Store, rec RiemannTraceReverseDivergenceEvaluation, trec artifact.ArtifactRef, x Expr) {
	t.Helper()
	two := MustRational(2, 1)
	px := DeriveParameterID(0)
	gxxRef := mustCompRef(t, s, trec, 0)
	gyyRef := mustCompRef(t, s, trec, 4)
	dGres, _, _, err := DifferentiateCoordinate(s, gxxRef, rec.ChartRef, 0, rec.Mapping, rec.DeclaringFnRef, 1, rec.ConditionRefs)
	if err != nil {
		t.Fatalf("task dG partial failed: %v", err)
	}
	dG := mustResolve(t, s, dGres)
	f := Add(MustRational(1, 1), Pow(cloneExpr(x), MustRational(2, 1)))
	fp := Mul(two, cloneExpr(x))
	fi := Pow(cloneExpr(f), MustRational(-1, 1))
	gxx := mustResolve(t, s, gxxRef)
	gyy := mustResolve(t, s, gyyRef)
	t1 := Mul(two, Mul(cloneExpr(fp), cloneExpr(fi)), cloneExpr(gxx))
	t2 := Neg(Mul(two, Mul(cloneExpr(fp), cloneExpr(fi), cloneExpr(fi), cloneExpr(fi)), cloneExpr(gyy)))
	for _, xv := range []int64{0, 1, 2} {
		total := new(big.Rat).Add(new(big.Rat).Add(
			evalAtRational(t, dG, px, xv), evalAtRational(t, t1, px, xv)),
			evalAtRational(t, t2, px, xv))
		if total.Sign() != 0 {
			t.Fatalf("task-formula total at x=%d is %v, want 0", xv, total)
		}
	}
	assertEvalValue(t, dG, px, 2, -48, 125)
	assertEvalValue(t, t1, px, 1, 2, 1)
	assertEvalValue(t, t2, px, 1, -2, 1)
}

// TestDivergenceProduct is the Case-A regression golden for
// ds^2 = dr^2 + r^2 dtheta^2 + dz^2 (flat product: all curvature values
// vanish, nonzero connection). D_y and D_z are exactly zero; D_x is a
// two-term factored sum (rr and thth diagonal routes; the zz route strips
// exactly). This is a regression case, not the sole correctness golden.
func TestDivergenceProduct(t *testing.T) {
	s := NewMemStore()
	chart, metric, fn, mapping, _, _, _, cond := prodSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	_, crec, _, err := EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatal(err)
	}
	_, srec, _, err := EvaluateRiemannScalar(s, crec)
	if err != nil {
		t.Fatal(err)
	}
	_, trec, _, err := EvaluateRiemannTraceReverse(s, crec, srec)
	if err != nil {
		t.Fatal(err)
	}
	result, recRef, _, err := EvaluateRiemannTraceReverseDivergence(s, trec)
	if err != nil {
		t.Fatalf("product divergence failed: %v", err)
	}
	comps := divergenceComps(t, s, result)
	if len(comps) != 3 {
		t.Fatalf("product divergence has %d components, want 3", len(comps))
	}
	zero := MustRational(0, 1)
	// y/z routes vanish: zero partials against zero products throughout.
	for _, i := range []int{1, 2} {
		if !comps[i].Equal(zero) {
			t.Fatalf("product D[%d] is not exactly zero:\n%s", i, showExpr(comps[i]))
		}
	}
	rec := divergenceRecord(t, s, recRef)
	invComps := tensorComps(t, s, rec.InvTensorRef)
	hessComps := tensorComps(t, s, mustTensorOf(t, s, rec.CovariantResultRef))
	// D_x pairs the rr and thth diagonal routes (the zz route strips
	// exactly since its derivative route is literal zero).
	if comps[0].Kind != TagAdd || len(comps[0].Children) != 2 {
		t.Fatalf("product D[0] is not a two-term sum:\n%s", showExpr(comps[0]))
	}
	assertPairMultiset(t, "product D[0] pairing",
		comps[0].Children,
		[][2]Expr{{invComps[0], hessComps[0]}, {invComps[4], hessComps[12]}})
	for _, k := range []int{0, 12} {
		if hessComps[k].Equal(zero) {
			t.Fatalf("product H-route %d is literal zero: nontrivial computation lost", k)
		}
	}
	// G_zz is the nonzero input: G_zz = -R/2 with R = -4/f gives 2/f.
	// Point: x=0 -> 2, x=1 -> 1, x=2 -> 2/5. (Coordinates here are x,y,z
	// with f = 1+x^2; there is no polar singularity in this fixture.)
	gzz := mustResolve(t, s, mustCompRef(t, s, trec, 8))
	px := DeriveParameterID(0)
	assertEvalValue(t, gzz, px, 0, 2, 1)
	assertEvalValue(t, gzz, px, 1, 1, 1)
	assertEvalValue(t, gzz, px, 2, 2, 5)
	if gzz.Equal(zero) {
		t.Fatal("product G_zz is literal zero: nonzero input lost")
	}
	// Ricci and scalar values pin the inputs feeding the divergence:
	// R_xx = -2/f: -2,-1,-2/5; R_yy = -2f: -2,-4,-10; R_zz exact zero;
	// R = -4/f: -4,-2,-4/5.
	tric := riemannContractionRecord(t, s, crec)
	tricComps := tensorComps(t, s, mustTensorOf(t, s, tric.ResultRef))
	assertEvalValue(t, tricComps[0], px, 0, -2, 1)
	assertEvalValue(t, tricComps[0], px, 1, -1, 1)
	assertEvalValue(t, tricComps[0], px, 2, -2, 5)
	assertEvalValue(t, tricComps[4], px, 0, -2, 1)
	assertEvalValue(t, tricComps[4], px, 1, -4, 1)
	assertEvalValue(t, tricComps[4], px, 2, -10, 1)
	if !tricComps[8].Equal(zero) {
		t.Fatalf("product R_zz is not exactly zero:\n%s", showExpr(tricComps[8]))
	}
	srec2 := scalarRecord(t, s, srec)
	scalar := mustResolve(t, s, srec2.ResultRef)
	assertEvalValue(t, scalar, px, 0, -4, 1)
	assertEvalValue(t, scalar, px, 1, -2, 1)
	assertEvalValue(t, scalar, px, 2, -4, 5)
	if err := CheckRiemannTraceReverseDivergence(s, recRef); err != nil {
		t.Fatalf("product divergence rejected by Check: %v", err)
	}
	if err := ReplayRiemannTraceReverseDivergence(s, recRef); err != nil {
		t.Fatalf("product divergence rejected by artifact replay: %v", err)
	}
}

// tamperDivergenceRecord decodes a divergence record, applies mutate, and
// re-mints it as a new artifact.
func tamperDivergenceRecord(t *testing.T, s Store, ref artifact.ArtifactRef, mutate func(*RiemannTraceReverseDivergenceEvaluation)) artifact.ArtifactRef {
	t.Helper()
	rec := divergenceRecord(t, s, ref)
	mutate(&rec)
	raw, err := rec.Encode()
	if err != nil {
		t.Fatalf("tampered record does not encode: %v", err)
	}
	bad, err := s.Put(FamilyRiemannTraceReverseDivergence, raw)
	if err != nil {
		t.Fatal(err)
	}
	return bad
}

// divergenceChain builds the full warped chain through the divergence record.
func divergenceChain(t *testing.T, s Store) (df, drec, trec, crec, srec artifact.ArtifactRef) {
	t.Helper()
	chart, metric, fn, mapping, _, _, _, cond := warpedSetup(t, s)
	conds := []artifact.ArtifactRef{cond}
	_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
	if err != nil {
		t.Fatal(err)
	}
	_, crec, _, err = EvaluateRiemannContraction(s, riemRec)
	if err != nil {
		t.Fatal(err)
	}
	_, srec, _, err = EvaluateRiemannScalar(s, crec)
	if err != nil {
		t.Fatal(err)
	}
	_, trec, _, err = EvaluateRiemannTraceReverse(s, crec, srec)
	if err != nil {
		t.Fatal(err)
	}
	df, drec, _, err = EvaluateRiemannTraceReverseDivergence(s, trec)
	if err != nil {
		t.Fatal(err)
	}
	return df, drec, trec, crec, srec
}

// TestDivergenceAdversarial proves corrupted divergence evidence fails Check
// and artifact Replay at the intended boundary. Polar 2D chains cover the
// structural cases fast; warped chains cover math-sensitive ones.
func TestDivergenceAdversarial(t *testing.T) {
	polarChain := func(t *testing.T, s *MemStore) (artifact.ArtifactRef, artifact.ArtifactRef) {
		t.Helper()
		chart, metric, fn, mapping, _, _, cond := polarSetup(t, s)
		conds := []artifact.ArtifactRef{cond}
		_, riemRec, _, err := EvaluateRiemann(s, metric, chart, mapping, fn, conds)
		if err != nil {
			t.Fatal(err)
		}
		_, crec, _, err := EvaluateRiemannContraction(s, riemRec)
		if err != nil {
			t.Fatal(err)
		}
		_, srec, _, err := EvaluateRiemannScalar(s, crec)
		if err != nil {
			t.Fatal(err)
		}
		_, trec, _, err := EvaluateRiemannTraceReverse(s, crec, srec)
		if err != nil {
			t.Fatal(err)
		}
		df, drec, _, err := EvaluateRiemannTraceReverseDivergence(s, trec)
		if err != nil {
			t.Fatal(err)
		}
		return df, drec
	}
	bothFail := func(t *testing.T, s *MemStore, bad artifact.ArtifactRef, reason string) {
		t.Helper()
		if err := CheckRiemannTraceReverseDivergence(s, bad); err == nil {
			t.Fatal("corrupted record accepted by Check")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("Check wrong boundary (want %q): %v", reason, err)
		}
		if err := ReplayRiemannTraceReverseDivergence(s, bad); err == nil {
			t.Fatal("corrupted record accepted by artifact replay")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("replay wrong boundary (want %q): %v", reason, err)
		}
	}

	t.Run("foreign-metric-source", func(t *testing.T) {
		s := NewMemStore()
		_, drec := polarChain(t, s)
		// A foreign chain over another geometry: identical fixtures would
		// collide byte-for-byte under content addressing (correctly
		// accepted as the same artifact), so the foreign geometry differs.
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
		_, sr2, _, err := EvaluateRiemannScalar(s2, cr2)
		if err != nil {
			t.Fatal(err)
		}
		_, tr2, _, err := EvaluateRiemannTraceReverse(s2, cr2, sr2)
		if err != nil {
			t.Fatal(err)
		}
		_, drec2, _, err := EvaluateRiemannTraceReverseDivergence(s2, tr2)
		if err != nil {
			t.Fatal(err)
		}
		_, rp, err := s2.Lookup(drec2)
		if err != nil {
			t.Fatal(err)
		}
		// Re-mint foreign bytes locally: dependencies do not resolve here.
		fr, err := s.Put(FamilyRiemannTraceReverseDivergence, rp)
		if err != nil {
			t.Fatal(err)
		}
		bothFail(t, s, fr, "unresolvable")
		_ = drec
	})

	t.Run("mismatched-chart-mapping-connection-inverse", func(t *testing.T) {
		s := NewMemStore()
		_, drec := polarChain(t, s)
		ghostChart := ComputeTestRef(t, "physmath", "coordinate_system/1")
		bad := tamperDivergenceRecord(t, s, drec, func(r *RiemannTraceReverseDivergenceEvaluation) { r.ChartRef = ghostChart })
		bothFail(t, s, bad, "unresolvable")
		rec := divergenceRecord(t, s, drec)
		swapped := CoordParamMap{Entries: []BoundParameterRef{
			rec.Mapping.Entries[1], rec.Mapping.Entries[0],
		}}
		badMap := tamperDivergenceRecord(t, s, drec, func(r *RiemannTraceReverseDivergenceEvaluation) { r.Mapping = swapped })
		// The record-to-Riemann mapping-equality gate fires before
		// revalidation: the stored selection no longer matches its chart.
		bothFail(t, s, badMap, "mapping differs")
		ghostConn := ComputeTestRef(t, "physmath", "affine_connection/1")
		badConn := tamperDivergenceRecord(t, s, drec, func(r *RiemannTraceReverseDivergenceEvaluation) { r.ConnectionRef = ghostConn })
		bothFail(t, s, badConn, "unresolvable")
		zero := mustStoreExpr(t, s, MustRational(0, 1))
		badInv, err := s.Put(FamilyTensor, Tensor{Rank: 2, Dimension: 2, IndexRefs: []artifact.ArtifactRef{
			mustStoreIndex(t, s, "a", VarianceContravariant),
			mustStoreIndex(t, s, "b", VarianceContravariant),
		}, ComponentRefs: []artifact.ArtifactRef{zero, zero, zero, zero}}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		badInvRef := tamperDivergenceRecord(t, s, drec, func(r *RiemannTraceReverseDivergenceEvaluation) { r.InvTensorRef = badInv })
		// Reference equality with scalar evidence fires before recompute:
		// a foreign inverse cannot stand in for the recorded one.
		bothFail(t, s, badInvRef, "differs from scalar evidence")
	})

	t.Run("wrong-indices-reordered-result", func(t *testing.T) {
		s := NewMemStore()
		_, drec := polarChain(t, s)
		rec := divergenceRecord(t, s, drec)
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
		// Wrong slot variance: contravariant free slot.
		_, cpayload, err := s.Lookup(rec.ChartRef)
		if err != nil {
			t.Fatal(err)
		}
		cs, err := DecodeCoordinateSystem(cpayload)
		if err != nil {
			t.Fatal(err)
		}
		upPayload, err := Index{Label: "div", Variance: VarianceContravariant, Space: &cs.SpaceRef}.Encode()
		if err != nil {
			t.Fatal(err)
		}
		upRef, err := s.Put(FamilyIndex, upPayload)
		if err != nil {
			t.Fatal(err)
		}
		wrongTensor, err := s.Put(FamilyTensor, Tensor{Rank: 1, Dimension: 2, IndexRefs: []artifact.ArtifactRef{upRef}, ComponentRefs: rt.ComponentRefs}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		wrongField, err := s.Put(FamilyTensorField, TensorField{TensorRef: wrongTensor, CoordinateSystemRef: rf.CoordinateSystemRef}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperDivergenceRecord(t, s, drec, func(r *RiemannTraceReverseDivergenceEvaluation) { r.ResultRef = wrongField })
		// A substituted slot reference fails full-reference identity before
		// variance inspection; the variance/space gates below remain as
		// defense for non-content-addressed stores.
		bothFail(t, s, bad, "identity broken")
	})

	t.Run("substituted-result-tampered-step", func(t *testing.T) {
		s := NewMemStore()
		_, drec := polarChain(t, s)
		rec := divergenceRecord(t, s, drec)
		other := mustStoreExpr(t, s, MustRational(13, 1))
		// Substitute the result field with an expression artifact.
		bad := tamperDivergenceRecord(t, s, drec, func(r *RiemannTraceReverseDivergenceEvaluation) { r.ResultRef = other })
		bothFail(t, s, bad, "not in closed target set")
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
		badDeriv := tamperDivergenceRecord(t, s, drec, func(r *RiemannTraceReverseDivergenceEvaluation) { r.DerivationRef = altDeriv })
		bothFail(t, s, badDeriv, "output mismatch")
	})

	t.Run("tampered-conditions-ghost-codec", func(t *testing.T) {
		s := NewMemStore()
		_, drec := polarChain(t, s)
		rec := divergenceRecord(t, s, drec)
		empty := tamperDivergenceRecord(t, s, drec, func(r *RiemannTraceReverseDivergenceEvaluation) { r.ConditionRefs = nil })
		bothFail(t, s, empty, "condition")
		ghost := ComputeTestRef(t, "physmath", "riemann_trace_reverse_divergence/1")
		if err := CheckRiemannTraceReverseDivergence(s, ghost); err == nil {
			t.Fatal("unresolvable record accepted")
		}
		if err := ReplayRiemannTraceReverseDivergence(s, ghost); err == nil {
			t.Fatal("unresolvable record replayed")
		}
		exprRef := mustStoreExpr(t, s, MustRational(1, 1))
		if err := CheckRiemannTraceReverseDivergence(s, exprRef); err == nil {
			t.Fatal("expression target accepted")
		}
		raw, err := rec.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeRiemannTraceReverseDivergenceEvaluation(raw); err != nil {
			t.Fatalf("record does not round-trip: %v", err)
		}
		if _, err := DecodeRiemannTraceReverseDivergenceEvaluation(raw[:len(raw)/2]); err == nil {
			t.Fatal("truncated record accepted")
		}
		if _, err := DecodeRiemannTraceReverseDivergenceEvaluation(append(append([]byte{}, raw...), 0x00)); err == nil {
			t.Fatal("trailing bytes accepted")
		}
		dup := rec
		dup.ConditionRefs = []artifact.ArtifactRef{rec.ConditionRefs[0], rec.ConditionRefs[0]}
		if err := dup.Validate(); err == nil {
			t.Fatal("duplicate-condition record validated")
		}
		emptyMap := rec
		emptyMap.Mapping = CoordParamMap{}
		if err := emptyMap.Validate(); err == nil {
			t.Fatal("empty-mapping record validated")
		}
	})

	t.Run("reordered-covariant-evidence", func(t *testing.T) {
		s := NewMemStore()
		_, drec := polarChain(t, s)
		// Substitute the covariant evidence field with the result field:
		// rank and content mismatch the recorded rank-3 evidence.
		bad := tamperDivergenceRecord(t, s, drec, func(r *RiemannTraceReverseDivergenceEvaluation) { r.CovariantResultRef = r.ResultRef })
		bothFail(t, s, bad, "covariant shape wrong")
	})
}

// mustStoreIndex mints an index/1 artifact without a space reference check.
func mustStoreIndex(t *testing.T, s Store, label string, v IndexVariance) artifact.ArtifactRef {
	t.Helper()
	desig := mustStoreExpr(t, s, NewEphemeralSymbol("idx_space_"+label))
	raw, err := Collection{ElementRefs: []artifact.ArtifactRef{desig}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	space, err := s.Put(FamilyCollection, raw)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Index{Label: label, Variance: v, Space: &space}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyIndex, p)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}
