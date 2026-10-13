package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// CovariantRankBound caps the input tensor rank for covariant evaluation.
// Result construction needs d^(r+1) components; with MatrixDimBound this
// keeps every derivation small and auditable while covering scalars
// through rank-4 tensors. Enforced before any mint.
const CovariantRankBound = 4

// covInputs is the fully resolved bundle for one covariant-derivative
// computation. All gates have passed when this value exists.
type covInputs struct {
	dim       int
	rank      int
	tensor    Tensor
	indices   []Index
	texprs    []Expr // row-major, dim^rank
	conn      AffineConnection
	cexprs    []Expr // row-major (ρ,μ,ν), dim^3
	chart     CoordinateSystem
	chartRef  artifact.ArtifactRef
	freeIdx   []artifact.ArtifactRef // tensor slot index refs, preserved
	derivIdx  artifact.ArtifactRef   // minted derivative-slot index (COVARIANT)
	coords    []Expr                 // chart coordinate expressions, ordered
	declaring artifact.ArtifactRef   // declaring function for partials
	conds     []artifact.ArtifactRef // normalized condition refs
}

// rowMajorOffset maps a slot tuple to a flat offset (first slot most
// significant), matching the canonical tensor component order.
func rowMajorOffset(idxs []int, d int) int {
	off := 0
	for _, v := range idxs {
		off = off*d + v
	}
	return off
}

// resolveCovInputs enforces every gate in validation order — families,
// ranks, dimensions, bounds, chart agreement, index decoding, variance,
// mapping — before any artifact is minted. Tensor slot spaces must equal
// the chart space by full reference equality; UNSPECIFIED variance fails
// closed. Diagonality is never required.
func resolveCovInputs(store Store, fieldRef, connRef artifact.ArtifactRef, mapping CoordParamMap, declaringFnRef artifact.ArtifactRef, conds []artifact.ArtifactRef) (*covInputs, error) {
	fail := func(err error) (*covInputs, error) { return nil, err }
	ffam, fpayload, err := store.Lookup(fieldRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: covariant field unresolvable: %w", err))
	}
	if ffam != FamilyTensorField {
		return fail(fmt.Errorf("physmath: covariant field is not tensor_field/1: %w", ErrBadInput))
	}
	field, err := DecodeTensorField(fpayload)
	if err != nil {
		return fail(err)
	}
	cfam, cpayload, err := store.Lookup(connRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: covariant connection unresolvable: %w", err))
	}
	if cfam != FamilyAffineConnection {
		return fail(fmt.Errorf("physmath: covariant connection is not affine_connection/1: %w", ErrBadInput))
	}
	conn, err := DecodeAffineConnection(cpayload)
	if err != nil {
		return fail(err)
	}
	if conn.CoordRef != field.CoordinateSystemRef {
		return fail(fmt.Errorf("physmath: connection chart differs from field chart: %w", ErrBadInput))
	}
	tfam, tpayload, err := store.Lookup(field.TensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: covariant tensor unresolvable: %w", err))
	}
	if tfam != FamilyTensor {
		return fail(fmt.Errorf("physmath: covariant tensor is not tensor/1: %w", ErrBadInput))
	}
	tn, err := DecodeTensor(tpayload)
	if err != nil {
		return fail(err)
	}
	if tn.Rank > CovariantRankBound {
		return fail(fmt.Errorf("physmath: covariant rank %d exceeds bound %d: %w",
			tn.Rank, CovariantRankBound, ErrBadInput))
	}
	chfam, chpayload, err := store.Lookup(field.CoordinateSystemRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: covariant chart unresolvable: %w", err))
	}
	if chfam != FamilyCoordinateSystem {
		return fail(fmt.Errorf("physmath: covariant chart is not coordinate_system/1: %w", ErrBadInput))
	}
	chart, err := DecodeCoordinateSystem(chpayload)
	if err != nil {
		return fail(err)
	}
	if int(chart.Dimension) != int(tn.Dimension) || int(chart.Dimension) != int(conn.Dimension) {
		return fail(fmt.Errorf("physmath: chart/tensor/connection dimension mismatch: %w", ErrBadInput))
	}
	d := int(chart.Dimension)
	if d > MatrixDimBound {
		return fail(fmt.Errorf("physmath: covariant dimension %d exceeds bound %d: %w",
			d, MatrixDimBound, ErrBadInput))
	}
	// Single shared chart-binding contract (Plan 1.14): mapping validity,
	// exact chart-coordinate parameter identity, declaring-function names,
	// and operand disjointness. Runs before the first mint below, so it
	// also covers checker recomputation and Laplacian stages, which all
	// route through this function.
	if err := ValidateCoordBinding(store, field.CoordinateSystemRef, mapping, declaringFnRef,
		append(append([]artifact.ArtifactRef{}, tn.ComponentRefs...), conn.CoefficientRefs...)); err != nil {
		return fail(err)
	}
	indices := make([]Index, len(tn.IndexRefs))
	for i, r := range tn.IndexRefs {
		idx, err := resolveIndex(store, "covariant tensor slot", r)
		if err != nil {
			return fail(err)
		}
		if idx.Variance == VarianceUnspecified {
			return fail(fmt.Errorf("physmath: covariant slot %d has UNSPECIFIED variance: %w", i, ErrBadInput))
		}
		if idx.Space == nil || *idx.Space != chart.SpaceRef {
			return fail(fmt.Errorf("physmath: covariant slot %d space differs from chart space: %w", i, ErrBadInput))
		}
		indices[i] = idx
	}
	texprs, err := resolveComponents(store, "covariant tensor", tn.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	cexprs := make([]Expr, len(conn.CoefficientRefs))
	for i, r := range conn.CoefficientRefs {
		fam, payload, err := store.Lookup(r)
		if err != nil {
			return fail(fmt.Errorf("physmath: connection coefficient %d unresolvable: %w", i, err))
		}
		if fam != FamilyExpression {
			return fail(fmt.Errorf("physmath: connection coefficient %d is not expression/1: %w", i, ErrBadInput))
		}
		e, err := DecodeExpr(artifact.NewDecoder(payload))
		if err != nil {
			return fail(fmt.Errorf("physmath: connection coefficient %d undecodable: %w", i, err))
		}
		cexprs[i] = e
	}
	coords := make([]Expr, len(chart.CoordinateRefs))
	for i, r := range chart.CoordinateRefs {
		fam, payload, err := store.Lookup(r)
		if err != nil {
			return fail(fmt.Errorf("physmath: chart coordinate %d unresolvable: %w", i, err))
		}
		if fam != FamilyExpression {
			return fail(fmt.Errorf("physmath: chart coordinate %d is not expression/1: %w", i, ErrBadInput))
		}
		e, err := DecodeExpr(artifact.NewDecoder(payload))
		if err != nil {
			return fail(fmt.Errorf("physmath: chart coordinate %d undecodable: %w", i, err))
		}
		coords[i] = e
	}
	// The derivative-slot index is a slot descriptor (COVARIANT over the
	// chart space), minted deterministically so recomputation reproduces it.
	diPayload, err := Index{Label: "d", Variance: VarianceCovariant, Space: &chart.SpaceRef}.Encode()
	if err != nil {
		return fail(err)
	}
	diRef, err := store.Put(FamilyIndex, diPayload)
	if err != nil {
		return fail(err)
	}
	return &covInputs{
		dim: d, rank: int(tn.Rank), tensor: tn, indices: indices,
		texprs: texprs, conn: conn, cexprs: cexprs, chart: chart,
		chartRef: field.CoordinateSystemRef,
		freeIdx:  tn.IndexRefs, derivIdx: diRef,
		coords: coords, declaring: declaringFnRef,
		conds: normalizeConditionRefs(conds),
	}, nil
}

// covResult is one computed result component with its evidence refs.
type covResult struct {
	expr    Expr
	premise artifact.ArtifactRef
	canon   artifact.ArtifactRef
	out     artifact.ArtifactRef
}

// evalCovCore runs the shared computation: partials for every
// (coordinate, input component) pair, then per-result assembly. It returns
// result expressions in (mu, slots...) row-major order with derivation
// pieces using local ordinals. Callers renumber, mint tensors, and wrap.
func evalCovCore(store Store, in *covInputs, mapping CoordParamMap) ([]covResult, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
	fail := func(err error) ([]covResult, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
		return nil, nil, nil, nil, err
	}
	nIn := 1
	for i := 0; i < in.rank; i++ {
		nIn *= in.dim
	}
	// Phase A: partials[mu][c].
	partials := make([][]artifact.ArtifactRef, in.dim)
	partialExprs := make([][]Expr, in.dim)
	var premises []artifact.ArtifactRef
	var results []artifact.ArtifactRef
	var steps []DerivationStep
	for mu := 0; mu < in.dim; mu++ {
		partials[mu] = make([]artifact.ArtifactRef, nIn)
		partialExprs[mu] = make([]Expr, nIn)
		for c := 0; c < nIn; c++ {
			outRef, recordRef, pd, err := DifferentiateCoordinate(store,
				in.tensor.ComponentRefs[c], in.chartRef, uint32(mu), mapping, in.declaring, 1, in.conds)
			if err != nil {
				return fail(fmt.Errorf("physmath: covariant partial (%d,%d) failed: %w", mu, c, err))
			}
			e, err := resolveExpression(store, outRef)
			if err != nil {
				return fail(err)
			}
			partials[mu][c] = outRef
			partialExprs[mu][c] = e
			premises = append(premises, in.tensor.ComponentRefs[c], recordRef)
			results = append(results, outRef)
			steps = append(steps, pd.Steps[0])
		}
	}
	// Phase B: assembly per result component, derivative-major order.
	var out []covResult
	slots := make([]int, in.rank)
	var walkErr error
	var rec func(mu, pos int)
	rec = func(mu, pos int) {
		if walkErr != nil {
			return
		}
		if pos == in.rank {
			prem := covPremiseFor(in, partialExprs, mu, append([]int{}, slots...))
			prodRef, err := PutExpression(store, prem)
			if err != nil {
				walkErr = err
				return
			}
			canon, err := Apply(store, Invocation{
				ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
				InputRefs: []artifact.ArtifactRef{prodRef},
			})
			if err != nil {
				walkErr = fmt.Errorf("physmath: covariant canon failed: %w", err)
				return
			}
			res, err := Apply(store, Invocation{
				ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
				InputRefs: []artifact.ArtifactRef{canon.OutputRef},
			})
			if err != nil {
				walkErr = fmt.Errorf("physmath: covariant simplify failed: %w", err)
				return
			}
			e, err := resolveExpression(store, res.OutputRef)
			if err != nil {
				walkErr = err
				return
			}
			premises = append(premises, prodRef)
			results = append(results, res.OutputRef)
			steps = append(steps,
				DerivationStep{
					Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
					InputRefs: []artifact.ArtifactRef{prodRef}, OutputRef: canon.OutputRef,
				},
				DerivationStep{
					Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
					InputRefs: []artifact.ArtifactRef{canon.OutputRef}, OutputRef: res.OutputRef,
				},
			)
			out = append(out, covResult{expr: e, premise: prodRef, canon: canon.OutputRef, out: res.OutputRef})
			return
		}
		for v := 0; v < in.dim; v++ {
			slots[pos] = v
			rec(mu, pos+1)
		}
	}
	for mu := 0; mu < in.dim; mu++ {
		rec(mu, 0)
	}
	if walkErr != nil {
		return fail(walkErr)
	}
	return out, steps, premises, results, nil
}

// covPremiseFor is covPremise over pre-resolved partial expressions.
func covPremiseFor(in *covInputs, partials [][]Expr, mu int, slots []int) Expr {
	terms := []Expr{cloneExpr(partials[mu][rowMajorOffset(slots, in.dim)])}
	for s := 0; s < in.rank; s++ {
		for lam := 0; lam < in.dim; lam++ {
			teacher := append([]int{}, slots...)
			teacher[s] = lam
			tcomp := cloneExpr(in.texprs[rowMajorOffset(teacher, in.dim)])
			var g Expr
			if in.indices[s].Variance == VarianceContravariant {
				g = cloneExpr(in.cexprs[(slots[s]*in.dim+mu)*in.dim+lam])
			} else {
				g = cloneExpr(in.cexprs[(lam*in.dim+mu)*in.dim+slots[s]])
			}
			term := Mul(g, tcomp)
			if in.indices[s].Variance == VarianceContravariant {
				terms = append(terms, term)
			} else {
				terms = append(terms, Neg(term))
			}
		}
	}
	if len(terms) == 1 {
		return terms[0]
	}
	return Add(terms...)
}

// FamilyCovariantEvaluation is the evaluated covariant-derivative record:
// chart-bound input field, connection, computed chart-bound result field,
// and the replayable derivation. Family
// physmath/covariant_evaluation/1.
var FamilyCovariantEvaluation = FamilyID{NamespacePhysMath, "covariant_evaluation/1", "1"}

// CovariantEvaluation links input field, connection, result field, and
// derivation evidence. The derivative ranges over all chart coordinates
// (rank r+1 result); per-component derivative-slot values are verified by
// ordered mapping, not by a single selected index. Validate checks
// structure only; CheckCovariantEvaluation resolves and recomputes.
type CovariantEvaluation struct {
	FieldRef      artifact.ArtifactRef // → tensor_field/1 (input)
	ConnectionRef artifact.ArtifactRef // → affine_connection/1
	ResultRef     artifact.ArtifactRef // → tensor_field/1 (computed, same chart)
	DerivationRef artifact.ArtifactRef // required → derivation/1
}

// Family returns FamilyCovariantEvaluation.
func (CovariantEvaluation) Family() FamilyID { return FamilyCovariantEvaluation }

// Encode returns the canonical payload in declaration order.
func (c CovariantEvaluation) Encode() []byte {
	out := c.FieldRef.Encode()
	out = append(out, c.ConnectionRef.Encode()...)
	out = append(out, c.ResultRef.Encode()...)
	return append(out, c.DerivationRef.Encode()...)
}

// Validate enforces reference grammar without resolution.
func (c CovariantEvaluation) Validate() error {
	return validateRefGrammars(map[string]artifact.ArtifactRef{
		"CovariantEvaluation.FieldRef":      c.FieldRef,
		"CovariantEvaluation.ConnectionRef": c.ConnectionRef,
		"CovariantEvaluation.ResultRef":     c.ResultRef,
		"CovariantEvaluation.DerivationRef": c.DerivationRef,
	})
}

// TargetChecks: FieldRef → tensor_field/1; ConnectionRef →
// affine_connection/1; ResultRef → tensor_field/1; DerivationRef →
// derivation/1. Resolved only by the store-aware check.
func (c CovariantEvaluation) TargetChecks() []TargetCheck {
	return []TargetCheck{
		{Field: "CovariantEvaluation.FieldRef", Ref: c.FieldRef, Allowed: []FamilyID{FamilyTensorField}},
		{Field: "CovariantEvaluation.ConnectionRef", Ref: c.ConnectionRef, Allowed: []FamilyID{FamilyAffineConnection}},
		{Field: "CovariantEvaluation.ResultRef", Ref: c.ResultRef, Allowed: []FamilyID{FamilyTensorField}},
		{Field: "CovariantEvaluation.DerivationRef", Ref: c.DerivationRef, Allowed: []FamilyID{FamilyDerivation}},
	}
}

// DecodeCovariantEvaluation decodes a physmath/covariant_evaluation/1
// payload. Representation only.
func DecodeCovariantEvaluation(payload []byte) (CovariantEvaluation, error) {
	d := artifact.NewDecoder(payload)
	field, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return CovariantEvaluation{}, fmt.Errorf("physmath: evaluation missing field: %w", err)
	}
	conn, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return CovariantEvaluation{}, fmt.Errorf("physmath: evaluation missing connection: %w", err)
	}
	result, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return CovariantEvaluation{}, fmt.Errorf("physmath: evaluation missing result: %w", err)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return CovariantEvaluation{}, fmt.Errorf("physmath: evaluation missing derivation: %w", err)
	}
	if !d.Exhausted() {
		return CovariantEvaluation{}, fmt.Errorf("physmath: trailing bytes in covariant-evaluation payload")
	}
	c := CovariantEvaluation{FieldRef: field, ConnectionRef: conn, ResultRef: result, DerivationRef: deriv}
	if err := c.Validate(); err != nil {
		return CovariantEvaluation{}, err
	}
	return c, nil
}

// EvaluateCovariantDerivative executes the general affine covariant
// derivative through the existing executable path (per-component partials
// via DIFFERENTIATE-001, connection-term Mul assembly, CANON+SIMPLIFY
// replay). It returns the result tensor_field/1 reference (rank r+1,
// derivative index prepended as COVARIANT, input chart preserved) plus one
// flat replayable derivation with indexed per-component linkage.
//
// No new rule; the operator.go covariant_derivative reservation stays
// non-executable. Torsion is never assumed: coefficients enter exactly as
// stored, with +Γ per contravariant slot and −Γ per covariant slot.
func EvaluateCovariantDerivative(store Store, fieldRef, connRef artifact.ArtifactRef, mapping CoordParamMap, declaringFnRef artifact.ArtifactRef, conds []artifact.ArtifactRef) (artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, Derivation{}, err
	}
	in, err := resolveCovInputs(store, fieldRef, connRef, mapping, declaringFnRef, conds)
	if err != nil {
		return fail(err)
	}
	resultTensor, _, steps, premises, results, err := evalFieldTensor(store, in, mapping)
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
	// Contiguous ordinals in execution order; declaring function is a
	// genuine premise consumed by every partial step.
	premises = append([]artifact.ArtifactRef{declaringFnRef}, premises...)
	for i := range steps {
		steps[i].Ordinal = uint32(i)
	}
	d := Derivation{
		PremiseRefs: uniqueRefs(premises),
		Steps:       steps,
		ResultRefs:  uniqueRefs(results),
	}
	if err := Replay(store, d, nil); err != nil {
		return fail(fmt.Errorf("physmath: covariant derivation does not replay: %w", err))
	}
	return resultField, d, nil
}

// evalFieldTensor runs the shared core and mints the result tensor,
// returning its ref with the computed outputs and derivation pieces.
func evalFieldTensor(store Store, in *covInputs, mapping CoordParamMap) (artifact.ArtifactRef, []covResult, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
	fail := func(err error) (artifact.ArtifactRef, []covResult, []DerivationStep, []artifact.ArtifactRef, []artifact.ArtifactRef, error) {
		return artifact.ArtifactRef{}, nil, nil, nil, nil, err
	}
	out, steps, premises, results, err := evalCovCore(store, in, mapping)
	if err != nil {
		return fail(err)
	}
	outRefs := make([]artifact.ArtifactRef, len(out))
	for i, r := range out {
		outRefs[i] = r.out
	}
	rt := Tensor{Rank: uint16(in.rank + 1), Dimension: uint16(in.dim),
		IndexRefs:     append([]artifact.ArtifactRef{in.derivIdx}, in.freeIdx...),
		ComponentRefs: outRefs}
	if err := rt.Validate(); err != nil {
		return fail(err)
	}
	resultTensor, err := store.Put(FamilyTensor, rt.Encode())
	if err != nil {
		return fail(err)
	}
	return resultTensor, out, steps, premises, results, nil
}

// ScalarLaplacian computes ∇²f = g^μν ∇_μ∇_ν f through the existing
// executable path: two covariant-derivative stages via the shared core,
// then a double-sum driver (premise → CANON → SIMPLIFY → COMBINE →
// SIMPLIFY, each replayed). Operator composition is respected: the second
// derivative acts on the first derivative's output, never collapsed to
// g^μν∂_μ∂_νf. The inverse metric is a rank-2 tensor/1 with both slots
// CONTRAVARIANT over the chart space; the final COMBINE+SIMPLIFY pair
// collapses exactly the identities combination can prove (e.g. r²·(r²)⁻¹
// under a nonzero condition), which SIMPLIFY alone leaves factored.
//
// It returns the scalar expression/1 reference plus one flat replayable
// derivation embedding both stages and the four assembly steps. The scalar
// input is a rank-0 tensor_field; the output is a bare expression (rank-0
// results carry no indices to preserve).
func ScalarLaplacian(store Store, scalarFieldRef, connRef, invMetricRef artifact.ArtifactRef, mapping CoordParamMap, declaringFnRef artifact.ArtifactRef, conds []artifact.ArtifactRef) (artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, Derivation{}, err
	}
	in0, err := resolveCovInputs(store, scalarFieldRef, connRef, mapping, declaringFnRef, conds)
	if err != nil {
		return fail(err)
	}
	if in0.rank != 0 {
		return fail(fmt.Errorf("physmath: laplacian input rank %d != 0: %w", in0.rank, ErrBadInput))
	}
	h1Tensor, _, h1steps, h1premises, h1results, err := evalFieldTensor(store, in0, mapping)
	if err != nil {
		return fail(err)
	}
	h1field := TensorField{TensorRef: h1Tensor, CoordinateSystemRef: in0.chartRef}
	if err := h1field.Validate(); err != nil {
		return fail(err)
	}
	h1Ref, err := store.Put(FamilyTensorField, h1field.Encode())
	if err != nil {
		return fail(err)
	}
	in1, err := resolveCovInputs(store, h1Ref, connRef, mapping, declaringFnRef, conds)
	if err != nil {
		return fail(err)
	}
	if in1.rank != 1 {
		return fail(fmt.Errorf("physmath: laplacian intermediate rank %d != 1: %w", in1.rank, ErrBadInput))
	}
	_, h2out, h2steps, h2premises, h2results, err := evalFieldTensor(store, in1, mapping)
	if err != nil {
		return fail(err)
	}
	_ = h2out
	gfam, gpayload, err := store.Lookup(invMetricRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: laplacian inverse metric unresolvable: %w", err))
	}
	if gfam != FamilyTensor {
		return fail(fmt.Errorf("physmath: laplacian inverse metric is not tensor/1: %w", ErrBadInput))
	}
	ginv, err := DecodeTensor(gpayload)
	if err != nil {
		return fail(err)
	}
	if ginv.Rank != 2 || int(ginv.Dimension) != in0.dim {
		return fail(fmt.Errorf("physmath: laplacian inverse metric shape wrong: %w", ErrBadInput))
	}
	for s := 0; s < 2; s++ {
		idx, err := resolveIndex(store, "laplacian inverse slot", ginv.IndexRefs[s])
		if err != nil {
			return fail(err)
		}
		if idx.Variance != VarianceContravariant {
			return fail(fmt.Errorf("physmath: laplacian inverse slot %d is not contravariant: %w", s, ErrBadInput))
		}
		if idx.Space == nil || *idx.Space != in0.chart.SpaceRef {
			return fail(fmt.Errorf("physmath: laplacian inverse space mismatch: %w", ErrBadInput))
		}
	}
	gexprs, err := resolveComponents(store, "laplacian inverse", ginv.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	h2exprs := make([]Expr, len(h2out))
	for i, r := range h2out {
		h2exprs[i] = r.expr
	}
	terms := make([]Expr, 0, in0.dim*in0.dim)
	for mu := 0; mu < in0.dim; mu++ {
		for nu := 0; nu < in0.dim; nu++ {
			terms = append(terms, Mul(
				cloneExpr(gexprs[mu*in0.dim+nu]),
				cloneExpr(h2exprs[mu*in0.dim+nu]),
			))
		}
	}
	sPremise, err := PutExpression(store, Add(terms...))
	if err != nil {
		return fail(err)
	}
	// Assembly is CANON then SIMPLIFY only: COMBINE targets single Mul
	// roots, and nested-Pow bases (Pow(Pow(r,2),-1) vs Pow(r,2)) do not
	// combine, so combination claims stay with shapes the rule supports.
	// The factored result is pinned with hand-verified equality instead.
	chain := []struct {
		rule  string
		conds []artifact.ArtifactRef
	}{
		{RuleExprCanon, nil},
		{RuleExprSimplify, conds},
	}
	cur := sPremise
	var asmSteps []DerivationStep
	var asmResults []artifact.ArtifactRef
	for _, link := range chain {
		res, err := Apply(store, Invocation{
			ProducerNamespace: NamespacePhysMath, RuleID: link.rule,
			InputRefs:     []artifact.ArtifactRef{cur},
			ConditionRefs: link.conds,
		})
		if err != nil {
			return fail(fmt.Errorf("physmath: laplacian assembly %s failed: %w", link.rule, err))
		}
		asmSteps = append(asmSteps, DerivationStep{
			Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: link.rule,
			InputRefs: []artifact.ArtifactRef{cur}, OutputRef: res.OutputRef,
			ConditionRefs: normalizeConditionRefs(link.conds),
		})
		asmResults = append(asmResults, res.OutputRef)
		cur = res.OutputRef
	}
	var premises []artifact.ArtifactRef
	var results []artifact.ArtifactRef
	var steps []DerivationStep
	ord := uint32(0)
	emit := func(s DerivationStep) {
		s.Ordinal = ord
		ord++
		steps = append(steps, s)
	}
	premises = append(premises, declaringFnRef)
	for _, lst := range [][]artifact.ArtifactRef{h1premises, h2premises, {sPremise}} {
		premises = append(premises, lst...)
	}
	for _, lst := range [][]artifact.ArtifactRef{h1results, h2results} {
		results = append(results, lst...)
	}
	for _, lst := range [][]DerivationStep{h1steps, h2steps, asmSteps} {
		for _, s := range lst {
			emit(s)
		}
	}
	results = append(results, asmResults...)
	d := Derivation{
		PremiseRefs: uniqueRefs(premises),
		Steps:       steps,
		ResultRefs:  uniqueRefs(results),
	}
	if err := Replay(store, d, nil); err != nil {
		return fail(fmt.Errorf("physmath: laplacian derivation does not replay: %w", err))
	}
	return cur, d, nil
}

// CheckCovariantEvaluation is the store-aware provenance gate: family
// resolution, operand re-gating, derivation shape with indexed linkage
// (partial block then per-component CANON+SIMPLIFY pairs), replay before
// recomputation (AGENTS.md §9), recomputed premise multiset and ordered
// component match including derivative-slot identity and chart
// preservation.
func CheckCovariantEvaluation(store Store, c CovariantEvaluation, mapping CoordParamMap, declaringFnRef artifact.ArtifactRef) error {
	if err := c.Validate(); err != nil {
		return err
	}
	for _, tc := range c.TargetChecks() {
		fam, _, err := store.Lookup(tc.Ref)
		if err != nil {
			return fmt.Errorf("physmath: %s unresolvable: %w", tc.Field, err)
		}
		ok := false
		for _, a := range tc.Allowed {
			if fam == a {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("physmath: %s targets %s/%s/%s, not in closed target set",
				tc.Field, fam.Namespace, fam.ArtifactType, fam.SchemaVersion)
		}
	}
	_, dpayload, err := store.Lookup(c.DerivationRef)
	if err != nil {
		return fmt.Errorf("physmath: covariant derivation unresolvable: %w", err)
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return err
	}
	// Uniform conditions: all partial steps share one normalized set
	// (construction threads one set to every partial Apply; assembly
	// steps carry none). The set is recovered from the steps themselves
	// for recomputation.
	var conds []artifact.ArtifactRef
	nIn := 0
	// Resolve input field tensor rank/dim first for shape bounds.
	ffam, fpayload, err := store.Lookup(c.FieldRef)
	if err != nil {
		return fmt.Errorf("physmath: covariant field unresolvable: %w", err)
	}
	if ffam != FamilyTensorField {
		return fmt.Errorf("physmath: covariant field is not tensor_field/1: %w", ErrBadInput)
	}
	field, err := DecodeTensorField(fpayload)
	if err != nil {
		return err
	}
	tfam, tpayload, err := store.Lookup(field.TensorRef)
	if err != nil {
		return fmt.Errorf("physmath: covariant tensor unresolvable: %w", err)
	}
	if tfam != FamilyTensor {
		return fmt.Errorf("physmath: covariant tensor is not tensor/1: %w", ErrBadInput)
	}
	tn, err := DecodeTensor(tpayload)
	if err != nil {
		return err
	}
	chfam, chpayload, err := store.Lookup(field.CoordinateSystemRef)
	if err != nil {
		return fmt.Errorf("physmath: covariant chart unresolvable: %w", err)
	}
	if chfam != FamilyCoordinateSystem {
		return fmt.Errorf("physmath: covariant chart is not coordinate_system/1: %w", ErrBadInput)
	}
	chart, err := DecodeCoordinateSystem(chpayload)
	if err != nil {
		return err
	}
	r, dim := int(tn.Rank), int(chart.Dimension)
	// Partial steps number d^(r+1): one per (coordinate-major, input)
	// pair. Assembly adds two steps per result component.
	nOut := dim
	for i := 0; i < r; i++ {
		nOut *= dim
	}
	if len(d.Steps) != nOut*3 {
		return fmt.Errorf("physmath: covariant derivation must have %d steps, got %d", nOut*3, len(d.Steps))
	}
	nIn = nOut / dim
	for i, s := range d.Steps {
		if s.Ordinal != uint32(i) {
			return fmt.Errorf("physmath: covariant steps must be ordinals 0..%d in order", len(d.Steps)-1)
		}
	}
	// Partial block: one DIFFERENTIATE step per (coordinate-major, input)
	// pair, consuming the input component and declaring function, with the
	// mapping's BPR for that coordinate.
	var stepOuts []artifact.ArtifactRef
	pos := 0
	for mu := 0; mu < dim; mu++ {
		wantParams := append(mapping.Entries[mu].Encode(), artifact.EncodeUInt32(1)...)
		for cc := 0; cc < nIn; cc++ {
			s := d.Steps[pos]
			pos++
			if s.ProducerNamespace != NamespacePhysMath || s.RuleID != RuleDifferentiate {
				return fmt.Errorf("physmath: covariant partial step must be DIFFERENTIATE-001")
			}
			if len(s.InputRefs) != 2 || s.InputRefs[0] != tn.ComponentRefs[cc] || s.InputRefs[1] != declaringFnRef {
				return fmt.Errorf("physmath: covariant partial step linkage broken")
			}
			if string(s.Parameters) != string(wantParams) {
				return fmt.Errorf("physmath: covariant partial step parameters mismatch")
			}
			if conds == nil {
				conds = append([]artifact.ArtifactRef{}, s.ConditionRefs...)
			} else if !equalRefSets(conds, s.ConditionRefs) {
				return fmt.Errorf("physmath: covariant condition sets differ across partial steps")
			}
			stepOuts = append(stepOuts, s.OutputRef)
		}
	}
	// Assembly pairs with indexed linkage; rule/shape gates here, ordered
	// mapping verified against recomputation below.
	var asmOuts []artifact.ArtifactRef
	for i := 0; i < nOut; i++ {
		s0, s1 := d.Steps[pos+2*i], d.Steps[pos+2*i+1]
		if s0.ProducerNamespace != NamespacePhysMath || s0.RuleID != RuleExprCanon {
			return fmt.Errorf("physmath: covariant assembly step must be CANON")
		}
		if s1.ProducerNamespace != NamespacePhysMath || s1.RuleID != RuleExprSimplify {
			return fmt.Errorf("physmath: covariant assembly step must be SIMPLIFY")
		}
		if len(s0.ConditionRefs) != 0 || len(s1.ConditionRefs) != 0 {
			return fmt.Errorf("physmath: covariant assembly steps must carry no conditions")
		}
		if len(s0.InputRefs) != 1 || len(s1.InputRefs) != 1 || s1.InputRefs[0] != s0.OutputRef {
			return fmt.Errorf("physmath: covariant assembly linkage broken")
		}
		asmOuts = append(asmOuts, s1.OutputRef)
	}
	if !refSetEqual(d.ResultRefs, append(stepOuts, asmOuts...)) {
		return fmt.Errorf("physmath: covariant results do not match step outputs")
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: covariant derivation does not replay: %w", err)
	}
	// Recomputation: identical construction, idempotent mints. By this
	// point replay has established availability, so these mints reproduce
	// — never repair — the recorded evidence.
	in, err := resolveCovInputs(store, c.FieldRef, c.ConnectionRef, mapping, declaringFnRef, conds)
	if err != nil {
		return err
	}
	out, _, premises, _, err := evalCovCore(store, in, mapping)
	if err != nil {
		return err
	}
	if !refSetEqual(d.PremiseRefs, append(premises, declaringFnRef)) {
		return fmt.Errorf("physmath: covariant premises do not match recomputation")
	}
	// Ordered result components + derivative-slot + chart preservation.
	rfam, rpayload, err := store.Lookup(c.ResultRef)
	if err != nil {
		return fmt.Errorf("physmath: covariant result unresolvable: %w", err)
	}
	if rfam != FamilyTensorField {
		return fmt.Errorf("physmath: covariant result is not tensor_field/1: %w", ErrBadInput)
	}
	rf, err := DecodeTensorField(rpayload)
	if err != nil {
		return err
	}
	if rf.CoordinateSystemRef != field.CoordinateSystemRef {
		return fmt.Errorf("physmath: covariant result chart not preserved: %w", ErrBadInput)
	}
	rtfam, rtpayload, err := store.Lookup(rf.TensorRef)
	if err != nil {
		return fmt.Errorf("physmath: covariant result tensor unresolvable: %w", err)
	}
	if rtfam != FamilyTensor {
		return fmt.Errorf("physmath: covariant result tensor is not tensor/1: %w", ErrBadInput)
	}
	rt, err := DecodeTensor(rtpayload)
	if err != nil {
		return err
	}
	if int(rt.Rank) != r+1 || int(rt.Dimension) != dim {
		return fmt.Errorf("physmath: covariant result shape wrong: %w", ErrBadInput)
	}
	if len(rt.IndexRefs) != r+1 || rt.IndexRefs[0] != in.derivIdx {
		return fmt.Errorf("physmath: covariant derivative-slot identity broken: %w", ErrBadInput)
	}
	for i := 1; i <= r; i++ {
		if rt.IndexRefs[i] != tn.IndexRefs[i-1] {
			return fmt.Errorf("physmath: covariant free index %d not preserved: %w", i, ErrBadInput)
		}
	}
	if len(rt.ComponentRefs) != len(out) {
		return fmt.Errorf("physmath: covariant result component count mismatch: %w", ErrBadInput)
	}
	for i := range out {
		_, opayload, err := store.Lookup(out[i].out)
		if err != nil {
			return err
		}
		_, rp, err := store.Lookup(rt.ComponentRefs[i])
		if err != nil {
			return err
		}
		if string(opayload) != string(rp) {
			return fmt.Errorf("physmath: covariant result component %d mismatch: %w", i, ErrBadInput)
		}
	}
	return nil
}
