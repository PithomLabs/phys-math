package physmath

import (
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// chartFixture mints a 2-coordinate system over a fresh space.
func chartFixture(t *testing.T, s Store, names ...string) (artifact.ArtifactRef, []artifact.ArtifactRef, artifact.ArtifactRef) {
	t.Helper()
	desig := mustStoreExpr(t, s, NewEphemeralSymbol("chart_space"))
	raw, err := Collection{ElementRefs: []artifact.ArtifactRef{desig}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	space, err := s.Put(FamilyCollection, raw)
	if err != nil {
		t.Fatal(err)
	}
	coords := make([]artifact.ArtifactRef, len(names))
	for i, n := range names {
		coords[i] = mustStoreExpr(t, s, NewEphemeralSymbol(n))
	}
	cs := CoordinateSystem{Dimension: uint16(len(names)), CoordinateRefs: coords, SpaceRef: space}
	if err := cs.Validate(); err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyCoordinateSystem, cs.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return ref, coords, space
}

func TestCoordinateSystemRoundTrip(t *testing.T) {
	s := NewMemStore()
	ref, coords, space := chartFixture(t, s, "r", "theta")
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyCoordinateSystem {
		t.Fatal("chart family wrong")
	}
	dec, err := DecodeCoordinateSystem(payload)
	if err != nil {
		t.Fatalf("chart does not decode: %v", err)
	}
	if dec.Dimension != 2 || len(dec.CoordinateRefs) != 2 || dec.SpaceRef != space {
		t.Fatal("chart round-trip changed identity")
	}
	if dec.CoordinateRefs[0] != coords[0] || dec.CoordinateRefs[1] != coords[1] {
		t.Fatal("coordinate order not preserved")
	}
	// Reordered coordinates are a distinct artifact.
	rev := CoordinateSystem{Dimension: 2,
		CoordinateRefs: []artifact.ArtifactRef{coords[1], coords[0]}, SpaceRef: space}
	revRef, err := s.Put(FamilyCoordinateSystem, rev.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if revRef == ref {
		t.Fatal("reordered coordinates identical to original")
	}
	// Duplicate coordinates rejected.
	dup := CoordinateSystem{Dimension: 2,
		CoordinateRefs: []artifact.ArtifactRef{coords[0], coords[0]}, SpaceRef: space}
	if err := dup.Validate(); err == nil {
		t.Fatal("duplicate coordinates accepted")
	}
	// Dimension/count mismatch rejected.
	bad := CoordinateSystem{Dimension: 3, CoordinateRefs: coords, SpaceRef: space}
	if err := bad.Validate(); err == nil {
		t.Fatal("dimension mismatch accepted")
	}
	// Truncated / trailing payloads rejected.
	if _, err := DecodeCoordinateSystem(payload[:len(payload)/2]); err == nil {
		t.Fatal("truncated chart accepted")
	}
	if _, err := DecodeCoordinateSystem(append(append([]byte{}, payload...), 0x00)); err == nil {
		t.Fatal("trailing bytes accepted")
	}
}

func TestTensorFieldBinding(t *testing.T) {
	s := NewMemStore()
	chart, _, _ := chartFixture(t, s, "r", "theta")
	tn := Tensor{Rank: 1, Dimension: 2,
		IndexRefs:     []artifact.ArtifactRef{mustStoreExpr(t, s, MustRational(0, 1))},
		ComponentRefs: []artifact.ArtifactRef{mustStoreExpr(t, s, MustRational(1, 1)), mustStoreExpr(t, s, MustRational(2, 1))},
	}
	_ = tn
	// Use a real index artifact for the tensor slot.
	sp := spaceRef(t, s, "field_space")
	ix := idxRef(t, s, "mu", VarianceContravariant, sp)
	tref := tensorRef(t, s, 1, 2, []artifact.ArtifactRef{ix}, rationalMatrix(1, 2))
	tf := TensorField{TensorRef: tref, CoordinateSystemRef: chart}
	if err := tf.Validate(); err != nil {
		t.Fatal(err)
	}
	raw := tf.Encode()
	dec, err := DecodeTensorField(raw)
	if err != nil {
		t.Fatalf("tensor field does not decode: %v", err)
	}
	fams := familiesMap(t, s, tref, chart)
	if err := ValidateAgainst(dec.TargetChecks(), fams); err != nil {
		t.Fatalf("tensor field targets rejected: %v", err)
	}
	// Zero references fail grammar.
	empty := TensorField{}
	if err := empty.Validate(); err == nil {
		t.Fatal("empty tensor field accepted")
	}
	if _, err := DecodeTensorField(raw[:len(raw)/2]); err == nil {
		t.Fatal("truncated tensor field accepted")
	}
}

func TestAffineConnectionShape(t *testing.T) {
	s := NewMemStore()
	chart, _, _ := chartFixture(t, s, "r", "theta")
	zero := mustStoreExpr(t, s, MustRational(0, 1))
	coeffs := make([]artifact.ArtifactRef, 8)
	for i := range coeffs {
		coeffs[i] = zero
	}
	c := AffineConnection{CoordRef: chart, Dimension: 2, CoefficientRefs: coeffs, ClaimSymmetric: true}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	raw := c.Encode()
	dec, err := DecodeAffineConnection(raw)
	if err != nil {
		t.Fatalf("connection does not decode: %v", err)
	}
	if !dec.ClaimSymmetric || dec.Dimension != 2 || len(dec.CoefficientRefs) != 8 {
		t.Fatal("connection round-trip changed content")
	}
	// Wrong coefficient count rejected (2³ = 8 required).
	short := AffineConnection{CoordRef: chart, Dimension: 2, CoefficientRefs: coeffs[:7]}
	if err := short.Validate(); err == nil {
		t.Fatal("short coefficient list accepted")
	}
	// Zero dimension rejected.
	zdim := AffineConnection{CoordRef: chart, Dimension: 0, CoefficientRefs: nil}
	if err := zdim.Validate(); err == nil {
		t.Fatal("zero dimension accepted")
	}
	if _, err := DecodeAffineConnection(raw[:len(raw)/2]); err == nil {
		t.Fatal("truncated connection accepted")
	}
}

func TestCoordParamMapGates(t *testing.T) {
	s := NewMemStore()
	chart, coords, _ := chartFixture(t, s, "r", "theta")
	_ = coords
	p0 := DeriveParameterID(0)
	p1 := DeriveParameterID(1)
	fn := Function{ParameterIDs: []ParameterID{p0, p1},
		ParamNames: []string{"r", "theta"},
		BodyRef:    mustStoreExpr(t, s, NewBoundSymbol("r", p0))}
	fnBytes, err := fn.Encode()
	if err != nil {
		t.Fatal(err)
	}
	fnRef, err := s.Put(FamilyFunction, fnBytes)
	if err != nil {
		t.Fatal(err)
	}
	good := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: fnRef, ParameterID: p0},
		{DeclaringArtifactRef: fnRef, ParameterID: p1},
	}}
	if err := CheckCoordParamMap(s, chart, good, fnRef); err != nil {
		t.Fatalf("valid mapping rejected: %v", err)
	}
	// Round-trip through encoding.
	raw, err := good.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dec, err := DecodeCoordParamMap(raw)
	if err != nil {
		t.Fatalf("mapping does not decode: %v", err)
	}
	if len(dec.Entries) != 2 || dec.Entries[0].ParameterID != p0 {
		t.Fatal("mapping round-trip changed order")
	}
	// Reordered parameters: entry count matches but order maps r->p1.
	// Order is semantic (entry i binds coordinate i); a reordered map is a
	// DIFFERENT mapping and must not validate against the same expectation:
	// here it still passes structural gates (both declared+distinct), so
	// assert it decodes to a distinct mapping instead.
	swapped := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: fnRef, ParameterID: p1},
		{DeclaringArtifactRef: fnRef, ParameterID: p0},
	}}
	if err := CheckCoordParamMap(s, chart, swapped, fnRef); err != nil {
		t.Fatalf("swapped mapping should pass structural gates: %v", err)
	}
	sraw, err := swapped.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(sraw) == string(raw) {
		t.Fatal("reordered parameters identical to original")
	}
	// Wrong count.
	short := CoordParamMap{Entries: []BoundParameterRef{{DeclaringArtifactRef: fnRef, ParameterID: p0}}}
	if err := CheckCoordParamMap(s, chart, short, fnRef); err == nil {
		t.Fatal("short mapping accepted")
	}
	// Cross-function entry.
	other, _ := mustStoreFunction(t, s, []string{"v"}, NewBoundSymbol("v", p0))
	cross := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: fnRef, ParameterID: p0},
		{DeclaringArtifactRef: other, ParameterID: p0},
	}}
	_ = other
	if err := CheckCoordParamMap(s, chart, cross, fnRef); err == nil {
		t.Fatal("cross-function entry accepted")
	}
	// Duplicate parameter identity.
	dup := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: fnRef, ParameterID: p0},
		{DeclaringArtifactRef: fnRef, ParameterID: p0},
	}}
	if err := CheckCoordParamMap(s, chart, dup, fnRef); err == nil {
		t.Fatal("duplicate parameter identity accepted")
	}
	// Unresolvable chart / wrong family chart.
	ghost := ComputeTestRef(t, "physmath", "coordinate_system/1")
	if err := CheckCoordParamMap(s, ghost, good, fnRef); err == nil {
		t.Fatal("unresolvable chart accepted")
	}
	exprRef := mustStoreExpr(t, s, MustRational(1, 1))
	if err := CheckCoordParamMap(s, exprRef, good, fnRef); err == nil {
		t.Fatal("wrong-family chart accepted")
	}
	// Truncated mapping payload.
	if _, err := DecodeCoordParamMap(raw[:len(raw)/2]); err == nil {
		t.Fatal("truncated mapping accepted")
	}
}
