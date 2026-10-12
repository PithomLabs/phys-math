package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// W1 representations for covariant differential geometry: coordinate
// systems, chart-bound tensor fields, affine connections, and the explicit
// coordinate-to-parameter mapping. All types are representation +
// structural validation only; execution lives in the evaluator, and
// store-aware checks resolve every reference.
//
// Naming note: no Go identifier uses the purity-banned term "Chart";
// the durable family is coordinate_system/1 and the type is
// CoordinateSystem.

// FamilyCoordinateSystem is a content-addressed coordinate identity:
// dimension, ordered distinct coordinate variables, and the resolved
// index space. Coordinate order is mathematically significant: systems
// with reordered coordinates are distinct artifacts.
// Family physmath/coordinate_system/1.
var FamilyCoordinateSystem = FamilyID{NamespacePhysMath, "coordinate_system/1", "1"}

// CoordinateSystem binds ordered coordinate variables to a dimension and
// an index space. It identifies the coordinate system but establishes no
// transformation law to any other system.
type CoordinateSystem struct {
	Dimension      uint16
	CoordinateRefs []artifact.ArtifactRef // SEQUENCE, ordered → expression/1, distinct
	SpaceRef       artifact.ArtifactRef   // → collection/1
}

// Family returns FamilyCoordinateSystem.
func (CoordinateSystem) Family() FamilyID { return FamilyCoordinateSystem }

// Encode returns the canonical payload in declaration order.
func (c CoordinateSystem) Encode() []byte {
	out := artifact.EncodeUInt16(c.Dimension)
	out = append(out, encodeRefSequence(c.CoordinateRefs)...)
	return append(out, c.SpaceRef.Encode()...)
}

// Validate enforces dimension/count agreement, distinct coordinates, and
// reference grammar. It never resolves: families and resolvability belong
// to TargetChecks and the store-aware checks.
func (c CoordinateSystem) Validate() error {
	if c.Dimension < 1 {
		return fmt.Errorf("physmath: coordinate system dimension must be >= 1")
	}
	if uint64(len(c.CoordinateRefs)) != uint64(c.Dimension) {
		return fmt.Errorf("physmath: coordinate refs %d != dimension %d",
			len(c.CoordinateRefs), c.Dimension)
	}
	fields := map[string]artifact.ArtifactRef{"CoordinateSystem.SpaceRef": c.SpaceRef}
	for i, r := range c.CoordinateRefs {
		fields[fmt.Sprintf("CoordinateSystem.CoordinateRefs[%d]", i)] = r
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	return checkDuplicateRefs("CoordinateSystem.CoordinateRefs", c.CoordinateRefs)
}

// TargetChecks: CoordinateRefs → expression/1; SpaceRef → collection/1.
func (c CoordinateSystem) TargetChecks() []TargetCheck {
	checks := []TargetCheck{
		{Field: "CoordinateSystem.SpaceRef", Ref: c.SpaceRef, Allowed: []FamilyID{FamilyCollection}},
	}
	for i, r := range c.CoordinateRefs {
		checks = append(checks, TargetCheck{
			Field: fmt.Sprintf("CoordinateSystem.CoordinateRefs[%d]", i),
			Ref:   r, Allowed: []FamilyID{FamilyExpression},
		})
	}
	return checks
}

// DecodeCoordinateSystem decodes a physmath/coordinate_system/1 payload.
// Representation only: no store Lookup, no execution.
func DecodeCoordinateSystem(payload []byte) (CoordinateSystem, error) {
	d := artifact.NewDecoder(payload)
	dim, err := d.UInt16()
	if err != nil {
		return CoordinateSystem{}, fmt.Errorf("physmath: coordinate system missing dimension: %w", err)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return CoordinateSystem{}, fmt.Errorf("physmath: coordinate system missing coordinates: %w", err)
	}
	coords := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return CoordinateSystem{}, fmt.Errorf("physmath: coordinate system bad coordinate %d: %w", i, err)
		}
		coords = append(coords, r)
	}
	space, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return CoordinateSystem{}, fmt.Errorf("physmath: coordinate system missing space: %w", err)
	}
	if !d.Exhausted() {
		return CoordinateSystem{}, fmt.Errorf("physmath: trailing bytes in coordinate-system payload")
	}
	c := CoordinateSystem{Dimension: dim, CoordinateRefs: coords, SpaceRef: space}
	if err := c.Validate(); err != nil {
		return CoordinateSystem{}, err
	}
	return c, nil
}

// FamilyTensorField is a chart-bound tensor: the underlying algebraic
// tensor plus the coordinate system its components are expressed in.
// Family physmath/tensor_field/1.
var FamilyTensorField = FamilyID{NamespacePhysMath, "tensor_field/1", "1"}

// TensorField binds a tensor/1 artifact to its coordinate system.
// Plain tensor/1 stays chart-agnostic for general algebra; covariant
// differentiation accepts and returns only TensorField, so the checker
// can resolve both references and reject incompatible charts.
type TensorField struct {
	TensorRef           artifact.ArtifactRef // → tensor/1
	CoordinateSystemRef artifact.ArtifactRef // → coordinate_system/1
}

// Family returns FamilyTensorField.
func (TensorField) Family() FamilyID { return FamilyTensorField }

// Encode returns the canonical payload in declaration order.
func (t TensorField) Encode() []byte {
	out := t.TensorRef.Encode()
	return append(out, t.CoordinateSystemRef.Encode()...)
}

// Validate enforces reference grammar without resolution.
func (t TensorField) Validate() error {
	return validateRefGrammars(map[string]artifact.ArtifactRef{
		"TensorField.TensorRef":           t.TensorRef,
		"TensorField.CoordinateSystemRef": t.CoordinateSystemRef,
	})
}

// TargetChecks: TensorRef → tensor/1; CoordinateSystemRef →
// coordinate_system/1. Resolved only by store-aware checks.
func (t TensorField) TargetChecks() []TargetCheck {
	return []TargetCheck{
		{Field: "TensorField.TensorRef", Ref: t.TensorRef, Allowed: []FamilyID{FamilyTensor}},
		{Field: "TensorField.CoordinateSystemRef", Ref: t.CoordinateSystemRef, Allowed: []FamilyID{FamilyCoordinateSystem}},
	}
}

// DecodeTensorField decodes a physmath/tensor_field/1 payload.
// Representation only.
func DecodeTensorField(payload []byte) (TensorField, error) {
	d := artifact.NewDecoder(payload)
	tensor, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return TensorField{}, fmt.Errorf("physmath: tensor field missing tensor: %w", err)
	}
	system, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return TensorField{}, fmt.Errorf("physmath: tensor field missing coordinate system: %w", err)
	}
	if !d.Exhausted() {
		return TensorField{}, fmt.Errorf("physmath: trailing bytes in tensor-field payload")
	}
	tf := TensorField{TensorRef: tensor, CoordinateSystemRef: system}
	if err := tf.Validate(); err != nil {
		return TensorField{}, err
	}
	return tf, nil
}

// FamilyAffineConnection is a non-tensor affine connection: ordered
// Γ^ρ_μν coefficients over one coordinate system, with explicitly claimed
// (never assumed) symmetry. Family physmath/affine_connection/1.
var FamilyAffineConnection = FamilyID{NamespacePhysMath, "affine_connection/1", "1"}

// AffineConnection holds connection coefficients Γ^ρ_μν in canonical
// row-major order over (ρ,μ,ν) for one coordinate system. It is NOT a
// tensor: no tensor transformation law is claimed or checked. Symmetry in
// the lower indices (torsion-freeness) and metric compatibility are
// additional properties recorded by ClaimSymmetric and verified only by
// store-aware checks, never assumed from the coefficient shape.
type AffineConnection struct {
	CoordRef        artifact.ArtifactRef // → coordinate_system/1
	Dimension       uint16
	CoefficientRefs []artifact.ArtifactRef // SEQUENCE, row-major (ρ,μ,ν) → expression/1
	ClaimSymmetric  bool                   // symmetry claim, verified only by checkers
	// SourceMetricRef optionally links the metric this connection was
	// derived from (→ symmetric_bilinear/1). Nil makes no derivation claim:
	// an axiomatic or externally supplied connection. When present, the
	// store-aware check can recompute every coefficient from that metric.
	SourceMetricRef *artifact.ArtifactRef // optional → symmetric_bilinear/1
}

// Family returns FamilyAffineConnection.
func (AffineConnection) Family() FamilyID { return FamilyAffineConnection }

// Encode returns the canonical payload in declaration order.
func (c AffineConnection) Encode() []byte {
	out := c.CoordRef.Encode()
	out = append(out, artifact.EncodeUInt16(c.Dimension)...)
	out = append(out, encodeRefSequence(c.CoefficientRefs)...)
	out = append(out, artifact.EncodeBool(c.ClaimSymmetric)...)
	if c.SourceMetricRef == nil {
		return append(out, artifact.EncodeOptional(false, nil)...)
	}
	return append(out, artifact.EncodeOptional(true, c.SourceMetricRef.Encode())...)
}

// Validate enforces the dim³ count (with overflow guard), dimension match
// against the count, and reference grammar. Coefficient values, symmetry,
// and resolvability belong to store-aware checks.
func (c AffineConnection) Validate() error {
	if c.Dimension < 1 {
		return fmt.Errorf("physmath: connection dimension must be >= 1")
	}
	want := uint64(c.Dimension) * uint64(c.Dimension) * uint64(c.Dimension)
	if want > 1<<24 {
		return fmt.Errorf("physmath: connection shape exceeds Phase-1 bound")
	}
	if uint64(len(c.CoefficientRefs)) != want {
		return fmt.Errorf("physmath: connection coefficients %d != dim^3 %d",
			len(c.CoefficientRefs), want)
	}
	fields := map[string]artifact.ArtifactRef{"AffineConnection.CoordRef": c.CoordRef}
	for i, r := range c.CoefficientRefs {
		fields[fmt.Sprintf("AffineConnection.CoefficientRefs[%d]", i)] = r
	}
	if c.SourceMetricRef != nil {
		fields["AffineConnection.SourceMetricRef"] = *c.SourceMetricRef
	}
	return validateRefGrammars(fields)
}

// TargetChecks: CoordRef → coordinate_system/1; CoefficientRefs →
// expression/1; SourceMetricRef → symmetric_bilinear/1 when present.
func (c AffineConnection) TargetChecks() []TargetCheck {
	checks := []TargetCheck{
		{Field: "AffineConnection.CoordRef", Ref: c.CoordRef, Allowed: []FamilyID{FamilyCoordinateSystem}},
	}
	if c.SourceMetricRef != nil {
		checks = append(checks, TargetCheck{
			Field: "AffineConnection.SourceMetricRef", Ref: *c.SourceMetricRef, Allowed: []FamilyID{FamilySymmetricBilinear},
		})
	}
	for i, r := range c.CoefficientRefs {
		checks = append(checks, TargetCheck{
			Field: fmt.Sprintf("AffineConnection.CoefficientRefs[%d]", i),
			Ref:   r, Allowed: []FamilyID{FamilyExpression},
		})
	}
	return checks
}

// DecodeAffineConnection decodes a physmath/affine_connection/1 payload.
// Representation only.
func DecodeAffineConnection(payload []byte) (AffineConnection, error) {
	d := artifact.NewDecoder(payload)
	coord, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return AffineConnection{}, fmt.Errorf("physmath: connection missing coordinate system: %w", err)
	}
	dim, err := d.UInt16()
	if err != nil {
		return AffineConnection{}, fmt.Errorf("physmath: connection missing dimension: %w", err)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return AffineConnection{}, fmt.Errorf("physmath: connection missing coefficients: %w", err)
	}
	coeffs := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return AffineConnection{}, fmt.Errorf("physmath: connection bad coefficient %d: %w", i, err)
		}
		coeffs = append(coeffs, r)
	}
	sym, err := d.Bool()
	if err != nil {
		return AffineConnection{}, fmt.Errorf("physmath: connection missing symmetry claim: %w", err)
	}
	present, err := d.OptionalPresence()
	if err != nil {
		return AffineConnection{}, fmt.Errorf("physmath: connection missing metric presence: %w", err)
	}
	var metric *artifact.ArtifactRef
	if present {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return AffineConnection{}, fmt.Errorf("physmath: connection bad metric: %w", err)
		}
		metric = &r
	}
	if !d.Exhausted() {
		return AffineConnection{}, fmt.Errorf("physmath: trailing bytes in connection payload")
	}
	c := AffineConnection{CoordRef: coord, Dimension: dim, CoefficientRefs: coeffs, ClaimSymmetric: sym, SourceMetricRef: metric}
	if err := c.Validate(); err != nil {
		return AffineConnection{}, err
	}
	return c, nil
}

// CoordParamMap is the explicit ordered binding between a coordinate
// system's entries and the differentiation parameter identities used by
// DIFFERENTIATE-001. Entry i binds coordinate i; order is semantic.
// It carries no independent family: it is validated inline wherever a
// coordinate-dependent differentiation is invoked.
type CoordParamMap struct {
	Entries []BoundParameterRef // SEQUENCE, one per coordinate, in order
}

// Encode returns the canonical payload.
func (m CoordParamMap) Encode() ([]byte, error) {
	elems := make([][]byte, len(m.Entries))
	for i, e := range m.Entries {
		elems[i] = e.Encode()
	}
	return artifact.EncodeSequence(elems), nil
}

// DecodeCoordParamMap decodes entries in order with exhaustion check.
func DecodeCoordParamMap(payload []byte) (CoordParamMap, error) {
	d := artifact.NewDecoder(payload)
	n, err := d.SequenceCount()
	if err != nil {
		return CoordParamMap{}, fmt.Errorf("physmath: coord-param map missing entries: %w", err)
	}
	entries := make([]BoundParameterRef, 0, n)
	for i := 0; i < n; i++ {
		e, err := DecodeBoundParameterRef(d)
		if err != nil {
			return CoordParamMap{}, fmt.Errorf("physmath: coord-param map bad entry %d: %w", i, err)
		}
		entries = append(entries, e)
	}
	if !d.Exhausted() {
		return CoordParamMap{}, fmt.Errorf("physmath: trailing bytes in coord-param map")
	}
	return CoordParamMap{Entries: entries}, nil
}

// CheckCoordParamMap validates a mapping against a coordinate system and
// the declaring function supplied to differentiation: entry count equals
// chart dimension, every entry references that declaring function, and
// parameter identities are distinct and declared by it. Display names are
// never consulted; reordered or mismatched declarations fail closed.
func CheckCoordParamMap(store Store, chartRef artifact.ArtifactRef, m CoordParamMap, declaringFnRef artifact.ArtifactRef) error {
	cfam, cpayload, err := store.Lookup(chartRef)
	if err != nil {
		return fmt.Errorf("physmath: coord-param chart unresolvable: %w", err)
	}
	if cfam != FamilyCoordinateSystem {
		return fmt.Errorf("physmath: coord-param chart is not coordinate_system/1")
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		return err
	}
	if len(m.Entries) != int(chart.Dimension) {
		return fmt.Errorf("physmath: coord-param entries %d != chart dimension %d: %w",
			len(m.Entries), chart.Dimension, ErrBadInput)
	}
	ffam, fpayload, err := store.Lookup(declaringFnRef)
	if err != nil {
		return fmt.Errorf("physmath: coord-param declaring function unresolvable: %w", err)
	}
	if ffam != FamilyFunction {
		return fmt.Errorf("physmath: coord-param declaring target is not function/1: %w", ErrBadInput)
	}
	fn, err := DecodeFunction(artifact.NewDecoder(fpayload))
	if err != nil {
		return err
	}
	seen := map[ParameterID]bool{}
	declared := map[ParameterID]bool{}
	for _, id := range fn.ParameterIDs {
		declared[id] = true
	}
	for i, e := range m.Entries {
		if e.DeclaringArtifactRef != declaringFnRef {
			return fmt.Errorf("physmath: coord-param entry %d references another function: %w", i, ErrBadInput)
		}
		if !declared[e.ParameterID] {
			return fmt.Errorf("physmath: coord-param entry %d not declared by function: %w", i, ErrBadInput)
		}
		if seen[e.ParameterID] {
			return fmt.Errorf("physmath: coord-param duplicate parameter identity at %d: %w", i, ErrBadInput)
		}
		seen[e.ParameterID] = true
	}
	return nil
}
