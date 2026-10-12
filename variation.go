package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// ReduceVariation implements ansatz-derived symmetry-reduced variation.
// Given a rank-2 ansatz tensor γ (e.g. the spatial shape in
// h_ij(x) = a²γ_ij(x)), a rank-2 variation-density tensor V (e.g. δF/δh
// components as expressions), and the scale expression a, it computes the
// reduced integrand
//
//	S = Σ_ij 2·a·γ_ij·V_ij
//
// through the existing executable path (Mul/Add construction + EXPR-CANON-001
// then EXPR-SIMPLIFY-001 replay). The factor 2, the γ contraction, and the
// sum are all computed from the represented operands — never hard-coded.
// What it does NOT do: spatial integration ∫d³x and fiducial-volume
// normalization remain represented-only assumptions owned by the caller
// (there is no integration rule). The homogeneous scalar counterpart runs
// through the same degenerate path (dimension-1 tensors); its volume
// normalization is likewise caller-owned.
//
// It returns the reduced-integrand expression/1 reference plus the flat
// replayable two-step derivation (premise → CANON → SIMPLIFY), mirroring
// DiagonalDeterminant. No new rule, no registry change for the operation
// itself; the certificate below is a new carried-only family.
func ReduceVariation(store Store, gammaRef, variationRef artifact.ArtifactRef, a Expr) (artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, Derivation{}, err
	}
	gfam, gpayload, err := store.Lookup(gammaRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: variation ansatz unresolvable: %w", err))
	}
	if gfam != FamilyTensor {
		return fail(fmt.Errorf("physmath: variation ansatz is not tensor/1: %w", ErrBadInput))
	}
	vfam, vpayload, err := store.Lookup(variationRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: variation density unresolvable: %w", err))
	}
	if vfam != FamilyTensor {
		return fail(fmt.Errorf("physmath: variation density is not tensor/1: %w", ErrBadInput))
	}
	g, err := DecodeTensor(gpayload)
	if err != nil {
		return fail(fmt.Errorf("physmath: variation ansatz undecodable: %w", err))
	}
	v, err := DecodeTensor(vpayload)
	if err != nil {
		return fail(fmt.Errorf("physmath: variation density undecodable: %w", err))
	}
	if g.Rank != 2 || v.Rank != 2 {
		return fail(fmt.Errorf("physmath: variation operands must be rank 2: %w", ErrBadInput))
	}
	if g.Dimension != v.Dimension {
		return fail(fmt.Errorf("physmath: variation dimension mismatch %d != %d: %w",
			g.Dimension, v.Dimension, ErrBadInput))
	}
	if g.Dimension < 1 {
		return fail(fmt.Errorf("physmath: variation dimension must be >= 1: %w", ErrBadInput))
	}
	if g.Dimension > ContractEvalDimensionBound {
		return fail(fmt.Errorf("physmath: variation dimension %d exceeds bound %d: %w",
			g.Dimension, ContractEvalDimensionBound, ErrBadInput))
	}
	gexprs, err := resolveComponents(store, "variation ansatz", g.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	vexprs, err := resolveComponents(store, "variation density", v.ComponentRefs)
	if err != nil {
		return fail(err)
	}
	two := MustRational(2, 1)
	terms := make([]Expr, 0, len(gexprs))
	for i := range gexprs {
		terms = append(terms, Mul(two, cloneExpr(a), cloneExpr(gexprs[i]), cloneExpr(vexprs[i])))
	}
	premise := Add(terms...)
	prodRef, err := PutExpression(store, premise)
	if err != nil {
		return fail(err)
	}
	canon, err := Apply(store, Invocation{
		ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
		InputRefs: []artifact.ArtifactRef{prodRef},
	})
	if err != nil {
		return fail(fmt.Errorf("physmath: variation canon failed: %w", err))
	}
	res, err := Apply(store, Invocation{
		ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
		InputRefs: []artifact.ArtifactRef{canon.OutputRef},
	})
	if err != nil {
		return fail(fmt.Errorf("physmath: variation simplify failed: %w", err))
	}
	d := Derivation{
		PremiseRefs: []artifact.ArtifactRef{prodRef},
		Steps: []DerivationStep{
			{
				Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
				InputRefs: []artifact.ArtifactRef{prodRef}, OutputRef: canon.OutputRef,
			},
			{
				Ordinal: 1, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
				InputRefs: []artifact.ArtifactRef{canon.OutputRef}, OutputRef: res.OutputRef,
			},
		},
		ResultRefs: []artifact.ArtifactRef{res.OutputRef},
	}
	if err := Replay(store, d, nil); err != nil {
		return fail(fmt.Errorf("physmath: variation derivation does not replay: %w", err))
	}
	return res.OutputRef, d, nil
}

// FamilyVariationReduction is the ansatz-derived variation record: the
// ansatz tensor, the variation-density tensor, the computed reduced
// integrand, and the replayable derivation. Family
// physmath/variation_reduction/1.
var FamilyVariationReduction = FamilyID{NamespacePhysMath, "variation_reduction/1", "1"}

// VariationReduction links ansatz, variation density, reduced integrand,
// and derivation evidence. Validate checks structure only; the store-aware
// CheckVariationReduction resolves operands, replays, and matches the
// recomputed integrand premise and result.
type VariationReduction struct {
	AnsatzRef     artifact.ArtifactRef // → tensor/1 (rank 2, e.g. γ)
	VariationRef  artifact.ArtifactRef // → tensor/1 (rank 2, e.g. δF/δh)
	ReducedRef    artifact.ArtifactRef // → expression/1 (computed integrand S)
	DerivationRef artifact.ArtifactRef // required → derivation/1 (CANON+SIMPLIFY evidence)
}

// Family returns FamilyVariationReduction.
func (VariationReduction) Family() FamilyID { return FamilyVariationReduction }

// Encode returns the canonical payload in declaration order.
func (c VariationReduction) Encode() []byte {
	out := c.AnsatzRef.Encode()
	out = append(out, c.VariationRef.Encode()...)
	out = append(out, c.ReducedRef.Encode()...)
	return append(out, c.DerivationRef.Encode()...)
}

// Validate enforces reference grammar without resolution: ranks,
// dimensions, and the bound belong exclusively to the store-aware check.
func (c VariationReduction) Validate() error {
	return validateRefGrammars(map[string]artifact.ArtifactRef{
		"VariationReduction.AnsatzRef":     c.AnsatzRef,
		"VariationReduction.VariationRef":  c.VariationRef,
		"VariationReduction.ReducedRef":    c.ReducedRef,
		"VariationReduction.DerivationRef": c.DerivationRef,
	})
}

// TargetChecks: AnsatzRef → tensor/1; VariationRef → tensor/1;
// ReducedRef → expression/1; DerivationRef → derivation/1. Resolved only
// by the store-aware check, never by Validate alone.
func (c VariationReduction) TargetChecks() []TargetCheck {
	return []TargetCheck{
		{Field: "VariationReduction.AnsatzRef", Ref: c.AnsatzRef, Allowed: []FamilyID{FamilyTensor}},
		{Field: "VariationReduction.VariationRef", Ref: c.VariationRef, Allowed: []FamilyID{FamilyTensor}},
		{Field: "VariationReduction.ReducedRef", Ref: c.ReducedRef, Allowed: []FamilyID{FamilyExpression}},
		{Field: "VariationReduction.DerivationRef", Ref: c.DerivationRef, Allowed: []FamilyID{FamilyDerivation}},
	}
}

// DecodeVariationReduction decodes a physmath/variation_reduction/1
// canonical payload. Representation only: no Lookup, no replay.
func DecodeVariationReduction(payload []byte) (VariationReduction, error) {
	d := artifact.NewDecoder(payload)
	ansatz, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return VariationReduction{}, fmt.Errorf("physmath: reduction missing ansatz: %w", err)
	}
	variation, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return VariationReduction{}, fmt.Errorf("physmath: reduction missing variation: %w", err)
	}
	reduced, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return VariationReduction{}, fmt.Errorf("physmath: reduction missing reduced integrand: %w", err)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return VariationReduction{}, fmt.Errorf("physmath: reduction missing derivation: %w", err)
	}
	if !d.Exhausted() {
		return VariationReduction{}, fmt.Errorf("physmath: trailing bytes in variation-reduction payload")
	}
	c := VariationReduction{AnsatzRef: ansatz, VariationRef: variation, ReducedRef: reduced, DerivationRef: deriv}
	if err := c.Validate(); err != nil {
		return VariationReduction{}, err
	}
	return c, nil
}

// variationPremise recomputes the expected sum premise from stored
// operands (idempotent content-addressed mint, mirroring
// diagonalProductRef). It executes no rewrite rule.
func variationPremise(store Store, gammaRef, variationRef artifact.ArtifactRef, a Expr) (artifact.ArtifactRef, error) {
	_, gpayload, err := store.Lookup(gammaRef)
	if err != nil {
		return artifact.ArtifactRef{}, fmt.Errorf("physmath: reduction ansatz unresolvable: %w", err)
	}
	_, vpayload, err := store.Lookup(variationRef)
	if err != nil {
		return artifact.ArtifactRef{}, fmt.Errorf("physmath: reduction variation unresolvable: %w", err)
	}
	g, err := DecodeTensor(gpayload)
	if err != nil {
		return artifact.ArtifactRef{}, err
	}
	v, err := DecodeTensor(vpayload)
	if err != nil {
		return artifact.ArtifactRef{}, err
	}
	gexprs, err := resolveComponents(store, "variation ansatz", g.ComponentRefs)
	if err != nil {
		return artifact.ArtifactRef{}, err
	}
	vexprs, err := resolveComponents(store, "variation density", v.ComponentRefs)
	if err != nil {
		return artifact.ArtifactRef{}, err
	}
	two := MustRational(2, 1)
	terms := make([]Expr, 0, len(gexprs))
	for i := range gexprs {
		terms = append(terms, Mul(two, cloneExpr(a), cloneExpr(gexprs[i]), cloneExpr(vexprs[i])))
	}
	return PutExpression(store, Add(terms...))
}

// CheckVariationReduction is the store-aware provenance gate for
// variation-reduction records. Validate checks structure only; this
// operation resolves operand families, re-gates rank/dimension/bound,
// verifies the exact two-step CANON+SIMPLIFY shape with step linkage,
// replays through the existing interface, and matches the recomputed
// premise and the reduced result. Replay precedes recomputation
// (AGENTS.md §9): availability is enforced on as-stored artifacts.
func CheckVariationReduction(store Store, c VariationReduction, a Expr) error {
	if err := c.Validate(); err != nil {
		return err
	}
	for _, tc := range c.TargetChecks() {
		fam, _, err := store.Lookup(tc.Ref)
		if err != nil {
			return fmt.Errorf("physmath: %s unresolvable: %w", tc.Field, err)
		}
		match := false
		for _, allow := range tc.Allowed {
			if fam == allow {
				match = true
				break
			}
		}
		if !match {
			return fmt.Errorf("physmath: %s targets %s/%s/%s, not in closed target set",
				tc.Field, fam.Namespace, fam.ArtifactType, fam.SchemaVersion)
		}
	}
	_, dpayload, err := store.Lookup(c.DerivationRef)
	if err != nil {
		return fmt.Errorf("physmath: reduction derivation unresolvable: %w", err)
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return err
	}
	if len(d.PremiseRefs) != 1 {
		return fmt.Errorf("physmath: reduction derivation must carry exactly one premise")
	}
	if len(d.ResultRefs) != 1 || d.ResultRefs[0] != c.ReducedRef {
		return fmt.Errorf("physmath: reduction derivation result does not match VariationReduction.ReducedRef")
	}
	if len(d.Steps) != 2 {
		return fmt.Errorf("physmath: reduction derivation must have exactly two steps")
	}
	s0, s1 := d.Steps[0], d.Steps[1]
	if s0.Ordinal != 0 || s1.Ordinal != 1 {
		return fmt.Errorf("physmath: reduction derivation steps must be ordinals 0,1 in order")
	}
	if s0.ProducerNamespace != NamespacePhysMath || s0.RuleID != RuleExprCanon {
		return fmt.Errorf("physmath: reduction derivation step 0 must be (physmath, EXPR-CANON-001)")
	}
	if s1.ProducerNamespace != NamespacePhysMath || s1.RuleID != RuleExprSimplify {
		return fmt.Errorf("physmath: reduction derivation step 1 must be (physmath, EXPR-SIMPLIFY-001)")
	}
	if len(s0.ConditionRefs) != 0 || len(s1.ConditionRefs) != 0 {
		return fmt.Errorf("physmath: reduction derivation steps must carry no conditions")
	}
	if len(s0.Parameters) != 0 || len(s1.Parameters) != 0 {
		return fmt.Errorf("physmath: reduction derivation steps must carry no parameters")
	}
	if len(s0.InputRefs) != 1 || s0.InputRefs[0] != d.PremiseRefs[0] {
		return fmt.Errorf("physmath: reduction derivation step 0 must consume the derivation premise")
	}
	if len(s1.InputRefs) != 1 || s1.InputRefs[0] != s0.OutputRef {
		return fmt.Errorf("physmath: reduction derivation step 1 must consume step 0 output")
	}
	if s1.OutputRef != d.ResultRefs[0] {
		return fmt.Errorf("physmath: reduction derivation step 1 must produce the derivation result")
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: reduction derivation does not replay: %w", err)
	}
	expected, err := variationPremise(store, c.AnsatzRef, c.VariationRef, a)
	if err != nil {
		return err
	}
	// Full operand re-gating (rank/dimension/boundfamilies already
	// resolved above; shapes rechecked here against decoded tensors).
	_, gpayload, err := store.Lookup(c.AnsatzRef)
	if err != nil {
		return fmt.Errorf("physmath: reduction ansatz unresolvable: %w", err)
	}
	g, err := DecodeTensor(gpayload)
	if err != nil {
		return err
	}
	_, vpayload, err := store.Lookup(c.VariationRef)
	if err != nil {
		return fmt.Errorf("physmath: reduction variation unresolvable: %w", err)
	}
	v, err := DecodeTensor(vpayload)
	if err != nil {
		return err
	}
	if g.Rank != 2 || v.Rank != 2 {
		return fmt.Errorf("physmath: reduction operands must be rank 2: %w", ErrBadInput)
	}
	if g.Dimension != v.Dimension || g.Dimension < 1 || g.Dimension > ContractEvalDimensionBound {
		return fmt.Errorf("physmath: reduction dimension invalid: %w", ErrBadInput)
	}
	if d.PremiseRefs[0] != expected {
		return fmt.Errorf("physmath: reduction premise does not match recomputed ansatz projection")
	}
	return nil
}
