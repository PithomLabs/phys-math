package physmath

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/PithomLabs/phys-artifact"
)

// Contracted Bianchi evaluated obligation (Plan 1.19; zero evidence bound
// in Plan 1.21):
//
//	nabla^mu G_mu_nu = 0.
//
// This record states the identity as an evaluated obligation over explicit
// divergence-evaluation artifacts. It binds the canonical claim, its
// declared scope, the ordered evaluated instances, their observed results,
// the union of their conditions, one composite derivation, per-instance
// zero-obligation witnesses, and a validated support status. It does not
// establish a general proof, implement field equations, or introduce
// dynamics.
//
// Evidence semantics: instance verification reuses the existing
// riemann_trace_reverse_divergence/1 records and their Check/Replay gates.
// The vanishing claim is additionally bound to per-instance closing
// derivations over the actual result components: every component has its
// own traceable closure path terminating in literal zero through the
// authorized rules, and the shared validator verifies the complete
// correspondence. Symbolic or structural identities, hand-verified factored
// cancellation, and numerical point-evaluation corroboration keep the
// evidence type they were established with; replay success alone never
// promotes an instance result into a universal claim. A POINTWISE witness
// establishes vanishing at its recorded coordinate assignment only, never
// symbolic identity over the geometry.
//
// No new executable rule family is introduced.

// FamilyRiemannTraceReverseDivergenceObligation is the evaluated-obligation
// record: a durable, replayable binding of evaluated contracted-divergence
// instances to the stated identity with explicit support status and scope.
// Family physmath/riemann_trace_reverse_divergence_obligation/1.
var FamilyRiemannTraceReverseDivergenceObligation = FamilyID{NamespacePhysMath, "riemann_trace_reverse_divergence_obligation/1", "1"}

// Canonical obligation claim: the divergence of the trace-reversed tensor
// vanishes. Stored verbatim so Check can reject claim substitution.
const ObligationStatementDivergenceVanishes = "divergence of trace-reversed tensor vanishes"

// Canonical obligation scope: only the referenced evaluated instances are
// covered. Stored verbatim so Check can reject scope broadening.
const ObligationScopeEvaluatedInstances = "evaluated instances only; no general proof"

// ObligationPartiallySupported is the validated support status for this
// milestone: the identity has been evaluated and verified on the
// explicitly referenced geometries and conditions, without a general
// proof. Any other status value is rejected.
const ObligationPartiallySupported = "PARTIALLY_SUPPORTED"

// Witness evidence classes for one evaluated instance. EXACT_SYMBOLIC
// means every result component closes to literal zero through CANON and
// SIMPLIFY with no substitution. POINTWISE means at least one component
// closes only after substitution at the recorded caller-supplied
// assignment, and establishes vanishing at that assignment only — never
// symbolic identity over the geometry.
const (
	ObligationWitnessSymbolic  = "EXACT_SYMBOLIC"
	ObligationWitnessPointwise = "POINTWISE"
)

// ObligationEvaluationPoint is one caller-supplied coordinate assignment
// for pointwise closure: the parameter to substitute and the exact value
// expression to substitute for it. A complete assignment set for one
// instance is a slice of these; nil (or empty) means symbolic-only demand.
type ObligationEvaluationPoint struct {
	ParameterID ParameterID
	Value       Expr
}

// RiemannTraceReverseDivergenceObligation links evaluated
// riemann_trace_reverse_divergence/1 instances to the stated identity.
// Validate checks structure only;
// CheckRiemannTraceReverseDivergenceObligation resolves and recomputes.
type RiemannTraceReverseDivergenceObligation struct {
	Statement      string                 // canonical claim, must equal ObligationStatementDivergenceVanishes
	Scope          string                 // declared scope, must equal ObligationScopeEvaluatedInstances
	Support        string                 // validated status, must equal ObligationPartiallySupported
	DivergenceRefs []artifact.ArtifactRef // SEQUENCE, ordered evaluated instances
	ResultRefs     []artifact.ArtifactRef // SEQUENCE, observed results parallel to DivergenceRefs
	ConditionRefs  []artifact.ArtifactRef // SET, union of instance conditions
	WitnessRefs    []artifact.ArtifactRef // SEQUENCE, closing derivations parallel to DivergenceRefs
	WitnessKinds   []string               // SEQUENCE, evidence class parallel to DivergenceRefs
	DerivationRef  artifact.ArtifactRef   // required → derivation/1 (composite)
}

// Family returns FamilyRiemannTraceReverseDivergenceObligation.
func (RiemannTraceReverseDivergenceObligation) Family() FamilyID {
	return FamilyRiemannTraceReverseDivergenceObligation
}

// Encode returns the canonical payload in declaration order.
func (r RiemannTraceReverseDivergenceObligation) Encode() ([]byte, error) {
	statement, err := artifact.EncodeString(r.Statement)
	if err != nil {
		return nil, err
	}
	scope, err := artifact.EncodeString(r.Scope)
	if err != nil {
		return nil, err
	}
	support, err := artifact.EncodeString(r.Support)
	if err != nil {
		return nil, err
	}
	out := append(statement, scope...)
	out = append(out, support...)
	out = append(out, encodeRefSequence(r.DivergenceRefs)...)
	out = append(out, encodeRefSequence(r.ResultRefs)...)
	set, err := encodeRefSet(r.ConditionRefs)
	if err != nil {
		return nil, err
	}
	out = append(out, set...)
	out = append(out, encodeRefSequence(r.WitnessRefs)...)
	kindElems := make([][]byte, len(r.WitnessKinds))
	for i, k := range r.WitnessKinds {
		enc, err := artifact.EncodeString(k)
		if err != nil {
			return nil, err
		}
		kindElems[i] = enc
	}
	out = append(out, artifact.EncodeSequence(kindElems)...)
	return append(out, r.DerivationRef.Encode()...), nil
}

// Validate enforces the canonical claim and scope, the validated support
// status, non-empty distinct instances with parallel results, and condition
// uniqueness. Instance and derivation linkage need the store: see
// CheckRiemannTraceReverseDivergenceObligation.
func (r RiemannTraceReverseDivergenceObligation) Validate() error {
	if r.Statement != ObligationStatementDivergenceVanishes {
		return fmt.Errorf("physmath: obligation statement mismatch: %w", ErrBadInput)
	}
	if r.Scope != ObligationScopeEvaluatedInstances {
		return fmt.Errorf("physmath: obligation scope mismatch: %w", ErrBadInput)
	}
	if r.Support != ObligationPartiallySupported {
		return fmt.Errorf("physmath: obligation support %q is not %q: %w", r.Support, ObligationPartiallySupported, ErrBadInput)
	}
	if len(r.DivergenceRefs) == 0 {
		return fmt.Errorf("physmath: obligation needs at least one evaluated instance: %w", ErrBadInput)
	}
	if len(r.ResultRefs) != len(r.DivergenceRefs) {
		return fmt.Errorf("physmath: obligation results %d != instances %d: %w", len(r.ResultRefs), len(r.DivergenceRefs), ErrBadInput)
	}
	if len(r.WitnessRefs) != len(r.DivergenceRefs) {
		return fmt.Errorf("physmath: obligation witnesses %d != instances %d: %w", len(r.WitnessRefs), len(r.DivergenceRefs), ErrBadInput)
	}
	if len(r.WitnessKinds) != len(r.DivergenceRefs) {
		return fmt.Errorf("physmath: obligation witness kinds %d != instances %d: %w", len(r.WitnessKinds), len(r.DivergenceRefs), ErrBadInput)
	}
	for i, k := range r.WitnessKinds {
		if k != ObligationWitnessSymbolic && k != ObligationWitnessPointwise {
			return fmt.Errorf("physmath: obligation witness kind %d %q is not a validated evidence class: %w", i, k, ErrBadInput)
		}
	}
	if err := checkDuplicateRefs("RiemannTraceReverseDivergenceObligation.DivergenceRefs", r.DivergenceRefs); err != nil {
		return err
	}
	fields := map[string]artifact.ArtifactRef{
		"RiemannTraceReverseDivergenceObligation.DerivationRef": r.DerivationRef,
	}
	for i, ref := range r.DivergenceRefs {
		fields[getRefFieldName("RiemannTraceReverseDivergenceObligation.DivergenceRefs", i)] = ref
	}
	for i, ref := range r.ResultRefs {
		fields[getRefFieldName("RiemannTraceReverseDivergenceObligation.ResultRefs", i)] = ref
	}
	for i, ref := range r.WitnessRefs {
		fields[getRefFieldName("RiemannTraceReverseDivergenceObligation.WitnessRefs", i)] = ref
	}
	for i, c := range r.ConditionRefs {
		fields[getRefFieldName("RiemannTraceReverseDivergenceObligation.ConditionRefs", i)] = c
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	return checkDuplicateRefs("RiemannTraceReverseDivergenceObligation.ConditionRefs", r.ConditionRefs)
}

// TargetChecks: DivergenceRefs → riemann_trace_reverse_divergence/1;
// ResultRefs → tensor_field/1; ConditionRefs → condition/1; WitnessRefs
// → derivation/1; DerivationRef → derivation/1. Resolved only by the
// store-aware check.
func (r RiemannTraceReverseDivergenceObligation) TargetChecks() []TargetCheck {
	checks := make([]TargetCheck, 0, len(r.DivergenceRefs)+len(r.ResultRefs)+len(r.ConditionRefs)+len(r.WitnessRefs)+1)
	for i, ref := range r.DivergenceRefs {
		checks = append(checks, TargetCheck{
			Field:   getRefFieldName("RiemannTraceReverseDivergenceObligation.DivergenceRefs", i),
			Ref:     ref,
			Allowed: []FamilyID{FamilyRiemannTraceReverseDivergence},
		})
	}
	for i, ref := range r.ResultRefs {
		checks = append(checks, TargetCheck{
			Field:   getRefFieldName("RiemannTraceReverseDivergenceObligation.ResultRefs", i),
			Ref:     ref,
			Allowed: []FamilyID{FamilyTensorField},
		})
	}
	for i, c := range r.ConditionRefs {
		checks = append(checks, TargetCheck{
			Field:   getRefFieldName("RiemannTraceReverseDivergenceObligation.ConditionRefs", i),
			Ref:     c,
			Allowed: []FamilyID{FamilyCondition},
		})
	}
	for i, ref := range r.WitnessRefs {
		checks = append(checks, TargetCheck{
			Field:   getRefFieldName("RiemannTraceReverseDivergenceObligation.WitnessRefs", i),
			Ref:     ref,
			Allowed: []FamilyID{FamilyDerivation},
		})
	}
	return append(checks, TargetCheck{
		Field:   "RiemannTraceReverseDivergenceObligation.DerivationRef",
		Ref:     r.DerivationRef,
		Allowed: []FamilyID{FamilyDerivation},
	})
}

// DecodeRiemannTraceReverseDivergenceObligation decodes a
// physmath/riemann_trace_reverse_divergence_obligation/1 payload.
// Representation only.
func DecodeRiemannTraceReverseDivergenceObligation(payload []byte) (RiemannTraceReverseDivergenceObligation, error) {
	d := artifact.NewDecoder(payload)
	statement, err := d.String()
	if err != nil {
		return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation missing statement: %w", err)
	}
	scope, err := d.String()
	if err != nil {
		return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation missing scope: %w", err)
	}
	support, err := d.String()
	if err != nil {
		return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation missing support: %w", err)
	}
	nd, err := d.SequenceCount()
	if err != nil {
		return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation missing instances: %w", err)
	}
	divergence := make([]artifact.ArtifactRef, 0, nd)
	for i := 0; i < nd; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation bad instance %d: %w", i, err)
		}
		divergence = append(divergence, r)
	}
	nr, err := d.SequenceCount()
	if err != nil {
		return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation missing results: %w", err)
	}
	results := make([]artifact.ArtifactRef, 0, nr)
	for i := 0; i < nr; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation bad result %d: %w", i, err)
		}
		results = append(results, r)
	}
	nc, err := d.SequenceCount()
	if err != nil {
		return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation missing conditions: %w", err)
	}
	conds := make([]artifact.ArtifactRef, 0, nc)
	for i := 0; i < nc; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation bad condition %d: %w", i, err)
		}
		conds = append(conds, r)
	}
	nw, err := d.SequenceCount()
	if err != nil {
		return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation missing witnesses: %w", err)
	}
	witnesses := make([]artifact.ArtifactRef, 0, nw)
	for i := 0; i < nw; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation bad witness %d: %w", i, err)
		}
		witnesses = append(witnesses, r)
	}
	nk, err := d.SequenceCount()
	if err != nil {
		return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation missing witness kinds: %w", err)
	}
	kinds := make([]string, 0, nk)
	for i := 0; i < nk; i++ {
		k, err := d.String()
		if err != nil {
			return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation bad witness kind %d: %w", i, err)
		}
		kinds = append(kinds, k)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: obligation missing derivation: %w", err)
	}
	if !d.Exhausted() {
		return RiemannTraceReverseDivergenceObligation{}, fmt.Errorf("physmath: trailing bytes in obligation payload")
	}
	r := RiemannTraceReverseDivergenceObligation{
		Statement: statement, Scope: scope, Support: support,
		DivergenceRefs: divergence, ResultRefs: results, ConditionRefs: conds,
		WitnessRefs: witnesses, WitnessKinds: kinds,
		DerivationRef: deriv,
	}
	if err := r.Validate(); err != nil {
		return RiemannTraceReverseDivergenceObligation{}, err
	}
	return r, nil
}

// collectSymbolPIDs returns the set of bound parameter identities occurring
// anywhere in e. Only symbols carrying a ParameterID are substitutable
// coordinates; constants and ephemeral designators are ignored.
func collectSymbolPIDs(e Expr) map[ParameterID]bool {
	out := map[ParameterID]bool{}
	var walk func(x Expr)
	walk = func(x Expr) {
		if x.Kind == TagSymbol && x.ParamID != nil {
			out[*x.ParamID] = true
		}
		for _, c := range x.Children {
			walk(c)
		}
		if x.Operand != nil {
			walk(*x.Operand)
		}
		if x.Base != nil {
			walk(*x.Base)
		}
		if x.Exp != nil {
			walk(*x.Exp)
		}
		if x.Radicand != nil {
			walk(*x.Radicand)
		}
		for _, a := range x.Args {
			walk(a)
		}
		if x.Left != nil {
			walk(*x.Left)
		}
		if x.Right != nil {
			walk(*x.Right)
		}
		for _, b := range x.Branches {
			walk(b)
		}
	}
	walk(e)
	return out
}

// witnessRuleAllowed reports whether rule is authorized inside a zero
// witness: normalization and caller-supplied substitution only. In
// particular DIFFERENTIATE is excluded: a derivative that happens to
// vanish cannot establish that its source component vanishes.
func witnessRuleAllowed(producer, rule string) bool {
	if producer != NamespacePhysMath {
		return false
	}
	return rule == RuleExprCanon || rule == RuleExprSimplify || rule == RuleExprSubstitute
}

// witnessSubstituteParams encodes one SUBSTITUTE parameter list from an
// explicit assignment set: entries sorted by canonical bytes, one entry
// per parameter. Values are minted by the caller; refs embedded here are
// durable evidence of the exact assignment used.
func witnessSubstituteParams(fn artifact.ArtifactRef, point []ObligationEvaluationPoint, values map[ParameterID]artifact.ArtifactRef) ([]byte, error) {
	elems := make([][]byte, 0, len(point))
	seen := map[ParameterID]bool{}
	for _, pt := range point {
		if seen[pt.ParameterID] {
			return nil, fmt.Errorf("physmath: witness assignment duplicates parameter: %w", ErrBadInput)
		}
		seen[pt.ParameterID] = true
		vref, ok := values[pt.ParameterID]
		if !ok {
			return nil, fmt.Errorf("physmath: witness assignment has no minted value: %w", ErrBadInput)
		}
		entry := append(BoundParameterRef{DeclaringArtifactRef: fn, ParameterID: pt.ParameterID}.Encode(), vref.Encode()...)
		elems = append(elems, entry)
	}
	sort.Slice(elems, func(i, j int) bool { return bytes.Compare(elems[i], elems[j]) < 0 })
	return artifact.EncodeSequence(elems), nil
}

// buildComponentClosure derives the closure path for one result component:
// CANON+SIMPLIFY when they close to literal zero (symbolic evidence),
// otherwise SUBSTITUTE at the caller-supplied assignment set followed by
// CANON+SIMPLIFY (pointwise evidence). Steps use ordinal placeholders for
// the caller to renumber. It returns the steps, their root premises and
// outputs, and whether substitution was needed. Failure is honest: a
// component that closes under neither regime is reported, never accepted.
func buildComponentClosure(store Store, compRef, declaringFnRef artifact.ArtifactRef, mappingPIDs map[ParameterID]bool, point []ObligationEvaluationPoint) (steps []DerivationStep, premises []artifact.ArtifactRef, results []artifact.ArtifactRef, pointwise bool, err error) {
	fail := func(err error) ([]DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, bool, error) {
		return nil, nil, nil, false, err
	}
	apply := func(rule string, inputs []artifact.ArtifactRef, params []byte) (artifact.ArtifactRef, error) {
		res, err := Apply(store, Invocation{
			ProducerNamespace: NamespacePhysMath, RuleID: rule,
			InputRefs: inputs, Parameters: params,
		})
		if err != nil {
			return artifact.ArtifactRef{}, err
		}
		return res.OutputRef, nil
	}
	canonOut, err := apply(RuleExprCanon, []artifact.ArtifactRef{compRef}, nil)
	if err != nil {
		return fail(fmt.Errorf("physmath: witness canon failed: %w", err))
	}
	simpOut, err := apply(RuleExprSimplify, []artifact.ArtifactRef{canonOut}, nil)
	if err != nil {
		return fail(fmt.Errorf("physmath: witness simplify failed: %w", err))
	}
	closed, err := resolveExpression(store, simpOut)
	if err != nil {
		return fail(err)
	}
	if closed.Equal(MustRational(0, 1)) {
		steps = []DerivationStep{
			{Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
				InputRefs: []artifact.ArtifactRef{compRef}, OutputRef: canonOut},
			{Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
				InputRefs: []artifact.ArtifactRef{canonOut}, OutputRef: simpOut},
		}
		return steps, []artifact.ArtifactRef{compRef}, []artifact.ArtifactRef{canonOut, simpOut}, false, nil
	}
	// Symbolic closure failed: the caller-supplied assignment set is the
	// only remaining regime. Absent means fail closed, never search.
	if len(point) == 0 {
		return fail(fmt.Errorf("physmath: witness component does not close symbolically and no evaluation assignment supplied: %w", ErrBadInput))
	}
	setPIDs := map[ParameterID]bool{}
	for _, pt := range point {
		if setPIDs[pt.ParameterID] {
			return fail(fmt.Errorf("physmath: witness assignment duplicates parameter: %w", ErrBadInput))
		}
		setPIDs[pt.ParameterID] = true
		if !mappingPIDs[pt.ParameterID] {
			return fail(fmt.Errorf("physmath: witness assignment out of instance scope: %w", ErrBadInput))
		}
	}
	comp, err := resolveExpression(store, compRef)
	if err != nil {
		return fail(err)
	}
	for pid := range collectSymbolPIDs(comp) {
		if !setPIDs[pid] {
			return fail(fmt.Errorf("physmath: witness assignment missing coordinate: %w", ErrBadInput))
		}
	}
	values := make(map[ParameterID]artifact.ArtifactRef, len(point))
	var valueRefs []artifact.ArtifactRef
	for _, pt := range point {
		vref, err := PutExpression(store, pt.Value)
		if err != nil {
			return fail(err)
		}
		values[pt.ParameterID] = vref
		valueRefs = append(valueRefs, vref)
	}
	params, err := witnessSubstituteParams(declaringFnRef, point, values)
	if err != nil {
		return fail(err)
	}
	subOut, err := apply(RuleExprSubstitute, []artifact.ArtifactRef{compRef, declaringFnRef}, params)
	if err != nil {
		return fail(fmt.Errorf("physmath: witness substitute failed: %w", err))
	}
	canonOut, err = apply(RuleExprCanon, []artifact.ArtifactRef{subOut}, nil)
	if err != nil {
		return fail(fmt.Errorf("physmath: witness canon failed: %w", err))
	}
	simpOut, err = apply(RuleExprSimplify, []artifact.ArtifactRef{canonOut}, nil)
	if err != nil {
		return fail(fmt.Errorf("physmath: witness simplify failed: %w", err))
	}
	closed, err = resolveExpression(store, simpOut)
	if err != nil {
		return fail(err)
	}
	if !closed.Equal(MustRational(0, 1)) {
		return fail(fmt.Errorf("physmath: witness component does not vanish at the supplied assignment: %w", ErrBadInput))
	}
	steps = []DerivationStep{
		{Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSubstitute,
			InputRefs: []artifact.ArtifactRef{compRef, declaringFnRef}, OutputRef: subOut, Parameters: params},
		{Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
			InputRefs: []artifact.ArtifactRef{subOut}, OutputRef: canonOut},
		{Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
			InputRefs: []artifact.ArtifactRef{canonOut}, OutputRef: simpOut},
	}
	premises = append([]artifact.ArtifactRef{compRef, declaringFnRef}, valueRefs...)
	return steps, premises, []artifact.ArtifactRef{subOut, canonOut, simpOut}, true, nil
}

// buildInstanceWitness derives the closing derivation for one instance:
// one closure path per result component, in component order, with local
// ordinals for the caller to renumber. It replays standalone before
// returning, so only genuinely closing evidence is ever minted.
func buildInstanceWitness(store Store, compRefs []artifact.ArtifactRef, declaringFnRef artifact.ArtifactRef, mapping CoordParamMap, point []ObligationEvaluationPoint) (Derivation, bool, error) {
	fail := func(err error) (Derivation, bool, error) {
		return Derivation{}, false, err
	}
	if len(compRefs) == 0 {
		return fail(fmt.Errorf("physmath: witness needs at least one result component: %w", ErrBadInput))
	}
	mappingPIDs := make(map[ParameterID]bool, len(mapping.Entries))
	for _, e := range mapping.Entries {
		mappingPIDs[e.ParameterID] = true
	}
	var steps []DerivationStep
	var premises []artifact.ArtifactRef
	var results []artifact.ArtifactRef
	pointwise := false
	for j, comp := range compRefs {
		csteps, cpremises, cresults, cpointwise, err := buildComponentClosure(store, comp, declaringFnRef, mappingPIDs, point)
		if err != nil {
			return fail(fmt.Errorf("physmath: witness component %d: %w", j, err))
		}
		steps = append(steps, csteps...)
		premises = append(premises, cpremises...)
		results = append(results, cresults...)
		pointwise = pointwise || cpointwise
	}
	for i := range steps {
		steps[i].Ordinal = uint32(i)
	}
	dd := Derivation{
		PremiseRefs: uniqueRefs(premises),
		Steps:       steps,
		ResultRefs:  uniqueRefs(results),
	}
	if err := Replay(store, dd, nil); err != nil {
		return fail(fmt.Errorf("physmath: witness derivation does not replay: %w", err))
	}
	return dd, pointwise, nil
}

// EvaluateRiemannTraceReverseDivergenceObligation records the contracted
// Bianchi identity as an evaluated obligation over verified divergence
// instances. Every instance must pass CheckRiemannTraceReverseDivergence;
// the composite derivation concatenates the instance derivations with
// contiguous ordinals in instance order. Caller-supplied points select the
// pointwise regime per instance: points[i] is the complete explicit
// assignment set for instance i, or nil for symbolic-only demand (fail
// closed when symbolic closure does not hold). Each instance additionally
// carries a closing witness derivation over its actual result components;
// witness blocks follow their instance blocks in the composite derivation.
// It returns the minted obligation record reference and the composite
// derivation. Support is always PARTIALLY_SUPPORTED: evaluated instances,
// never a general proof.
func EvaluateRiemannTraceReverseDivergenceObligation(store Store, divergenceRefs []artifact.ArtifactRef, points [][]ObligationEvaluationPoint) (artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, Derivation{}, err
	}
	if len(divergenceRefs) == 0 {
		return fail(fmt.Errorf("physmath: obligation needs at least one evaluated instance: %w", ErrBadInput))
	}
	if len(points) != len(divergenceRefs) {
		return fail(fmt.Errorf("physmath: obligation points %d != instances %d: %w", len(points), len(divergenceRefs), ErrBadInput))
	}
	seen := map[artifact.ArtifactRef]bool{}
	for _, ref := range divergenceRefs {
		if seen[ref] {
			return fail(fmt.Errorf("physmath: obligation duplicate evaluated instance: %w", ErrBadInput))
		}
		seen[ref] = true
	}
	var steps []DerivationStep
	var premises []artifact.ArtifactRef
	var results []artifact.ArtifactRef
	resultRefs := make([]artifact.ArtifactRef, 0, len(divergenceRefs))
	var conditions []artifact.ArtifactRef
	var witnessRefs []artifact.ArtifactRef
	var witnessKinds []string
	for i, ref := range divergenceRefs {
		if err := CheckRiemannTraceReverseDivergence(store, ref); err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d rejected: %w", i, err))
		}
		fam, payload, err := store.Lookup(ref)
		if err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d unresolvable: %w", i, err))
		}
		if fam != FamilyRiemannTraceReverseDivergence {
			return fail(fmt.Errorf("physmath: obligation instance %d is not riemann_trace_reverse_divergence/1: %w", i, ErrBadInput))
		}
		rec, err := DecodeRiemannTraceReverseDivergenceEvaluation(payload)
		if err != nil {
			return fail(err)
		}
		_, dpayload, err := store.Lookup(rec.DerivationRef)
		if err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d derivation unresolvable: %w", i, err))
		}
		dd, err := DecodeDerivation(dpayload)
		if err != nil {
			return fail(err)
		}
		steps = append(steps, dd.Steps...)
		premises = append(premises, ref, rec.ResultRef)
		premises = append(premises, dd.PremiseRefs...)
		results = append(results, dd.ResultRefs...)
		resultRefs = append(resultRefs, rec.ResultRef)
		conditions = append(conditions, rec.ConditionRefs...)
		// Zero-obligation witness over the actual result components.
		_, rfPayload, err := store.Lookup(rec.ResultRef)
		if err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d result unresolvable: %w", i, err))
		}
		rfield, err := DecodeTensorField(rfPayload)
		if err != nil {
			return fail(err)
		}
		_, rtPayload, err := store.Lookup(rfield.TensorRef)
		if err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d result tensor unresolvable: %w", i, err))
		}
		rtensor, err := DecodeTensor(rtPayload)
		if err != nil {
			return fail(err)
		}
		witness, pointwise, err := buildInstanceWitness(store, rtensor.ComponentRefs, rec.DeclaringFnRef, rec.Mapping, points[i])
		if err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d: %w", i, err))
		}
		steps = append(steps, witness.Steps...)
		premises = append(premises, witness.PremiseRefs...)
		results = append(results, witness.ResultRefs...)
		witnessPayload, err := witness.Encode()
		if err != nil {
			return fail(err)
		}
		witnessRef, err := store.Put(FamilyDerivation, witnessPayload)
		if err != nil {
			return fail(err)
		}
		witnessRefs = append(witnessRefs, witnessRef)
		if pointwise {
			witnessKinds = append(witnessKinds, ObligationWitnessPointwise)
		} else {
			witnessKinds = append(witnessKinds, ObligationWitnessSymbolic)
		}
	}
	for i := range steps {
		steps[i].Ordinal = uint32(i)
	}
	dd := Derivation{
		PremiseRefs: uniqueRefs(premises),
		Steps:       steps,
		ResultRefs:  uniqueRefs(results),
	}
	if err := Replay(store, dd, nil); err != nil {
		return fail(fmt.Errorf("physmath: obligation derivation does not replay: %w", err))
	}
	ddPayload, err := dd.Encode()
	if err != nil {
		return fail(err)
	}
	derivRef, err := store.Put(FamilyDerivation, ddPayload)
	if err != nil {
		return fail(err)
	}
	rec := RiemannTraceReverseDivergenceObligation{
		Statement:      ObligationStatementDivergenceVanishes,
		Scope:          ObligationScopeEvaluatedInstances,
		Support:        ObligationPartiallySupported,
		DivergenceRefs: append([]artifact.ArtifactRef{}, divergenceRefs...),
		ResultRefs:     resultRefs,
		ConditionRefs:  uniqueRefs(conditions),
		WitnessRefs:    witnessRefs,
		WitnessKinds:   append([]string{}, witnessKinds...),
		DerivationRef:  derivRef,
	}
	if err := rec.Validate(); err != nil {
		return fail(err)
	}
	recPayload, err := rec.Encode()
	if err != nil {
		return fail(err)
	}
	recRef, err := store.Put(FamilyRiemannTraceReverseDivergenceObligation, recPayload)
	if err != nil {
		return fail(err)
	}
	return recRef, dd, nil
}

// validateWitnessCorrespondence enforces the exact component-to-witness
// correspondence for one instance: the witness block parses, in order, as
// exactly one closure per ordered result component, each closure rooted at
// its exact stored reference and terminating in literal zero.
//
// A closure has one of two exact shapes, mirroring the constructor:
// symbolic [CANON, SIMPLIFY], or pointwise [SUBSTITUTE, CANON, SIMPLIFY].
// Shape-driven parsing (rather than consumption counting) is required
// because content addressing collapses fixed-point nodes: canon(0)
// re-mints the same reference, so distinct closures legitimately share
// references and only explicit shapes disambiguate them. This mirrors the
// divergence checker's assembly-pair assertions.
func validateWitnessCorrespondence(store Store, instance int, compRefs []artifact.ArtifactRef, declaringFnRef artifact.ArtifactRef, mapping CoordParamMap, kind string, wd Derivation) error {
	n := len(compRefs)
	if n == 0 {
		return fmt.Errorf("physmath: obligation instance %d has no result components: %w", instance, ErrBadInput)
	}
	mappingPIDs := make(map[ParameterID]bool, len(mapping.Entries))
	for _, e := range mapping.Entries {
		mappingPIDs[e.ParameterID] = true
	}
	assigned := map[ParameterID]artifact.ArtifactRef{}
	pointwise := false
	pos := 0
	// parseSubstitute validates one SUBSTITUTE step's parameters and
	// records its assignments for conflict detection.
	parseSubstitute := func(s *DerivationStep, j int) error {
		pd := artifact.NewDecoder(s.Parameters)
		nn, err := pd.SequenceCount()
		if err != nil {
			return fmt.Errorf("physmath: obligation instance %d witness step %d bad substitute parameters: %w", instance, j, err)
		}
		if nn == 0 {
			return fmt.Errorf("physmath: obligation instance %d witness step %d substitutes nothing: %w", instance, j, ErrBadInput)
		}
		seenStep := map[ParameterID]bool{}
		for k := 0; k < nn; k++ {
			bpr, err := DecodeBoundParameterRef(pd)
			if err != nil {
				return fmt.Errorf("physmath: obligation instance %d witness step %d bad substitute target: %w", instance, j, err)
			}
			vref, err := artifact.DecodeArtifactRef(pd)
			if err != nil {
				return fmt.Errorf("physmath: obligation instance %d witness step %d bad substitute value: %w", instance, j, err)
			}
			if bpr.DeclaringArtifactRef != declaringFnRef {
				return fmt.Errorf("physmath: obligation instance %d witness step %d substitutes a foreign declaring context: %w", instance, j, ErrBadInput)
			}
			if !mappingPIDs[bpr.ParameterID] {
				return fmt.Errorf("physmath: obligation instance %d witness step %d substitution out of scope: %w", instance, j, ErrBadInput)
			}
			if seenStep[bpr.ParameterID] {
				return fmt.Errorf("physmath: obligation instance %d witness step %d duplicates parameter: %w", instance, j, ErrBadInput)
			}
			seenStep[bpr.ParameterID] = true
			if prev, ok := assigned[bpr.ParameterID]; ok && prev != vref {
				return fmt.Errorf("physmath: obligation instance %d witness conflicting assignments: %w", instance, ErrBadInput)
			}
			assigned[bpr.ParameterID] = vref
		}
		if !pd.Exhausted() {
			return fmt.Errorf("physmath: obligation instance %d witness step %d trailing substitute parameters: %w", instance, j, ErrBadInput)
		}
		return nil
	}
	for gj := 0; gj < n; gj++ {
		if pos >= len(wd.Steps) {
			return fmt.Errorf("physmath: obligation instance %d witness closure coverage broken: %d closures for %d components: %w", instance, gj, n, ErrBadInput)
		}
		root := &wd.Steps[pos]
		if !witnessRuleAllowed(root.ProducerNamespace, root.RuleID) {
			return fmt.Errorf("physmath: obligation instance %d witness step %d uses unauthorized rule: %w", instance, pos, ErrBadInput)
		}
		var lastOut artifact.ArtifactRef
		if root.RuleID == RuleExprSubstitute {
			if pos+3 > len(wd.Steps) {
				return fmt.Errorf("physmath: obligation instance %d witness closure %d truncated: %w", instance, gj, ErrBadInput)
			}
			sub, cn, sm := &wd.Steps[pos], &wd.Steps[pos+1], &wd.Steps[pos+2]
			for _, s := range []*DerivationStep{cn, sm} {
				if !witnessRuleAllowed(s.ProducerNamespace, s.RuleID) {
					return fmt.Errorf("physmath: obligation instance %d witness step uses unauthorized rule: %w", instance, ErrBadInput)
				}
			}
			if cn.RuleID != RuleExprCanon || sm.RuleID != RuleExprSimplify {
				return fmt.Errorf("physmath: obligation instance %d witness closure %d malformed: %w", instance, gj, ErrBadInput)
			}
			if len(sub.InputRefs) != 2 || sub.InputRefs[0] != compRefs[gj] || sub.InputRefs[1] != declaringFnRef {
				return fmt.Errorf("physmath: obligation instance %d witness closure root %d mismatch: %w", instance, gj, ErrBadInput)
			}
			if err := parseSubstitute(sub, pos); err != nil {
				return err
			}
			if len(cn.InputRefs) != 1 || cn.InputRefs[0] != sub.OutputRef {
				return fmt.Errorf("physmath: obligation instance %d witness closure %d linkage broken: %w", instance, gj, ErrBadInput)
			}
			if len(sm.InputRefs) != 1 || sm.InputRefs[0] != cn.OutputRef {
				return fmt.Errorf("physmath: obligation instance %d witness closure %d linkage broken: %w", instance, gj, ErrBadInput)
			}
			lastOut = sm.OutputRef
			pointwise = true
			pos += 3
		} else {
			if pos+2 > len(wd.Steps) {
				return fmt.Errorf("physmath: obligation instance %d witness closure %d truncated: %w", instance, gj, ErrBadInput)
			}
			cn, sm := &wd.Steps[pos], &wd.Steps[pos+1]
			if root.RuleID != RuleExprCanon || sm.RuleID != RuleExprSimplify {
				return fmt.Errorf("physmath: obligation instance %d witness closure %d malformed: %w", instance, gj, ErrBadInput)
			}
			if !witnessRuleAllowed(sm.ProducerNamespace, sm.RuleID) {
				return fmt.Errorf("physmath: obligation instance %d witness step uses unauthorized rule: %w", instance, ErrBadInput)
			}
			if len(cn.InputRefs) != 1 || cn.InputRefs[0] != compRefs[gj] {
				return fmt.Errorf("physmath: obligation instance %d witness closure root %d mismatch: %w", instance, gj, ErrBadInput)
			}
			if len(sm.InputRefs) != 1 || sm.InputRefs[0] != cn.OutputRef {
				return fmt.Errorf("physmath: obligation instance %d witness closure %d linkage broken: %w", instance, gj, ErrBadInput)
			}
			lastOut = sm.OutputRef
			pos += 2
		}
		e, err := resolveExpression(store, lastOut)
		if err != nil {
			return fmt.Errorf("physmath: obligation instance %d witness closure %d terminal unresolvable: %w", instance, gj, err)
		}
		if !e.Equal(MustRational(0, 1)) {
			return fmt.Errorf("physmath: obligation instance %d witness closure %d does not terminate in zero: %w", instance, gj, ErrBadInput)
		}
	}
	if pos != len(wd.Steps) {
		return fmt.Errorf("physmath: obligation instance %d witness has %d orphaned steps: %w", instance, len(wd.Steps)-pos, ErrBadInput)
	}
	if kind == ObligationWitnessSymbolic && pointwise {
		return fmt.Errorf("physmath: obligation instance %d witness kind mismatch: EXACT_SYMBOLIC witness substitutes: %w", instance, ErrBadInput)
	}
	if kind == ObligationWitnessPointwise && !pointwise {
		return fmt.Errorf("physmath: obligation instance %d witness kind mismatch: POINTWISE witness has no substitution: %w", instance, ErrBadInput)
	}
	return nil
}

// resolveAndValidateObligation resolves the stored obligation record and
// validates its complete agreement from stored artifacts: family
// resolution, canonical claim/scope/support, per-instance revalidation,
// result linkage, condition-union agreement, and composite-derivation
// linkage by ordered blocks. It performs no replay itself; callers
// re-execute exactly the validated derivation. Neither
// CheckRiemannTraceReverseDivergenceObligation nor
// ReplayRiemannTraceReverseDivergenceObligation calls the other.
func resolveAndValidateObligation(store Store, ref artifact.ArtifactRef) (RiemannTraceReverseDivergenceObligation, Derivation, error) {
	fail := func(err error) (RiemannTraceReverseDivergenceObligation, Derivation, error) {
		return RiemannTraceReverseDivergenceObligation{}, Derivation{}, err
	}
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return fail(fmt.Errorf("physmath: obligation record unresolvable: %w", err))
	}
	if fam != FamilyRiemannTraceReverseDivergenceObligation {
		return fail(fmt.Errorf("physmath: target is not riemann_trace_reverse_divergence_obligation/1: %w", ErrBadInput))
	}
	rec, err := DecodeRiemannTraceReverseDivergenceObligation(payload)
	if err != nil {
		return fail(err)
	}
	for _, tc := range rec.TargetChecks() {
		tfam, _, err := store.Lookup(tc.Ref)
		if err != nil {
			return fail(fmt.Errorf("physmath: %s unresolvable: %w", tc.Field, err))
		}
		ok := false
		for _, a := range tc.Allowed {
			if tfam == a {
				ok = true
				break
			}
		}
		if !ok {
			return fail(fmt.Errorf("physmath: %s targets %s/%s/%s, not in closed target set",
				tc.Field, tfam.Namespace, tfam.ArtifactType, tfam.SchemaVersion))
		}
	}
	_, dpayload, err := store.Lookup(rec.DerivationRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: obligation derivation unresolvable: %w", err))
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return fail(err)
	}
	// Every instance must itself verify; an obligation over an unverified
	// instance is never accepted, however replayable its own derivation.
	var wantPremises []artifact.ArtifactRef
	var wantResults []artifact.ArtifactRef
	var wantConditions []artifact.ArtifactRef
	offset := 0
	for i, instance := range rec.DivergenceRefs {
		if err := CheckRiemannTraceReverseDivergence(store, instance); err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d rejected: %w", i, err))
		}
		_, ipayload, err := store.Lookup(instance)
		if err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d unresolvable: %w", i, err))
		}
		irec, err := DecodeRiemannTraceReverseDivergenceEvaluation(ipayload)
		if err != nil {
			return fail(err)
		}
		// The stored result must be the instance's own evaluated result:
		// a substituted geometry or result fails here even when every
		// inner derivation replays.
		if irec.ResultRef != rec.ResultRefs[i] {
			return fail(fmt.Errorf("physmath: obligation result %d does not match evaluated instance: %w", i, ErrBadInput))
		}
		_, idpayload, err := store.Lookup(irec.DerivationRef)
		if err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d derivation unresolvable: %w", i, err))
		}
		idd, err := DecodeDerivation(idpayload)
		if err != nil {
			return fail(err)
		}
		// Composite linkage: each instance derivation occupies one
		// contiguous block in instance order, with content equal up to
		// the composite ordinal renumbering.
		if len(d.Steps) < offset+len(idd.Steps) {
			return fail(fmt.Errorf("physmath: obligation derivation shorter than instance %d evidence", i))
		}
		for j, s := range idd.Steps {
			s.Ordinal = uint32(offset + j)
			want, err := s.Encode()
			if err != nil {
				return fail(err)
			}
			got, err := d.Steps[offset+j].Encode()
			if err != nil {
				return fail(err)
			}
			if string(got) != string(want) {
				return fail(fmt.Errorf("physmath: obligation instance %d step %d mismatch", i, j))
			}
		}
		offset += len(idd.Steps)
		wantPremises = append(wantPremises, instance, irec.ResultRef)
		wantPremises = append(wantPremises, idd.PremiseRefs...)
		wantResults = append(wantResults, idd.ResultRefs...)
		wantConditions = append(wantConditions, irec.ConditionRefs...)
		// Zero-obligation witness: the stored closing derivation must
		// match its composite block, then prove the exact
		// component-to-witness correspondence against the actual result.
		_, wdpayload, err := store.Lookup(rec.WitnessRefs[i])
		if err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d witness unresolvable: %w", i, err))
		}
		wd, err := DecodeDerivation(wdpayload)
		if err != nil {
			return fail(err)
		}
		if len(d.Steps) < offset+len(wd.Steps) {
			return fail(fmt.Errorf("physmath: obligation derivation shorter than instance %d witness evidence", i))
		}
		for j, s := range wd.Steps {
			s.Ordinal = uint32(offset + j)
			want, err := s.Encode()
			if err != nil {
				return fail(err)
			}
			got, err := d.Steps[offset+j].Encode()
			if err != nil {
				return fail(err)
			}
			if string(got) != string(want) {
				return fail(fmt.Errorf("physmath: obligation instance %d witness step %d mismatch", i, j))
			}
		}
		_, wrfPayload, err := store.Lookup(rec.ResultRefs[i])
		if err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d result unresolvable: %w", i, err))
		}
		wfield, err := DecodeTensorField(wrfPayload)
		if err != nil {
			return fail(err)
		}
		_, wrtPayload, err := store.Lookup(wfield.TensorRef)
		if err != nil {
			return fail(fmt.Errorf("physmath: obligation instance %d result tensor unresolvable: %w", i, err))
		}
		wtensor, err := DecodeTensor(wrtPayload)
		if err != nil {
			return fail(err)
		}
		if err := validateWitnessCorrespondence(store, i, wtensor.ComponentRefs, irec.DeclaringFnRef, irec.Mapping, rec.WitnessKinds[i], wd); err != nil {
			return fail(err)
		}
		offset += len(wd.Steps)
		wantPremises = append(wantPremises, wd.PremiseRefs...)
		wantResults = append(wantResults, wd.ResultRefs...)
	}
	if offset != len(d.Steps) {
		return fail(fmt.Errorf("physmath: obligation derivation has %d steps, instances cover %d", len(d.Steps), offset))
	}
	if !refSetEqual(d.PremiseRefs, uniqueRefs(wantPremises)) {
		return fail(fmt.Errorf("physmath: obligation premises do not match evaluated instances"))
	}
	if !refSetEqual(d.ResultRefs, uniqueRefs(wantResults)) {
		return fail(fmt.Errorf("physmath: obligation results do not match evaluated instances"))
	}
	if !refSetEqual(rec.ConditionRefs, uniqueRefs(wantConditions)) {
		return fail(fmt.Errorf("physmath: obligation conditions do not match evaluated instances"))
	}
	return rec, d, nil
}

// CheckRiemannTraceReverseDivergenceObligation is the store-aware
// obligation gate: it validates the complete stored record against its
// evaluated instances through the shared helper, then replays the
// validated derivation before trusting the obligation.
func CheckRiemannTraceReverseDivergenceObligation(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateObligation(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: obligation derivation does not replay: %w", err)
	}
	return nil
}

// ReplayRiemannTraceReverseDivergenceObligation is artifact-level replay
// for a stored obligation record: it validates the complete instance
// linkage (claim, scope, support, results, conditions, derivation blocks)
// and then re-executes the validated derivation. A corrupted record is
// rejected even when an inner divergence derivation remains independently
// replayable.
func ReplayRiemannTraceReverseDivergenceObligation(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateObligation(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: obligation derivation does not replay: %w", err)
	}
	return nil
}
