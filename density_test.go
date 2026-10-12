package physmath

import (
	"errors"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// diagMetric mints a diagonal metric from component expressions.
func diagMetric(t *testing.T, s Store, comps ...Expr) (artifact.ArtifactRef, SymmetricBilinear) {
	t.Helper()
	n := len(comps)
	if n < 1 || n > 8 {
		t.Fatalf("diagMetric needs 1-8 diagonal entries, got %d", len(comps))
	}
	zero := mustStoreExpr(t, s, MustRational(0, 1))
	refs := make([]artifact.ArtifactRef, n*n)
	for i := range refs {
		refs[i] = zero
	}
	for i, c := range comps {
		refs[i*n+i] = mustStoreExpr(t, s, c)
	}
	m := SymmetricBilinear{Dimension: uint16(n), ComponentRefs: refs, InverseRef: nil}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilySymmetricBilinear, m.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return ref, m
}

func detBytes(t *testing.T, s Store, ref artifact.ArtifactRef) []byte {
	t.Helper()
	fam, p, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyExpression {
		t.Fatalf("det is not expression/1: %v", fam)
	}
	return p
}

func TestDiagonalDeterminantMinkowski(t *testing.T) {
	s := NewMemStore()
	ref, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	detRef, d, err := DiagonalDeterminant(s, ref)
	if err != nil {
		t.Fatalf("Minkowski det failed: %v", err)
	}
	// det = -1 exactly, via the replayed CANON derivation.
	want, err := MustRational(-1, 1).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(detBytes(t, s, detRef)) != string(want) {
		t.Fatal("Minkowski determinant is not exactly -1")
	}
	// The derivation replays (single CANON step over the Mul premise).
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("det derivation does not replay: %v", err)
	}
	if len(d.Steps) != 2 || d.Steps[0].RuleID != RuleExprCanon || d.Steps[1].RuleID != RuleExprSimplify {
		t.Fatal("det derivation is not CANON+SIMPLIFY")
	}
}

func TestDiagonalDeterminantRindlerSymbolic(t *testing.T) {
	s := NewMemStore()
	xbar := NewEphemeralSymbol("xbar")
	negx2 := Neg(Pow(xbar, MustRational(2, 1)))
	ref, _ := diagMetric(t, s, negx2, MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	detRef, _, err := DiagonalDeterminant(s, ref)
	if err != nil {
		t.Fatalf("Rindler det failed: %v", err)
	}
	// Deterministic: identical inputs re-mint the identical reference.
	detRef2, _, err := DiagonalDeterminant(s, ref)
	if err != nil {
		t.Fatal(err)
	}
	if detRef != detRef2 {
		t.Fatal("determinant computation nondeterministic")
	}
	// Shape: canonical product mentioning xbar (no numeric claim made).
	e, err := DecodeExpr(artifact.NewDecoder(detBytes(t, s, detRef)))
	if err != nil {
		t.Fatal(err)
	}
	if e.Kind != TagMul && e.Kind != TagNeg {
		t.Fatalf("unexpected det shape: %v", e.Kind)
	}
}

func TestDiagonalDeterminantNegatives(t *testing.T) {
	s := NewMemStore()
	one := mustStoreExpr(t, s, MustRational(1, 1))
	two := mustStoreExpr(t, s, MustRational(2, 1))
	// Non-diagonal: off-diagonal nonzero.
	refs := []artifact.ArtifactRef{one, two, one, one}
	zero := mustStoreExpr(t, s, MustRational(0, 1))
	_ = zero
	_ = refs
	a, b := mustStoreExpr(t, s, MustRational(1, 1)), mustStoreExpr(t, s, MustRational(2, 1))
	nondiag := SymmetricBilinear{Dimension: 2,
		ComponentRefs: []artifact.ArtifactRef{a, b, one, one}, InverseRef: nil}
	if err := nondiag.Validate(); err != nil {
		t.Fatalf("shape rejected: %v", err)
	}
	ndRef, err := s.Put(FamilySymmetricBilinear, nondiag.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := DiagonalDeterminant(s, ndRef); err == nil {
		t.Fatal("non-diagonal metric determinant computed")
	}
	// Wrong family target.
	if _, _, err := DiagonalDeterminant(s, one); err == nil {
		t.Fatal("non-metric target accepted")
	}
	// Unresolvable component.
	ghost := nondiag
	ghost.ComponentRefs[0] = ComputeTestRef(t, "physmath", "expression/1")
	gr, err := s.Put(FamilySymmetricBilinear, ghost.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := DiagonalDeterminant(s, gr); err == nil {
		t.Fatal("unresolvable component accepted")
	}
	// Dropped dependency: derivation premise removed breaks replay.
	ref, _ := diagMetric(t, s, MustRational(1, 1), MustRational(1, 1))
	_, d, err := DiagonalDeterminant(s, ref)
	if err != nil {
		t.Fatal(err)
	}
	d.PremiseRefs = nil
	if err := Replay(s, d, nil); err == nil {
		t.Fatal("replay succeeded with dropped premise")
	}
}

func TestCheckSignature(t *testing.T) {
	s := NewMemStore()
	pos, _ := diagMetric(t, s, MustRational(1, 1), MustRational(2, 1))
	if err := CheckSignature(s, pos, SignaturePositiveDefinite); err != nil {
		t.Fatalf("positive-definite rejected: %v", err)
	}
	if err := CheckSignature(s, pos, SignatureLorentzian); err == nil {
		t.Fatal("all-positive metric accepted as Lorentzian")
	}
	lor, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	if err := CheckSignature(s, lor, SignatureLorentzian); err != nil {
		t.Fatalf("Lorentzian rejected: %v", err)
	}
	if err := CheckSignature(s, lor, SignaturePositiveDefinite); err == nil {
		t.Fatal("Lorentzian accepted as positive-definite")
	}
	// Two negatives: neither class.
	two, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(-2, 1))
	if err := CheckSignature(s, two, SignatureLorentzian); err == nil {
		t.Fatal("double-negative accepted as Lorentzian")
	}
	// Zero diagonal: degenerate, matches nothing.
	zero, _ := diagMetric(t, s, MustRational(0, 1), MustRational(1, 1))
	if err := CheckSignature(s, zero, SignaturePositiveDefinite); err == nil {
		t.Fatal("degenerate metric classified")
	}
	// Symbolic diagonal: uncertainty preserved via distinct sentinel.
	xbar := NewEphemeralSymbol("xbar")
	sym, _ := diagMetric(t, s, Neg(Pow(xbar, MustRational(2, 1))), MustRational(1, 1))
	if err := CheckSignature(s, sym, SignatureLorentzian); !errors.Is(err, ErrSignatureUnverifiable) {
		t.Fatalf("symbolic signature did not fail with unverifiable sentinel: %v", err)
	}
	// Invalid class string.
	if err := CheckSignature(s, pos, "RIEMANNIAN"); err == nil {
		t.Fatal("open signature class accepted")
	}
	// Symmetry alone never implies a class: an asymmetric payload with
	// rational diagonal still classifies by diagonal only if diagonality
	// holds — asymmetric input is rejected upstream by determinant, and
	// here classification reads diagonal entries only (documented limit).
}

func condRef(t *testing.T, s Store, label string) artifact.ArtifactRef {
	t.Helper()
	body, err := EncodeRelationBody(OpGTE, NewEphemeralSymbol(label), MustRational(0, 1))
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

func TestVolumeDensityContracts(t *testing.T) {
	s := NewMemStore()
	// Minkowski: det -1, factor 1, Lorentzian, no reduction.
	mref, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	detRef, mdetDeriv, err := DiagonalDeterminant(s, mref)
	if err != nil {
		t.Fatal(err)
	}
	mdetDerivRef := mustStoreDerivation(t, s, mdetDeriv)
	one := mustStoreExpr(t, s, MustRational(1, 1))
	mink := VolumeDensity{MetricRef: mref, DeterminantRef: detRef,
		DeterminantDerivationRef: mdetDerivRef,
		SignatureClass:           SignatureLorentzian, FactorRef: one}
	if err := mink.Validate(); err != nil {
		t.Fatalf("Minkowski volume rejected: %v", err)
	}
	if err := CheckSignature(s, mref, mink.SignatureClass); err != nil {
		t.Fatalf("Minkowski signature inconsistent: %v", err)
	}
	if err := CheckVolumeDensity(s, mink); err != nil {
		t.Fatalf("Minkowski provenance rejected: %v", err)
	}
	fams := familiesMap(t, s, mref, detRef, mdetDerivRef, one)
	if err := ValidateAgainst(mink.TargetChecks(), fams); err != nil {
		t.Fatalf("Minkowski volume targets rejected: %v", err)
	}

	// Rindler: factor |xbar| represented, reduction to xbar REQUIRES a condition.
	xbar := NewEphemeralSymbol("xbar")
	rref, _ := diagMetric(t, s, Neg(Pow(xbar, MustRational(2, 1))),
		MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	rdet, rdetDeriv, err := DiagonalDeterminant(s, rref)
	if err != nil {
		t.Fatal(err)
	}
	rdetDerivRef := mustStoreDerivation(t, s, rdetDeriv)
	absx := mustStoreExpr(t, s, NewEphemeralSymbol("abs_xbar"))
	unreduced := VolumeDensity{MetricRef: rref, DeterminantRef: rdet,
		DeterminantDerivationRef: rdetDerivRef,
		SignatureClass:           SignatureLorentzian, FactorRef: absx}
	if err := unreduced.Validate(); err != nil {
		t.Fatalf("unreduced Rindler volume rejected: %v", err)
	}
	// Unlicensed reduction: ReductionRef without conditions fails closed.
	x := mustStoreExpr(t, s, xbar)
	illicit := unreduced
	illicit.ReductionRef = &x
	if err := illicit.Validate(); err == nil {
		t.Fatal("unlicensed absolute-value reduction accepted")
	}
	// Licensed reduction: with cited xbar>0 condition.
	cond := condRef(t, s, "xbar")
	licensed := unreduced
	licensed.ReductionRef = &x
	licensed.ReductionConditionRefs = []artifact.ArtifactRef{cond}
	licensed.DomainAssumptionRefs = []artifact.ArtifactRef{cond}
	if err := licensed.Validate(); err != nil {
		t.Fatalf("licensed Rindler reduction rejected: %v", err)
	}
	if err := CheckVolumeDensity(s, licensed); err != nil {
		t.Fatalf("licensed Rindler provenance rejected: %v", err)
	}
	fams2 := familiesMap(t, s, rref, rdet, rdetDerivRef, absx, x, cond)
	if err := ValidateAgainst(licensed.TargetChecks(), fams2); err != nil {
		t.Fatalf("licensed targets rejected: %v", err)
	}

	// FRW spatial: det a^6 represented, factor |a|^3, reduction needs a>0.
	a := NewEphemeralSymbol("a")
	a6 := Pow(a, MustRational(6, 1))
	frw, _ := diagMetric(t, s, Mul(a, a), Mul(a, a))
	_ = frw
	_ = a6
	fdet := mustStoreExpr(t, s, Pow(a, MustRational(6, 1)))
	fabs := mustStoreExpr(t, s, NewEphemeralSymbol("abs_a_cubed"))
	acond := condRef(t, s, "a")
	fac := mustStoreExpr(t, s, Pow(a, MustRational(3, 1)))
	frwVol := VolumeDensity{MetricRef: rref, DeterminantRef: fdet,
		DeterminantDerivationRef: rdetDerivRef,
		SignatureClass:           SignaturePositiveDefinite, FactorRef: fabs,
		ReductionRef: &fac, ReductionConditionRefs: []artifact.ArtifactRef{acond}}
	if err := frwVol.Validate(); err != nil {
		t.Fatalf("FRW volume with a>0 rejected: %v", err)
	}
	noCond := frwVol
	noCond.ReductionConditionRefs = nil
	if err := noCond.Validate(); err == nil {
		t.Fatal("FRW reduction without a>0 accepted")
	}

	// Invalid signature class; determinant-value consistency is NOT
	// established by Validate (documented boundary: the computation path
	// binds det to metric, Validate never recomputes).
	badSig := mink
	badSig.SignatureClass = "EUCLIDEAN"
	if err := badSig.Validate(); err == nil {
		t.Fatal("open signature class accepted")
	}
	wrongDet := mink
	wrongDet.DeterminantRef = one // det(-1) replaced by 1: Validate cannot tell
	if err := wrongDet.Validate(); err != nil {
		t.Fatalf("Validate unexpectedly recomputes determinant: %v", err)
	}
	// The store-aware provenance gate MUST tell: substituted output fails.
	if err := CheckVolumeDensity(s, wrongDet); err == nil {
		t.Fatal("substituted determinant accepted by provenance check")
	}
	// Missing derivation evidence fails structural validation (presence gate).
	noDeriv := mink
	noDeriv.DeterminantDerivationRef = artifact.ArtifactRef{}
	if err := noDeriv.Validate(); err == nil {
		t.Fatal("missing derivation reference accepted by Validate")
	}
}

// Forward-compatibility (QE minisuperspace §6): flat FRW spatial metric
// diag(a^2,a^2,a^2). The determinant derivation replays deterministically,
// but the canonical product is NOT identified with a^6: combining
// Pow(a,2)^3 into Pow(a,6) needs exponent addition the 7-rule engine
// lacks. This test pins that boundary instead of hiding it.
func TestDiagonalDeterminantFRWSpatial(t *testing.T) {
	s := NewMemStore()
	a := NewEphemeralSymbol("a")
	a2 := Pow(a, MustRational(2, 1))
	ref, _ := diagMetric(t, s, a2, a2, a2)
	if err := CheckSignature(s, ref, SignaturePositiveDefinite); err == nil {
		t.Fatal("symbolic diagonal classified positive-definite")
	} else {
		t.Logf("signature honestly unverifiable: %v", err)
	}
	detRef, d, err := DiagonalDeterminant(s, ref)
	if err != nil {
		t.Fatalf("FRW det failed: %v", err)
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("FRW det derivation does not replay: %v", err)
	}
	// Determinism across recomputation.
	detRef2, _, err := DiagonalDeterminant(s, ref)
	if err != nil {
		t.Fatal(err)
	}
	if detRef != detRef2 {
		t.Fatal("FRW det nondeterministic")
	}
	// The product form must NOT equal Pow(a,6): exponent addition is
	// absent by design (no new rules in this slice).
	sixth := mustStoreExpr(t, s, Pow(a, MustRational(6, 1)))
	if detRef == sixth {
		t.Fatal("product form collapsed to a^6 without an exponent rule")
	}
	// Volume density over the FRW metric with a>0-licensed reduction.
	cond := condRef(t, s, "a")
	factor := mustStoreExpr(t, s, NewEphemeralSymbol("abs_a_cubed"))
	red := mustStoreExpr(t, s, Pow(a, MustRational(3, 1)))
	fdetDerivRef := mustStoreDerivation(t, s, d)
	vol := VolumeDensity{MetricRef: ref, DeterminantRef: detRef,
		DeterminantDerivationRef: fdetDerivRef,
		SignatureClass:           SignaturePositiveDefinite, FactorRef: factor,
		ReductionRef:           &red,
		ReductionConditionRefs: []artifact.ArtifactRef{cond},
		DomainAssumptionRefs:   []artifact.ArtifactRef{cond}}
	if err := vol.Validate(); err != nil {
		t.Fatalf("FRW volume with a>0 rejected: %v", err)
	}
	if err := CheckVolumeDensity(s, vol); err != nil {
		t.Fatalf("FRW provenance rejected: %v", err)
	}
	fams := familiesMap(t, s, ref, detRef, fdetDerivRef, factor, red, cond)
	if err := ValidateAgainst(vol.TargetChecks(), fams); err != nil {
		t.Fatalf("FRW volume targets rejected: %v", err)
	}
}
