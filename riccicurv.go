package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// Ricci contraction and scalar curvature from a verified Riemann evaluation
// (Plan 1.16). Trusted path: source metric → Riemann evaluation → Ricci
// contraction → scalar curvature, each stage verifying the actual stored
// result of the preceding stage.
//
// Exported identifiers avoid the purity-banned vocabulary (no Ricci,
// Curvature, or Metric): the operations are RiemannContraction (trace over
// slots 0 and 2 of R^rho_sigma_mu_nu) and RiemannScalar (double contraction
// with the inverse source metric). Mathematical terminology remains in
// documentation and comments only.
//
// No new executable rules: assembly reuses Add/Mul with EXPR-CANON-001 and
// EXPR-SIMPLIFY-001. No Bianchi identities, Einstein tensor, or dynamics.

// FamilyRiemannContraction is the Ricci-trace record: a verified Riemann
// evaluation contracted over its first and third slots into a covariant
// rank-2 result field, with replayable derivation. Family
// physmath/riemann_contraction/1.
var FamilyRiemannContraction = FamilyID{NamespacePhysMath, "riemann_contraction/1", "1"}

// RiemannContractionEvaluation links a source Riemann evaluation to its
// contracted rank-2 result field and derivation evidence. Validate checks
// structure only; CheckRiemannContraction resolves and recomputes.
type RiemannContractionEvaluation struct {
	RiemannRef    artifact.ArtifactRef // → riemann_evaluation/1 (source)
	ChartRef      artifact.ArtifactRef // → coordinate_system/1
	ResultRef     artifact.ArtifactRef // → tensor_field/1 (rank 2, covariant)
	DerivationRef artifact.ArtifactRef // required → derivation/1 (composite)
}

// Family returns FamilyRiemannContraction.
func (RiemannContractionEvaluation) Family() FamilyID { return FamilyRiemannContraction }

// Encode returns the canonical payload in declaration order.
func (r RiemannContractionEvaluation) Encode() ([]byte, error) {
	out := r.RiemannRef.Encode()
	out = append(out, r.ChartRef.Encode()...)
	out = append(out, r.ResultRef.Encode()...)
	return append(out, r.DerivationRef.Encode()...), nil
}

// Validate enforces reference grammar without resolution.
func (r RiemannContractionEvaluation) Validate() error {
	return validateRefGrammars(map[string]artifact.ArtifactRef{
		"RiemannContractionEvaluation.RiemannRef":    r.RiemannRef,
		"RiemannContractionEvaluation.ChartRef":      r.ChartRef,
		"RiemannContractionEvaluation.ResultRef":     r.ResultRef,
		"RiemannContractionEvaluation.DerivationRef": r.DerivationRef,
	})
}

// TargetChecks: RiemannRef → riemann_evaluation/1; ChartRef →
// coordinate_system/1; ResultRef → tensor_field/1; DerivationRef →
// derivation/1. Resolved only by the store-aware check.
func (r RiemannContractionEvaluation) TargetChecks() []TargetCheck {
	return []TargetCheck{
		{Field: "RiemannContractionEvaluation.RiemannRef", Ref: r.RiemannRef, Allowed: []FamilyID{FamilyRiemannEvaluation}},
		{Field: "RiemannContractionEvaluation.ChartRef", Ref: r.ChartRef, Allowed: []FamilyID{FamilyCoordinateSystem}},
		{Field: "RiemannContractionEvaluation.ResultRef", Ref: r.ResultRef, Allowed: []FamilyID{FamilyTensorField}},
		{Field: "RiemannContractionEvaluation.DerivationRef", Ref: r.DerivationRef, Allowed: []FamilyID{FamilyDerivation}},
	}
}

// DecodeRiemannContractionEvaluation decodes a
// physmath/riemann_contraction/1 payload. Representation only.
func DecodeRiemannContractionEvaluation(payload []byte) (RiemannContractionEvaluation, error) {
	d := artifact.NewDecoder(payload)
	riemann, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannContractionEvaluation{}, fmt.Errorf("physmath: contraction missing riemann: %w", err)
	}
	chart, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannContractionEvaluation{}, fmt.Errorf("physmath: contraction missing chart: %w", err)
	}
	result, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannContractionEvaluation{}, fmt.Errorf("physmath: contraction missing result: %w", err)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannContractionEvaluation{}, fmt.Errorf("physmath: contraction missing derivation: %w", err)
	}
	if !d.Exhausted() {
		return RiemannContractionEvaluation{}, fmt.Errorf("physmath: trailing bytes in riemann-contraction payload")
	}
	r := RiemannContractionEvaluation{
		RiemannRef: riemann, ChartRef: chart, ResultRef: result, DerivationRef: deriv,
	}
	if err := r.Validate(); err != nil {
		return RiemannContractionEvaluation{}, err
	}
	return r, nil
}

// ricciResult is one computed Ricci component with its evidence refs.
type ricciResult struct {
	expr    Expr
	premise artifact.ArtifactRef
	canon   artifact.ArtifactRef
	out     artifact.ArtifactRef
}

// evalContractionCore sums R^rho_sigma_rho_nu over rho for every
// (sigma,nu) in row-major order: one Add premise per component through
// CANON+SIMPLIFY. It returns result expressions with derivation pieces using
// local ordinals. Callers renumber, mint tensors, and wrap.
func evalContractionCore(store Store, comps []Expr, dim int) ([]ricciResult, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
	fail := func(err error) ([]ricciResult, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
		return nil, nil, nil, nil, err
	}
	var out []ricciResult
	var steps []DerivationStep
	var premises []artifact.ArtifactRef
	var results []artifact.ArtifactRef
	for sigma := 0; sigma < dim; sigma++ {
		for nu := 0; nu < dim; nu++ {
			terms := make([]Expr, 0, dim)
			for rho := 0; rho < dim; rho++ {
				terms = append(terms, cloneExpr(comps[riemannIndex(rho, sigma, rho, nu, dim)]))
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
				return fail(fmt.Errorf("physmath: contraction coefficient canon failed: %w", err))
			}
			res, err := Apply(store, Invocation{
				ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
				InputRefs: []artifact.ArtifactRef{canon.OutputRef},
			})
			if err != nil {
				return fail(fmt.Errorf("physmath: contraction coefficient simplify failed: %w", err))
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
			out = append(out, ricciResult{expr: e, premise: premise, canon: canon.OutputRef, out: res.OutputRef})
		}
	}
	return out, steps, premises, results, nil
}

// EvaluateRiemannContraction contracts slots 0 and 2 of a verified Riemann
// evaluation into the covariant rank-2 result R_sigma_nu = sum_rho
// R^rho_sigma_rho_nu. The source record must pass CheckRiemannEvaluation;
// its result field supplies chart, ordered indices, and components. It
// returns the result field reference, the minted contraction record
// reference, and one flat replayable composite derivation.
func EvaluateRiemannContraction(store Store, riemannRef artifact.ArtifactRef) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, artifact.ArtifactRef{}, Derivation{}, err
	}
	if err := CheckRiemannEvaluation(store, riemannRef); err != nil {
		return fail(fmt.Errorf("physmath: contraction source riemann rejected: %w", err))
	}
	rfam, rpayload, err := store.Lookup(riemannRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction riemann unresolvable: %w", err))
	}
	if rfam != FamilyRiemannEvaluation {
		return fail(fmt.Errorf("physmath: contraction source is not riemann_evaluation/1: %w", ErrBadInput))
	}
	rec, err := DecodeRiemannEvaluation(rpayload)
	if err != nil {
		return fail(err)
	}
	_, fpayload, err := store.Lookup(rec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction riemann result unresolvable: %w", err))
	}
	rf, err := DecodeTensorField(fpayload)
	if err != nil {
		return fail(err)
	}
	if rf.CoordinateSystemRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: contraction riemann chart not preserved: %w", ErrBadInput))
	}
	_, tpayload, err := store.Lookup(rf.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction riemann tensor unresolvable: %w", err))
	}
	rt, err := DecodeTensor(tpayload)
	if err != nil {
		return fail(err)
	}
	if int(rt.Rank) != 4 {
		return fail(fmt.Errorf("physmath: contraction riemann rank %d != 4: %w", rt.Rank, ErrBadInput))
	}
	d := int(rt.Dimension)
	if d > MatrixDimBound {
		return fail(fmt.Errorf("physmath: contraction dimension %d exceeds bound %d: %w",
			d, MatrixDimBound, ErrBadInput))
	}
	if len(rt.IndexRefs) != 4 {
		return fail(fmt.Errorf("physmath: contraction riemann index count wrong: %w", ErrBadInput))
	}
	wantVariances := []IndexVariance{VarianceContravariant, VarianceCovariant, VarianceCovariant, VarianceCovariant}
	_, cpayload, err := store.Lookup(rec.ChartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction chart unresolvable: %w", err))
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		return fail(err)
	}
	for i, r := range rt.IndexRefs {
		idx, err := resolveIndex(store, "contraction riemann slot", r)
		if err != nil {
			return fail(err)
		}
		if idx.Variance != wantVariances[i] {
			return fail(fmt.Errorf("physmath: contraction riemann slot %d variance wrong: %w", i, ErrBadInput))
		}
		if idx.Space == nil || *idx.Space != chart.SpaceRef {
			return fail(fmt.Errorf("physmath: contraction riemann slot %d space mismatch: %w", i, ErrBadInput))
		}
	}
	cexprs, err := resolveComponents(store, "contraction riemann", rt.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	// Covariant slot descriptors, minted deterministically so recomputation
	// reproduces them exactly.
	slotLabels := []string{"rc0", "rc1"}
	idxRefs := make([]artifact.ArtifactRef, 2)
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
	out, coreSteps, corePremises, coreResults, err := evalContractionCore(store, cexprs, d)
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
	field := TensorField{TensorRef: resultTensor, CoordinateSystemRef: rec.ChartRef}
	if err := field.Validate(); err != nil {
		return fail(err)
	}
	resultField, err := store.Put(FamilyTensorField, field.Encode())
	if err != nil {
		return fail(err)
	}
	// One flat derivation: source Riemann evidence, then contraction
	// assembly, with contiguous ordinals in execution order.
	_, srcDpayload, err := store.Lookup(rec.DerivationRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction source derivation unresolvable: %w", err))
	}
	srcDeriv, err := DecodeDerivation(srcDpayload)
	if err != nil {
		return fail(err)
	}
	steps := make([]DerivationStep, 0, len(srcDeriv.Steps)+len(coreSteps))
	steps = append(steps, srcDeriv.Steps...)
	steps = append(steps, coreSteps...)
	for i := range steps {
		steps[i].Ordinal = uint32(i)
	}
	compPremises := append([]artifact.ArtifactRef{}, rt.ComponentRefs...)
	compPremises = append(compPremises, corePremises...)
	dd := Derivation{
		PremiseRefs: uniqueRefs(append(append([]artifact.ArtifactRef{}, srcDeriv.PremiseRefs...), compPremises...)),
		Steps:       steps,
		ResultRefs:  uniqueRefs(append(append([]artifact.ArtifactRef{}, srcDeriv.ResultRefs...), coreResults...)),
	}
	if err := Replay(store, dd, nil); err != nil {
		return fail(fmt.Errorf("physmath: contraction derivation does not replay: %w", err))
	}
	ddPayload, err := dd.Encode()
	if err != nil {
		return fail(err)
	}
	derivRef, err := store.Put(FamilyDerivation, ddPayload)
	if err != nil {
		return fail(err)
	}
	crec := RiemannContractionEvaluation{
		RiemannRef: riemannRef, ChartRef: rec.ChartRef,
		ResultRef: resultField, DerivationRef: derivRef,
	}
	if err := crec.Validate(); err != nil {
		return fail(err)
	}
	crecPayload, err := crec.Encode()
	if err != nil {
		return fail(err)
	}
	recRef, err := store.Put(FamilyRiemannContraction, crecPayload)
	if err != nil {
		return fail(err)
	}
	return resultField, recRef, dd, nil
}

// resolveAndValidateContraction resolves the stored
// riemann_contraction/1 record and validates its complete agreement from
// stored artifacts: family resolution, source Riemann revalidation,
// contracted-slot variance/space contract, ordered component comparison
// against idempotent recomputation, result index/chart checks, and
// derivation linkage. It performs no replay itself; callers re-execute
// exactly the validated derivation. Neither CheckRiemannContraction nor
// ReplayRiemannContraction calls the other.
func resolveAndValidateContraction(store Store, ref artifact.ArtifactRef) (RiemannContractionEvaluation, Derivation, error) {
	fail := func(err error) (RiemannContractionEvaluation, Derivation, error) {
		return RiemannContractionEvaluation{}, Derivation{}, err
	}
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction record unresolvable: %w", err))
	}
	if fam != FamilyRiemannContraction {
		return fail(fmt.Errorf("physmath: target is not riemann_contraction/1: %w", ErrBadInput))
	}
	rec, err := DecodeRiemannContractionEvaluation(payload)
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
		return fail(fmt.Errorf("physmath: contraction derivation unresolvable: %w", err))
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return fail(err)
	}
	// The source Riemann record must itself verify; a contraction over an
	// unverified Riemann result is never accepted.
	if err := CheckRiemannEvaluation(store, rec.RiemannRef); err != nil {
		return fail(fmt.Errorf("physmath: contraction source riemann rejected: %w", err))
	}
	rfam, rpayload, err := store.Lookup(rec.RiemannRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction riemann unresolvable: %w", err))
	}
	if rfam != FamilyRiemannEvaluation {
		return fail(fmt.Errorf("physmath: contraction source is not riemann_evaluation/1: %w", ErrBadInput))
	}
	rrec, err := DecodeRiemannEvaluation(rpayload)
	if err != nil {
		return fail(err)
	}
	if rrec.ChartRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: contraction chart differs from riemann chart: %w", ErrBadInput))
	}
	_, fpayload, err := store.Lookup(rrec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction riemann result unresolvable: %w", err))
	}
	rf, err := DecodeTensorField(fpayload)
	if err != nil {
		return fail(err)
	}
	_, tpayload, err := store.Lookup(rf.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction riemann tensor unresolvable: %w", err))
	}
	rt, err := DecodeTensor(tpayload)
	if err != nil {
		return fail(err)
	}
	if int(rt.Rank) != 4 {
		return fail(fmt.Errorf("physmath: contraction riemann rank %d != 4: %w", rt.Rank, ErrBadInput))
	}
	dim := int(rt.Dimension)
	if dim > MatrixDimBound {
		return fail(fmt.Errorf("physmath: contraction dimension %d exceeds bound %d: %w",
			dim, MatrixDimBound, ErrBadInput))
	}
	_, cpayload, err := store.Lookup(rec.ChartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction chart unresolvable: %w", err))
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		return fail(err)
	}
	if int(chart.Dimension) != dim {
		return fail(fmt.Errorf("physmath: contraction chart dimension %d != tensor dimension %d: %w",
			chart.Dimension, dim, ErrBadInput))
	}
	wantVariances := []IndexVariance{VarianceContravariant, VarianceCovariant, VarianceCovariant, VarianceCovariant}
	if len(rt.IndexRefs) != 4 {
		return fail(fmt.Errorf("physmath: contraction riemann index count wrong: %w", ErrBadInput))
	}
	for i, r := range rt.IndexRefs {
		idx, err := resolveIndex(store, "contraction riemann slot", r)
		if err != nil {
			return fail(err)
		}
		if idx.Variance != wantVariances[i] {
			return fail(fmt.Errorf("physmath: contraction riemann slot %d variance wrong: %w", i, ErrBadInput))
		}
		if idx.Space == nil || *idx.Space != chart.SpaceRef {
			return fail(fmt.Errorf("physmath: contraction riemann slot %d space mismatch: %w", i, ErrBadInput))
		}
	}
	cexprs, err := resolveComponents(store, "contraction riemann", rt.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	// Source derivation prefix must match the recorded Riemann derivation
	// exactly (construction embeds it first with preserved ordinals).
	_, sdpayload, err := store.Lookup(rrec.DerivationRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction source derivation unresolvable: %w", err))
	}
	srcDeriv, err := DecodeDerivation(sdpayload)
	if err != nil {
		return fail(err)
	}
	if len(d.Steps) < len(srcDeriv.Steps) {
		return fail(fmt.Errorf("physmath: contraction derivation shorter than its riemann evidence"))
	}
	for i, s := range srcDeriv.Steps {
		want, err := s.Encode()
		if err != nil {
			return fail(err)
		}
		got, err := d.Steps[i].Encode()
		if err != nil {
			return fail(err)
		}
		if string(got) != string(want) {
			return fail(fmt.Errorf("physmath: contraction riemann evidence step %d mismatch", i))
		}
	}
	// Idempotent recomputation: identical inputs reproduce identical
	// references, so recomputation verifies rather than repairs.
	out, coreSteps, corePremises, coreResults, err := evalContractionCore(store, cexprs, dim)
	if err != nil {
		return fail(err)
	}
	coreStored := d.Steps[len(srcDeriv.Steps):]
	if len(coreStored) != len(coreSteps) {
		return fail(fmt.Errorf("physmath: contraction core steps %d != recomputed %d",
			len(coreStored), len(coreSteps)))
	}
	for i := 0; i < len(out); i++ {
		s0, s1 := coreStored[2*i], coreStored[2*i+1]
		if s0.ProducerNamespace != NamespacePhysMath || s0.RuleID != RuleExprCanon {
			return fail(fmt.Errorf("physmath: contraction assembly step %d must be CANON", i))
		}
		if s1.ProducerNamespace != NamespacePhysMath || s1.RuleID != RuleExprSimplify {
			return fail(fmt.Errorf("physmath: contraction assembly step %d must be SIMPLIFY", i))
		}
		if len(s0.InputRefs) != 1 || s0.InputRefs[0] != out[i].premise {
			return fail(fmt.Errorf("physmath: contraction assembly step %d premise mismatch", i))
		}
		if s0.OutputRef != out[i].canon || len(s1.InputRefs) != 1 || s1.InputRefs[0] != out[i].canon {
			return fail(fmt.Errorf("physmath: contraction assembly step %d linkage broken", i))
		}
		if s1.OutputRef != out[i].out {
			return fail(fmt.Errorf("physmath: contraction assembly step %d output mismatch", i))
		}
	}
	compPremises := append([]artifact.ArtifactRef{}, rt.ComponentRefs...)
	compPremises = append(compPremises, corePremises...)
	wantPremises := uniqueRefs(append(append([]artifact.ArtifactRef{}, srcDeriv.PremiseRefs...), compPremises...))
	if !refSetEqual(d.PremiseRefs, wantPremises) {
		return fail(fmt.Errorf("physmath: contraction premises do not match recomputation"))
	}
	wantResults := uniqueRefs(append(append([]artifact.ArtifactRef{}, srcDeriv.ResultRefs...), coreResults...))
	if !refSetEqual(d.ResultRefs, wantResults) {
		return fail(fmt.Errorf("physmath: contraction results do not match recomputation"))
	}
	// Result field: same chart, rank-2 covariant tensor, deterministic slot
	// descriptors, chart-space agreement, ordered components byte-equal.
	rfam2, rpayload2, err := store.Lookup(rec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction result unresolvable: %w", err))
	}
	if rfam2 != FamilyTensorField {
		return fail(fmt.Errorf("physmath: contraction result is not tensor_field/1: %w", ErrBadInput))
	}
	rff, err := DecodeTensorField(rpayload2)
	if err != nil {
		return fail(err)
	}
	if rff.CoordinateSystemRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: contraction result chart not preserved: %w", ErrBadInput))
	}
	rtfam, rtpayload, err := store.Lookup(rff.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction result tensor unresolvable: %w", err))
	}
	if rtfam != FamilyTensor {
		return fail(fmt.Errorf("physmath: contraction result tensor is not tensor/1: %w", ErrBadInput))
	}
	rtn, err := DecodeTensor(rtpayload)
	if err != nil {
		return fail(err)
	}
	if int(rtn.Rank) != 2 || int(rtn.Dimension) != dim {
		return fail(fmt.Errorf("physmath: contraction result shape wrong: %w", ErrBadInput))
	}
	slotLabels := []string{"rc0", "rc1"}
	if len(rtn.IndexRefs) != 2 {
		return fail(fmt.Errorf("physmath: contraction result index count wrong: %w", ErrBadInput))
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
			return fail(fmt.Errorf("physmath: contraction result slot %d identity broken: %w", i, ErrBadInput))
		}
		idx, err := resolveIndex(store, "contraction result slot", rtn.IndexRefs[i])
		if err != nil {
			return fail(err)
		}
		if idx.Variance != VarianceCovariant {
			return fail(fmt.Errorf("physmath: contraction result slot %d variance wrong: %w", i, ErrBadInput))
		}
		if idx.Space == nil || *idx.Space != chart.SpaceRef {
			return fail(fmt.Errorf("physmath: contraction result slot %d space mismatch: %w", i, ErrBadInput))
		}
	}
	if len(rtn.ComponentRefs) != len(out) {
		return fail(fmt.Errorf("physmath: contraction result component count mismatch: %w", ErrBadInput))
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
			return fail(fmt.Errorf("physmath: contraction result component %d mismatch: %w", i, ErrBadInput))
		}
	}
	return rec, d, nil
}

// CheckRiemannContraction is the store-aware provenance gate: it validates
// the complete stored record against its derivation through the shared
// helper, then replays the validated derivation before trusting the result.
func CheckRiemannContraction(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateContraction(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: contraction derivation does not replay: %w", err)
	}
	return nil
}

// ReplayRiemannContraction is artifact-level replay for a stored
// riemann_contraction/1 record: it validates the complete record against
// its derivation (source revalidation, slot contract, ordered components,
// index contract) and then re-executes the validated derivation. A corrupted
// record is rejected even when an inner derivation remains independently
// replayable.
func ReplayRiemannContraction(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateContraction(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: contraction derivation does not replay: %w", err)
	}
	return nil
}

// FamilyRiemannScalar is the scalar-curvature record: a verified Ricci
// contraction doubly contracted with the inverse source metric, with
// determinant and inverse-metric derivation evidence and replayable
// derivation. Family physmath/riemann_scalar/1.
var FamilyRiemannScalar = FamilyID{NamespacePhysMath, "riemann_scalar/1", "1"}

// RiemannScalarEvaluation links a source Ricci contraction and source metric
// to its determinant evidence, inverse-metric evidence, scalar result, and
// derivation. The stored ConditionRefs are the exact set used by the
// determinant and inverse-metric derivations: the checker verifies metric,
// conditions, determinant evidence, inverse-metric evidence, contraction,
// and scalar result as one consistent chain. Validate checks structure only.
type RiemannScalarEvaluation struct {
	ContractionRef   artifact.ArtifactRef   // → riemann_contraction/1 (source)
	MetricRef        artifact.ArtifactRef   // → symmetric_bilinear/1 (source)
	ChartRef         artifact.ArtifactRef   // → coordinate_system/1
	ConditionRefs    []artifact.ArtifactRef // SET → condition/1 (exact set used)
	DetRef           artifact.ArtifactRef   // → expression/1 (determinant value)
	DetDerivationRef artifact.ArtifactRef   // → derivation/1 (determinant evidence)
	InvTensorRef     artifact.ArtifactRef   // → tensor/1 (rank-2 contravariant inverse)
	InvDerivationRef artifact.ArtifactRef   // → derivation/1 (inverse evidence)
	ResultRef        artifact.ArtifactRef   // → expression/1 (scalar result)
	DerivationRef    artifact.ArtifactRef   // required → derivation/1 (composite)
}

// Family returns FamilyRiemannScalar.
func (RiemannScalarEvaluation) Family() FamilyID { return FamilyRiemannScalar }

// Encode returns the canonical payload in declaration order.
func (r RiemannScalarEvaluation) Encode() ([]byte, error) {
	out := r.ContractionRef.Encode()
	out = append(out, r.MetricRef.Encode()...)
	out = append(out, r.ChartRef.Encode()...)
	set, err := encodeRefSet(r.ConditionRefs)
	if err != nil {
		return nil, err
	}
	out = append(out, set...)
	out = append(out, r.DetRef.Encode()...)
	out = append(out, r.DetDerivationRef.Encode()...)
	out = append(out, r.InvTensorRef.Encode()...)
	out = append(out, r.InvDerivationRef.Encode()...)
	out = append(out, r.ResultRef.Encode()...)
	return append(out, r.DerivationRef.Encode()...), nil
}

// Validate enforces reference grammar and condition-set uniqueness without
// resolution. Substance belongs to the store-aware check.
func (r RiemannScalarEvaluation) Validate() error {
	fields := map[string]artifact.ArtifactRef{
		"RiemannScalarEvaluation.ContractionRef":   r.ContractionRef,
		"RiemannScalarEvaluation.MetricRef":        r.MetricRef,
		"RiemannScalarEvaluation.ChartRef":         r.ChartRef,
		"RiemannScalarEvaluation.DetRef":           r.DetRef,
		"RiemannScalarEvaluation.DetDerivationRef": r.DetDerivationRef,
		"RiemannScalarEvaluation.InvTensorRef":     r.InvTensorRef,
		"RiemannScalarEvaluation.InvDerivationRef": r.InvDerivationRef,
		"RiemannScalarEvaluation.ResultRef":        r.ResultRef,
		"RiemannScalarEvaluation.DerivationRef":    r.DerivationRef,
	}
	for i, c := range r.ConditionRefs {
		fields[getRefFieldName("RiemannScalarEvaluation.ConditionRefs", i)] = c
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	return checkDuplicateRefs("RiemannScalarEvaluation.ConditionRefs", r.ConditionRefs)
}

// TargetChecks: ContractionRef → riemann_contraction/1; MetricRef →
// symmetric_bilinear/1; ChartRef → coordinate_system/1; ConditionRefs →
// condition/1; DetRef/ResultRef → expression/1; DetDerivationRef,
// InvDerivationRef, DerivationRef → derivation/1; InvTensorRef → tensor/1.
// Resolved only by the store-aware check.
func (r RiemannScalarEvaluation) TargetChecks() []TargetCheck {
	checks := []TargetCheck{
		{Field: "RiemannScalarEvaluation.ContractionRef", Ref: r.ContractionRef, Allowed: []FamilyID{FamilyRiemannContraction}},
		{Field: "RiemannScalarEvaluation.MetricRef", Ref: r.MetricRef, Allowed: []FamilyID{FamilySymmetricBilinear}},
		{Field: "RiemannScalarEvaluation.ChartRef", Ref: r.ChartRef, Allowed: []FamilyID{FamilyCoordinateSystem}},
		{Field: "RiemannScalarEvaluation.DetRef", Ref: r.DetRef, Allowed: []FamilyID{FamilyExpression}},
		{Field: "RiemannScalarEvaluation.DetDerivationRef", Ref: r.DetDerivationRef, Allowed: []FamilyID{FamilyDerivation}},
		{Field: "RiemannScalarEvaluation.InvTensorRef", Ref: r.InvTensorRef, Allowed: []FamilyID{FamilyTensor}},
		{Field: "RiemannScalarEvaluation.InvDerivationRef", Ref: r.InvDerivationRef, Allowed: []FamilyID{FamilyDerivation}},
		{Field: "RiemannScalarEvaluation.ResultRef", Ref: r.ResultRef, Allowed: []FamilyID{FamilyExpression}},
		{Field: "RiemannScalarEvaluation.DerivationRef", Ref: r.DerivationRef, Allowed: []FamilyID{FamilyDerivation}},
	}
	for i, c := range r.ConditionRefs {
		checks = append(checks, TargetCheck{
			Field: getRefFieldName("RiemannScalarEvaluation.ConditionRefs", i),
			Ref:   c, Allowed: []FamilyID{FamilyCondition},
		})
	}
	return checks
}

// DecodeRiemannScalarEvaluation decodes a physmath/riemann_scalar/1 payload.
// Representation only: no store Lookup, no execution.
func DecodeRiemannScalarEvaluation(payload []byte) (RiemannScalarEvaluation, error) {
	d := artifact.NewDecoder(payload)
	contraction, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar missing contraction: %w", err)
	}
	metric, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar missing metric: %w", err)
	}
	chart, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar missing chart: %w", err)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar missing conditions: %w", err)
	}
	conds := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar bad condition %d: %w", i, err)
		}
		conds = append(conds, r)
	}
	det, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar missing determinant: %w", err)
	}
	detDeriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar missing determinant derivation: %w", err)
	}
	invTensor, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar missing inverse tensor: %w", err)
	}
	invDeriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar missing inverse derivation: %w", err)
	}
	result, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar missing result: %w", err)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: scalar missing derivation: %w", err)
	}
	if !d.Exhausted() {
		return RiemannScalarEvaluation{}, fmt.Errorf("physmath: trailing bytes in riemann-scalar payload")
	}
	r := RiemannScalarEvaluation{
		ContractionRef: contraction, MetricRef: metric, ChartRef: chart,
		ConditionRefs: conds, DetRef: det, DetDerivationRef: detDeriv,
		InvTensorRef: invTensor, InvDerivationRef: invDeriv,
		ResultRef: result, DerivationRef: deriv,
	}
	if err := r.Validate(); err != nil {
		return RiemannScalarEvaluation{}, err
	}
	return r, nil
}

// EvaluateRiemannScalar contracts a verified Ricci contraction with the
// inverse source metric: R = sum_sigma_nu g^sigma_nu R_sigma_nu. The source
// metric and exact condition set resolve through the bound Riemann record —
// chained verification, never caller-supplied substitutes. Determinant and
// inverse-metric evidence derive from that same metric. A singular or
// otherwise non-invertible source metric fails closed before any scalar is
// accepted. It returns the scalar expression reference, the minted scalar
// record reference, and one flat replayable composite derivation.
func EvaluateRiemannScalar(store Store, contractionRef artifact.ArtifactRef) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, artifact.ArtifactRef{}, Derivation{}, err
	}
	if err := CheckRiemannContraction(store, contractionRef); err != nil {
		return fail(fmt.Errorf("physmath: scalar source contraction rejected: %w", err))
	}
	cfam, cpayload, err := store.Lookup(contractionRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar contraction unresolvable: %w", err))
	}
	if cfam != FamilyRiemannContraction {
		return fail(fmt.Errorf("physmath: scalar source is not riemann_contraction/1: %w", ErrBadInput))
	}
	crec, err := DecodeRiemannContractionEvaluation(cpayload)
	if err != nil {
		return fail(err)
	}
	rfam, rpayload, err := store.Lookup(crec.RiemannRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar riemann unresolvable: %w", err))
	}
	if rfam != FamilyRiemannEvaluation {
		return fail(fmt.Errorf("physmath: scalar riemann is not riemann_evaluation/1: %w", ErrBadInput))
	}
	rrec, err := DecodeRiemannEvaluation(rpayload)
	if err != nil {
		return fail(err)
	}
	if crec.ChartRef != rrec.ChartRef {
		return fail(fmt.Errorf("physmath: scalar chart differs from riemann chart: %w", ErrBadInput))
	}
	conds := normalizeConditionRefs(rrec.ConditionRefs)
	_, fpayload, err := store.Lookup(crec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar contraction result unresolvable: %w", err))
	}
	cf, err := DecodeTensorField(fpayload)
	if err != nil {
		return fail(err)
	}
	if cf.CoordinateSystemRef != crec.ChartRef {
		return fail(fmt.Errorf("physmath: scalar contraction chart not preserved: %w", ErrBadInput))
	}
	_, tpayload, err := store.Lookup(cf.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar contraction tensor unresolvable: %w", err))
	}
	ct, err := DecodeTensor(tpayload)
	if err != nil {
		return fail(err)
	}
	if int(ct.Rank) != 2 {
		return fail(fmt.Errorf("physmath: scalar contraction rank %d != 2: %w", ct.Rank, ErrBadInput))
	}
	d := int(ct.Dimension)
	if d > MatrixDimBound {
		return fail(fmt.Errorf("physmath: scalar dimension %d exceeds bound %d: %w",
			d, MatrixDimBound, ErrBadInput))
	}
	ricComps, err := resolveComponents(store, "scalar contraction", ct.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	_, chpayload, err := store.Lookup(crec.ChartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar chart unresolvable: %w", err))
	}
	chart, err := DecodeCoordinateSystem(chpayload)
	if err != nil {
		return fail(err)
	}
	if int(chart.Dimension) != d {
		return fail(fmt.Errorf("physmath: scalar chart dimension %d != tensor dimension %d: %w",
			chart.Dimension, d, ErrBadInput))
	}
	// Determinant and inverse from the chained source metric under the
	// chained exact conditions. Both derivations become stored evidence.
	detRef, detDeriv, err := GeneralDeterminant(store, rrec.MetricRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar determinant failed: %w", err))
	}
	detRef2, invComps, invDeriv, err := InverseMatrix(store, rrec.MetricRef, conds)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar inverse failed: %w", err))
	}
	if detRef2 != detRef {
		return fail(fmt.Errorf("physmath: scalar determinant/inverse evidence inconsistent: %w", ErrBadInput))
	}
	// Contravariant inverse slot descriptors, minted deterministically so
	// recomputation reproduces them exactly.
	invIdx := make([]artifact.ArtifactRef, 2)
	for i, label := range []string{"imu", "inu"} {
		p, err := Index{Label: label, Variance: VarianceContravariant, Space: &chart.SpaceRef}.Encode()
		if err != nil {
			return fail(err)
		}
		idxRef, err := store.Put(FamilyIndex, p)
		if err != nil {
			return fail(err)
		}
		invIdx[i] = idxRef
	}
	ginv := Tensor{Rank: 2, Dimension: uint16(d), IndexRefs: invIdx, ComponentRefs: invComps}
	if err := ginv.Validate(); err != nil {
		return fail(err)
	}
	invTensor, err := store.Put(FamilyTensor, ginv.Encode())
	if err != nil {
		return fail(err)
	}
	gexprs, err := resolveComponents(store, "scalar inverse", invComps)
	if err != nil {
		return fail(err)
	}
	terms := make([]Expr, 0, d*d)
	for sigma := 0; sigma < d; sigma++ {
		for nu := 0; nu < d; nu++ {
			terms = append(terms, Mul(
				cloneExpr(gexprs[sigma*d+nu]),
				cloneExpr(ricComps[sigma*d+nu]),
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
		return fail(fmt.Errorf("physmath: scalar assembly canon failed: %w", err))
	}
	res, err := Apply(store, Invocation{
		ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
		InputRefs: []artifact.ArtifactRef{canon.OutputRef},
	})
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar assembly simplify failed: %w", err))
	}
	// One flat derivation: determinant evidence, inverse evidence,
	// assembly, with contiguous ordinals in execution order. The chain
	// records ride in the premises as explicit provenance links.
	steps := make([]DerivationStep, 0, len(detDeriv.Steps)+len(invDeriv.Steps)+2)
	steps = append(steps, detDeriv.Steps...)
	steps = append(steps, invDeriv.Steps...)
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
	for i := range steps {
		steps[i].Ordinal = uint32(i)
	}
	premises := []artifact.ArtifactRef{contractionRef, crec.RiemannRef, premise}
	premises = append(premises, detDeriv.PremiseRefs...)
	premises = append(premises, invDeriv.PremiseRefs...)
	premises = append(premises, ct.ComponentRefs...)
	premises = append(premises, invComps...)
	results := []artifact.ArtifactRef{res.OutputRef}
	results = append(results, detDeriv.ResultRefs...)
	results = append(results, invDeriv.ResultRefs...)
	dd := Derivation{
		PremiseRefs: uniqueRefs(premises),
		Steps:       steps,
		ResultRefs:  uniqueRefs(results),
	}
	if err := Replay(store, dd, nil); err != nil {
		return fail(fmt.Errorf("physmath: scalar derivation does not replay: %w", err))
	}
	ddPayload, err := dd.Encode()
	if err != nil {
		return fail(err)
	}
	derivRef, err := store.Put(FamilyDerivation, ddPayload)
	if err != nil {
		return fail(err)
	}
	detPayload, err := detDeriv.Encode()
	if err != nil {
		return fail(err)
	}
	detDerivRef, err := store.Put(FamilyDerivation, detPayload)
	if err != nil {
		return fail(err)
	}
	invPayload, err := invDeriv.Encode()
	if err != nil {
		return fail(err)
	}
	invDerivRef, err := store.Put(FamilyDerivation, invPayload)
	if err != nil {
		return fail(err)
	}
	srec := RiemannScalarEvaluation{
		ContractionRef: contractionRef, MetricRef: rrec.MetricRef, ChartRef: crec.ChartRef,
		ConditionRefs: conds, DetRef: detRef, DetDerivationRef: detDerivRef,
		InvTensorRef: invTensor, InvDerivationRef: invDerivRef,
		ResultRef: res.OutputRef, DerivationRef: derivRef,
	}
	if err := srec.Validate(); err != nil {
		return fail(err)
	}
	spayload, err := srec.Encode()
	if err != nil {
		return fail(err)
	}
	recRef, err := store.Put(FamilyRiemannScalar, spayload)
	if err != nil {
		return fail(err)
	}
	return res.OutputRef, recRef, dd, nil
}

// sameStepShape reports whether two derivation steps agree on everything
// except ordinal: producer, rule, inputs, output, conditions, and parameters.
// Used where a recomputed evidence block sits at a shifted position inside a
// composite derivation whose ordinals run contiguously from zero.
func sameStepShape(a, b DerivationStep) bool {
	if a.ProducerNamespace != b.ProducerNamespace || a.RuleID != b.RuleID {
		return false
	}
	if a.OutputRef != b.OutputRef {
		return false
	}
	if len(a.InputRefs) != len(b.InputRefs) {
		return false
	}
	for i := range a.InputRefs {
		if a.InputRefs[i] != b.InputRefs[i] {
			return false
		}
	}
	if string(a.Parameters) != string(b.Parameters) {
		return false
	}
	return equalRefSets(a.ConditionRefs, b.ConditionRefs)
}

// resolveAndValidateScalar resolves the stored riemann_scalar/1 record and
// validates metric, conditions, determinant evidence, inverse-metric
// evidence, contraction, and scalar result as one consistent chain from
// stored artifacts. It performs no replay itself; callers re-execute exactly
// the validated derivation. Neither CheckRiemannScalar nor
// ReplayRiemannScalar calls the other.
func resolveAndValidateScalar(store Store, ref artifact.ArtifactRef) (RiemannScalarEvaluation, Derivation, error) {
	fail := func(err error) (RiemannScalarEvaluation, Derivation, error) {
		return RiemannScalarEvaluation{}, Derivation{}, err
	}
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar record unresolvable: %w", err))
	}
	if fam != FamilyRiemannScalar {
		return fail(fmt.Errorf("physmath: target is not riemann_scalar/1: %w", ErrBadInput))
	}
	rec, err := DecodeRiemannScalarEvaluation(payload)
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
		return fail(fmt.Errorf("physmath: scalar derivation unresolvable: %w", err))
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return fail(err)
	}
	// The source contraction must itself verify; a scalar over an
	// unverified contraction is never accepted.
	if err := CheckRiemannContraction(store, rec.ContractionRef); err != nil {
		return fail(fmt.Errorf("physmath: scalar source contraction rejected: %w", err))
	}
	cfam, cpayload, err := store.Lookup(rec.ContractionRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar contraction unresolvable: %w", err))
	}
	if cfam != FamilyRiemannContraction {
		return fail(fmt.Errorf("physmath: scalar source is not riemann_contraction/1: %w", ErrBadInput))
	}
	crec, err := DecodeRiemannContractionEvaluation(cpayload)
	if err != nil {
		return fail(err)
	}
	rfam, rpayload, err := store.Lookup(crec.RiemannRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar riemann unresolvable: %w", err))
	}
	if rfam != FamilyRiemannEvaluation {
		return fail(fmt.Errorf("physmath: scalar riemann is not riemann_evaluation/1: %w", ErrBadInput))
	}
	rrec, err := DecodeRiemannEvaluation(rpayload)
	if err != nil {
		return fail(err)
	}
	// Chain consistency: one metric, one chart, one exact condition set
	// across contraction, Riemann, and scalar records. A substituted metric
	// or condition set fails here even if every inner derivation replays.
	if rec.MetricRef != rrec.MetricRef {
		return fail(fmt.Errorf("physmath: scalar metric differs from riemann metric: %w", ErrBadInput))
	}
	if rec.ChartRef != rrec.ChartRef || rec.ChartRef != crec.ChartRef {
		return fail(fmt.Errorf("physmath: scalar chart breaks the provenance chain: %w", ErrBadInput))
	}
	if !equalRefSets(rec.ConditionRefs, rrec.ConditionRefs) {
		return fail(fmt.Errorf("physmath: scalar condition set differs from riemann evidence: %w", ErrBadInput))
	}
	_, fpayload, err := store.Lookup(crec.ResultRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar contraction result unresolvable: %w", err))
	}
	cf, err := DecodeTensorField(fpayload)
	if err != nil {
		return fail(err)
	}
	if cf.CoordinateSystemRef != rec.ChartRef {
		return fail(fmt.Errorf("physmath: scalar contraction chart not preserved: %w", ErrBadInput))
	}
	_, tpayload, err := store.Lookup(cf.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar contraction tensor unresolvable: %w", err))
	}
	ct, err := DecodeTensor(tpayload)
	if err != nil {
		return fail(err)
	}
	if int(ct.Rank) != 2 {
		return fail(fmt.Errorf("physmath: scalar contraction rank %d != 2: %w", ct.Rank, ErrBadInput))
	}
	dim := int(ct.Dimension)
	if dim > MatrixDimBound {
		return fail(fmt.Errorf("physmath: scalar dimension %d exceeds bound %d: %w",
			dim, MatrixDimBound, ErrBadInput))
	}
	_, chpayload, err := store.Lookup(rec.ChartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar chart unresolvable: %w", err))
	}
	chart, err := DecodeCoordinateSystem(chpayload)
	if err != nil {
		return fail(err)
	}
	if int(chart.Dimension) != dim {
		return fail(fmt.Errorf("physmath: scalar chart dimension %d != tensor dimension %d: %w",
			chart.Dimension, dim, ErrBadInput))
	}
	ricComps, err := resolveComponents(store, "scalar contraction", ct.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	// Determinant evidence: replay the stored derivation, require its
	// output to equal the recorded determinant, and require its steps to
	// carry no conditions (GeneralDeterminant threads none). Recompute
	// from the recorded metric and require exact equality: a separately
	// recomputed determinant for another metric cannot masquerade here.
	_, detPayload, err := store.Lookup(rec.DetDerivationRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar determinant derivation unresolvable: %w", err))
	}
	detDeriv, err := DecodeDerivation(detPayload)
	if err != nil {
		return fail(err)
	}
	if err := Replay(store, detDeriv, nil); err != nil {
		return fail(fmt.Errorf("physmath: scalar determinant evidence does not replay: %w", err))
	}
	if len(detDeriv.ResultRefs) != 1 || detDeriv.ResultRefs[0] != rec.DetRef {
		return fail(fmt.Errorf("physmath: scalar determinant evidence result mismatch"))
	}
	for _, s := range detDeriv.Steps {
		if len(s.ConditionRefs) != 0 {
			return fail(fmt.Errorf("physmath: scalar determinant step carries conditions: %w", ErrBadInput))
		}
	}
	reDetRef, reDetDeriv, err := GeneralDeterminant(store, rec.MetricRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar determinant recomputation failed: %w", err))
	}
	if reDetRef != rec.DetRef {
		return fail(fmt.Errorf("physmath: scalar determinant does not match the source metric"))
	}
	if len(detDeriv.Steps) != len(reDetDeriv.Steps) {
		return fail(fmt.Errorf("physmath: scalar determinant evidence shape mismatch"))
	}
	for i := range detDeriv.Steps {
		want, err := reDetDeriv.Steps[i].Encode()
		if err != nil {
			return fail(err)
		}
		got, err := detDeriv.Steps[i].Encode()
		if err != nil {
			return fail(err)
		}
		if string(got) != string(want) {
			return fail(fmt.Errorf("physmath: scalar determinant evidence step %d mismatch", i))
		}
	}
	// Inverse evidence: replay, ordered outputs, and per-step condition sets
	// equal to the recorded exact set — a substituted condition set fails
	// here even if the metric matches. Recompute with the recorded
	// conditions and require equality: no separately recomputed inverse may
	// conceal inconsistent stored evidence.
	_, invPayload, err := store.Lookup(rec.InvDerivationRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar inverse derivation unresolvable: %w", err))
	}
	invDeriv, err := DecodeDerivation(invPayload)
	if err != nil {
		return fail(err)
	}
	if err := Replay(store, invDeriv, nil); err != nil {
		return fail(fmt.Errorf("physmath: scalar inverse evidence does not replay: %w", err))
	}
	_, ginvPayload, err := store.Lookup(rec.InvTensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar inverse tensor unresolvable: %w", err))
	}
	ginv, err := DecodeTensor(ginvPayload)
	if err != nil {
		return fail(err)
	}
	if int(ginv.Rank) != 2 || int(ginv.Dimension) != dim || len(ginv.IndexRefs) != 2 {
		return fail(fmt.Errorf("physmath: scalar inverse tensor shape wrong: %w", ErrBadInput))
	}
	wantIdx := make([]artifact.ArtifactRef, 2)
	for i, label := range []string{"imu", "inu"} {
		p, err := Index{Label: label, Variance: VarianceContravariant, Space: &chart.SpaceRef}.Encode()
		if err != nil {
			return fail(err)
		}
		idxRef, err := store.Put(FamilyIndex, p)
		if err != nil {
			return fail(err)
		}
		wantIdx[i] = idxRef
	}
	for i := range wantIdx {
		if ginv.IndexRefs[i] != wantIdx[i] {
			return fail(fmt.Errorf("physmath: scalar inverse slot %d identity broken: %w", i, ErrBadInput))
		}
		idx, err := resolveIndex(store, "scalar inverse slot", ginv.IndexRefs[i])
		if err != nil {
			return fail(err)
		}
		if idx.Variance != VarianceContravariant {
			return fail(fmt.Errorf("physmath: scalar inverse slot %d variance wrong: %w", i, ErrBadInput))
		}
		if idx.Space == nil || *idx.Space != chart.SpaceRef {
			return fail(fmt.Errorf("physmath: scalar inverse slot %d space mismatch: %w", i, ErrBadInput))
		}
	}
	reDetRef2, reInvComps, reInvDeriv, err := InverseMatrix(store, rec.MetricRef, rec.ConditionRefs)
	if err != nil {
		return fail(fmt.Errorf("physmath: scalar inverse recomputation failed: %w", err))
	}
	if reDetRef2 != rec.DetRef {
		return fail(fmt.Errorf("physmath: scalar inverse determinant does not match evidence"))
	}
	if len(reInvComps) != len(ginv.ComponentRefs) {
		return fail(fmt.Errorf("physmath: scalar inverse component count mismatch: %w", ErrBadInput))
	}
	for i := range reInvComps {
		if reInvComps[i] != ginv.ComponentRefs[i] {
			return fail(fmt.Errorf("physmath: scalar inverse component %d mismatch", i))
		}
	}
	if len(invDeriv.Steps) != len(reInvDeriv.Steps) {
		return fail(fmt.Errorf("physmath: scalar inverse evidence shape mismatch"))
	}
	for i := range invDeriv.Steps {
		if !sameStepShape(invDeriv.Steps[i], reInvDeriv.Steps[i]) {
			return fail(fmt.Errorf("physmath: scalar inverse evidence step %d mismatch", i))
		}
	}
	// Exact-base gate re-run: the recorded conditions must byte-match
	// NEQ(det,0) for this metric's determinant. A substituted set that
	// happens to replay elsewhere cannot license this inverse.
	detExpr, err := resolveExpression(store, rec.DetRef)
	if err != nil {
		return fail(err)
	}
	wantCond, err := nonzeroDetCondition(store, detExpr)
	if err != nil {
		return fail(err)
	}
	licensed := false
	for _, c := range rec.ConditionRefs {
		if c == wantCond {
			licensed = true
			break
		}
	}
	if !licensed {
		return fail(fmt.Errorf("physmath: scalar conditions license no exact determinant nonzero: %w", ErrBadInput))
	}
	// Scalar assembly recompute: ordered premise, canon/simplify linkage,
	// exact output match.
	gexprs, err := resolveComponents(store, "scalar inverse", ginv.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	terms := make([]Expr, 0, dim*dim)
	for sigma := 0; sigma < dim; sigma++ {
		for nu := 0; nu < dim; nu++ {
			terms = append(terms, Mul(
				cloneExpr(gexprs[sigma*dim+nu]),
				cloneExpr(ricComps[sigma*dim+nu]),
			))
		}
	}
	premise, err := PutExpression(store, Add(terms...))
	if err != nil {
		return fail(err)
	}
	// Composite derivation layout: determinant evidence, inverse evidence,
	// then the two assembly steps.
	nAsm := 2
	wantLen := len(detDeriv.Steps) + len(invDeriv.Steps) + nAsm
	if len(d.Steps) != wantLen {
		return fail(fmt.Errorf("physmath: scalar derivation steps %d != recomputed %d",
			len(d.Steps), wantLen))
	}
	off := len(detDeriv.Steps) + len(invDeriv.Steps)
	s0, s1 := d.Steps[off], d.Steps[off+1]
	if s0.ProducerNamespace != NamespacePhysMath || s0.RuleID != RuleExprCanon {
		return fail(fmt.Errorf("physmath: scalar assembly step must be CANON"))
	}
	if s1.ProducerNamespace != NamespacePhysMath || s1.RuleID != RuleExprSimplify {
		return fail(fmt.Errorf("physmath: scalar assembly step must be SIMPLIFY"))
	}
	if len(s0.InputRefs) != 1 || s0.InputRefs[0] != premise {
		return fail(fmt.Errorf("physmath: scalar assembly premise mismatch"))
	}
	if len(s1.InputRefs) != 1 || s1.InputRefs[0] != s0.OutputRef {
		return fail(fmt.Errorf("physmath: scalar assembly linkage broken"))
	}
	if s1.OutputRef != rec.ResultRef {
		return fail(fmt.Errorf("physmath: scalar assembly output does not match the record"))
	}
	// The embedded evidence blocks must byte-match the stored evidence
	// derivations (construction embeds them first, in order).
	for i, s := range detDeriv.Steps {
		want, err := s.Encode()
		if err != nil {
			return fail(err)
		}
		got, err := d.Steps[i].Encode()
		if err != nil {
			return fail(err)
		}
		if string(got) != string(want) {
			return fail(fmt.Errorf("physmath: scalar determinant block step %d mismatch", i))
		}
	}
	for i, s := range invDeriv.Steps {
		got := d.Steps[len(detDeriv.Steps)+i]
		if !sameStepShape(got, s) {
			return fail(fmt.Errorf("physmath: scalar inverse block step %d mismatch", i))
		}
		if got.Ordinal != uint32(len(detDeriv.Steps)+i) {
			return fail(fmt.Errorf("physmath: scalar inverse block step %d ordinal broken", i))
		}
	}
	wantPremises := []artifact.ArtifactRef{rec.ContractionRef, crec.RiemannRef, premise}
	wantPremises = append(wantPremises, detDeriv.PremiseRefs...)
	wantPremises = append(wantPremises, invDeriv.PremiseRefs...)
	wantPremises = append(wantPremises, ct.ComponentRefs...)
	wantPremises = append(wantPremises, ginv.ComponentRefs...)
	if !refSetEqual(d.PremiseRefs, uniqueRefs(wantPremises)) {
		return fail(fmt.Errorf("physmath: scalar premises do not match recomputation"))
	}
	wantResults := []artifact.ArtifactRef{rec.ResultRef}
	wantResults = append(wantResults, detDeriv.ResultRefs...)
	wantResults = append(wantResults, invDeriv.ResultRefs...)
	if !refSetEqual(d.ResultRefs, uniqueRefs(wantResults)) {
		return fail(fmt.Errorf("physmath: scalar results do not match recomputation"))
	}
	return rec, d, nil
}

// CheckRiemannScalar is the store-aware provenance gate: it validates metric,
// conditions, determinant evidence, inverse-metric evidence, contraction,
// and scalar result as one consistent chain through the shared helper, then
// replays the validated derivation before trusting the result.
func CheckRiemannScalar(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateScalar(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: scalar derivation does not replay: %w", err)
	}
	return nil
}

// ReplayRiemannScalar is artifact-level replay for a stored riemann_scalar/1
// record: it validates the complete chain (metric, exact conditions,
// determinant and inverse evidence, contraction, scalar result) and then
// re-executes the validated derivation. A corrupted record is rejected even
// when an inner derivation remains independently replayable.
func ReplayRiemannScalar(store Store, ref artifact.ArtifactRef) error {
	_, d, err := resolveAndValidateScalar(store, ref)
	if err != nil {
		return err
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: scalar derivation does not replay: %w", err)
	}
	return nil
}
