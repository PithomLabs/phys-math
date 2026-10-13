package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// Guarded coordinate differentiation (Plan 1.14 prerequisite 2).
//
// Trusted coordinate differentiation in reusable geometry must use
// DifferentiateCoordinate and preserve derivation provenance. Direct calls
// to raw differentiation rules must not be treated as trusted curvature
// evidence without the same validations.
//
// The mathematics is executed by the existing DIFFERENTIATE-001 rule; this
// family records and verifies the trusted operation. It introduces no new
// executable rule family and changes no mathematical semantics.
//
// Provenance contract: the record binds the source expression, the chart,
// the selected coordinate index with its ordered mapping, the
// declaring-function context, the differentiation order and conditions, the
// computed result, and the single-step derivation. Check revalidates every
// relationship from stored artifacts (never trusting cached construction);
// Replay re-executes the recorded DIFFERENTIATE-001 step, so an altered
// differentiation variable either fails the identity gate or fails output
// equality. A consistently rewritten wrong-variable derivation is caught by
// Check, not by Replay alone.
//
// Vacuous differentiation is valid: a validly declared coordinate absent
// from the source differentiates to zero and replays. Residual limitation
// (documented, not fixed): byte-identical (display, PID) symbols under
// different declaring functions are indistinguishable here, as in
// checkDiffTarget. Raw DIFFERENTIATE-001/TOTAL-DERIVATIVE-001 callers
// outside this path remain unprotected by design.

// FamilyCoordinateDifferentiation is the guarded coordinate-differentiation
// record. Family physmath/coordinate_differentiation/1.
var FamilyCoordinateDifferentiation = FamilyID{NamespacePhysMath, "coordinate_differentiation/1", "1"}

// CoordinateDifferentiation binds one trusted chart-coordinate derivative.
type CoordinateDifferentiation struct {
	SourceRef      artifact.ArtifactRef   // → expression/1 (input)
	ChartRef       artifact.ArtifactRef   // → coordinate_system/1
	CoordIndex     uint32                 // selected chart position
	Mapping        CoordParamMap          // inline ordered chart→parameter binding
	DeclaringFnRef artifact.ArtifactRef   // → function/1 (declaring context + ParamNames)
	Order          uint32                 // >= 1, passed to DIFFERENTIATE-001
	ConditionRefs  []artifact.ArtifactRef // SET → condition/1
	ResultRef      artifact.ArtifactRef   // → expression/1 (computed)
	DerivationRef  artifact.ArtifactRef   // → derivation/1 (single DIFFERENTIATE-001 step)
}

// Family returns FamilyCoordinateDifferentiation.
func (CoordinateDifferentiation) Family() FamilyID { return FamilyCoordinateDifferentiation }

// Encode returns the canonical payload in declaration order.
func (c CoordinateDifferentiation) Encode() ([]byte, error) {
	out := c.SourceRef.Encode()
	out = append(out, c.ChartRef.Encode()...)
	out = append(out, artifact.EncodeUInt32(c.CoordIndex)...)
	mapPayload, err := c.Mapping.Encode()
	if err != nil {
		return nil, err
	}
	out = append(out, artifact.EncodeBytes(mapPayload)...)
	out = append(out, c.DeclaringFnRef.Encode()...)
	out = append(out, artifact.EncodeUInt32(c.Order)...)
	set, err := encodeRefSet(c.ConditionRefs)
	if err != nil {
		return nil, err
	}
	out = append(out, set...)
	out = append(out, c.ResultRef.Encode()...)
	return append(out, c.DerivationRef.Encode()...), nil
}

// Validate enforces grammar, Order >= 1, a non-empty mapping with distinct
// PIDs, and condition-set uniqueness. Chart/range/declared-identity gates
// need the store: see CheckCoordinateDifferentiation.
func (c CoordinateDifferentiation) Validate() error {
	if c.Order < 1 {
		return fmt.Errorf("physmath: coordinate differentiation Order must be >= 1: %w", ErrBadParameters)
	}
	if len(c.Mapping.Entries) == 0 {
		return fmt.Errorf("physmath: coordinate differentiation needs a non-empty mapping: %w", ErrBadInput)
	}
	seen := map[ParameterID]bool{}
	for i, e := range c.Mapping.Entries {
		if seen[e.ParameterID] {
			return fmt.Errorf("physmath: coordinate differentiation duplicate parameter identity at %d: %w", i, ErrBadInput)
		}
		seen[e.ParameterID] = true
	}
	fields := map[string]artifact.ArtifactRef{
		"CoordinateDifferentiation.SourceRef":      c.SourceRef,
		"CoordinateDifferentiation.ChartRef":       c.ChartRef,
		"CoordinateDifferentiation.DeclaringFnRef": c.DeclaringFnRef,
		"CoordinateDifferentiation.ResultRef":      c.ResultRef,
		"CoordinateDifferentiation.DerivationRef":  c.DerivationRef,
	}
	for i, r := range c.ConditionRefs {
		fields[fmt.Sprintf("CoordinateDifferentiation.ConditionRefs[%d]", i)] = r
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	return checkDuplicateRefs("CoordinateDifferentiation.ConditionRefs", c.ConditionRefs)
}

// TargetChecks: SourceRef/ResultRef → expression/1; ChartRef →
// coordinate_system/1; DeclaringFnRef → function/1; DerivationRef →
// derivation/1; ConditionRefs → condition/1. Resolved only by the
// store-aware check.
func (c CoordinateDifferentiation) TargetChecks() []TargetCheck {
	checks := []TargetCheck{
		{Field: "CoordinateDifferentiation.SourceRef", Ref: c.SourceRef, Allowed: []FamilyID{FamilyExpression}},
		{Field: "CoordinateDifferentiation.ChartRef", Ref: c.ChartRef, Allowed: []FamilyID{FamilyCoordinateSystem}},
		{Field: "CoordinateDifferentiation.DeclaringFnRef", Ref: c.DeclaringFnRef, Allowed: []FamilyID{FamilyFunction}},
		{Field: "CoordinateDifferentiation.ResultRef", Ref: c.ResultRef, Allowed: []FamilyID{FamilyExpression}},
		{Field: "CoordinateDifferentiation.DerivationRef", Ref: c.DerivationRef, Allowed: []FamilyID{FamilyDerivation}},
	}
	for i, r := range c.ConditionRefs {
		checks = append(checks, TargetCheck{
			Field: fmt.Sprintf("CoordinateDifferentiation.ConditionRefs[%d]", i),
			Ref:   r, Allowed: []FamilyID{FamilyCondition},
		})
	}
	return checks
}

// DecodeCoordinateDifferentiation decodes a
// physmath/coordinate_differentiation/1 payload in declaration order.
// Representation only: no store Lookup, no execution.
func DecodeCoordinateDifferentiation(payload []byte) (CoordinateDifferentiation, error) {
	d := artifact.NewDecoder(payload)
	source, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff missing source: %w", err)
	}
	chart, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff missing chart: %w", err)
	}
	index, err := d.UInt32()
	if err != nil {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff missing coordinate index: %w", err)
	}
	mapPayload, err := d.Bytes()
	if err != nil {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff missing mapping: %w", err)
	}
	mapping, err := DecodeCoordParamMap(mapPayload)
	if err != nil {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff bad mapping: %w", err)
	}
	declaring, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff missing declaring function: %w", err)
	}
	order, err := d.UInt32()
	if err != nil {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff missing order: %w", err)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff missing conditions: %w", err)
	}
	conds := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff bad condition %d: %w", i, err)
		}
		conds = append(conds, r)
	}
	result, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff missing result: %w", err)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: coord-diff missing derivation: %w", err)
	}
	if !d.Exhausted() {
		return CoordinateDifferentiation{}, fmt.Errorf("physmath: trailing bytes in coordinate-differentiation payload")
	}
	c := CoordinateDifferentiation{
		SourceRef: source, ChartRef: chart, CoordIndex: index,
		Mapping: mapping, DeclaringFnRef: declaring, Order: order,
		ConditionRefs: conds, ResultRef: result, DerivationRef: deriv,
	}
	if err := c.Validate(); err != nil {
		return CoordinateDifferentiation{}, err
	}
	return c, nil
}

// checkCoordSelection reasserts the selected-coordinate leg at one index:
// the coordinate symbol's PID equals the mapping entry's PID, the entry
// references the declaring function, and the declared name matches the
// coordinate display. The shared exact-identity enforcement lives in
// ValidateCoordBinding; this is the additional index-specific check.
func checkCoordSelection(store Store, chart CoordinateSystem, mapping CoordParamMap, declaringFnRef artifact.ArtifactRef, fn Function, coordIndex uint32) error {
	if uint64(coordIndex) >= uint64(len(mapping.Entries)) || uint64(coordIndex) >= uint64(chart.Dimension) {
		return fmt.Errorf("physmath: coordinate index %d out of range (dim %d, entries %d): %w",
			coordIndex, chart.Dimension, len(mapping.Entries), ErrBadInput)
	}
	coord, err := resolveExpression(store, chart.CoordinateRefs[coordIndex])
	if err != nil {
		return fmt.Errorf("physmath: selected coordinate %d unresolvable: %w", coordIndex, err)
	}
	if coord.Kind != TagSymbol || coord.ParamID == nil {
		return fmt.Errorf("physmath: selected coordinate %d is not a bound symbol: %w", coordIndex, ErrBadInput)
	}
	entry := mapping.Entries[coordIndex]
	if entry.DeclaringArtifactRef != declaringFnRef {
		return fmt.Errorf("physmath: selected mapping entry %d references another function: %w", coordIndex, ErrBadInput)
	}
	if entry.ParameterID != *coord.ParamID {
		return fmt.Errorf("physmath: selected coordinate PID mismatch at index %d: coordinate %q binds another identity than the mapping entry: %w",
			coordIndex, coord.DisplayName, ErrBadInput)
	}
	slot := -1
	for i, id := range fn.ParameterIDs {
		if id == entry.ParameterID {
			slot = i
			break
		}
	}
	if slot < 0 {
		return fmt.Errorf("physmath: selected parameter not declared by function: %w", ErrBadInput)
	}
	if len(fn.ParamNames) != len(fn.ParameterIDs) || fn.ParamNames[slot] != coord.DisplayName {
		return fmt.Errorf("physmath: declared parameter name does not match chart coordinate %q at index %d: %w",
			coord.DisplayName, coordIndex, ErrBadInput)
	}
	return nil
}

// coordDiffParams builds the DIFFERENTIATE-001 parameters for one selection.
func coordDiffParams(entry BoundParameterRef, order uint32) []byte {
	return append(entry.Encode(), artifact.EncodeUInt32(order)...)
}

// DifferentiateCoordinate executes one trusted chart-coordinate derivative.
//
// It establishes source identity, chart identity, selected-coordinate and
// mapped-parameter identity, declaring-function and parameter-name
// metadata, and the differentiation request passed to DIFFERENTIATE-001,
// then records the result with its single-step derivation in a
// coordinate_differentiation/1 artifact. It returns the result expression
// reference, the record reference, and the replayable derivation.
//
// Vacuous differentiation stays valid: a validly declared coordinate absent
// from the source differentiates to zero. If any required relationship
// cannot be established from stored artifacts, it fails closed.
func DifferentiateCoordinate(store Store, sourceRef, chartRef artifact.ArtifactRef, coordIndex uint32, mapping CoordParamMap, declaringFnRef artifact.ArtifactRef, order uint32, conds []artifact.ArtifactRef) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, artifact.ArtifactRef{}, Derivation{}, err
	}
	if order < 1 {
		return fail(fmt.Errorf("physmath: coordinate differentiation Order must be >= 1: %w", ErrBadParameters))
	}
	if err := ValidateCoordBinding(store, chartRef, mapping, declaringFnRef, []artifact.ArtifactRef{sourceRef}); err != nil {
		return fail(err)
	}
	_, cpayload, err := store.Lookup(chartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: coord-diff chart unresolvable: %w", err))
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		return fail(err)
	}
	fn, err := resolveFunction(store, declaringFnRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: coord-diff declaring function unresolvable: %w", err))
	}
	if err := checkCoordSelection(store, chart, mapping, declaringFnRef, fn, coordIndex); err != nil {
		return fail(err)
	}
	params := coordDiffParams(mapping.Entries[coordIndex], order)
	res, err := Apply(store, Invocation{
		ProducerNamespace: NamespacePhysMath, RuleID: RuleDifferentiate,
		InputRefs:     []artifact.ArtifactRef{sourceRef, declaringFnRef},
		ConditionRefs: conds,
		Parameters:    params,
	})
	if err != nil {
		return fail(fmt.Errorf("physmath: guarded coordinate differentiation failed: %w", err))
	}
	d := Derivation{
		PremiseRefs: []artifact.ArtifactRef{sourceRef, declaringFnRef},
		Steps: []DerivationStep{{
			Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleDifferentiate,
			InputRefs:     []artifact.ArtifactRef{sourceRef, declaringFnRef},
			OutputRef:     res.OutputRef,
			ConditionRefs: normalizeConditionRefs(conds),
			Parameters:    params,
		}},
		ResultRefs: []artifact.ArtifactRef{res.OutputRef},
	}
	if err := Replay(store, d, nil); err != nil {
		return fail(fmt.Errorf("physmath: coord-diff derivation does not replay: %w", err))
	}
	dpayload, err := d.Encode()
	if err != nil {
		return fail(err)
	}
	derivRef, err := store.Put(FamilyDerivation, dpayload)
	if err != nil {
		return fail(err)
	}
	rec := CoordinateDifferentiation{
		SourceRef: sourceRef, ChartRef: chartRef, CoordIndex: coordIndex,
		Mapping: mapping, DeclaringFnRef: declaringFnRef, Order: order,
		ConditionRefs: normalizeConditionRefs(conds),
		ResultRef:     res.OutputRef, DerivationRef: derivRef,
	}
	if err := rec.Validate(); err != nil {
		return fail(err)
	}
	rpayload, err := rec.Encode()
	if err != nil {
		return fail(err)
	}
	recRef, err := store.Put(FamilyCoordinateDifferentiation, rpayload)
	if err != nil {
		return fail(err)
	}
	return res.OutputRef, recRef, d, nil
}

// CheckCoordinateDifferentiation is the store-aware provenance gate: family
// resolution, record/derivation shape with exact linkage (single
// DIFFERENTIATE-001 step consuming source and declaring function with the
// mapped BPR and order), revalidation of the full chart binding and the
// selected-coordinate leg from stored artifacts, and replay before trust.
func CheckCoordinateDifferentiation(store Store, ref artifact.ArtifactRef) error {
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return fmt.Errorf("physmath: coord-diff record unresolvable: %w", err)
	}
	if fam != FamilyCoordinateDifferentiation {
		return fmt.Errorf("physmath: target is not coordinate_differentiation/1: %w", ErrBadInput)
	}
	rec, err := DecodeCoordinateDifferentiation(payload)
	if err != nil {
		return err
	}
	for _, tc := range rec.TargetChecks() {
		tfam, _, err := store.Lookup(tc.Ref)
		if err != nil {
			return fmt.Errorf("physmath: %s unresolvable: %w", tc.Field, err)
		}
		ok := false
		for _, a := range tc.Allowed {
			if tfam == a {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("physmath: %s targets %s/%s/%s, not in closed target set",
				tc.Field, tfam.Namespace, tfam.ArtifactType, tfam.SchemaVersion)
		}
	}
	dfam, dpayload, err := store.Lookup(rec.DerivationRef)
	if err != nil {
		return fmt.Errorf("physmath: coord-diff derivation unresolvable: %w", err)
	}
	if dfam != FamilyDerivation {
		return fmt.Errorf("physmath: coord-diff derivation is not derivation/1: %w", ErrBadInput)
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return err
	}
	if len(d.PremiseRefs) != 2 || len(d.ResultRefs) != 1 || len(d.Steps) != 1 {
		return fmt.Errorf("physmath: coord-diff derivation must carry two premises, one step, one result")
	}
	if d.ResultRefs[0] != rec.ResultRef {
		return fmt.Errorf("physmath: coord-diff derivation result does not match the record")
	}
	s := d.Steps[0]
	if s.Ordinal != 0 || s.ProducerNamespace != NamespacePhysMath || s.RuleID != RuleDifferentiate {
		return fmt.Errorf("physmath: coord-diff step must be ordinal-0 DIFFERENTIATE-001")
	}
	if len(s.InputRefs) != 2 || s.InputRefs[0] != rec.SourceRef || s.InputRefs[1] != rec.DeclaringFnRef {
		return fmt.Errorf("physmath: coord-diff step linkage broken")
	}
	wantParams := coordDiffParams(rec.Mapping.Entries[rec.CoordIndex], rec.Order)
	if string(s.Parameters) != string(wantParams) {
		return fmt.Errorf("physmath: coord-diff step parameters mismatch the recorded selection")
	}
	if !equalRefSets(s.ConditionRefs, rec.ConditionRefs) {
		return fmt.Errorf("physmath: coord-diff condition sets differ from the record")
	}
	if s.OutputRef != rec.ResultRef {
		return fmt.Errorf("physmath: coord-diff step output does not match the record")
	}
	// Index well-formedness before the shared binding revalidation.
	if uint64(rec.CoordIndex) >= uint64(len(rec.Mapping.Entries)) {
		return fmt.Errorf("physmath: coord-diff index %d out of mapping range: %w", rec.CoordIndex, ErrBadInput)
	}
	if err := ValidateCoordBinding(store, rec.ChartRef, rec.Mapping, rec.DeclaringFnRef, []artifact.ArtifactRef{rec.SourceRef}); err != nil {
		return err
	}
	_, cpayload, err := store.Lookup(rec.ChartRef)
	if err != nil {
		return fmt.Errorf("physmath: coord-diff chart unresolvable: %w", err)
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		return err
	}
	fn, err := resolveFunction(store, rec.DeclaringFnRef)
	if err != nil {
		return fmt.Errorf("physmath: coord-diff declaring function unresolvable: %w", err)
	}
	if err := checkCoordSelection(store, chart, rec.Mapping, rec.DeclaringFnRef, fn, rec.CoordIndex); err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: coord-diff derivation does not replay: %w", err)
	}
	return nil
}

// ReplayCoordinateDifferentiation re-executes the recorded derivation. It
// inherits the DIFFERENTIATE-001 identity contract because Replay
// re-applies the rule per step and requires exact output equality. It is
// the re-execution gate, not the provenance gate: use
// CheckCoordinateDifferentiation to also bind the selection to the chart,
// mapping, and declaring context.
func ReplayCoordinateDifferentiation(store Store, ref artifact.ArtifactRef) error {
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return fmt.Errorf("physmath: coord-diff record unresolvable: %w", err)
	}
	if fam != FamilyCoordinateDifferentiation {
		return fmt.Errorf("physmath: target is not coordinate_differentiation/1: %w", ErrBadInput)
	}
	rec, err := DecodeCoordinateDifferentiation(payload)
	if err != nil {
		return err
	}
	_, dpayload, err := store.Lookup(rec.DerivationRef)
	if err != nil {
		return fmt.Errorf("physmath: coord-diff derivation unresolvable: %w", err)
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return err
	}
	return Replay(store, d, nil)
}
