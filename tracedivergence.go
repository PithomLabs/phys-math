package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// Contracted differential identity of the trace-reversed tensor (Plan 1.18):
//
//	D_nu = sum_lambda_mu g^lambda_mu (nabla_lambda G)_mu_nu = 0.
//
// The rank-three covariant derivative has ordered slots (lambda, mu, nu);
// the inverse metric contracts slots 0 and 1; nu remains free. This
// milestone verifies the identity on explicitly evaluated geometries. It
// does not establish a general proof, implement field equations, or
// introduce dynamics.
//
// Trusted coordinate differentiation uses the guarded
// coordinate-differentiation path with derivation provenance. The
// mathematics executes through existing rules (DIFFERENTIATE-001,
// EXPR-CANON-001, EXPR-SIMPLIFY-001); no new executable rule family is
// introduced.

// FamilyRiemannTraceReverseDivergence is the divergence record: a verified
// trace-reversed tensor differentiated covariantly through the
// metric-derived connection and contracted with the inverse source metric,
// with replayable composite derivation. Family
// physmath/riemann_trace_reverse_divergence/1.
var FamilyRiemannTraceReverseDivergence = FamilyID{NamespacePhysMath, "riemann_trace_reverse_divergence/1", "1"}

// RiemannTraceReverseDivergenceEvaluation links a source trace-reversed
// evaluation to its metric, chart binding, built connection, reused inverse
// tensor, covariant-derivative evidence, divergence result, and composite
// derivation. Validate checks structure only;
// CheckRiemannTraceReverseDivergence resolves and recomputes.
type RiemannTraceReverseDivergenceEvaluation struct {
	TraceReverseRef    artifact.ArtifactRef   // → riemann_trace_reverse/1 (source)
	MetricRef          artifact.ArtifactRef   // → symmetric_bilinear/1 (source)
	ChartRef           artifact.ArtifactRef   // → coordinate_system/1
	Mapping            CoordParamMap          // inline ordered chart→parameter binding
	DeclaringFnRef     artifact.ArtifactRef   // → function/1
	ConditionRefs      []artifact.ArtifactRef // SET → condition/1 (exact set used)
	ConnectionRef      artifact.ArtifactRef   // → affine_connection/1 (built)
	InvTensorRef       artifact.ArtifactRef   // → tensor/1 (rank-2 contravariant inverse, reused)
	CovariantResultRef artifact.ArtifactRef   // → tensor_field/1 (rank-3 derivative evidence)
	ResultRef          artifact.ArtifactRef   // → tensor_field/1 (rank-1 divergence)
	DerivationRef      artifact.ArtifactRef   // required → derivation/1 (composite)
}

// Family returns FamilyRiemannTraceReverseDivergence.
func (RiemannTraceReverseDivergenceEvaluation) Family() FamilyID {
	return FamilyRiemannTraceReverseDivergence
}

// Encode returns the canonical payload in declaration order.
func (r RiemannTraceReverseDivergenceEvaluation) Encode() ([]byte, error) {
	out := r.TraceReverseRef.Encode()
	out = append(out, r.MetricRef.Encode()...)
	out = append(out, r.ChartRef.Encode()...)
	mapPayload, err := r.Mapping.Encode()
	if err != nil {
		return nil, err
	}
	out = append(out, artifact.EncodeBytes(mapPayload)...)
	out = append(out, r.DeclaringFnRef.Encode()...)
	set, err := encodeRefSet(r.ConditionRefs)
	if err != nil {
		return nil, err
	}
	out = append(out, set...)
	out = append(out, r.ConnectionRef.Encode()...)
	out = append(out, r.InvTensorRef.Encode()...)
	out = append(out, r.CovariantResultRef.Encode()...)
	out = append(out, r.ResultRef.Encode()...)
	return append(out, r.DerivationRef.Encode()...), nil
}

// Validate enforces grammar, a non-empty mapping with distinct PIDs, and
// condition-set uniqueness. Chart/range/declared-identity gates need the
// store: see CheckRiemannTraceReverseDivergence.
func (r RiemannTraceReverseDivergenceEvaluation) Validate() error {
	if len(r.Mapping.Entries) == 0 {
		return fmt.Errorf("physmath: divergence evaluation needs a non-empty mapping: %w", ErrBadInput)
	}
	seen := map[ParameterID]bool{}
	for i, e := range r.Mapping.Entries {
		if seen[e.ParameterID] {
			return fmt.Errorf("physmath: divergence evaluation duplicate parameter identity at %d: %w", i, ErrBadInput)
		}
		seen[e.ParameterID] = true
	}
	fields := map[string]artifact.ArtifactRef{
		"RiemannTraceReverseDivergenceEvaluation.TraceReverseRef":    r.TraceReverseRef,
		"RiemannTraceReverseDivergenceEvaluation.MetricRef":          r.MetricRef,
		"RiemannTraceReverseDivergenceEvaluation.ChartRef":           r.ChartRef,
		"RiemannTraceReverseDivergenceEvaluation.DeclaringFnRef":     r.DeclaringFnRef,
		"RiemannTraceReverseDivergenceEvaluation.ConnectionRef":      r.ConnectionRef,
		"RiemannTraceReverseDivergenceEvaluation.InvTensorRef":       r.InvTensorRef,
		"RiemannTraceReverseDivergenceEvaluation.CovariantResultRef": r.CovariantResultRef,
		"RiemannTraceReverseDivergenceEvaluation.ResultRef":          r.ResultRef,
		"RiemannTraceReverseDivergenceEvaluation.DerivationRef":      r.DerivationRef,
	}
	for i, c := range r.ConditionRefs {
		fields[getRefFieldName("RiemannTraceReverseDivergenceEvaluation.ConditionRefs", i)] = c
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	return checkDuplicateRefs("RiemannTraceReverseDivergenceEvaluation.ConditionRefs", r.ConditionRefs)
}

// TargetChecks: TraceReverseRef → riemann_trace_reverse/1; MetricRef →
// symmetric_bilinear/1; ChartRef → coordinate_system/1; DeclaringFnRef →
// function/1; ConditionRefs → condition/1; ConnectionRef →
// affine_connection/1; InvTensorRef → tensor/1; CovariantResultRef,
// ResultRef → tensor_field/1; DerivationRef → derivation/1. Resolved only
// by the store-aware check.
func (r RiemannTraceReverseDivergenceEvaluation) TargetChecks() []TargetCheck {
	checks := []TargetCheck{
		{Field: "RiemannTraceReverseDivergenceEvaluation.TraceReverseRef", Ref: r.TraceReverseRef, Allowed: []FamilyID{FamilyRiemannTraceReverse}},
		{Field: "RiemannTraceReverseDivergenceEvaluation.MetricRef", Ref: r.MetricRef, Allowed: []FamilyID{FamilySymmetricBilinear}},
		{Field: "RiemannTraceReverseDivergenceEvaluation.ChartRef", Ref: r.ChartRef, Allowed: []FamilyID{FamilyCoordinateSystem}},
		{Field: "RiemannTraceReverseDivergenceEvaluation.DeclaringFnRef", Ref: r.DeclaringFnRef, Allowed: []FamilyID{FamilyFunction}},
		{Field: "RiemannTraceReverseDivergenceEvaluation.ConnectionRef", Ref: r.ConnectionRef, Allowed: []FamilyID{FamilyAffineConnection}},
		{Field: "RiemannTraceReverseDivergenceEvaluation.InvTensorRef", Ref: r.InvTensorRef, Allowed: []FamilyID{FamilyTensor}},
		{Field: "RiemannTraceReverseDivergenceEvaluation.CovariantResultRef", Ref: r.CovariantResultRef, Allowed: []FamilyID{FamilyTensorField}},
		{Field: "RiemannTraceReverseDivergenceEvaluation.ResultRef", Ref: r.ResultRef, Allowed: []FamilyID{FamilyTensorField}},
		{Field: "RiemannTraceReverseDivergenceEvaluation.DerivationRef", Ref: r.DerivationRef, Allowed: []FamilyID{FamilyDerivation}},
	}
	for i, c := range r.ConditionRefs {
		checks = append(checks, TargetCheck{
			Field: getRefFieldName("RiemannTraceReverseDivergenceEvaluation.ConditionRefs", i),
			Ref:   c, Allowed: []FamilyID{FamilyCondition},
		})
	}
	return checks
}

// DecodeRiemannTraceReverseDivergenceEvaluation decodes a
// physmath/riemann_trace_reverse_divergence/1 payload. Representation only.
func DecodeRiemannTraceReverseDivergenceEvaluation(payload []byte) (RiemannTraceReverseDivergenceEvaluation, error) {
	d := artifact.NewDecoder(payload)
	trev, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing trace-reverse: %w", err)
	}
	metric, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing metric: %w", err)
	}
	chart, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing chart: %w", err)
	}
	mapPayload, err := d.Bytes()
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing mapping: %w", err)
	}
	mapping, err := DecodeCoordParamMap(mapPayload)
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence bad mapping: %w", err)
	}
	declaring, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing declaring function: %w", err)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing conditions: %w", err)
	}
	conds := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence bad condition %d: %w", i, err)
		}
		conds = append(conds, r)
	}
	conn, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing connection: %w", err)
	}
	invTensor, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing inverse tensor: %w", err)
	}
	covResult, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing covariant result: %w", err)
	}
	result, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing result: %w", err)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: divergence missing derivation: %w", err)
	}
	if !d.Exhausted() {
		return RiemannTraceReverseDivergenceEvaluation{}, fmt.Errorf("physmath: trailing bytes in divergence payload")
	}
	r := RiemannTraceReverseDivergenceEvaluation{
		TraceReverseRef: trev, MetricRef: metric, ChartRef: chart,
		Mapping: mapping, DeclaringFnRef: declaring, ConditionRefs: conds,
		ConnectionRef: conn, InvTensorRef: invTensor, CovariantResultRef: covResult,
		ResultRef: result, DerivationRef: deriv,
	}
	if err := r.Validate(); err != nil {
		return RiemannTraceReverseDivergenceEvaluation{}, err
	}
	return r, nil
}

// divergenceResult is one computed divergence component with its evidence.
type divergenceResult struct {
	expr    Expr
	premise artifact.ArtifactRef
	canon   artifact.ArtifactRef
	out     artifact.ArtifactRef
}

// evalDivergenceCore contracts the derivative index (slot 0) with the first
// tensor index (slot 1) of a rank-three covariant derivative using the
// inverse metric: D_nu = sum_lambda_mu g^lambda_mu H_(lambda,mu,nu), with nu
// free. One Add premise per free index through CANON+SIMPLIFY. It returns
// result expressions with derivation pieces using local ordinals. Callers
// renumber, mint tensors, and wrap.
func evalDivergenceCore(store Store, hessExprs []Expr, invExprs []Expr, dim int) ([]divergenceResult, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
	fail := func(err error) ([]divergenceResult, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
		return nil, nil, nil, nil, err
	}
	var out []divergenceResult
	var steps []DerivationStep
	var premises []artifact.ArtifactRef
	var results []artifact.ArtifactRef
	for nu := 0; nu < dim; nu++ {
		terms := make([]Expr, 0, dim*dim)
		for lam := 0; lam < dim; lam++ {
			for mu := 0; mu < dim; mu++ {
				terms = append(terms, Mul(
					cloneExpr(invExprs[lam*dim+mu]),
					cloneExpr(hessExprs[(lam*dim+mu)*dim+nu]),
				))
			}
		}
		premise, err := PutExpression(store, Add(terms...))
		if err != nil {
			return fail(err)
		}
		canon, err := Apply(store, Invocation{
			ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
			InputRefs: []artifact.ArtifactRef{premise},
		})
		if err != nil {
			return fail(fmt.Errorf("physmath: divergence coefficient canon failed: %w", err))
		}
		res, err := Apply(store, Invocation{
			ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
			InputRefs: []artifact.ArtifactRef{canon.OutputRef},
		})
		if err != nil {
			return fail(fmt.Errorf("physmath: divergence coefficient simplify failed: %w", err))
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
		out = append(out, divergenceResult{expr: e, premise: premise, canon: canon.OutputRef, out: res.OutputRef})
	}
	return out, steps, premises, results, nil
}

// EvaluateRiemannTraceReverseDivergence computes the contracted
// differential identity D_nu = sum_lambda_mu g^lambda_mu
// (nabla_lambda G)_mu_nu for a verified trace-reversed tensor. The source
// metric, chart binding, conditions, and inverse tensor resolve through the
// verified provenance chain; the Levi-Civita connection is built from the
// same source metric and asserted equal to Riemann's recorded connection.
// It returns the rank-one covariant result field, the minted divergence
// record reference, and one flat replayable composite derivation.
func EvaluateRiemannTraceReverseDivergence(store Store, traceReverseRef artifact.ArtifactRef) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, artifact.ArtifactRef{}, Derivation{}, err
	}
	if err := CheckRiemannTraceReverse(store, traceReverseRef); err != nil {
		return fail(fmt.Errorf("physmath: divergence source trace-reverse rejected: %w", err))
	}
	trfam, trpayload, err := store.Lookup(traceReverseRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence trace-reverse unresolvable: %w", err))
	}
	if trfam != FamilyRiemannTraceReverse {
		return fail(fmt.Errorf("physmath: divergence source is not riemann_trace_reverse/1: %w", ErrBadInput))
	}
	trec, err := DecodeRiemannTraceReverseEvaluation(trpayload)
	if err != nil {
		return fail(err)
	}
	cfam, cpayload, err := store.Lookup(trec.ContractionRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence contraction unresolvable: %w", err))
	}
	if cfam != FamilyRiemannContraction {
		return fail(fmt.Errorf("physmath: divergence contraction is not riemann_contraction/1: %w", ErrBadInput))
	}
	crec, err := DecodeRiemannContractionEvaluation(cpayload)
	if err != nil {
		return fail(err)
	}
	sfam, spayload, err := store.Lookup(trec.ScalarRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence scalar unresolvable: %w", err))
	}
	if sfam != FamilyRiemannScalar {
		return fail(fmt.Errorf("physmath: divergence scalar is not riemann_scalar/1: %w", ErrBadInput))
	}
	srec, err := DecodeRiemannScalarEvaluation(spayload)
	if err != nil {
		return fail(err)
	}
	if srec.ContractionRef != trec.ContractionRef {
		return fail(fmt.Errorf("physmath: divergence scalar built over another contraction: %w", ErrBadInput))
	}
	rfam, rpayload, err := store.Lookup(crec.RiemannRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence riemann unresolvable: %w", err))
	}
	if rfam != FamilyRiemannEvaluation {
		return fail(fmt.Errorf("physmath: divergence riemann is not riemann_evaluation/1: %w", ErrBadInput))
	}
	rrec, err := DecodeRiemannEvaluation(rpayload)
	if err != nil {
		return fail(err)
	}
	if srec.MetricRef != rrec.MetricRef || rrec.ChartRef != trec.ChartRef || crec.ChartRef != trec.ChartRef || srec.ChartRef != trec.ChartRef {
		return fail(fmt.Errorf("physmath: divergence provenance chain breaks metric/chart agreement: %w", ErrBadInput))
	}
	mapping, declaring, conds := rrec.Mapping, rrec.DeclaringFnRef, normalizeConditionRefs(rrec.ConditionRefs)
	if err := ValidateCoordBinding(store, trec.ChartRef, mapping, declaring, nil); err != nil {
		return fail(err)
	}
	// Build the connection from the same source metric and require it to
	// equal Riemann's recorded connection, binding both paths together.
	connRef, leviDeriv, err := LeviCivita(store, rrec.MetricRef, trec.ChartRef, mapping, declaring, conds)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence connection failed: %w", err))
	}
	if connRef != rrec.ConnectionRef {
		return fail(fmt.Errorf("physmath: divergence connection differs from riemann evidence: %w", ErrBadInput))
	}
	_ = leviDeriv
	// Covariant derivative of the rank-two G field through the guarded API.
	covField, covDeriv, err := EvaluateCovariantDerivative(store, trec.ResultRef, connRef, mapping, declaring, conds)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence covariant derivative failed: %w", err))
	}
	_, covPayload, err := store.Lookup(covField)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence covariant field unresolvable: %w", err))
	}
	covTF, err := DecodeTensorField(covPayload)
	if err != nil {
		return fail(err)
	}
	if covTF.CoordinateSystemRef != trec.ChartRef {
		return fail(fmt.Errorf("physmath: divergence covariant chart not preserved: %w", ErrBadInput))
	}
	_, covTpayload, err := store.Lookup(covTF.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence covariant tensor unresolvable: %w", err))
	}
	covT, err := DecodeTensor(covTpayload)
	if err != nil {
		return fail(err)
	}
	d := int(covT.Dimension)
	if int(covT.Rank) != 3 || d > MatrixDimBound {
		return fail(fmt.Errorf("physmath: divergence covariant shape wrong: %w", ErrBadInput))
	}
	hessExprs, err := resolveComponents(store, "divergence covariant", covT.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	// Inverse tensor from scalar evidence (verified there; re-resolved
	// here, never recomputed behind the record's back).
	_, ginvPayload, err := store.Lookup(srec.InvTensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence inverse tensor unresolvable: %w", err))
	}
	ginv, err := DecodeTensor(ginvPayload)
	if err != nil {
		return fail(err)
	}
	if int(ginv.Rank) != 2 || int(ginv.Dimension) != d {
		return fail(fmt.Errorf("physmath: divergence inverse shape wrong: %w", ErrBadInput))
	}
	invExprs, err := resolveComponents(store, "divergence inverse", ginv.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	out, coreSteps, corePremises, coreResults, err := evalDivergenceCore(store, hessExprs, invExprs, d)
	if err != nil {
		return fail(err)
	}
	outRefs := make([]artifact.ArtifactRef, len(out))
	for i, r := range out {
		outRefs[i] = r.out
	}
	_, chpayload, err := store.Lookup(trec.ChartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence chart unresolvable: %w", err))
	}
	chart, err := DecodeCoordinateSystem(chpayload)
	if err != nil {
		return fail(err)
	}
	if int(chart.Dimension) != d {
		return fail(fmt.Errorf("physmath: divergence chart dimension %d != tensor dimension %d: %w",
			chart.Dimension, d, ErrBadInput))
	}
	// Free-slot descriptor, minted deterministically so recomputation
	// reproduces it exactly.
	slotPayload, err := Index{Label: "div", Variance: VarianceCovariant, Space: &chart.SpaceRef}.Encode()
	if err != nil {
		return fail(err)
	}
	idxRef, err := store.Put(FamilyIndex, slotPayload)
	if err != nil {
		return fail(err)
	}
	rt := Tensor{Rank: 1, Dimension: uint16(d), IndexRefs: []artifact.ArtifactRef{idxRef}, ComponentRefs: outRefs}
	if err := rt.Validate(); err != nil {
		return fail(err)
	}
	resultTensor, err := store.Put(FamilyTensor, rt.Encode())
	if err != nil {
		return fail(err)
	}
	field := TensorField{TensorRef: resultTensor, CoordinateSystemRef: trec.ChartRef}
	if err := field.Validate(); err != nil {
		return fail(err)
	}
	resultField, err := store.Put(FamilyTensorField, field.Encode())
	if err != nil {
		return fail(err)
	}
	// One flat derivation: covariant evidence, then contraction assembly,
	// with contiguous ordinals in execution order.
	steps := make([]DerivationStep, 0, len(covDeriv.Steps)+len(coreSteps))
	steps = append(steps, covDeriv.Steps...)
	steps = append(steps, coreSteps...)
	for i := range steps {
		steps[i].Ordinal = uint32(i)
	}
	premises := []artifact.ArtifactRef{traceReverseRef, trec.ContractionRef, trec.ScalarRef, rrec.MetricRef, connRef, srec.InvTensorRef, covField}
	premises = append(premises, covDeriv.PremiseRefs...)
	premises = append(premises, corePremises...)
	results := []artifact.ArtifactRef{}
	results = append(results, covDeriv.ResultRefs...)
	results = append(results, coreResults...)
	dd := Derivation{
		PremiseRefs: uniqueRefs(premises),
		Steps:       steps,
		ResultRefs:  uniqueRefs(results),
	}
	if err := Replay(store, dd, nil); err != nil {
		return fail(fmt.Errorf("physmath: divergence derivation does not replay: %w", err))
	}
	ddPayload, err := dd.Encode()
	if err != nil {
		return fail(err)
	}
	derivRef, err := store.Put(FamilyDerivation, ddPayload)
	if err != nil {
		return fail(err)
	}
	rec := RiemannTraceReverseDivergenceEvaluation{
		TraceReverseRef: traceReverseRef, MetricRef: rrec.MetricRef, ChartRef: trec.ChartRef,
		Mapping: mapping, DeclaringFnRef: declaring, ConditionRefs: conds,
		ConnectionRef: connRef, InvTensorRef: srec.InvTensorRef, CovariantResultRef: covField,
		ResultRef: resultField, DerivationRef: derivRef,
	}
	if err := rec.Validate(); err != nil {
		return fail(err)
	}
	drecPayload, err := rec.Encode()
	if err != nil {
		return fail(err)
	}
	recRef, err := store.Put(FamilyRiemannTraceReverseDivergence, drecPayload)
	if err != nil {
		return fail(err)
	}
	return resultField, recRef, dd, nil
}

// resolveAndValidateDivergence resolves the stored divergence record and
// validates its complete agreement from stored artifacts: family resolution,
// source revalidation, pair linkage, single-metric/chart/binding agreement,
// connection and inverse evidence, ordered component comparison against
// idempotent recomputation, result index contract, and derivation linkage.
// It performs no replay itself; callers re-execute exactly the validated
// derivation. Neither CheckRiemannTraceReverseDivergence nor
// ReplayRiemannTraceReverseDivergence calls the other.
func resolveAndValidateDivergence(store Store, ref artifact.ArtifactRef) (RiemannTraceReverseDivergenceEvaluation, Derivation, error) {
	fail := func(err error) (RiemannTraceReverseDivergenceEvaluation, Derivation, error) {
		return RiemannTraceReverseDivergenceEvaluation{}, Derivation{}, err
	}
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence record unresolvable: %w", err))
	}
	if fam != FamilyRiemannTraceReverseDivergence {
		return fail(fmt.Errorf("physmath: target is not riemann_trace_reverse_divergence/1: %w", ErrBadInput))
	}
	rec, err := DecodeRiemannTraceReverseDivergenceEvaluation(payload)
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
		return fail(fmt.Errorf("physmath: divergence derivation unresolvable: %w", err))
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return fail(err)
	}
	// The source trace-reverse record must itself verify; divergence over
	// an unverified source is never accepted.
	if err := CheckRiemannTraceReverse(store, rec.TraceReverseRef); err != nil {
		return fail(fmt.Errorf("physmath: divergence source trace-reverse rejected: %w", err))
	}
	trfam, trpayload, err := store.Lookup(rec.TraceReverseRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence trace-reverse unresolvable: %w", err))
	}
	if trfam != FamilyRiemannTraceReverse {
		return fail(fmt.Errorf("physmath: divergence source is not riemann_trace_reverse/1: %w", ErrBadInput))
	}
	trec, err := DecodeRiemannTraceReverseEvaluation(trpayload)
	if err != nil {
		return fail(err)
	}
	cfam, cpayload, err := store.Lookup(trec.ContractionRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence contraction unresolvable: %w", err))
	}
	if cfam != FamilyRiemannContraction {
		return fail(fmt.Errorf("physmath: divergence contraction is not riemann_contraction/1: %w", ErrBadInput))
	}
	crec, err := DecodeRiemannContractionEvaluation(cpayload)
	if err != nil {
		return fail(err)
	}
	sfam, spayload, err := store.Lookup(trec.ScalarRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence scalar unresolvable: %w", err))
	}
	if sfam != FamilyRiemannScalar {
		return fail(fmt.Errorf("physmath: divergence scalar is not riemann_scalar/1: %w", ErrBadInput))
	}
	srec, err := DecodeRiemannScalarEvaluation(spayload)
	if err != nil {
		return fail(err)
	}
	if srec.ContractionRef != trec.ContractionRef {
		return fail(fmt.Errorf("physmath: divergence scalar built over another contraction: %w", ErrBadInput))
	}
	rfam, rpayload, err := store.Lookup(crec.RiemannRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence riemann unresolvable: %w", err))
	}
	if rfam != FamilyRiemannEvaluation {
		return fail(fmt.Errorf("physmath: divergence riemann is not riemann_evaluation/1: %w", ErrBadInput))
	}
	rrec, err := DecodeRiemannEvaluation(rpayload)
	if err != nil {
		return fail(err)
	}
	// One metric, one chart, one exact binding across the whole chain. A
	// substituted metric, chart, mapping, declaring context, or condition
	// set fails here even if every inner derivation replays.
	if rrec.MetricRef != rec.MetricRef || srec.MetricRef != rec.MetricRef {
		return fail(fmt.Errorf("physmath: divergence metric breaks the provenance chain: %w", ErrBadInput))
	}
	if trec.ChartRef != rec.ChartRef || crec.ChartRef != rec.ChartRef || srec.ChartRef != rec.ChartRef || rrec.ChartRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: divergence chart breaks the provenance chain: %w", ErrBadInput))
	}
	if !equalRefSets(rrec.ConditionRefs, rec.ConditionRefs) {
		return fail(fmt.Errorf("physmath: divergence condition set differs from riemann evidence: %w", ErrBadInput))
	}
	// Mapping equality compares canonical bytes; Decode already validated
	// structure.
	recMapBytes, err := rec.Mapping.Encode()
	if err != nil {
		return fail(err)
	}
	rMapBytes, err := rrec.Mapping.Encode()
	if err != nil {
		return fail(err)
	}
	if string(rMapBytes) != string(recMapBytes) {
		return fail(fmt.Errorf("physmath: divergence mapping differs from riemann evidence: %w", ErrBadInput))
	}
	if rrec.DeclaringFnRef != rec.DeclaringFnRef {
		return fail(fmt.Errorf("physmath: divergence declaring function differs from riemann evidence: %w", ErrBadInput))
	}
	// Connection recomputed from the recorded metric must equal the
	// recorded connection (and hence Riemann's, by construction).
	reconn, _, err := LeviCivita(store, rec.MetricRef, rec.ChartRef, rec.Mapping, rec.DeclaringFnRef, rec.ConditionRefs)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence connection recomputation failed: %w", err))
	}
	if reconn != rec.ConnectionRef {
		return fail(fmt.Errorf("physmath: divergence connection does not match the source metric"))
	}
	// Inverse tensor must equal scalar evidence (reference equality: the
	// same artifact, verified there against the same metric).
	if srec.InvTensorRef != rec.InvTensorRef {
		return fail(fmt.Errorf("physmath: divergence inverse differs from scalar evidence: %w", ErrBadInput))
	}
	_, ginvPayload, err := store.Lookup(rec.InvTensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence inverse tensor unresolvable: %w", err))
	}
	ginv, err := DecodeTensor(ginvPayload)
	if err != nil {
		return fail(err)
	}
	_, chpayload, err := store.Lookup(rec.ChartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence chart unresolvable: %w", err))
	}
	chart, err := DecodeCoordinateSystem(chpayload)
	if err != nil {
		return fail(err)
	}
	dim := int(chart.Dimension)
	if dim > MatrixDimBound {
		return fail(fmt.Errorf("physmath: divergence dimension %d exceeds bound %d: %w",
			dim, MatrixDimBound, ErrBadInput))
	}
	if int(ginv.Rank) != 2 || int(ginv.Dimension) != dim {
		return fail(fmt.Errorf("physmath: divergence inverse shape wrong: %w", ErrBadInput))
	}
	// Covariant-derivative evidence: resolve the recorded field and require
	// rank 3 over the chart before recomputing.
	_, covPayload, err := store.Lookup(rec.CovariantResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence covariant result unresolvable: %w", err))
	}
	covTF, err := DecodeTensorField(covPayload)
	if err != nil {
		return fail(err)
	}
	if covTF.CoordinateSystemRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: divergence covariant chart not preserved: %w", ErrBadInput))
	}
	_, covTpayload, err := store.Lookup(covTF.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence covariant tensor unresolvable: %w", err))
	}
	covT, err := DecodeTensor(covTpayload)
	if err != nil {
		return fail(err)
	}
	if int(covT.Rank) != 3 || int(covT.Dimension) != dim {
		return fail(fmt.Errorf("physmath: divergence covariant shape wrong: %w", ErrBadInput))
	}
	if len(covT.IndexRefs) != 3 {
		return fail(fmt.Errorf("physmath: divergence covariant index count wrong: %w", ErrBadInput))
	}
	for i, r := range covT.IndexRefs {
		idx, err := resolveIndex(store, "divergence covariant slot", r)
		if err != nil {
			return fail(err)
		}
		if idx.Variance != VarianceCovariant {
			return fail(fmt.Errorf("physmath: divergence covariant slot %d variance wrong: %w", i, ErrBadInput))
		}
		if idx.Space == nil || *idx.Space != chart.SpaceRef {
			return fail(fmt.Errorf("physmath: divergence covariant slot %d space mismatch: %w", i, ErrBadInput))
		}
	}
	// Recompute the covariant derivative idempotently: identical inputs
	// reproduce identical references, so recomputation verifies rather
	// than repairs. The recomputed field must equal the recorded evidence.
	// (The recomputation resolves the trace-reverse result itself and
	// fails closed when unresolvable, so no separate lookup is needed.)
	reCovField, reCovDeriv, err := EvaluateCovariantDerivative(store, trec.ResultRef, rec.ConnectionRef, rec.Mapping, rec.DeclaringFnRef, rec.ConditionRefs)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence covariant recomputation failed: %w", err))
	}
	if reCovField != rec.CovariantResultRef {
		return fail(fmt.Errorf("physmath: divergence covariant evidence does not match recomputation"))
	}
	// Contraction recompute with ordered comparison.
	hexprs, err := resolveComponents(store, "divergence covariant", covT.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	gexprs, err := resolveComponents(store, "divergence inverse", ginv.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	out, coreSteps, corePremises, coreResults, err := evalDivergenceCore(store, hexprs, gexprs, dim)
	if err != nil {
		return fail(err)
	}
	nCov := len(reCovDeriv.Steps)
	if len(d.Steps) < nCov {
		return fail(fmt.Errorf("physmath: divergence derivation shorter than its covariant evidence"))
	}
	for i, s := range reCovDeriv.Steps {
		want, err := s.Encode()
		if err != nil {
			return fail(err)
		}
		got, err := d.Steps[i].Encode()
		if err != nil {
			return fail(err)
		}
		if string(got) != string(want) {
			return fail(fmt.Errorf("physmath: divergence covariant evidence step %d mismatch", i))
		}
	}
	coreStored := d.Steps[nCov:]
	if len(coreStored) != len(coreSteps) {
		return fail(fmt.Errorf("physmath: divergence core steps %d != recomputed %d",
			len(coreStored), len(coreSteps)))
	}
	for i := 0; i < len(out); i++ {
		s0, s1 := coreStored[2*i], coreStored[2*i+1]
		if s0.ProducerNamespace != NamespacePhysMath || s0.RuleID != RuleExprCanon {
			return fail(fmt.Errorf("physmath: divergence assembly step %d must be CANON", i))
		}
		if s1.ProducerNamespace != NamespacePhysMath || s1.RuleID != RuleExprSimplify {
			return fail(fmt.Errorf("physmath: divergence assembly step %d must be SIMPLIFY", i))
		}
		if len(s0.InputRefs) != 1 || s0.InputRefs[0] != out[i].premise {
			return fail(fmt.Errorf("physmath: divergence assembly step %d premise mismatch", i))
		}
		if s0.OutputRef != out[i].canon || len(s1.InputRefs) != 1 || s1.InputRefs[0] != out[i].canon {
			return fail(fmt.Errorf("physmath: divergence assembly step %d linkage broken", i))
		}
		if s1.OutputRef != out[i].out {
			return fail(fmt.Errorf("physmath: divergence assembly step %d output mismatch", i))
		}
	}
	wantPremises := []artifact.ArtifactRef{rec.TraceReverseRef, trec.ContractionRef, trec.ScalarRef, rrec.MetricRef, rec.ConnectionRef, srec.InvTensorRef, reCovField}
	wantPremises = append(wantPremises, reCovDeriv.PremiseRefs...)
	wantPremises = append(wantPremises, corePremises...)
	if !refSetEqual(d.PremiseRefs, uniqueRefs(wantPremises)) {
		return fail(fmt.Errorf("physmath: divergence premises do not match recomputation"))
	}
	wantResults := []artifact.ArtifactRef{}
	wantResults = append(wantResults, reCovDeriv.ResultRefs...)
	wantResults = append(wantResults, coreResults...)
	if !refSetEqual(d.ResultRefs, uniqueRefs(wantResults)) {
		return fail(fmt.Errorf("physmath: divergence results do not match recomputation"))
	}
	// Result field: same chart, rank-1 covariant tensor, deterministic slot
	// descriptor, chart-space agreement, ordered components byte-equal.
	rfam2, rpayload2, err := store.Lookup(rec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence result unresolvable: %w", err))
	}
	if rfam2 != FamilyTensorField {
		return fail(fmt.Errorf("physmath: divergence result is not tensor_field/1: %w", ErrBadInput))
	}
	rff, err := DecodeTensorField(rpayload2)
	if err != nil {
		return fail(err)
	}
	if rff.CoordinateSystemRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: divergence result chart not preserved: %w", ErrBadInput))
	}
	rtfam, rtpayload, err := store.Lookup(rff.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: divergence result tensor unresolvable: %w", err))
	}
	if rtfam != FamilyTensor {
		return fail(fmt.Errorf("physmath: divergence result tensor is not tensor/1: %w", ErrBadInput))
	}
	rtn, err := DecodeTensor(rtpayload)
	if err != nil {
		return fail(err)
	}
	if int(rtn.Rank) != 1 || int(rtn.Dimension) != dim {
		return fail(fmt.Errorf("physmath: divergence result shape wrong: %w", ErrBadInput))
	}
	slotPayload, err := Index{Label: "div", Variance: VarianceCovariant, Space: &chart.SpaceRef}.Encode()
	if err != nil {
		return fail(err)
	}
	wantSlot, err := store.Put(FamilyIndex, slotPayload)
	if err != nil {
		return fail(err)
	}
	if len(rtn.IndexRefs) != 1 || rtn.IndexRefs[0] != wantSlot {
		return fail(fmt.Errorf("physmath: divergence result slot identity broken: %w", ErrBadInput))
	}
	idx, err := resolveIndex(store, "divergence result slot", rtn.IndexRefs[0])
	if err != nil {
		return fail(err)
	}
	if idx.Variance != VarianceCovariant {
		return fail(fmt.Errorf("physmath: divergence result slot variance wrong: %w", ErrBadInput))
	}
	if idx.Space == nil || *idx.Space != chart.SpaceRef {
		return fail(fmt.Errorf("physmath: divergence result slot space mismatch: %w", ErrBadInput))
	}
	if len(rtn.ComponentRefs) != len(out) {
		return fail(fmt.Errorf("physmath: divergence result component count mismatch: %w", ErrBadInput))
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
			return fail(fmt.Errorf("physmath: divergence result component %d mismatch: %w", i, ErrBadInput))
		}
	}
	return rec, d, nil
}

// CheckRiemannTraceReverseDivergence is the store-aware provenance gate: it
// validates the complete stored record against its derivation through the
// shared helper, then replays the validated derivation before trusting the
// result.
func CheckRiemannTraceReverseDivergence(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateDivergence(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: divergence derivation does not replay: %w", err)
	}
	return nil
}

// ReplayRiemannTraceReverseDivergence is artifact-level replay for a stored
// divergence record: it validates the complete chain (metric, binding,
// connection and inverse evidence, derivative and contraction linkage) and
// then re-executes the validated derivation. A corrupted record is rejected
// even when an inner derivation remains independently replayable.
func ReplayRiemannTraceReverseDivergence(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateDivergence(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: divergence derivation does not replay: %w", err)
	}
	return nil
}
