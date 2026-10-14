package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// Contracted Bianchi evaluated obligation (Plan 1.19):
//
//	nabla^mu G_mu_nu = 0.
//
// This record states the identity as an evaluated obligation over explicit
// divergence-evaluation artifacts. It binds the canonical claim, its
// declared scope, the ordered evaluated instances, their observed results,
// the union of their conditions, one composite derivation, and a validated
// support status. It does not establish a general proof, implement field
// equations, or introduce dynamics.
//
// Evidence semantics: instance verification reuses the existing
// riemann_trace_reverse_divergence/1 records and their Check/Replay gates.
// Symbolic or structural identities, hand-verified factored cancellation,
// and numerical point-evaluation corroboration keep the evidence type they
// were established with; replay success alone never promotes an instance
// result into a universal claim.
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
	for i, c := range r.ConditionRefs {
		fields[getRefFieldName("RiemannTraceReverseDivergenceObligation.ConditionRefs", i)] = c
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	return checkDuplicateRefs("RiemannTraceReverseDivergenceObligation.ConditionRefs", r.ConditionRefs)
}

// TargetChecks: DivergenceRefs → riemann_trace_reverse_divergence/1;
// ResultRefs → tensor_field/1; ConditionRefs → condition/1; DerivationRef
// → derivation/1. Resolved only by the store-aware check.
func (r RiemannTraceReverseDivergenceObligation) TargetChecks() []TargetCheck {
	checks := make([]TargetCheck, 0, len(r.DivergenceRefs)+len(r.ResultRefs)+len(r.ConditionRefs)+1)
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
		DerivationRef: deriv,
	}
	if err := r.Validate(); err != nil {
		return RiemannTraceReverseDivergenceObligation{}, err
	}
	return r, nil
}

// EvaluateRiemannTraceReverseDivergenceObligation records the contracted
// Bianchi identity as an evaluated obligation over verified divergence
// instances. Every instance must pass CheckRiemannTraceReverseDivergence;
// the composite derivation concatenates the instance derivations with
// contiguous ordinals in instance order. It returns the minted obligation
// record reference and the composite derivation. Support is always
// PARTIALLY_SUPPORTED: evaluated instances, never a general proof.
func EvaluateRiemannTraceReverseDivergenceObligation(store Store, divergenceRefs []artifact.ArtifactRef) (artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, Derivation{}, err
	}
	if len(divergenceRefs) == 0 {
		return fail(fmt.Errorf("physmath: obligation needs at least one evaluated instance: %w", ErrBadInput))
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
