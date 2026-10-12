package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// ContractEvalDimensionBound caps the shared operand dimension for
// CONTRACT-EVAL-001, mirroring SymmetricBilinearDimensionBound. At most
// d result components are built, each from d products, with two rule
// executions per component. The bound is enforced after shape gates and
// before any artifact is minted: exceeding it fails deterministically with
// no result reference and no certificate.
const ContractEvalDimensionBound = 64

// DecodeTensor decodes a physmath/tensor/1 canonical payload in declaration
// order: UInt16(rank) || UInt16(dimension) || Sequence(index refs) ||
// Sequence(component refs). It is representation only: no store Lookup,
// no execution. Shape is enforced with Validate.
func DecodeTensor(payload []byte) (Tensor, error) {
	d := artifact.NewDecoder(payload)
	rank, err := d.UInt16()
	if err != nil {
		return Tensor{}, fmt.Errorf("physmath: tensor missing rank: %w", err)
	}
	dim, err := d.UInt16()
	if err != nil {
		return Tensor{}, fmt.Errorf("physmath: tensor missing dimension: %w", err)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return Tensor{}, fmt.Errorf("physmath: tensor missing index sequence: %w", err)
	}
	idx := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return Tensor{}, fmt.Errorf("physmath: tensor bad index ref %d: %w", i, err)
		}
		idx = append(idx, r)
	}
	m, err := d.SequenceCount()
	if err != nil {
		return Tensor{}, fmt.Errorf("physmath: tensor missing component sequence: %w", err)
	}
	comps := make([]artifact.ArtifactRef, 0, m)
	for i := 0; i < m; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return Tensor{}, fmt.Errorf("physmath: tensor bad component ref %d: %w", i, err)
		}
		comps = append(comps, r)
	}
	if !d.Exhausted() {
		return Tensor{}, fmt.Errorf("physmath: trailing bytes in tensor payload")
	}
	t := Tensor{Rank: rank, Dimension: dim, IndexRefs: idx, ComponentRefs: comps}
	if err := t.Validate(); err != nil {
		return Tensor{}, err
	}
	return t, nil
}

// contractionInputs is the fully resolved operand bundle for one
// CONTRACT-EVAL-001 operation. All gates in §3A–§3B have passed when this
// value exists; construction may proceed.
type contractionInputs struct {
	dim          int
	slot         uint32
	freeIndexRef artifact.ArtifactRef
	tensorExprs  []Expr // row-major, d*d
	vectorExprs  []Expr // d
}

// resolveIndex looks up an index/1 artifact and decodes it.
func resolveIndex(store Store, field string, ref artifact.ArtifactRef) (Index, error) {
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return Index{}, fmt.Errorf("physmath: %s unresolvable: %w", field, err)
	}
	if fam != FamilyIndex {
		return Index{}, fmt.Errorf("physmath: %s is not index/1", field)
	}
	idx, err := DecodeIndex(payload)
	if err != nil {
		return Index{}, fmt.Errorf("physmath: %s undecodable: %w", field, err)
	}
	return idx, nil
}

// resolveComponents looks up d*n expression/1 artifacts and decodes them.
func resolveComponents(store Store, what string, refs []artifact.ArtifactRef) ([]Expr, error) {
	out := make([]Expr, 0, len(refs))
	for i, r := range refs {
		fam, payload, err := store.Lookup(r)
		if err != nil {
			return nil, fmt.Errorf("physmath: %s component %d unresolvable: %w", what, i, err)
		}
		if fam != FamilyExpression {
			return nil, fmt.Errorf("physmath: %s component %d is not expression/1", what, i)
		}
		e, err := DecodeExpr(artifact.NewDecoder(payload))
		if err != nil {
			return nil, fmt.Errorf("physmath: %s component %d undecodable: %w", what, i, err)
		}
		out = append(out, e)
	}
	return out, nil
}

// resolveContractionOperands enforces every §3A–§3B gate in validation
// order — resolve, decode, rank/position, variance opposition, label
// equality, space equality, dimension equality, dimension bound — before
// any artifact is minted. Labels compare by byte equality; spaces compare
// by resolved collection/1 reference equality. Diagonality is not required.
func resolveContractionOperands(store Store, tensorRef, vectorRef artifact.ArtifactRef, slot uint32) (*contractionInputs, error) {
	fail := func(err error) (*contractionInputs, error) { return nil, err }
	tfam, tpayload, err := store.Lookup(tensorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction tensor unresolvable: %w", err))
	}
	if tfam != FamilyTensor {
		return fail(fmt.Errorf("physmath: contraction tensor is not tensor/1"))
	}
	vfam, vpayload, err := store.Lookup(vectorRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction vector unresolvable: %w", err))
	}
	if vfam != FamilyTensor {
		return fail(fmt.Errorf("physmath: contraction vector is not tensor/1"))
	}
	t, err := DecodeTensor(tpayload)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction tensor undecodable: %w", err))
	}
	v, err := DecodeTensor(vpayload)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction vector undecodable: %w", err))
	}
	if t.Rank != 2 {
		return fail(fmt.Errorf("physmath: contraction tensor rank %d != 2: %w", t.Rank, ErrBadInput))
	}
	if v.Rank != 1 {
		return fail(fmt.Errorf("physmath: contraction vector rank %d != 1: %w", v.Rank, ErrBadInput))
	}
	if slot > 1 {
		return fail(fmt.Errorf("physmath: contraction slot %d out of tensor rank 2: %w", slot, ErrBadInput))
	}
	if t.Dimension != v.Dimension {
		return fail(fmt.Errorf("physmath: contraction dimension mismatch %d != %d: %w",
			t.Dimension, v.Dimension, ErrBadInput))
	}
	if t.Dimension < 1 {
		return fail(fmt.Errorf("physmath: contraction dimension must be >= 1: %w", ErrBadInput))
	}
	if t.Dimension > ContractEvalDimensionBound {
		return fail(fmt.Errorf("physmath: contraction dimension %d exceeds bound %d: %w",
			t.Dimension, ContractEvalDimensionBound, ErrBadInput))
	}
	tIdx, err := resolveIndex(store, "contraction tensor slot", t.IndexRefs[slot])
	if err != nil {
		return fail(err)
	}
	vIdx, err := resolveIndex(store, "contraction vector slot", v.IndexRefs[0])
	if err != nil {
		return fail(err)
	}
	if tIdx.Variance == VarianceUnspecified || vIdx.Variance == VarianceUnspecified {
		return fail(fmt.Errorf("physmath: contraction with UNSPECIFIED variance fails closed: %w", ErrBadInput))
	}
	if tIdx.Variance == vIdx.Variance {
		return fail(fmt.Errorf("physmath: contraction requires opposing variances, got %s/%s: %w",
			tIdx.Variance, vIdx.Variance, ErrBadInput))
	}
	if tIdx.Label != vIdx.Label {
		return fail(fmt.Errorf("physmath: contraction label mismatch %q != %q: %w",
			tIdx.Label, vIdx.Label, ErrBadInput))
	}
	if tIdx.Space == nil || vIdx.Space == nil {
		return fail(fmt.Errorf("physmath: contraction requires resolved space evidence on both slots: %w", ErrBadInput))
	}
	tSpaceFam, _, err := store.Lookup(*tIdx.Space)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction tensor space unresolvable: %w", err))
	}
	if tSpaceFam != FamilyCollection {
		return fail(fmt.Errorf("physmath: contraction tensor space is not collection/1: %w", ErrBadInput))
	}
	vSpaceFam, _, err := store.Lookup(*vIdx.Space)
	if err != nil {
		return fail(fmt.Errorf("physmath: contraction vector space unresolvable: %w", err))
	}
	if vSpaceFam != FamilyCollection {
		return fail(fmt.Errorf("physmath: contraction vector space is not collection/1: %w", ErrBadInput))
	}
	if *tIdx.Space != *vIdx.Space {
		return fail(fmt.Errorf("physmath: contraction space mismatch: %w", ErrBadInput))
	}
	texprs, err := resolveComponents(store, "contraction tensor", t.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	vexprs, err := resolveComponents(store, "contraction vector", v.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	free := uint32(0)
	if slot == 0 {
		free = 1
	}
	return &contractionInputs{
		dim:          int(t.Dimension),
		slot:         slot,
		freeIndexRef: t.IndexRefs[free],
		tensorExprs:  texprs,
		vectorExprs:  vexprs,
	}, nil
}

// contractionPremise builds the unevaluated sum for output component i:
// Add(Mul(T[off],V[k]) for k in 0..d-1) with row-major offsets off = i*d+k
// for slot 1 and off = k*d+i for slot 0.
func contractionPremise(in *contractionInputs, i int) Expr {
	terms := make([]Expr, 0, in.dim)
	for k := 0; k < in.dim; k++ {
		var off int
		if in.slot == 1 {
			off = i*in.dim + k
		} else {
			off = k*in.dim + i
		}
		terms = append(terms, Mul(cloneExpr(in.tensorExprs[off]), cloneExpr(in.vectorExprs[k])))
	}
	return Add(terms...)
}

// uniqueRefs deduplicates preserving first order (SET semantics for
// derivation premise/result lists, mirroring EncodeSet).
func uniqueRefs(refs []artifact.ArtifactRef) []artifact.ArtifactRef {
	seen := map[artifact.ArtifactRef]bool{}
	out := make([]artifact.ArtifactRef, 0, len(refs))
	for _, r := range refs {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// EvaluateContraction executes CONTRACT-EVAL-001: single-pair contraction
// of a rank-2 tensor with a rank-1 vector through the existing executable
// path (per-component Mul/Add construction + EXPR-CANON-001 then
// EXPR-SIMPLIFY-001 replay). It returns the result tensor/1 reference
// (rank 1, carrying the free-slot index artifact) plus the flat replayable
// 2d-step derivation (per-component CANON then SIMPLIFY pairs). No new
// rule, no registry change. The result is w (the bare contraction), never
// a negated or metric-adjusted variant.
func EvaluateContraction(store Store, tensorRef, vectorRef artifact.ArtifactRef, slot uint32) (artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, Derivation{}, err
	}
	in, err := resolveContractionOperands(store, tensorRef, vectorRef, slot)
	if err != nil {
		return fail(err)
	}
	premises := make([]artifact.ArtifactRef, 0, in.dim)
	results := make([]artifact.ArtifactRef, 0, in.dim)
	steps := make([]DerivationStep, 0, 2*in.dim)
	for i := 0; i < in.dim; i++ {
		prodRef, err := PutExpression(store, contractionPremise(in, i))
		if err != nil {
			return fail(err)
		}
		canon, err := Apply(store, Invocation{
			ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
			InputRefs: []artifact.ArtifactRef{prodRef},
		})
		if err != nil {
			return fail(fmt.Errorf("physmath: contraction component %d canon failed: %w", i, err))
		}
		res, err := Apply(store, Invocation{
			ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
			InputRefs: []artifact.ArtifactRef{canon.OutputRef},
		})
		if err != nil {
			return fail(fmt.Errorf("physmath: contraction component %d simplify failed: %w", i, err))
		}
		premises = append(premises, prodRef)
		results = append(results, res.OutputRef)
		steps = append(steps,
			DerivationStep{
				Ordinal: uint32(2 * i), ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
				InputRefs: []artifact.ArtifactRef{prodRef}, OutputRef: canon.OutputRef,
			},
			DerivationStep{
				Ordinal: uint32(2*i + 1), ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
				InputRefs: []artifact.ArtifactRef{canon.OutputRef}, OutputRef: res.OutputRef,
			},
		)
	}
	resRefs := make([]artifact.ArtifactRef, len(results))
	copy(resRefs, results)
	rt := Tensor{Rank: 1, Dimension: uint16(in.dim),
		IndexRefs: []artifact.ArtifactRef{in.freeIndexRef}, ComponentRefs: resRefs}
	if err := rt.Validate(); err != nil {
		return fail(err)
	}
	tensorPayload := rt.Encode()
	resultRef, err := store.Put(FamilyTensor, tensorPayload)
	if err != nil {
		return fail(err)
	}
	d := Derivation{
		PremiseRefs: uniqueRefs(premises),
		Steps:       steps,
		ResultRefs:  uniqueRefs(results),
	}
	if err := Replay(store, d, nil); err != nil {
		return fail(fmt.Errorf("physmath: contraction derivation does not replay: %w", err))
	}
	return resultRef, d, nil
}

// FamilyContractionEvaluation is the evaluated tensor×vector contraction
// record: exact operand identities, the contracted slot, the computed
// result tensor, and the replayable derivation. The square-bracketed
// evaluation binds the stored operands to the exact output through the
// existing executable path. Family physmath/contraction_evaluation/1.
var FamilyContractionEvaluation = FamilyID{NamespacePhysMath, "contraction_evaluation/1", "1"}

// ContractionEvaluation links a rank-2 tensor, a rank-1 vector, the
// contracted tensor slot, the computed rank-1 result tensor, and the exact
// 2d-step CANON+SIMPLIFY derivation. Determinant-style provenance applies:
// Validate checks structure only (slot bound, reference grammar); the
// store-aware CheckContractionEvaluation resolves operands, replays the
// derivation, and matches ordered premises and components.
type ContractionEvaluation struct {
	TensorRef            artifact.ArtifactRef // → tensor/1 (rank 2)
	VectorRef            artifact.ArtifactRef // → tensor/1 (rank 1)
	ContractedTensorSlot uint32               // tensor slot, must be < 2
	ResultTensorRef      artifact.ArtifactRef // → tensor/1 (rank 1, computed w)
	DerivationRef        artifact.ArtifactRef // required → derivation/1 (flat 2d-step evidence)
}

// Family returns FamilyContractionEvaluation.
func (ContractionEvaluation) Family() FamilyID { return FamilyContractionEvaluation }

// Encode returns the canonical payload in declaration order.
func (c ContractionEvaluation) Encode() []byte {
	out := c.TensorRef.Encode()
	out = append(out, c.VectorRef.Encode()...)
	out = append(out, artifact.EncodeUInt32(c.ContractedTensorSlot)...)
	out = append(out, c.ResultTensorRef.Encode()...)
	return append(out, c.DerivationRef.Encode()...)
}

// Validate enforces the slot bound and reference grammar without
// resolution: rank, variance, label, space, dimension, and bound gates
// belong exclusively to the store-aware check. It never executes.
func (c ContractionEvaluation) Validate() error {
	if c.ContractedTensorSlot > 1 {
		return fmt.Errorf("physmath: contraction evaluation slot %d out of tensor rank 2", c.ContractedTensorSlot)
	}
	return validateRefGrammars(map[string]artifact.ArtifactRef{
		"ContractionEvaluation.TensorRef":       c.TensorRef,
		"ContractionEvaluation.VectorRef":       c.VectorRef,
		"ContractionEvaluation.ResultTensorRef": c.ResultTensorRef,
		"ContractionEvaluation.DerivationRef":   c.DerivationRef,
	})
}

// TargetChecks: TensorRef → tensor/1; VectorRef → tensor/1;
// ResultTensorRef → tensor/1; DerivationRef → derivation/1. Resolved only
// by the store-aware check, never by Validate alone.
func (c ContractionEvaluation) TargetChecks() []TargetCheck {
	return []TargetCheck{
		{Field: "ContractionEvaluation.TensorRef", Ref: c.TensorRef, Allowed: []FamilyID{FamilyTensor}},
		{Field: "ContractionEvaluation.VectorRef", Ref: c.VectorRef, Allowed: []FamilyID{FamilyTensor}},
		{Field: "ContractionEvaluation.ResultTensorRef", Ref: c.ResultTensorRef, Allowed: []FamilyID{FamilyTensor}},
		{Field: "ContractionEvaluation.DerivationRef", Ref: c.DerivationRef, Allowed: []FamilyID{FamilyDerivation}},
	}
}

// DecodeContractionEvaluation decodes a physmath/contraction_evaluation/1
// canonical payload in declaration order. Representation only: no store
// Lookup, no replay. Provenance requires CheckContractionEvaluation.
func DecodeContractionEvaluation(payload []byte) (ContractionEvaluation, error) {
	d := artifact.NewDecoder(payload)
	tensor, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return ContractionEvaluation{}, fmt.Errorf("physmath: evaluation missing tensor: %w", err)
	}
	vector, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return ContractionEvaluation{}, fmt.Errorf("physmath: evaluation missing vector: %w", err)
	}
	slot, err := d.UInt32()
	if err != nil {
		return ContractionEvaluation{}, fmt.Errorf("physmath: evaluation missing slot: %w", err)
	}
	result, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return ContractionEvaluation{}, fmt.Errorf("physmath: evaluation missing result tensor: %w", err)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return ContractionEvaluation{}, fmt.Errorf("physmath: evaluation missing derivation: %w", err)
	}
	if !d.Exhausted() {
		return ContractionEvaluation{}, fmt.Errorf("physmath: trailing bytes in contraction-evaluation payload")
	}
	c := ContractionEvaluation{
		TensorRef: tensor, VectorRef: vector, ContractedTensorSlot: slot,
		ResultTensorRef: result, DerivationRef: deriv,
	}
	if err := c.Validate(); err != nil {
		return ContractionEvaluation{}, err
	}
	return c, nil
}

// refMultisetEqual reports whether two ref lists agree as multisets.
func refMultisetEqual(a, b []artifact.ArtifactRef) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[artifact.ArtifactRef]int{}
	for _, r := range a {
		counts[r]++
	}
	for _, r := range b {
		counts[r]--
		if counts[r] < 0 {
			return false
		}
	}
	return true
}

// refSetEqual reports whether a SET-encoded unique list matches the unique
// members of an ordered list.
func refSetEqual(set []artifact.ArtifactRef, ordered []artifact.ArtifactRef) bool {
	return refMultisetEqual(set, uniqueRefs(ordered))
}

// CheckContractionEvaluation is the store-aware provenance gate for
// contraction-evaluation records. Validate checks structure only; this
// operation resolves and verifies the actual evidence:
//
//  1. Every TargetChecks entry resolves to its closed allowed family.
//  2. Operands pass the full §3A–§3B gates (rank, slot, variance, label,
//     space, dimension, bound) via resolveContractionOperands.
//  3. The derivation decodes with exactly 2d steps in per-component
//     CANON→SIMPLIFY pairs, ordinals 0..2d-1 in declaration order, empty
//     conditions/parameters, with indexed linkage steps[2i] consuming the
//     i-th premise and steps[2i+1] consuming steps[2i]'s output.
//  4. Step-input premises match recomputed expected premises as a
//     multiset; derivation PremiseRefs match as a set.
//  5. Step outputs match the result tensor's ordered ComponentRefs by
//     full-ref equality; derivation ResultRefs match as a set; the result
//     tensor is rank 1, dimension d, carrying the free-slot index.
//  6. The derivation replays through the existing Replay interface.
//
// Guarantee limit (VolumeDensity lesson): the check binds
// determinant-relevant operand content (component expressions, labels,
// spaces, slot), not exact artifact identity beyond the referenced refs —
// immutable content-addressed artifacts make ref equality the identity.
func CheckContractionEvaluation(store Store, c ContractionEvaluation) error {
	if err := c.Validate(); err != nil {
		return err
	}
	for _, tc := range c.TargetChecks() {
		fam, _, err := store.Lookup(tc.Ref)
		if err != nil {
			return fmt.Errorf("physmath: %s unresolvable: %w", tc.Field, err)
		}
		match := false
		for _, a := range tc.Allowed {
			if fam == a {
				match = true
				break
			}
		}
		if !match {
			return fmt.Errorf("physmath: %s targets %s/%s/%s, not in closed target set",
				tc.Field, fam.Namespace, fam.ArtifactType, fam.SchemaVersion)
		}
	}
	in, err := resolveContractionOperands(store, c.TensorRef, c.VectorRef, c.ContractedTensorSlot)
	if err != nil {
		return err
	}
	_, dpayload, err := store.Lookup(c.DerivationRef)
	if err != nil {
		return fmt.Errorf("physmath: contraction derivation unresolvable: %w", err)
	}
	dd, err := DecodeDerivation(dpayload)
	if err != nil {
		return err
	}
	if len(dd.Steps) != 2*in.dim {
		return fmt.Errorf("physmath: contraction derivation must have exactly %d steps, got %d", 2*in.dim, len(dd.Steps))
	}
	inputs := make([]artifact.ArtifactRef, 0, in.dim)
	outputs := make([]artifact.ArtifactRef, 0, in.dim)
	for i := 0; i < in.dim; i++ {
		s0, s1 := dd.Steps[2*i], dd.Steps[2*i+1]
		if s0.Ordinal != uint32(2*i) || s1.Ordinal != uint32(2*i+1) {
			return fmt.Errorf("physmath: contraction derivation steps must be ordinals 0..%d in order", 2*in.dim-1)
		}
		if s0.ProducerNamespace != NamespacePhysMath || s0.RuleID != RuleExprCanon {
			return fmt.Errorf("physmath: contraction derivation step %d must be (physmath, EXPR-CANON-001)", 2*i)
		}
		if s1.ProducerNamespace != NamespacePhysMath || s1.RuleID != RuleExprSimplify {
			return fmt.Errorf("physmath: contraction derivation step %d must be (physmath, EXPR-SIMPLIFY-001)", 2*i+1)
		}
		if len(s0.ConditionRefs) != 0 || len(s1.ConditionRefs) != 0 {
			return fmt.Errorf("physmath: contraction derivation steps must carry no conditions")
		}
		if len(s0.Parameters) != 0 || len(s1.Parameters) != 0 {
			return fmt.Errorf("physmath: contraction derivation steps must carry no parameters")
		}
		if len(s0.InputRefs) != 1 {
			return fmt.Errorf("physmath: contraction derivation step %d must consume exactly one premise", 2*i)
		}
		inputs = append(inputs, s0.InputRefs[0])
		if len(s1.InputRefs) != 1 || s1.InputRefs[0] != s0.OutputRef {
			return fmt.Errorf("physmath: contraction derivation step %d must consume step %d output", 2*i+1, 2*i)
		}
		outputs = append(outputs, s1.OutputRef)
	}
	if !refSetEqual(dd.ResultRefs, outputs) {
		return fmt.Errorf("physmath: contraction derivation results do not match step outputs")
	}
	// Replay BEFORE recomputation (AGENTS.md §9): availability is enforced
	// on as-stored artifacts; recomputed mints below must not heal missing
	// evidence before this gate rejects it.
	if err := Replay(store, dd, nil); err != nil {
		return fmt.Errorf("physmath: contraction derivation does not replay: %w", err)
	}
	// Recompute ordered expected premises (idempotent content-addressed
	// mints, mirroring diagonalProductRef) and verify step linkage against
	// them. By this point replay has established availability, so these
	// mints reproduce — never repair — the recorded evidence.
	expected := make([]artifact.ArtifactRef, 0, in.dim)
	for i := 0; i < in.dim; i++ {
		pref, err := PutExpression(store, contractionPremise(in, i))
		if err != nil {
			return err
		}
		expected = append(expected, pref)
	}
	if !refSetEqual(dd.PremiseRefs, expected) {
		return fmt.Errorf("physmath: contraction derivation premises do not match recomputed premises")
	}
	if !refMultisetEqual(inputs, expected) {
		return fmt.Errorf("physmath: contraction derivation step premises do not match recomputed premises")
	}
	rfam, rpayload, err := store.Lookup(c.ResultTensorRef)
	if err != nil {
		return fmt.Errorf("physmath: contraction result tensor unresolvable: %w", err)
	}
	if rfam != FamilyTensor {
		return fmt.Errorf("physmath: contraction result is not tensor/1")
	}
	rt, err := DecodeTensor(rpayload)
	if err != nil {
		return fmt.Errorf("physmath: contraction result tensor undecodable: %w", err)
	}
	if rt.Rank != 1 {
		return fmt.Errorf("physmath: contraction result tensor rank %d != 1", rt.Rank)
	}
	if int(rt.Dimension) != in.dim {
		return fmt.Errorf("physmath: contraction result dimension %d != %d", rt.Dimension, in.dim)
	}
	if len(rt.IndexRefs) != 1 || rt.IndexRefs[0] != in.freeIndexRef {
		return fmt.Errorf("physmath: contraction result must carry the free-slot index")
	}
	if len(rt.ComponentRefs) != len(outputs) {
		return fmt.Errorf("physmath: contraction result component count mismatch")
	}
	for i := range outputs {
		if rt.ComponentRefs[i] != outputs[i] {
			return fmt.Errorf("physmath: contraction result component %d does not match step output", i)
		}
	}
	return nil
}
