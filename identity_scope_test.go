package physmath

import (
	"strings"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// idChart builds a 2-coordinate chart (x PID 0, y PID 1) with declaring
// function and mapping, returning all handles a test needs.
type idHandles struct {
	chart   artifact.ArtifactRef
	mapping CoordParamMap
	declFn  artifact.ArtifactRef
	x, y    Expr
	px, py  ParameterID
}

func idSetup(t *testing.T, s Store) idHandles {
	t.Helper()
	px := DeriveParameterID(0)
	py := DeriveParameterID(1)
	x := NewBoundSymbol("x", px)
	y := NewBoundSymbol("y", py)
	desig := mustStoreExpr(t, s, NewEphemeralSymbol("id_space"))
	spaceRaw, err := Collection{ElementRefs: []artifact.ArtifactRef{desig}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	space, err := s.Put(FamilyCollection, spaceRaw)
	if err != nil {
		t.Fatal(err)
	}
	cs := CoordinateSystem{Dimension: 2,
		CoordinateRefs: []artifact.ArtifactRef{
			mustStoreExpr(t, s, x), mustStoreExpr(t, s, y),
		},
		SpaceRef: space}
	if err := cs.Validate(); err != nil {
		t.Fatal(err)
	}
	chart, err := s.Put(FamilyCoordinateSystem, cs.Encode())
	if err != nil {
		t.Fatal(err)
	}
	fnObj := Function{ParameterIDs: []ParameterID{px, py},
		ParamNames: []string{"x", "y"},
		BodyRef:    mustStoreExpr(t, s, Mul(x, y))}
	fnBytes, err := fnObj.Encode()
	if err != nil {
		t.Fatal(err)
	}
	declFn, err := s.Put(FamilyFunction, fnBytes)
	if err != nil {
		t.Fatal(err)
	}
	mapping := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: declFn, ParameterID: px},
		{DeclaringArtifactRef: declFn, ParameterID: py},
	}}
	return idHandles{chart: chart, mapping: mapping, declFn: declFn, x: x, y: y, px: px, py: py}
}

func mintExprRef(t *testing.T, s Store, e Expr) artifact.ArtifactRef {
	t.Helper()
	r, err := PutExpression(s, e)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// Case 1: legitimate disjoint chart-and-scale identity is accepted.
func TestDisjointIdentityAccept(t *testing.T) {
	s := NewMemStore()
	h := idSetup(t, s)
	// Scale symbol "a" on an unmapped PID: independent quantity, legitimate.
	a := NewBoundSymbol("a", DeriveParameterID(2))
	op := mintExprRef(t, s, Mul(a, h.x))
	if err := CheckDisjointIdentity(s, h.chart, h.mapping, h.declFn, []artifact.ArtifactRef{op}); err != nil {
		t.Fatalf("disjoint identity rejected: %v", err)
	}
}

// Case 2: unintended reuse of a chart-coordinate PID under another display
// is rejected at the identity boundary (not a later unrelated check).
func TestDisjointIdentityCollision(t *testing.T) {
	s := NewMemStore()
	h := idSetup(t, s)
	// "a" bound to x's PID: mathematically distinct quantity colliding.
	collided := mintExprRef(t, s, Mul(NewBoundSymbol("a", h.px), h.y))
	err := CheckDisjointIdentity(s, h.chart, h.mapping, h.declFn, []artifact.ArtifactRef{collided})
	if err == nil {
		t.Fatal("collided identity accepted")
	}
	if !strings.Contains(err.Error(), "reserved for coordinate") {
		t.Fatalf("wrong rejection boundary: %v", err)
	}
}

// Case 3: mapping order is semantic in both directions. Operands using
// ("y", px): rejected under [px->x, py->y], accepted under swapped
// [px->y, py->x] — proving order decides meaning both ways.
func TestDisjointIdentityOrderBothWays(t *testing.T) {
	s := NewMemStore()
	h := idSetup(t, s)
	op := mintExprRef(t, s, NewBoundSymbol("y", h.px))
	if err := CheckDisjointIdentity(s, h.chart, h.mapping, h.declFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("order-sensitive mismatch accepted under original mapping")
	} else if !strings.Contains(err.Error(), "reserved for coordinate") {
		t.Fatalf("wrong boundary under original mapping: %v", err)
	}
	swapped := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: h.declFn, ParameterID: h.py},
		{DeclaringArtifactRef: h.declFn, ParameterID: h.px},
	}}
	// Swapped mapping binds position 0 to py("y"): ("y",px) now matches.
	// NOTE: swapped mapping still passes CheckCoordParamMap structurally
	// (both entries declared+distinct); the semantic difference is which
	// coordinate position 0 denotes — asserted via acceptance flip.
	if err := CheckDisjointIdentity(s, h.chart, swapped, h.declFn, []artifact.ArtifactRef{op}); err != nil {
		t.Fatalf("same operands rejected under swapped mapping: %v", err)
	}
	// And the mirror: ("x",py) accepted originally... is rejected: x's PID
	// is px, not py — wait, ("x",py) mismatches under BOTH mappings. Use
	// ("x",px): accepted originally, rejected swapped.
	opX := mintExprRef(t, s, NewBoundSymbol("x", h.px))
	if err := CheckDisjointIdentity(s, h.chart, h.mapping, h.declFn, []artifact.ArtifactRef{opX}); err != nil {
		t.Fatalf("shared coordinate identity rejected: %v", err)
	}
	if err := CheckDisjointIdentity(s, h.chart, swapped, h.declFn, []artifact.ArtifactRef{opX}); err == nil {
		t.Fatal("position change did not alter operand meaning")
	}
}

// Case 4: legitimate reuse of one explicitly shared identity is accepted:
// the chart coordinate expressions themselves pass verbatim.
func TestDisjointIdentitySharedReuse(t *testing.T) {
	s := NewMemStore()
	h := idSetup(t, s)
	_, cpayload, err := s.Lookup(h.chart)
	if err != nil {
		t.Fatal(err)
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		t.Fatal(err)
	}
	// Operands ARE the coordinate refs: byte-identical shared identity.
	if err := CheckDisjointIdentity(s, h.chart, h.mapping, h.declFn, chart.CoordinateRefs); err != nil {
		t.Fatalf("shared coordinate identity rejected: %v", err)
	}
}

// Case 5: malformed mappings and unresolvable references fail closed.
func TestDisjointIdentityMalformed(t *testing.T) {
	s := NewMemStore()
	h := idSetup(t, s)
	op := mintExprRef(t, s, h.x)
	short := CoordParamMap{Entries: h.mapping.Entries[:1]}
	if err := CheckDisjointIdentity(s, h.chart, short, h.declFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("short mapping accepted")
	}
	ghost := ComputeTestRef(t, "physmath", "coordinate_system/1")
	if err := CheckDisjointIdentity(s, ghost, h.mapping, h.declFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("unresolvable chart accepted")
	}
	ghostOp := ComputeTestRef(t, "physmath", "expression/1")
	if err := CheckDisjointIdentity(s, h.chart, h.mapping, h.declFn, []artifact.ArtifactRef{ghostOp}); err == nil {
		t.Fatal("unresolvable operand accepted")
	}
	wrongFam := mustStoreExpr(t, s, MustRational(1, 1))
	_ = wrongFam
	relBody, err := EncodeRelationBody(OpEQ, h.x, h.y)
	if err != nil {
		t.Fatal(err)
	}
	relRef, err := s.Put(FamilyRelation, relBody)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckDisjointIdentity(s, relRef, h.mapping, h.declFn, []artifact.ArtifactRef{op}); err == nil {
		t.Fatal("wrong-family chart accepted")
	}
}

// Case 6: construction entry rejects a collided tensor before minting.
// A rank-1 tensor over the chart space whose component binds ("q", px)
// must fail EvaluateCovariantDerivative at the identity gate.
func TestDisjointIdentityConstructionGate(t *testing.T) {
	s := NewMemStore()
	h := idSetup(t, s)
	_, cpayload, err := s.Lookup(h.chart)
	if err != nil {
		t.Fatal(err)
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		t.Fatal(err)
	}
	ixPayload, err := Index{Label: "v", Variance: VarianceContravariant, Space: &chart.SpaceRef}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ix, err := s.Put(FamilyIndex, ixPayload)
	if err != nil {
		t.Fatal(err)
	}
	collided := NewBoundSymbol("q", h.px)
	tn2 := Tensor{Rank: 1, Dimension: 2,
		IndexRefs:     []artifact.ArtifactRef{ix},
		ComponentRefs: []artifact.ArtifactRef{mintExprRef(t, s, collided), mintExprRef(t, s, h.y)},
	}
	if err := tn2.Validate(); err != nil {
		t.Fatal(err)
	}
	tref, err := s.Put(FamilyTensor, tn2.Encode())
	if err != nil {
		t.Fatal(err)
	}
	field, err := s.Put(FamilyTensorField,
		TensorField{TensorRef: tref, CoordinateSystemRef: h.chart}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	zero := mintExprRef(t, s, MustRational(0, 1))
	coeffs := make([]artifact.ArtifactRef, 8)
	for i := range coeffs {
		coeffs[i] = zero
	}
	conn, err := s.Put(FamilyAffineConnection, AffineConnection{
		CoordRef: h.chart, Dimension: 2, CoefficientRefs: coeffs,
	}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = EvaluateCovariantDerivative(s, field, conn, h.mapping, h.declFn, nil)
	if err == nil {
		t.Fatal("collided tensor accepted by evaluator")
	}
	if !strings.Contains(err.Error(), "reserved for coordinate") {
		t.Fatalf("wrong rejection boundary (expected identity gate): %v", err)
	}
}
