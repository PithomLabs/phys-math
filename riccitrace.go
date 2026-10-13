package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// Trace-reversed Ricci combination from verified Ricci contraction and
// scalar curvature (Plan 1.17):
//
//	G_mu_nu = R_mu_nu - (1/2) R g_mu_nu.
//
// Exported identifiers avoid the purity-banned vocabulary: the operation is
// named RiemannTraceReverse throughout, with the Einstein-tensor meaning
// documented here in prose. Trusted path: source metric → Riemann evaluation
// → Ricci contraction → scalar curvature → trace-reversed result, each stage
// verifying the actual stored result of the preceding stage.
//
// No new executable rules: assembly reuses Add/Mul/Neg with EXPR-CANON-001
// and EXPR-SIMPLIFY-001. No Bianchi identities, field-equation solving, or
// dynamics.

// FamilyRiemannTraceReverse is the trace-reversal record: verified Ricci
// and scalar evaluations combined with the source metric into a covariant
// rank-2 result field, with replayable derivation. Family
// physmath/riemann_trace_reverse/1.
var FamilyRiemannTraceReverse = FamilyID{NamespacePhysMath, "riemann_trace_reverse/1", "1"}

// RiemannTraceReverseEvaluation links source contraction and scalar
// evaluations, the source metric, and the computed result field with
// derivation evidence. Validate checks structure only;
// CheckRiemannTraceReverse resolves and recomputes.
type RiemannTraceReverseEvaluation struct {
	ContractionRef artifact.ArtifactRef // → riemann_contraction/1 (Ricci source)
	ScalarRef      artifact.ArtifactRef // → riemann_scalar/1 (scalar source)
	MetricRef      artifact.ArtifactRef // → symmetric_bilinear/1 (source)
	ChartRef       artifact.ArtifactRef // → coordinate_system/1
	ResultRef      artifact.ArtifactRef // → tensor_field/1 (rank 2, covariant)
	DerivationRef  artifact.ArtifactRef // required → derivation/1 (composite)
}

// Family returns FamilyRiemannTraceReverse.
func (RiemannTraceReverseEvaluation) Family() FamilyID { return FamilyRiemannTraceReverse }

// Encode returns the canonical payload in declaration order.
func (r RiemannTraceReverseEvaluation) Encode() ([]byte, error) {
	out := r.ContractionRef.Encode()
	out = append(out, r.ScalarRef.Encode()...)
	out = append(out, r.MetricRef.Encode()...)
	out = append(out, r.ChartRef.Encode()...)
	out = append(out, r.ResultRef.Encode()...)
	return append(out, r.DerivationRef.Encode()...), nil
}

// Validate enforces reference grammar without resolution.
func (r RiemannTraceReverseEvaluation) Validate() error {
	return validateRefGrammars(map[string]artifact.ArtifactRef{
		"RiemannTraceReverseEvaluation.ContractionRef": r.ContractionRef,
		"RiemannTraceReverseEvaluation.ScalarRef":      r.ScalarRef,
		"RiemannTraceReverseEvaluation.MetricRef":      r.MetricRef,
		"RiemannTraceReverseEvaluation.ChartRef":       r.ChartRef,
		"RiemannTraceReverseEvaluation.ResultRef":      r.ResultRef,
		"RiemannTraceReverseEvaluation.DerivationRef":  r.DerivationRef,
	})
}

// TargetChecks: ContractionRef → riemann_contraction/1; ScalarRef →
// riemann_scalar/1; MetricRef → symmetric_bilinear/1; ChartRef →
// coordinate_system/1; ResultRef → tensor_field/1; DerivationRef →
// derivation/1. Resolved only by the store-aware check.
func (r RiemannTraceReverseEvaluation) TargetChecks() []TargetCheck {
	return []TargetCheck{
		{Field: "RiemannTraceReverseEvaluation.ContractionRef", Ref: r.ContractionRef, Allowed: []FamilyID{FamilyRiemannContraction}},
		{Field: "RiemannTraceReverseEvaluation.ScalarRef", Ref: r.ScalarRef, Allowed: []FamilyID{FamilyRiemannScalar}},
		{Field: "RiemannTraceReverseEvaluation.MetricRef", Ref: r.MetricRef, Allowed: []FamilyID{FamilySymmetricBilinear}},
		{Field: "RiemannTraceReverseEvaluation.ChartRef", Ref: r.ChartRef, Allowed: []FamilyID{FamilyCoordinateSystem}},
		{Field: "RiemannTraceReverseEvaluation.ResultRef", Ref: r.ResultRef, Allowed: []FamilyID{FamilyTensorField}},
		{Field: "RiemannTraceReverseEvaluation.DerivationRef", Ref: r.DerivationRef, Allowed: []FamilyID{FamilyDerivation}},
	}
}

// DecodeRiemannTraceReverseEvaluation decodes a
// physmath/riemann_trace_reverse/1 payload. Representation only.
func DecodeRiemannTraceReverseEvaluation(payload []byte) (RiemannTraceReverseEvaluation, error) {
	d := artifact.NewDecoder(payload)
	contraction, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseEvaluation{}, fmt.Errorf("physmath: trace-reverse missing contraction: %w", err)
	}
	scalar, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseEvaluation{}, fmt.Errorf("physmath: trace-reverse missing scalar: %w", err)
	}
	metric, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseEvaluation{}, fmt.Errorf("physmath: trace-reverse missing metric: %w", err)
	}
	chart, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseEvaluation{}, fmt.Errorf("physmath: trace-reverse missing chart: %w", err)
	}
	result, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseEvaluation{}, fmt.Errorf("physmath: trace-reverse missing result: %w", err)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseEvaluation{}, fmt.Errorf("physmath: trace-reverse missing derivation: %w", err)
	}
	if !d.Exhausted() {
		return RiemannTraceReverseEvaluation{}, fmt.Errorf("physmath: trailing bytes in trace-reverse payload")
	}
	r := RiemannTraceReverseEvaluation{
		ContractionRef: contraction, ScalarRef: scalar, MetricRef: metric,
		ChartRef: chart, ResultRef: result, DerivationRef: deriv,
	}
	if err := r.Validate(); err != nil {
		return RiemannTraceReverseEvaluation{}, err
	}
	return r, nil
}

// traceReverseResult is one computed result component with its evidence refs.
type traceReverseResult struct {
	expr    Expr
	premise artifact.ArtifactRef
	canon   artifact.ArtifactRef
	out     artifact.ArtifactRef
}

// evalTraceReverseCore assembles G_mu_nu = R_mu_nu - (1/2) R g_mu_nu for
// every (mu,nu) in row-major order: one Add premise per component through
// CANON+SIMPLIFY. It returns result expressions with derivation pieces using
// local ordinals. Callers renumber, mint tensors, and wrap.
func evalTraceReverseCore(store Store, ricComps []Expr, scalar Expr, metricComps []Expr, dim int) ([]traceReverseResult, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
	fail := func(err error) ([]traceReverseResult, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
		return nil, nil, nil, nil, err
	}
	half := MustRational(1, 2)
	var out []traceReverseResult
	var steps []DerivationStep
	var premises []artifact.ArtifactRef
	var results []artifact.ArtifactRef
	for mu := 0; mu < dim; mu++ {
		for nu := 0; nu < dim; nu++ {
			premise, err := PutExpression(store, Add(
				cloneExpr(ricComps[mu*dim+nu]),
				Neg(Mul(half, cloneExpr(scalar), cloneExpr(metricComps[mu*dim+nu]))),
			))
			if err != nil {
				return fail(err)
			}
			canon, err := Apply(store, Invocation{
				ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
				InputRefs: []artifact.ArtifactRef{premise},
			})
			if err != nil {
				return fail(fmt.Errorf("physmath: trace-reverse coefficient canon failed: %w", err))
			}
			res, err := Apply(store, Invocation{
				ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
				InputRefs: []artifact.ArtifactRef{canon.OutputRef},
			})
			if err != nil {
				return fail(fmt.Errorf("physmath: trace-reverse coefficient simplify failed: %w", err))
			}
			e, err := resolveExpression(store, res.OutputRef)
			if err != nil {
				return fail(err)
			}
			premises = append(premises, premise)
			results = append(results, res.OutputRef)
			steps = append(steps,
				DerivationStep{
					Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
					InputRefs: []artifact.ArtifactRef{premise}, OutputRef: canon.OutputRef,
				},
				DerivationStep{
					Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
					InputRefs: []artifact.ArtifactRef{canon.OutputRef}, OutputRef: res.OutputRef,
				},
			)
			out = append(out, traceReverseResult{expr: e, premise: premise, canon: canon.OutputRef, out: res.OutputRef})
		}
	}
	return out, steps, premises, results, nil
}

// EvaluateRiemannTraceReverse combines a verified Ricci contraction with a
// verified scalar curvature and the source metric into G_mu_nu = R_mu_nu -
// (1/2) R g_mu_nu. Both source references are explicit so a mismatched
// Ricci/scalar pair is expressible and rejectable: the scalar record must
// bind the same contraction, and one metric and chart must agree across all
// three records. It returns the result field reference, the minted record
// reference, and one flat replayable composite derivation.
func EvaluateRiemannTraceReverse(store Store, contractionRef, scalarRef artifact.ArtifactRef) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, artifact.ArtifactRef{}, Derivation{}, err
	}
	if err := CheckRiemannContraction(store, contractionRef); err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse source contraction rejected: %w", err))
	}
	if err := CheckRiemannScalar(store, scalarRef); err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse source scalar rejected: %w", err))
	}
	cfam, cpayload, err := store.Lookup(contractionRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse contraction unresolvable: %w", err))
	}
	if cfam != FamilyRiemannContraction {
		return fail(fmt.Errorf("physmath: trace-reverse contraction is not riemann_contraction/1: %w", ErrBadInput))
	}
	crec, err := DecodeRiemannContractionEvaluation(cpayload)
	if err != nil {
		return fail(err)
	}
	sfam, spayload, err := store.Lookup(scalarRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse scalar unresolvable: %w", err))
	}
	if sfam != FamilyRiemannScalar {
		return fail(fmt.Errorf("physmath: trace-reverse scalar is not riemann_scalar/1: %w", ErrBadInput))
	}
	srec, err := DecodeRiemannScalarEvaluation(spayload)
	if err != nil {
		return fail(err)
	}
	// Pair linkage: the scalar must be built over this exact contraction,
	// and one metric and chart must agree across the chain.
	if srec.ContractionRef != contractionRef {
		return fail(fmt.Errorf("physmath: trace-reverse scalar built over another contraction: %w", ErrBadInput))
	}
	rfam, rpayload, err := store.Lookup(crec.RiemannRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse riemann unresolvable: %w", err))
	}
	if rfam != FamilyRiemannEvaluation {
		return fail(fmt.Errorf("physmath: trace-reverse riemann is not riemann_evaluation/1: %w", ErrBadInput))
	}
	rrec, err := DecodeRiemannEvaluation(rpayload)
	if err != nil {
		return fail(err)
	}
	if srec.MetricRef != rrec.MetricRef {
		return fail(fmt.Errorf("physmath: trace-reverse scalar metric differs from riemann metric: %w", ErrBadInput))
	}
	if crec.ChartRef != rrec.ChartRef || srec.ChartRef != rrec.ChartRef {
		return fail(fmt.Errorf("physmath: trace-reverse chart breaks the provenance chain: %w", ErrBadInput))
	}
	_, fpayload, err := store.Lookup(crec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse contraction result unresolvable: %w", err))
	}
	cf, err := DecodeTensorField(fpayload)
	if err != nil {
		return fail(err)
	}
	if cf.CoordinateSystemRef != crec.ChartRef {
		return fail(fmt.Errorf("physmath: trace-reverse contraction chart not preserved: %w", ErrBadInput))
	}
	_, tpayload, err := store.Lookup(cf.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse contraction tensor unresolvable: %w", err))
	}
	ct, err := DecodeTensor(tpayload)
	if err != nil {
		return fail(err)
	}
	if int(ct.Rank) != 2 {
		return fail(fmt.Errorf("physmath: trace-reverse contraction rank %d != 2: %w", ct.Rank, ErrBadInput))
	}
	d := int(ct.Dimension)
	if d > MatrixDimBound {
		return fail(fmt.Errorf("physmath: trace-reverse dimension %d exceeds bound %d: %w",
			d, MatrixDimBound, ErrBadInput))
	}
	ricComps, err := resolveComponents(store, "trace-reverse contraction", ct.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	scalarExpr, err := resolveExpression(store, srec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse scalar result unresolvable: %w", err))
	}
	mfam, mpayload, err := store.Lookup(srec.MetricRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse metric unresolvable: %w", err))
	}
	if mfam != FamilySymmetricBilinear {
		return fail(fmt.Errorf("physmath: trace-reverse metric is not symmetric_bilinear/1: %w", ErrBadInput))
	}
	metric, err := DecodeSymmetricBilinear(mpayload)
	if err != nil {
		return fail(err)
	}
	if int(metric.Dimension) != d {
		return fail(fmt.Errorf("physmath: trace-reverse metric dimension %d != contraction dimension %d: %w",
			metric.Dimension, d, ErrBadInput))
	}
	metricComps, err := resolveComponents(store, "trace-reverse metric", metric.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	// Covariant slot descriptors, minted deterministically so recomputation
	// reproduces them exactly.
	slotLabels := []string{"rt0", "rt1"}
	idxRefs := make([]artifact.ArtifactRef, 2)
	_, chpayload, err := store.Lookup(crec.ChartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse chart unresolvable: %w", err))
	}
	chart, err := DecodeCoordinateSystem(chpayload)
	if err != nil {
		return fail(err)
	}
	for i := range slotLabels {
		p, err := Index{Label: slotLabels[i], Variance: VarianceCovariant, Space: &chart.SpaceRef}.Encode()
		if err != nil {
			return fail(err)
		}
		idxRef, err := store.Put(FamilyIndex, p)
		if err != nil {
			return fail(err)
		}
		idxRefs[i] = idxRef
	}
	out, coreSteps, corePremises, coreResults, err := evalTraceReverseCore(store, ricComps, scalarExpr, metricComps, d)
	if err != nil {
		return fail(err)
	}
	outRefs := make([]artifact.ArtifactRef, len(out))
	for i, r := range out {
		outRefs[i] = r.out
	}
	rtn := Tensor{Rank: 2, Dimension: uint16(d), IndexRefs: idxRefs, ComponentRefs: outRefs}
	if err := rtn.Validate(); err != nil {
		return fail(err)
	}
	resultTensor, err := store.Put(FamilyTensor, rtn.Encode())
	if err != nil {
		return fail(err)
	}
	field := TensorField{TensorRef: resultTensor, CoordinateSystemRef: crec.ChartRef}
	if err := field.Validate(); err != nil {
		return fail(err)
	}
	resultField, err := store.Put(FamilyTensorField, field.Encode())
	if err != nil {
		return fail(err)
	}
	// One flat derivation: contraction evidence, scalar evidence, then the
	// combination assembly, with contiguous ordinals in execution order.
	_, cdpayload, err := store.Lookup(crec.DerivationRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse contraction derivation unresolvable: %w", err))
	}
	cDeriv, err := DecodeDerivation(cdpayload)
	if err != nil {
		return fail(err)
	}
	_, sdpayload, err := store.Lookup(srec.DerivationRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse scalar derivation unresolvable: %w", err))
	}
	sDeriv, err := DecodeDerivation(sdpayload)
	if err != nil {
		return fail(err)
	}
	steps := make([]DerivationStep, 0, len(cDeriv.Steps)+len(sDeriv.Steps)+len(coreSteps))
	steps = append(steps, cDeriv.Steps...)
	steps = append(steps, sDeriv.Steps...)
	steps = append(steps, coreSteps...)
	for i := range steps {
		steps[i].Ordinal = uint32(i)
	}
	premises := []artifact.ArtifactRef{contractionRef, scalarRef, crec.RiemannRef}
	premises = append(premises, cDeriv.PremiseRefs...)
	premises = append(premises, sDeriv.PremiseRefs...)
	premises = append(premises, ct.ComponentRefs...)
	premises = append(premises, metric.ComponentRefs...)
	premises = append(premises, srec.ResultRef)
	premises = append(premises, corePremises...)
	results := []artifact.ArtifactRef{}
	results = append(results, cDeriv.ResultRefs...)
	results = append(results, sDeriv.ResultRefs...)
	results = append(results, coreResults...)
	dd := Derivation{
		PremiseRefs: uniqueRefs(premises),
		Steps:       steps,
		ResultRefs:  uniqueRefs(results),
	}
	if err := Replay(store, dd, nil); err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse derivation does not replay: %w", err))
	}
	ddPayload, err := dd.Encode()
	if err != nil {
		return fail(err)
	}
	derivRef, err := store.Put(FamilyDerivation, ddPayload)
	if err != nil {
		return fail(err)
	}
	rec := RiemannTraceReverseEvaluation{
		ContractionRef: contractionRef, ScalarRef: scalarRef, MetricRef: srec.MetricRef,
		ChartRef: crec.ChartRef, ResultRef: resultField, DerivationRef: derivRef,
	}
	if err := rec.Validate(); err != nil {
		return fail(err)
	}
	trecPayload, err := rec.Encode()
	if err != nil {
		return fail(err)
	}
	recRef, err := store.Put(FamilyRiemannTraceReverse, trecPayload)
	if err != nil {
		return fail(err)
	}
	return resultField, recRef, dd, nil
}

// resolveAndValidateTraceReverse resolves the stored
// riemann_trace_reverse/1 record and validates its complete agreement from
// stored artifacts: family resolution, source revalidation, pair linkage,
// single-metric/chart agreement, ordered component comparison against
// idempotent recomputation, result index contract, and derivation linkage.
// It performs no replay itself; callers re-execute exactly the validated
// derivation. Neither CheckRiemannTraceReverse nor ReplayRiemannTraceReverse
// calls the other.
func resolveAndValidateTraceReverse(store Store, ref artifact.ArtifactRef) (RiemannTraceReverseEvaluation, Derivation, error) {
	fail := func(err error) (RiemannTraceReverseEvaluation, Derivation, error) {
		return RiemannTraceReverseEvaluation{}, Derivation{}, err
	}
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse record unresolvable: %w", err))
	}
	if fam != FamilyRiemannTraceReverse {
		return fail(fmt.Errorf("physmath: target is not riemann_trace_reverse/1: %w", ErrBadInput))
	}
	rec, err := DecodeRiemannTraceReverseEvaluation(payload)
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
		return fail(fmt.Errorf("physmath: trace-reverse derivation unresolvable: %w", err))
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return fail(err)
	}
	// Both sources must themselves verify; combination over unverified
	// inputs is never accepted.
	if err := CheckRiemannContraction(store, rec.ContractionRef); err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse source contraction rejected: %w", err))
	}
	if err := CheckRiemannScalar(store, rec.ScalarRef); err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse source scalar rejected: %w", err))
	}
	cfam, cpayload, err := store.Lookup(rec.ContractionRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse contraction unresolvable: %w", err))
	}
	if cfam != FamilyRiemannContraction {
		return fail(fmt.Errorf("physmath: trace-reverse contraction is not riemann_contraction/1: %w", ErrBadInput))
	}
	crec, err := DecodeRiemannContractionEvaluation(cpayload)
	if err != nil {
		return fail(err)
	}
	sfam, spayload, err := store.Lookup(rec.ScalarRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse scalar unresolvable: %w", err))
	}
	if sfam != FamilyRiemannScalar {
		return fail(fmt.Errorf("physmath: trace-reverse scalar is not riemann_scalar/1: %w", ErrBadInput))
	}
	srec, err := DecodeRiemannScalarEvaluation(spayload)
	if err != nil {
		return fail(err)
	}
	// Pair linkage plus single-metric/chart agreement: a scalar built over
	// another contraction, or any metric/chart drift across the chain,
	// fails here even if every inner derivation replays.
	if srec.ContractionRef != rec.ContractionRef {
		return fail(fmt.Errorf("physmath: trace-reverse scalar built over another contraction: %w", ErrBadInput))
	}
	if srec.MetricRef != rec.MetricRef {
		return fail(fmt.Errorf("physmath: trace-reverse scalar metric differs from record metric: %w", ErrBadInput))
	}
	rfam, rpayload, err := store.Lookup(crec.RiemannRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse riemann unresolvable: %w", err))
	}
	if rfam != FamilyRiemannEvaluation {
		return fail(fmt.Errorf("physmath: trace-reverse riemann is not riemann_evaluation/1: %w", ErrBadInput))
	}
	rrec, err := DecodeRiemannEvaluation(rpayload)
	if err != nil {
		return fail(err)
	}
	if rrec.MetricRef != rec.MetricRef {
		return fail(fmt.Errorf("physmath: trace-reverse riemann metric differs from record metric: %w", ErrBadInput))
	}
	if crec.ChartRef != rec.ChartRef || srec.ChartRef != rec.ChartRef || rrec.ChartRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: trace-reverse chart breaks the provenance chain: %w", ErrBadInput))
	}
	_, fpayload, err := store.Lookup(crec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse contraction result unresolvable: %w", err))
	}
	cf, err := DecodeTensorField(fpayload)
	if err != nil {
		return fail(err)
	}
	if cf.CoordinateSystemRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: trace-reverse contraction chart not preserved: %w", ErrBadInput))
	}
	_, tpayload, err := store.Lookup(cf.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse contraction tensor unresolvable: %w", err))
	}
	ct, err := DecodeTensor(tpayload)
	if err != nil {
		return fail(err)
	}
	if int(ct.Rank) != 2 {
		return fail(fmt.Errorf("physmath: trace-reverse contraction rank %d != 2: %w", ct.Rank, ErrBadInput))
	}
	dim := int(ct.Dimension)
	if dim > MatrixDimBound {
		return fail(fmt.Errorf("physmath: trace-reverse dimension %d exceeds bound %d: %w",
			dim, MatrixDimBound, ErrBadInput))
	}
	ricComps, err := resolveComponents(store, "trace-reverse contraction", ct.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	scalarExpr, err := resolveExpression(store, srec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse scalar result unresolvable: %w", err))
	}
	_, mpayload, err := store.Lookup(rec.MetricRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse metric unresolvable: %w", err))
	}
	metric, err := DecodeSymmetricBilinear(mpayload)
	if err != nil {
		return fail(err)
	}
	if int(metric.Dimension) != dim {
		return fail(fmt.Errorf("physmath: trace-reverse metric dimension %d != contraction dimension %d: %w",
			metric.Dimension, dim, ErrBadInput))
	}
	metricComps, err := resolveComponents(store, "trace-reverse metric", metric.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	// Source derivation evidence must match the recorded contraction and
	// scalar derivations in order (construction embeds contraction first
	// with preserved ordinals, then scalar with shifted ordinals).
	_, cdpayload, err := store.Lookup(crec.DerivationRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse contraction derivation unresolvable: %w", err))
	}
	cDeriv, err := DecodeDerivation(cdpayload)
	if err != nil {
		return fail(err)
	}
	_, sdpayload, err := store.Lookup(srec.DerivationRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse scalar derivation unresolvable: %w", err))
	}
	sDeriv, err := DecodeDerivation(sdpayload)
	if err != nil {
		return fail(err)
	}
	if len(d.Steps) < len(cDeriv.Steps)+len(sDeriv.Steps) {
		return fail(fmt.Errorf("physmath: trace-reverse derivation shorter than its source evidence"))
	}
	for i, s := range cDeriv.Steps {
		want, err := s.Encode()
		if err != nil {
			return fail(err)
		}
		got, err := d.Steps[i].Encode()
		if err != nil {
			return fail(err)
		}
		if string(got) != string(want) {
			return fail(fmt.Errorf("physmath: trace-reverse contraction evidence step %d mismatch", i))
		}
	}
	for i, s := range sDeriv.Steps {
		if !sameStepShape(d.Steps[len(cDeriv.Steps)+i], s) {
			return fail(fmt.Errorf("physmath: trace-reverse scalar evidence step %d mismatch", i))
		}
		if d.Steps[len(cDeriv.Steps)+i].Ordinal != uint32(len(cDeriv.Steps)+i) {
			return fail(fmt.Errorf("physmath: trace-reverse scalar evidence step %d ordinal broken", i))
		}
	}
	// Idempotent recomputation: identical inputs reproduce identical
	// references, so recomputation verifies rather than repairs.
	out, coreSteps, corePremises, coreResults, err := evalTraceReverseCore(store, ricComps, scalarExpr, metricComps, dim)
	if err != nil {
		return fail(err)
	}
	coreStored := d.Steps[len(cDeriv.Steps)+len(sDeriv.Steps):]
	if len(coreStored) != len(coreSteps) {
		return fail(fmt.Errorf("physmath: trace-reverse core steps %d != recomputed %d",
			len(coreStored), len(coreSteps)))
	}
	for i := 0; i < len(out); i++ {
		s0, s1 := coreStored[2*i], coreStored[2*i+1]
		if s0.ProducerNamespace != NamespacePhysMath || s0.RuleID != RuleExprCanon {
			return fail(fmt.Errorf("physmath: trace-reverse assembly step %d must be CANON", i))
		}
		if s1.ProducerNamespace != NamespacePhysMath || s1.RuleID != RuleExprSimplify {
			return fail(fmt.Errorf("physmath: trace-reverse assembly step %d must be SIMPLIFY", i))
		}
		if len(s0.InputRefs) != 1 || s0.InputRefs[0] != out[i].premise {
			return fail(fmt.Errorf("physmath: trace-reverse assembly step %d premise mismatch", i))
		}
		if s0.OutputRef != out[i].canon || len(s1.InputRefs) != 1 || s1.InputRefs[0] != out[i].canon {
			return fail(fmt.Errorf("physmath: trace-reverse assembly step %d linkage broken", i))
		}
		if s1.OutputRef != out[i].out {
			return fail(fmt.Errorf("physmath: trace-reverse assembly step %d output mismatch", i))
		}
	}
	wantPremises := []artifact.ArtifactRef{rec.ContractionRef, rec.ScalarRef, crec.RiemannRef}
	wantPremises = append(wantPremises, cDeriv.PremiseRefs...)
	wantPremises = append(wantPremises, sDeriv.PremiseRefs...)
	wantPremises = append(wantPremises, ct.ComponentRefs...)
	wantPremises = append(wantPremises, metric.ComponentRefs...)
	wantPremises = append(wantPremises, srec.ResultRef)
	wantPremises = append(wantPremises, corePremises...)
	if !refSetEqual(d.PremiseRefs, uniqueRefs(wantPremises)) {
		return fail(fmt.Errorf("physmath: trace-reverse premises do not match recomputation"))
	}
	wantResults := []artifact.ArtifactRef{}
	wantResults = append(wantResults, cDeriv.ResultRefs...)
	wantResults = append(wantResults, sDeriv.ResultRefs...)
	wantResults = append(wantResults, coreResults...)
	if !refSetEqual(d.ResultRefs, uniqueRefs(wantResults)) {
		return fail(fmt.Errorf("physmath: trace-reverse results do not match recomputation"))
	}
	// Result field: same chart, rank-2 covariant tensor, deterministic slot
	// descriptors, chart-space agreement, ordered components byte-equal.
	rfam2, rpayload2, err := store.Lookup(rec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse result unresolvable: %w", err))
	}
	if rfam2 != FamilyTensorField {
		return fail(fmt.Errorf("physmath: trace-reverse result is not tensor_field/1: %w", ErrBadInput))
	}
	rff, err := DecodeTensorField(rpayload2)
	if err != nil {
		return fail(err)
	}
	if rff.CoordinateSystemRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: trace-reverse result chart not preserved: %w", ErrBadInput))
	}
	rtfam, rtpayload, err := store.Lookup(rff.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse result tensor unresolvable: %w", err))
	}
	if rtfam != FamilyTensor {
		return fail(fmt.Errorf("physmath: trace-reverse result tensor is not tensor/1: %w", ErrBadInput))
	}
	rtn, err := DecodeTensor(rtpayload)
	if err != nil {
		return fail(err)
	}
	if int(rtn.Rank) != 2 || int(rtn.Dimension) != dim {
		return fail(fmt.Errorf("physmath: trace-reverse result shape wrong: %w", ErrBadInput))
	}
	_, chpayload, err := store.Lookup(rec.ChartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: trace-reverse chart unresolvable: %w", err))
	}
	chart, err := DecodeCoordinateSystem(chpayload)
	if err != nil {
		return fail(err)
	}
	slotLabels := []string{"rt0", "rt1"}
	if len(rtn.IndexRefs) != 2 {
		return fail(fmt.Errorf("physmath: trace-reverse result index count wrong: %w", ErrBadInput))
	}
	for i := range slotLabels {
		p, err := Index{Label: slotLabels[i], Variance: VarianceCovariant, Space: &chart.SpaceRef}.Encode()
		if err != nil {
			return fail(err)
		}
		wantIdx, err := store.Put(FamilyIndex, p)
		if err != nil {
			return fail(err)
		}
		if rtn.IndexRefs[i] != wantIdx {
			return fail(fmt.Errorf("physmath: trace-reverse result slot %d identity broken: %w", i, ErrBadInput))
		}
		idx, err := resolveIndex(store, "trace-reverse result slot", rtn.IndexRefs[i])
		if err != nil {
			return fail(err)
		}
		if idx.Variance != VarianceCovariant {
			return fail(fmt.Errorf("physmath: trace-reverse result slot %d variance wrong: %w", i, ErrBadInput))
		}
		if idx.Space == nil || *idx.Space != chart.SpaceRef {
			return fail(fmt.Errorf("physmath: trace-reverse result slot %d space mismatch: %w", i, ErrBadInput))
		}
	}
	if len(rtn.ComponentRefs) != len(out) {
		return fail(fmt.Errorf("physmath: trace-reverse result component count mismatch: %w", ErrBadInput))
	}
	for i := range out {
		_, opayload, err := store.Lookup(out[i].out)
		if err != nil {
			return fail(err)
		}
		_, rp, err := store.Lookup(rtn.ComponentRefs[i])
		if err != nil {
			return fail(err)
		}
		if string(opayload) != string(rp) {
			return fail(fmt.Errorf("physmath: trace-reverse result component %d mismatch: %w", i, ErrBadInput))
		}
	}
	return rec, d, nil
}

// CheckRiemannTraceReverse is the store-aware provenance gate: it validates
// the complete stored record against its derivation through the shared
// helper, then replays the validated derivation before trusting the result.
func CheckRiemannTraceReverse(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateTraceReverse(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: trace-reverse derivation does not replay: %w", err)
	}
	return nil
}

// ReplayRiemannTraceReverse is artifact-level replay for a stored
// riemann_trace_reverse/1 record: it validates the complete record against
// its derivation (pair linkage, single metric/chart, ordered components,
// index contract) and then re-executes the validated derivation. A corrupted
// record is rejected even when an inner derivation remains independently
// replayable.
func ReplayRiemannTraceReverse(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateTraceReverse(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: trace-reverse derivation does not replay: %w", err)
	}
	return nil
}
