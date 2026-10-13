package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// Riemann tensor from a metric-derived Levi-Civita connection (Plan 1.15,
// first curvature milestone).
//
// Convention (declared once, used everywhere here): with connection
// coefficients Gamma^rho_mu_nu in row-major (rho,mu,nu) order and result
// components R^rho_sigma_mu_nu in row-major (rho,sigma,mu,nu) order,
//
//	R^rho_sigma_mu_nu = d_mu Gamma^rho_nu_sigma - d_nu Gamma^rho_mu_sigma
//	                  + Gamma^rho_mu_lambda Gamma^lambda_nu_sigma
//	                  - Gamma^rho_nu_lambda Gamma^lambda_mu_sigma.
//
// The result variance is [CONTRAVARIANT, COVARIANT, COVARIANT, COVARIANT]:
// components are R^rho_sigma_mu_nu. For a metric with g_xx = 1 the mixed
// component R^x_yxy coincides with the fully covariant R_xyxy; that equality
// is a property of such a golden metric, never a general equivalence.
//
// Trusted coordinate differentiation in reusable geometry must use the guarded
// coordinate-differentiation path and preserve derivation provenance. The
// mathematics below executes through the existing DIFFERENTIATE-001,
// EXPR-CANON-001, and EXPR-SIMPLIFY-001 rules; no new executable rule family
// is introduced.
//
// Scope (first milestone only): Riemann representation, execution, Check, and
// Replay from a source metric. No Ricci or scalar execution, no Bianchi
// identity claims, no metric-compatibility claims.

// FamilyRiemannEvaluation is the metric-derived Riemann record:
// chart-bound input metric, derived connection, computed chart-bound result
// field, and the replayable composite derivation. Family
// physmath/riemann_evaluation/1.
var FamilyRiemannEvaluation = FamilyID{NamespacePhysMath, "riemann_evaluation/1", "1"}

// RiemannEvaluation links source metric, chart binding, derived connection,
// per-component guarded differentiation records, computed result field, and
// derivation evidence. Validate checks structure only;
// CheckRiemannEvaluation resolves and recomputes.
type RiemannEvaluation struct {
	MetricRef         artifact.ArtifactRef   // → symmetric_bilinear/1 (source)
	ChartRef          artifact.ArtifactRef   // → coordinate_system/1
	Mapping           CoordParamMap          // inline ordered chart→parameter binding
	DeclaringFnRef    artifact.ArtifactRef   // → function/1
	ConditionRefs     []artifact.ArtifactRef // SET → condition/1
	ConnectionRef     artifact.ArtifactRef   // → affine_connection/1 (derived)
	PartialRecordRefs []artifact.ArtifactRef // SEQUENCE, 2 per result component in (rho,sigma,mu,nu) order
	ResultRef         artifact.ArtifactRef   // → tensor_field/1 (rank 4)
	DerivationRef     artifact.ArtifactRef   // required → derivation/1 (composite)
}

// Family returns FamilyRiemannEvaluation.
func (RiemannEvaluation) Family() FamilyID { return FamilyRiemannEvaluation }

// Encode returns the canonical payload in declaration order.
func (r RiemannEvaluation) Encode() ([]byte, error) {
	out := r.MetricRef.Encode()
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
	out = append(out, encodeRefSequence(r.PartialRecordRefs)...)
	out = append(out, r.ResultRef.Encode()...)
	return append(out, r.DerivationRef.Encode()...), nil
}

// Validate enforces grammar, a non-empty mapping with distinct PIDs, and
// condition-set uniqueness. Chart/range/declared-identity gates need the
// store: see CheckRiemannEvaluation.
func (r RiemannEvaluation) Validate() error {
	if len(r.Mapping.Entries) == 0 {
		return fmt.Errorf("physmath: riemann evaluation needs a non-empty mapping: %w", ErrBadInput)
	}
	seen := map[ParameterID]bool{}
	for i, e := range r.Mapping.Entries {
		if seen[e.ParameterID] {
			return fmt.Errorf("physmath: riemann evaluation duplicate parameter identity at %d: %w", i, ErrBadInput)
		}
		seen[e.ParameterID] = true
	}
	fields := map[string]artifact.ArtifactRef{
		"RiemannEvaluation.MetricRef":      r.MetricRef,
		"RiemannEvaluation.ChartRef":       r.ChartRef,
		"RiemannEvaluation.DeclaringFnRef": r.DeclaringFnRef,
		"RiemannEvaluation.ConnectionRef":  r.ConnectionRef,
		"RiemannEvaluation.ResultRef":      r.ResultRef,
		"RiemannEvaluation.DerivationRef":  r.DerivationRef,
	}
	for i, ref := range r.PartialRecordRefs {
		fields[fmt.Sprintf("RiemannEvaluation.PartialRecordRefs[%d]", i)] = ref
	}
	for i, ref := range r.ConditionRefs {
		fields[fmt.Sprintf("RiemannEvaluation.ConditionRefs[%d]", i)] = ref
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	return checkDuplicateRefs("RiemannEvaluation.ConditionRefs", r.ConditionRefs)
}

// TargetChecks: MetricRef → symmetric_bilinear/1; ChartRef →
// coordinate_system/1; DeclaringFnRef → function/1; ConditionRefs →
// condition/1; ConnectionRef → affine_connection/1; PartialRecordRefs →
// coordinate_differentiation/1; ResultRef → tensor_field/1; DerivationRef →
// derivation/1. Resolved only by the store-aware check.
func (r RiemannEvaluation) TargetChecks() []TargetCheck {
	checks := []TargetCheck{
		{Field: "RiemannEvaluation.MetricRef", Ref: r.MetricRef, Allowed: []FamilyID{FamilySymmetricBilinear}},
		{Field: "RiemannEvaluation.ChartRef", Ref: r.ChartRef, Allowed: []FamilyID{FamilyCoordinateSystem}},
		{Field: "RiemannEvaluation.DeclaringFnRef", Ref: r.DeclaringFnRef, Allowed: []FamilyID{FamilyFunction}},
		{Field: "RiemannEvaluation.ConnectionRef", Ref: r.ConnectionRef, Allowed: []FamilyID{FamilyAffineConnection}},
		{Field: "RiemannEvaluation.ResultRef", Ref: r.ResultRef, Allowed: []FamilyID{FamilyTensorField}},
		{Field: "RiemannEvaluation.DerivationRef", Ref: r.DerivationRef, Allowed: []FamilyID{FamilyDerivation}},
	}
	for i, ref := range r.PartialRecordRefs {
		checks = append(checks, TargetCheck{
			Field: fmt.Sprintf("RiemannEvaluation.PartialRecordRefs[%d]", i),
			Ref:   ref, Allowed: []FamilyID{FamilyCoordinateDifferentiation},
		})
	}
	for i, ref := range r.ConditionRefs {
		checks = append(checks, TargetCheck{
			Field: fmt.Sprintf("RiemannEvaluation.ConditionRefs[%d]", i),
			Ref:   ref, Allowed: []FamilyID{FamilyCondition},
		})
	}
	return checks
}

// DecodeRiemannEvaluation decodes a physmath/riemann_evaluation/1 payload in
// declaration order. Representation only: no store Lookup, no execution.
func DecodeRiemannEvaluation(payload []byte) (RiemannEvaluation, error) {
	d := artifact.NewDecoder(payload)
	metric, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannEvaluation{}, fmt.Errorf("physmath: riemann missing metric: %w", err)
	}
	chart, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannEvaluation{}, fmt.Errorf("physmath: riemann missing chart: %w", err)
	}
	mapPayload, err := d.Bytes()
	if err != nil {
		return RiemannEvaluation{}, fmt.Errorf("physmath: riemann missing mapping: %w", err)
	}
	mapping, err := DecodeCoordParamMap(mapPayload)
	if err != nil {
		return RiemannEvaluation{}, fmt.Errorf("physmath: riemann bad mapping: %w", err)
	}
	declaring, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannEvaluation{}, fmt.Errorf("physmath: riemann missing declaring function: %w", err)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return RiemannEvaluation{}, fmt.Errorf("physmath: riemann missing conditions: %w", err)
	}
	conds := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return RiemannEvaluation{}, fmt.Errorf("physmath: riemann bad condition %d: %w", i, err)
		}
		conds = append(conds, r)
	}
	conn, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannEvaluation{}, fmt.Errorf("physmath: riemann missing connection: %w", err)
	}
	m, err := d.SequenceCount()
	if err != nil {
		return RiemannEvaluation{}, fmt.Errorf("physmath: riemann missing partial records: %w", err)
	}
	records := make([]artifact.ArtifactRef, 0, m)
	for i := 0; i < m; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return RiemannEvaluation{}, fmt.Errorf("physmath: riemann bad partial record %d: %w", i, err)
		}
		records = append(records, r)
	}
	result, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannEvaluation{}, fmt.Errorf("physmath: riemann missing result: %w", err)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannEvaluation{}, fmt.Errorf("physmath: riemann missing derivation: %w", err)
	}
	if !d.Exhausted() {
		return RiemannEvaluation{}, fmt.Errorf("physmath: trailing bytes in riemann-evaluation payload")
	}
	r := RiemannEvaluation{
		MetricRef: metric, ChartRef: chart, Mapping: mapping,
		DeclaringFnRef: declaring, ConditionRefs: conds, ConnectionRef: conn,
		PartialRecordRefs: records, ResultRef: result, DerivationRef: deriv,
	}
	if err := r.Validate(); err != nil {
		return RiemannEvaluation{}, err
	}
	return r, nil
}

// riemannInputs is the fully resolved bundle for one Riemann computation.
// All gates have passed when this value exists.
type riemannInputs struct {
	dim       int
	chart     CoordinateSystem
	chartRef  artifact.ArtifactRef
	mapping   CoordParamMap
	declaring artifact.ArtifactRef
	conds     []artifact.ArtifactRef // normalized condition refs
	connRefs  []artifact.ArtifactRef // row-major (rho,mu,nu), dim^3
	connExprs []Expr
	idxRefs   []artifact.ArtifactRef // ordered (rho,sigma,mu,nu) slot descriptors
}

// gammaIndex maps a (rho,mu,nu) slot triple to the flat connection offset,
// matching the canonical AffineConnection coefficient order.
func gammaIndex(rho, mu, nu, d int) int {
	return (rho*d+mu)*d + nu
}

// riemannIndex maps a (rho,sigma,mu,nu) slot tuple to the flat result offset,
// matching the canonical tensor component order.
func riemannIndex(rho, sigma, mu, nu, d int) int {
	return ((rho*d+sigma)*d+mu)*d + nu
}

// resolveRiemannInputs enforces every gate in validation order — families,
// metric/chart dimensions, bounds, and the shared chart-binding contract
// over the metric components — before any artifact is minted. Singular or
// otherwise non-invertible metrics fail later inside LeviCivita's general
// inverse path; that failure propagates here before any Riemann result.
func resolveRiemannInputs(store Store, metricRef, chartRef artifact.ArtifactRef, mapping CoordParamMap, declaringFnRef artifact.ArtifactRef, conds []artifact.ArtifactRef) (*riemannInputs, error) {
	fail := func(err error) (*riemannInputs, error) { return nil, err }
	mfam, mpayload, err := store.Lookup(metricRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: riemann metric unresolvable: %w", err))
	}
	if mfam != FamilySymmetricBilinear {
		return fail(fmt.Errorf("physmath: riemann metric is not symmetric_bilinear/1: %w", ErrBadInput))
	}
	metric, err := DecodeSymmetricBilinear(mpayload)
	if err != nil {
		return fail(err)
	}
	cfam, cpayload, err := store.Lookup(chartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: riemann chart unresolvable: %w", err))
	}
	if cfam != FamilyCoordinateSystem {
		return fail(fmt.Errorf("physmath: riemann chart is not coordinate_system/1: %w", ErrBadInput))
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		return fail(err)
	}
	if int(chart.Dimension) != int(metric.Dimension) {
		return fail(fmt.Errorf("physmath: chart dimension %d != metric dimension %d: %w",
			chart.Dimension, metric.Dimension, ErrBadInput))
	}
	d := int(metric.Dimension)
	if d > MatrixDimBound {
		return fail(fmt.Errorf("physmath: riemann dimension %d exceeds bound %d: %w",
			d, MatrixDimBound, ErrBadInput))
	}
	if err := ValidateCoordBinding(store, chartRef, mapping, declaringFnRef, metric.ComponentRefs); err != nil {
		return fail(err)
	}
	return &riemannInputs{
		dim: d, chart: chart, chartRef: chartRef,
		mapping: mapping, declaring: declaringFnRef,
		conds: normalizeConditionRefs(conds),
	}, nil
}

// riemannResult is one computed result component with its evidence refs.
type riemannResult struct {
	expr    Expr
	premise artifact.ArtifactRef
	canon   artifact.ArtifactRef
	out     artifact.ArtifactRef
}

// evalRiemannCore runs the shared computation: guarded partials of every
// connection coefficient with respect to every coordinate, then per-result
// assembly in (rho,sigma,mu,nu) row-major order. It returns result
// expressions with derivation pieces using local ordinals, the ordered
// per-component record references (two per component: the mu-partial and the
// nu-partial evidence), and the premise/result sets. Callers renumber, mint
// tensors, and wrap.
func evalRiemannCore(store Store, in *riemannInputs) ([]riemannResult, []artifact.ArtifactRef, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
	fail := func(err error) ([]riemannResult, []artifact.ArtifactRef, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
		return nil, nil, nil, nil, nil, err
	}
	d := in.dim
	nGamma := d * d * d
	// Phase A: partials[mu][k] for every coordinate mu and coefficient k,
	// with the minted differentiation record retained alongside each output.
	partials := make([][]artifact.ArtifactRef, d)
	partialRecs := make([][]artifact.ArtifactRef, d)
	var premises []artifact.ArtifactRef
	var results []artifact.ArtifactRef
	var steps []DerivationStep
	for mu := 0; mu < d; mu++ {
		partials[mu] = make([]artifact.ArtifactRef, nGamma)
		partialRecs[mu] = make([]artifact.ArtifactRef, nGamma)
		for k := 0; k < nGamma; k++ {
			outRef, recordRef, pd, err := DifferentiateCoordinate(store,
				in.connRefs[k], in.chartRef, uint32(mu), in.mapping, in.declaring, 1, in.conds)
			if err != nil {
				return fail(riemannPartialErr(mu, k, err))
			}
			if len(pd.Steps) != 1 {
				return fail(fmt.Errorf("physmath: guarded riemann partial derivation shape broken: %w", ErrBadInput))
			}
			partials[mu][k] = outRef
			partialRecs[mu][k] = recordRef
			premises = append(premises, in.connRefs[k], recordRef)
			results = append(results, outRef)
			steps = append(steps, pd.Steps[0])
		}
	}
	partialExprs := make([][]Expr, d)
	for mu := 0; mu < d; mu++ {
		partialExprs[mu] = make([]Expr, nGamma)
		for k := 0; k < nGamma; k++ {
			e, err := resolveExpression(store, partials[mu][k])
			if err != nil {
				return fail(err)
			}
			partialExprs[mu][k] = e
		}
	}
	// Phase B: assembly per result component in row-major order.
	var out []riemannResult
	var compRecords []artifact.ArtifactRef
	for rho := 0; rho < d; rho++ {
		for sigma := 0; sigma < d; sigma++ {
			for mu := 0; mu < d; mu++ {
				for nu := 0; nu < d; nu++ {
					terms := []Expr{
						cloneExpr(partialExprs[mu][gammaIndex(rho, nu, sigma, d)]),
						Neg(cloneExpr(partialExprs[nu][gammaIndex(rho, mu, sigma, d)])),
					}
					for lam := 0; lam < d; lam++ {
						terms = append(terms, Mul(
							cloneExpr(in.connExprs[gammaIndex(rho, mu, lam, d)]),
							cloneExpr(in.connExprs[gammaIndex(lam, nu, sigma, d)]),
						))
						terms = append(terms, Neg(Mul(
							cloneExpr(in.connExprs[gammaIndex(rho, nu, lam, d)]),
							cloneExpr(in.connExprs[gammaIndex(lam, mu, sigma, d)]),
						)))
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
						return fail(fmt.Errorf("physmath: riemann coefficient canon failed: %w", err))
					}
					res, err := Apply(store, Invocation{
						ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
						InputRefs: []artifact.ArtifactRef{canon.OutputRef},
					})
					if err != nil {
						return fail(fmt.Errorf("physmath: riemann coefficient simplify failed: %w", err))
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
					out = append(out, riemannResult{expr: e, premise: premise, canon: canon.OutputRef, out: res.OutputRef})
					// Ordered per-component evidence: the mu-partial and
					// nu-partial differentiation records (content-addressed
					// sharing yields identical refs where inputs coincide).
					compRecords = append(compRecords,
						partialRecs[mu][gammaIndex(rho, nu, sigma, d)],
						partialRecs[nu][gammaIndex(rho, mu, sigma, d)])
				}
			}
		}
	}
	return out, compRecords, steps, premises, results, nil
}

// riemannPartialErr reports a guarded-partial failure with its coordinate/component position.
func riemannPartialErr(mu, k int, err error) error {
	return fmt.Errorf("physmath: riemann partial (%d,%d) failed: %w", mu, k, err)
}

// EvaluateRiemann computes the Riemann tensor of the Levi-Civita connection
// derived from the source metric, through the existing executable path:
// general inverse, guarded Levi-Civita partials, guarded Riemann partials
// via DIFFERENTIATE-001, and per-component CANON+SIMPLIFY assembly. It
// returns the chart-bound rank-4 result field (variance
// [CONTRAVARIANT, COVARIANT, COVARIANT, COVARIANT], components
// R^rho_sigma_mu_nu in row-major order), the minted riemann_evaluation/1
// record reference, and one flat replayable composite derivation.
//
// A singular or otherwise non-invertible source metric fails closed inside
// the inverse path before any Riemann result is accepted. No new rule; the
// operator.go covariant_derivative reservation stays non-executable.
func EvaluateRiemann(store Store, metricRef, chartRef artifact.ArtifactRef, mapping CoordParamMap, declaringFnRef artifact.ArtifactRef, conds []artifact.ArtifactRef) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, artifact.ArtifactRef{}, Derivation{}, err
	}
	in, err := resolveRiemannInputs(store, metricRef, chartRef, mapping, declaringFnRef, conds)
	if err != nil {
		return fail(err)
	}
	connRef, leviDeriv, err := LeviCivita(store, metricRef, chartRef, mapping, declaringFnRef, conds)
	if err != nil {
		return fail(fmt.Errorf("physmath: riemann connection failed: %w", err))
	}
	_, connPayload, err := store.Lookup(connRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: riemann connection unresolvable: %w", err))
	}
	conn, err := DecodeAffineConnection(connPayload)
	if err != nil {
		return fail(err)
	}
	cexprs, err := resolveComponents(store, "riemann connection", conn.CoefficientRefs)
	if err != nil {
		return fail(err)
	}
	in.connRefs = append([]artifact.ArtifactRef{}, conn.CoefficientRefs...)
	in.connExprs = cexprs
	// Derivative-free slot descriptors, minted deterministically so
	// recomputation reproduces them exactly.
	slotLabels := []string{"rho", "sigma", "mu", "nu"}
	slotVariances := []IndexVariance{VarianceContravariant, VarianceCovariant, VarianceCovariant, VarianceCovariant}
	in.idxRefs = make([]artifact.ArtifactRef, 4)
	for i := range slotLabels {
		p, err := Index{Label: slotLabels[i], Variance: slotVariances[i], Space: &in.chart.SpaceRef}.Encode()
		if err != nil {
			return fail(err)
		}
		idxRef, err := store.Put(FamilyIndex, p)
		if err != nil {
			return fail(err)
		}
		in.idxRefs[i] = idxRef
	}
	out, compRecords, coreSteps, corePremises, coreResults, err := evalRiemannCore(store, in)
	if err != nil {
		return fail(err)
	}
	outRefs := make([]artifact.ArtifactRef, len(out))
	for i, r := range out {
		outRefs[i] = r.out
	}
	rt := Tensor{Rank: 4, Dimension: uint16(in.dim),
		IndexRefs: append([]artifact.ArtifactRef{}, in.idxRefs...), ComponentRefs: outRefs}
	if err := rt.Validate(); err != nil {
		return fail(err)
	}
	resultTensor, err := store.Put(FamilyTensor, rt.Encode())
	if err != nil {
		return fail(err)
	}
	field := TensorField{TensorRef: resultTensor, CoordinateSystemRef: in.chartRef}
	if err := field.Validate(); err != nil {
		return fail(err)
	}
	resultField, err := store.Put(FamilyTensorField, field.Encode())
	if err != nil {
		return fail(err)
	}
	// One flat derivation: Levi-Civita evidence, then core partials and
	// assembly, with contiguous ordinals in execution order. The declaring
	// function is a genuine premise consumed by every partial step.
	steps := make([]DerivationStep, 0, len(leviDeriv.Steps)+len(coreSteps))
	steps = append(steps, leviDeriv.Steps...)
	steps = append(steps, coreSteps...)
	for i := range steps {
		steps[i].Ordinal = uint32(i)
	}
	d := Derivation{
		PremiseRefs: uniqueRefs(append(append([]artifact.ArtifactRef{declaringFnRef}, leviDeriv.PremiseRefs...), corePremises...)),
		Steps:       steps,
		ResultRefs:  uniqueRefs(append(append([]artifact.ArtifactRef{}, leviDeriv.ResultRefs...), coreResults...)),
	}
	if err := Replay(store, d, nil); err != nil {
		return fail(fmt.Errorf("physmath: riemann derivation does not replay: %w", err))
	}
	dpayload, err := d.Encode()
	if err != nil {
		return fail(err)
	}
	derivRef, err := store.Put(FamilyDerivation, dpayload)
	if err != nil {
		return fail(err)
	}
	rec := RiemannEvaluation{
		MetricRef: metricRef, ChartRef: chartRef, Mapping: mapping,
		DeclaringFnRef: declaringFnRef, ConditionRefs: normalizeConditionRefs(conds),
		ConnectionRef: connRef, PartialRecordRefs: compRecords,
		ResultRef: resultField, DerivationRef: derivRef,
	}
	if err := rec.Validate(); err != nil {
		return fail(err)
	}
	rpayload, err := rec.Encode()
	if err != nil {
		return fail(err)
	}
	recRef, err := store.Put(FamilyRiemannEvaluation, rpayload)
	if err != nil {
		return fail(err)
	}
	return resultField, recRef, d, nil
}

// resolveAndValidateRiemann resolves the stored riemann_evaluation/1 record
// and validates its complete agreement from stored artifacts: family
// resolution, record shape, connection metric-verification by recompute,
// per-partial parameter linkage, ordered record and component comparison
// against idempotent recomputation, result index/variance/chart checks, and
// derivation premise/result linkage. It performs no replay itself; callers
// re-execute exactly the validated derivation. Neither
// CheckRiemannEvaluation nor ReplayRiemannEvaluation calls the other: both
// share this helper, so the validation contract is written once and replay
// happens exactly once per entry point.
func resolveAndValidateRiemann(store Store, ref artifact.ArtifactRef) (RiemannEvaluation, Derivation, error) {
	fail := func(err error) (RiemannEvaluation, Derivation, error) {
		return RiemannEvaluation{}, Derivation{}, err
	}
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return fail(fmt.Errorf("physmath: riemann record unresolvable: %w", err))
	}
	if fam != FamilyRiemannEvaluation {
		return fail(fmt.Errorf("physmath: target is not riemann_evaluation/1: %w", ErrBadInput))
	}
	rec, err := DecodeRiemannEvaluation(payload)
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
		return fail(fmt.Errorf("physmath: riemann derivation unresolvable: %w", err))
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return fail(err)
	}
	// Recompute the connection from the recorded source metric: exact
	// reference equality proves the recorded connection was derived from
	// this metric. A nil source is never accepted; callers needing
	// structural validation only must use CheckAffineConnectionShape
	// explicitly, which is never metric evidence.
	recompConn, leviDeriv, err := LeviCivita(store, rec.MetricRef, rec.ChartRef, rec.Mapping, rec.DeclaringFnRef, rec.ConditionRefs)
	if err != nil {
		return fail(fmt.Errorf("physmath: riemann connection recomputation failed: %w", err))
	}
	if recompConn != rec.ConnectionRef {
		return fail(fmt.Errorf("physmath: riemann connection does not match the source metric"))
	}
	// The stored derivation opens with the recomputed Levi-Civita steps in
	// order (construction emits them first with preserved ordinals).
	if len(d.Steps) < len(leviDeriv.Steps) {
		return fail(fmt.Errorf("physmath: riemann derivation shorter than its connection evidence"))
	}
	for i, s := range leviDeriv.Steps {
		want, err := s.Encode()
		if err != nil {
			return fail(err)
		}
		got, err := d.Steps[i].Encode()
		if err != nil {
			return fail(err)
		}
		if string(got) != string(want) {
			return fail(fmt.Errorf("physmath: riemann connection evidence step %d mismatch", i))
		}
	}
	// Recompute the Riemann core idempotently: identical inputs reproduce
	// identical references, so recomputation verifies rather than repairs.
	_, connPayload, err := store.Lookup(rec.ConnectionRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: riemann connection unresolvable: %w", err))
	}
	conn, err := DecodeAffineConnection(connPayload)
	if err != nil {
		return fail(err)
	}
	cexprs, err := resolveComponents(store, "riemann connection", conn.CoefficientRefs)
	if err != nil {
		return fail(err)
	}
	_, cpayload, err := store.Lookup(rec.ChartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: riemann chart unresolvable: %w", err))
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		return fail(err)
	}
	bin := &riemannInputs{
		dim: int(chart.Dimension), chart: chart, chartRef: rec.ChartRef,
		mapping: rec.Mapping, declaring: rec.DeclaringFnRef,
		conds:     normalizeConditionRefs(rec.ConditionRefs),
		connRefs:  append([]artifact.ArtifactRef{}, conn.CoefficientRefs...),
		connExprs: cexprs,
	}
	out, compRecords, coreSteps, corePremises, coreResults, err := evalRiemannCore(store, bin)
	if err != nil {
		return fail(err)
	}
	if len(rec.PartialRecordRefs) != len(compRecords) {
		return fail(fmt.Errorf("physmath: riemann partial record count %d != recomputed %d",
			len(rec.PartialRecordRefs), len(compRecords)))
	}
	for i := range compRecords {
		if rec.PartialRecordRefs[i] != compRecords[i] {
			return fail(fmt.Errorf("physmath: riemann partial record %d mismatch", i))
		}
	}
	// Core steps follow the connection evidence in order: partial block
	// with per-coordinate parameters, then per-component CANON+SIMPLIFY
	// pairs chained through the recomputed assembly premises.
	coreStored := d.Steps[len(leviDeriv.Steps):]
	if len(coreStored) != len(coreSteps) {
		return fail(fmt.Errorf("physmath: riemann core steps %d != recomputed %d",
			len(coreStored), len(coreSteps)))
	}
	nPartial := len(bin.connRefs) * bin.dim
	for i := 0; i < nPartial; i++ {
		mu := i / len(bin.connRefs)
		k := i % len(bin.connRefs)
		s := coreStored[i]
		if s.ProducerNamespace != NamespacePhysMath || s.RuleID != RuleDifferentiate {
			return fail(fmt.Errorf("physmath: riemann partial step %d must be DIFFERENTIATE-001", i))
		}
		if len(s.InputRefs) != 2 || s.InputRefs[0] != bin.connRefs[k] || s.InputRefs[1] != rec.DeclaringFnRef {
			return fail(fmt.Errorf("physmath: riemann partial step %d linkage broken", i))
		}
		wantParams := append(bin.mapping.Entries[mu].Encode(), artifact.EncodeUInt32(1)...)
		if string(s.Parameters) != string(wantParams) {
			return fail(fmt.Errorf("physmath: riemann partial step %d parameters mismatch", i))
		}
	}
	// Ordered assembly linkage: recomputed premises in component order.
	nOut := len(out)
	asmPremises := make([]artifact.ArtifactRef, 0, nOut)
	asmCanons := make([]artifact.ArtifactRef, 0, nOut)
	asmResults := make([]artifact.ArtifactRef, 0, nOut)
	for i := 0; i < nOut; i++ {
		asmPremises = append(asmPremises, out[i].premise)
		asmCanons = append(asmCanons, out[i].canon)
		asmResults = append(asmResults, out[i].out)
	}
	for i := 0; i < nOut; i++ {
		s0, s1 := coreStored[nPartial+2*i], coreStored[nPartial+2*i+1]
		if s0.ProducerNamespace != NamespacePhysMath || s0.RuleID != RuleExprCanon {
			return fail(fmt.Errorf("physmath: riemann assembly step %d must be CANON", i))
		}
		if s1.ProducerNamespace != NamespacePhysMath || s1.RuleID != RuleExprSimplify {
			return fail(fmt.Errorf("physmath: riemann assembly step %d must be SIMPLIFY", i))
		}
		if len(s0.InputRefs) != 1 || s0.InputRefs[0] != asmPremises[i] {
			return fail(fmt.Errorf("physmath: riemann assembly step %d premise mismatch", i))
		}
		if s0.OutputRef != asmCanons[i] || len(s1.InputRefs) != 1 || s1.InputRefs[0] != asmCanons[i] {
			return fail(fmt.Errorf("physmath: riemann assembly step %d linkage broken", i))
		}
		if s1.OutputRef != asmResults[i] {
			return fail(fmt.Errorf("physmath: riemann assembly step %d output mismatch", i))
		}
	}
	// Premise/result sets must match recomputation exactly (records and
	// intermediate outputs included, never silently repaired).
	wantPremises := uniqueRefs(append(append([]artifact.ArtifactRef{rec.DeclaringFnRef}, leviDeriv.PremiseRefs...), corePremises...))
	if !refSetEqual(d.PremiseRefs, wantPremises) {
		return fail(fmt.Errorf("physmath: riemann premises do not match recomputation"))
	}
	wantResults := uniqueRefs(append(append([]artifact.ArtifactRef{}, leviDeriv.ResultRefs...), coreResults...))
	if !refSetEqual(d.ResultRefs, wantResults) {
		return fail(fmt.Errorf("physmath: riemann results do not match recomputation"))
	}
	// Result field: same chart, rank-4 tensor, deterministic slot
	// descriptors with exact variance, chart-space agreement, and ordered
	// components byte-equal to recomputation.
	rfam, rpayload, err := store.Lookup(rec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: riemann result unresolvable: %w", err))
	}
	if rfam != FamilyTensorField {
		return fail(fmt.Errorf("physmath: riemann result is not tensor_field/1: %w", ErrBadInput))
	}
	rf, err := DecodeTensorField(rpayload)
	if err != nil {
		return fail(err)
	}
	if rf.CoordinateSystemRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: riemann result chart not preserved: %w", ErrBadInput))
	}
	rtfam, rtpayload, err := store.Lookup(rf.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: riemann result tensor unresolvable: %w", err))
	}
	if rtfam != FamilyTensor {
		return fail(fmt.Errorf("physmath: riemann result tensor is not tensor/1: %w", ErrBadInput))
	}
	rt, err := DecodeTensor(rtpayload)
	if err != nil {
		return fail(err)
	}
	if int(rt.Rank) != 4 || int(rt.Dimension) != bin.dim {
		return fail(fmt.Errorf("physmath: riemann result shape wrong: %w", ErrBadInput))
	}
	wantIdx := make([]artifact.ArtifactRef, 4)
	slotLabels := []string{"rho", "sigma", "mu", "nu"}
	slotVariances := []IndexVariance{VarianceContravariant, VarianceCovariant, VarianceCovariant, VarianceCovariant}
	for i := range slotLabels {
		p, err := Index{Label: slotLabels[i], Variance: slotVariances[i], Space: &chart.SpaceRef}.Encode()
		if err != nil {
			return fail(err)
		}
		idxRef, err := store.Put(FamilyIndex, p)
		if err != nil {
			return fail(err)
		}
		wantIdx[i] = idxRef
	}
	if len(rt.IndexRefs) != 4 {
		return fail(fmt.Errorf("physmath: riemann result index count wrong: %w", ErrBadInput))
	}
	for i := range wantIdx {
		if rt.IndexRefs[i] != wantIdx[i] {
			return fail(fmt.Errorf("physmath: riemann result slot %d identity broken: %w", i, ErrBadInput))
		}
		idx, err := resolveIndex(store, "riemann result slot", rt.IndexRefs[i])
		if err != nil {
			return fail(err)
		}
		if idx.Variance != slotVariances[i] {
			return fail(fmt.Errorf("physmath: riemann result slot %d variance wrong: %w", i, ErrBadInput))
		}
		if idx.Space == nil || *idx.Space != chart.SpaceRef {
			return fail(fmt.Errorf("physmath: riemann result slot %d space mismatch: %w", i, ErrBadInput))
		}
	}
	if len(rt.ComponentRefs) != len(out) {
		return fail(fmt.Errorf("physmath: riemann result component count mismatch: %w", ErrBadInput))
	}
	for i := range out {
		_, opayload, err := store.Lookup(out[i].out)
		if err != nil {
			return fail(err)
		}
		_, rp, err := store.Lookup(rt.ComponentRefs[i])
		if err != nil {
			return fail(err)
		}
		if string(opayload) != string(rp) {
			return fail(fmt.Errorf("physmath: riemann result component %d mismatch: %w", i, ErrBadInput))
		}
	}
	return rec, d, nil
}

// CheckRiemannEvaluation is the store-aware provenance gate: it validates
// the complete stored record against its derivation through the shared
// helper, then replays the validated derivation before trusting the result.
func CheckRiemannEvaluation(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateRiemann(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: riemann derivation does not replay: %w", err)
	}
	return nil
}

// ReplayRiemannEvaluation is artifact-level replay for a stored
// riemann_evaluation/1 record: it validates the complete record against its
// derivation (metric-verified connection, ordered records and components,
// index contract) and then re-executes the validated derivation. A corrupted
// record is rejected even when an inner derivation remains independently
// replayable: replaying an isolated Derivation via raw Replay establishes
// only that derivation's own contract, never the validity of a referencing
// record.
func ReplayRiemannEvaluation(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateRiemann(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: riemann derivation does not replay: %w", err)
	}
	return nil
}
