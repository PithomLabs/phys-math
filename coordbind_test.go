package physmath

import (
	"errors"
	"strings"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// bindFixture mints a chart over explicit coordinate symbols with an
// explicit declaring function and mapping. Unlike covChart, coordinate PIDs,
// parameter names, and mapped PIDs are chosen independently so tests can
// separate display equality from parameter identity.
func bindFixture(t *testing.T, s Store, coords []Expr, fnPIDs []ParameterID, fnNames []string, mapPIDs []ParameterID) (artifact.ArtifactRef, artifact.ArtifactRef, CoordParamMap) {
	t.Helper()
	desig := mustStoreExpr(t, s, NewEphemeralSymbol("bind_space"))
	spaceRaw, err := Collection{ElementRefs: []artifact.ArtifactRef{desig}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	space, err := s.Put(FamilyCollection, spaceRaw)
	if err != nil {
		t.Fatal(err)
	}
	crefs := make([]artifact.ArtifactRef, len(coords))
	for i, c := range coords {
		crefs[i] = mustStoreExpr(t, s, c)
	}
	cs := CoordinateSystem{Dimension: uint16(len(coords)), CoordinateRefs: crefs, SpaceRef: space}
	if err := cs.Validate(); err != nil {
		t.Fatal(err)
	}
	chart, err := s.Put(FamilyCoordinateSystem, cs.Encode())
	if err != nil {
		t.Fatal(err)
	}
	fnObj := Function{ParameterIDs: fnPIDs, ParamNames: fnNames, BodyRef: crefs[0]}
	fnBytes, err := fnObj.Encode()
	if err != nil {
		t.Fatal(err)
	}
	declFn, err := s.Put(FamilyFunction, fnBytes)
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]BoundParameterRef, len(mapPIDs))
	for i, pid := range mapPIDs {
		entries[i] = BoundParameterRef{DeclaringArtifactRef: declFn, ParameterID: pid}
	}
	return chart, declFn, CoordParamMap{Entries: entries}
}

// TestCoordBindingAcceptValid pins the happy path: exact PID, declaring
// context, and name agreement at every position.
func TestCoordBindingAcceptValid(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	op := mustStoreExpr(t, s, syms[0])
	if err := ValidateCoordBinding(s, chart, mapping, declFn, []artifact.ArtifactRef{op}); err != nil {
		t.Fatalf("valid binding rejected: %v", err)
	}
	// No operands: chart/mapping/declaring agreement alone passes.
	if err := ValidateCoordBinding(s, chart, mapping, declFn, nil); err != nil {
		t.Fatalf("operand-free valid binding rejected: %v", err)
	}
}

// TestCoordBindingSameDisplayDifferentPID is the clarification's focused
// negative: chart ("r", PID0) against mapping ("r", PID2) — identical
// displays, different PIDs — must fail at the shared binding boundary with
// a PID-mismatch reason, even though CheckCoordParamMap passes
// structurally (both entries declared and distinct).
func TestCoordBindingSameDisplayDifferentPID(t *testing.T) {
	s := NewMemStore()
	p0, p1, p2 := DeriveParameterID(0), DeriveParameterID(1), DeriveParameterID(2)
	chart, declFn, mapping := bindFixture(t, s,
		[]Expr{NewBoundSymbol("r", p0), NewBoundSymbol("theta", p1)},
		[]ParameterID{p0, p1, p2}, []string{"x", "theta", "r"},
		[]ParameterID{p2, p1})
	if err := CheckCoordParamMap(s, chart, mapping, declFn); err != nil {
		t.Fatalf("fixture mapping should pass structural gates: %v", err)
	}
	op := mustStoreExpr(t, s, NewBoundSymbol("r", p0))
	err := ValidateCoordBinding(s, chart, mapping, declFn, []artifact.ArtifactRef{op})
	if err == nil {
		t.Fatal("same-display/different-PID binding accepted")
	}
	if !errors.Is(err, ErrBadInput) || !strings.Contains(err.Error(), "PID mismatch") {
		t.Fatalf("wrong rejection boundary (want PID mismatch): %v", err)
	}
	// The guarded constructor inherits the same boundary.
	if _, _, _, err := DifferentiateCoordinate(s, op, chart, 0, mapping, declFn, 1, nil); err == nil {
		t.Fatal("guarded differentiation accepted same-display/different-PID binding")
	} else if !strings.Contains(err.Error(), "PID mismatch") {
		t.Fatalf("guarded path wrong boundary: %v", err)
	}
}

// TestCoordBindingNameMismatch is the mirror case: same PID, conflicting
// display. The mapping entry binds the chart PID but the declaring
// vocabulary names it differently.
func TestCoordBindingNameMismatch(t *testing.T) {
	s := NewMemStore()
	p0, p1 := DeriveParameterID(0), DeriveParameterID(1)
	chart, declFn, mapping := bindFixture(t, s,
		[]Expr{NewBoundSymbol("r", p0), NewBoundSymbol("theta", p1)},
		[]ParameterID{p0, p1}, []string{"q", "theta"},
		[]ParameterID{p0, p1})
	op := mustStoreExpr(t, s, NewBoundSymbol("r", p0))
	err := ValidateCoordBinding(s, chart, mapping, declFn, []artifact.ArtifactRef{op})
	if err == nil {
		t.Fatal("name-mismatched binding accepted")
	}
	if !errors.Is(err, ErrBadInput) || !strings.Contains(err.Error(), "does not match chart coordinate") {
		t.Fatalf("wrong rejection boundary (want name mismatch): %v", err)
	}
}

// TestCoordBindingSwappedMapping: reordered entries bind position 0 to the
// wrong PID. Structurally valid per CheckCoordParamMap, rejected here.
func TestCoordBindingSwappedMapping(t *testing.T) {
	s := NewMemStore()
	chart, _, declFn, syms := covChart(t, s, "x", "y")
	px, py := DeriveParameterID(0), DeriveParameterID(1)
	swapped := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: declFn, ParameterID: py},
		{DeclaringArtifactRef: declFn, ParameterID: px},
	}}
	if err := CheckCoordParamMap(s, chart, swapped, declFn); err != nil {
		t.Fatalf("swapped mapping should pass structural gates: %v", err)
	}
	op := mustStoreExpr(t, s, syms[0])
	if err := ValidateCoordBinding(s, chart, swapped, declFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("swapped mapping accepted at the binding boundary")
	}
}

// TestCoordBindingMalformed covers the remaining fail-closed gates.
func TestCoordBindingMalformed(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	op := mustStoreExpr(t, s, syms[0])

	short := CoordParamMap{Entries: mapping.Entries[:1]}
	if err := ValidateCoordBinding(s, chart, short, declFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("short mapping accepted")
	}

	other, _ := mustStoreFunction(t, s, []string{"v"}, NewBoundSymbol("v", DeriveParameterID(0)))
	cross := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: declFn, ParameterID: DeriveParameterID(0)},
		{DeclaringArtifactRef: other, ParameterID: DeriveParameterID(0)},
	}}
	if err := ValidateCoordBinding(s, chart, cross, declFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("cross-function entry accepted")
	}

	dup := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: declFn, ParameterID: DeriveParameterID(0)},
		{DeclaringArtifactRef: declFn, ParameterID: DeriveParameterID(0)},
	}}
	if err := ValidateCoordBinding(s, chart, dup, declFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("duplicate PID mapping accepted")
	}

	ghost := ComputeTestRef(t, "physmath", "coordinate_system/1")
	if err := ValidateCoordBinding(s, ghost, mapping, declFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("unresolvable chart accepted")
	}

	exprRef := mustStoreExpr(t, s, MustRational(1, 1))
	if err := ValidateCoordBinding(s, exprRef, mapping, declFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("wrong-family chart accepted")
	}

	// Nameless declaring function: ErrMissingIdentity, not a silent pass.
	legacy := legacyFunction(t, s, NewBoundSymbol("x", DeriveParameterID(0)))
	if err := ValidateCoordBinding(s, chart, mapping, legacy, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("nameless declaring function accepted")
	} else if !errors.Is(err, ErrMissingIdentity) && !errors.Is(err, ErrBadInput) {
		t.Fatalf("nameless function wrong reason: %v", err)
	}

	// Non-symbol chart coordinate fails closed (no display to compare).
	nonSym := mustStoreExpr(t, s, MustRational(7, 1))
	desig2 := mustStoreExpr(t, s, NewEphemeralSymbol("ns_space"))
	raw2, _ := Collection{ElementRefs: []artifact.ArtifactRef{desig2}}.Encode()
	sp2, _ := s.Put(FamilyCollection, raw2)
	nsChart := CoordinateSystem{Dimension: 1, CoordinateRefs: []artifact.ArtifactRef{nonSym}, SpaceRef: sp2}
	nsRef, _ := s.Put(FamilyCoordinateSystem, nsChart.Encode())
	p0 := DeriveParameterID(0)
	nsFn, _ := mustStoreFunction(t, s, []string{"x"}, NewBoundSymbol("x", p0))
	nsMap := CoordParamMap{Entries: []BoundParameterRef{{DeclaringArtifactRef: nsFn, ParameterID: p0}}}
	if err := ValidateCoordBinding(s, nsRef, nsMap, nsFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("non-symbol chart coordinate accepted")
	}

	// Unresolvable operand fails closed.
	ghostOp := ComputeTestRef(t, "physmath", "expression/1")
	if err := ValidateCoordBinding(s, chart, mapping, declFn, []artifact.ArtifactRef{ghostOp}); err == nil {
		t.Fatal("unresolvable operand accepted")
	}
}

// TestCoordBindingSharedAndDisjoint preserves legitimate sharing: chart
// coordinate refs verbatim and independent unmapped parameters pass.
func TestCoordBindingSharedAndDisjoint(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	_, cpayload, err := s.Lookup(chart)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCoordBinding(s, chart, mapping, declFn, cs.CoordinateRefs); err != nil {
		t.Fatalf("shared coordinate identity rejected: %v", err)
	}
	indep := mustStoreExpr(t, s, Mul(NewBoundSymbol("a", DeriveParameterID(2)), syms[0]))
	if err := ValidateCoordBinding(s, chart, mapping, declFn, []artifact.ArtifactRef{indep}); err != nil {
		t.Fatalf("disjoint independent parameter rejected: %v", err)
	}
}
