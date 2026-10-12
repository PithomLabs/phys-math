package physmath

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/PithomLabs/phys-artifact"
)

// NamespacePhysMath is the durable PhysMath namespace. It is intentionally
// unchanged by the Go module rename (plan9.3 D12): it is baked into the
// ParameterID formula and every family identifier.
const NamespacePhysMath = "physmath"

// FamilyID identifies a durable artifact family:
// Namespace / ArtifactType / SchemaVersion (spec §5.8).
type FamilyID struct {
	Namespace     string
	ArtifactType  string
	SchemaVersion string
}

// Family constants for the 23 Phase-1 PhysMath durable families: the 16
// original (spec §5.6) plus SymmetricBilinear, Contraction, Domain, Integral (K1
// foundation slice A1-A3, representation + structural validation only).
var (
	FamilyExpression          = FamilyID{NamespacePhysMath, "expression/1", "1"}
	FamilyFunction            = FamilyID{NamespacePhysMath, "function/1", "1"}
	FamilyFunctionApplication = FamilyID{NamespacePhysMath, "function_application/1", "1"}
	FamilyMap                 = FamilyID{NamespacePhysMath, "map/1", "1"}
	FamilyTensor              = FamilyID{NamespacePhysMath, "tensor/1", "1"}
	FamilyIndex               = FamilyID{NamespacePhysMath, "index/1", "1"}
	FamilyCollection          = FamilyID{NamespacePhysMath, "collection/1", "1"}
	FamilyOperatorApplication = FamilyID{NamespacePhysMath, "operator_application/1", "1"}
	FamilyTransformation      = FamilyID{NamespacePhysMath, "transformation/1", "1"}
	FamilyRelation            = FamilyID{NamespacePhysMath, "relation/1", "1"}
	FamilyEquation            = FamilyID{NamespacePhysMath, "equation/1", "1"}
	FamilyEquationSet         = FamilyID{NamespacePhysMath, "equation_set/1", "1"}
	FamilyDefinition          = FamilyID{NamespacePhysMath, "definition/1", "1"}
	FamilyCondition           = FamilyID{NamespacePhysMath, "condition/1", "1"}
	FamilyDerivation          = FamilyID{NamespacePhysMath, "derivation/1", "1"}
	FamilyTheorem             = FamilyID{NamespacePhysMath, "theorem/1", "1"}
)

// AllPhysMathFamilies returns the complete Phase-1 PhysMath family registry.
func AllPhysMathFamilies() []FamilyID {
	return []FamilyID{
		FamilyExpression, FamilyFunction, FamilyFunctionApplication, FamilyMap,
		FamilyTensor, FamilyIndex, FamilyCollection, FamilyOperatorApplication,
		FamilyTransformation, FamilyRelation, FamilyEquation, FamilyEquationSet,
		FamilyDefinition, FamilyCondition, FamilyDerivation, FamilyTheorem,
		FamilySymmetricBilinear, FamilyContraction, FamilyDomain, FamilyIntegral,
		FamilyVolumeDensity, FamilyContractionEvaluation, FamilyVariationReduction,
	}
}

// Phase-1 executable rule identities (spec §§11.5, 17; plan9.3 D29).
const (
	RuleExprCanon       = "EXPR-CANON-001"
	RuleExprSimplify    = "EXPR-SIMPLIFY-001"
	RuleExprSubstitute  = "EXPR-SUBSTITUTE-001"
	RuleRatNormalize    = "RAT-NORMALIZE-001"
	RuleDifferentiate   = "DIFFERENTIATE-001"
	RuleTotalDerivative = "TOTAL-DERIVATIVE-001"
	RuleExprCombine     = "EXPR-COMBINE-001"
	RuleSelectBranch    = "SELECT-BRANCH-001"
	RuleExprApply       = "EXPR-APPLY-001"
)

// Phase1Rules is the exhaustive executable rule vocabulary.
var Phase1Rules = []string{
	RuleExprCanon, RuleExprSimplify, RuleExprSubstitute, RuleRatNormalize,
	RuleDifferentiate, RuleSelectBranch, RuleExprApply, RuleTotalDerivative,
	RuleExprCombine,
}

// ValidPhase1RulePair reports whether (producer, rule) is executable in Phase 1:
// exactly (physmath, one of the nine closed RuleIDs). Unknown pairs fail closed.
func ValidPhase1RulePair(producer, rule string) bool {
	if producer != NamespacePhysMath {
		return false
	}
	for _, r := range Phase1Rules {
		if r == rule {
			return true
		}
	}
	return false
}

// Closed Phase-1 sort identifiers for Map domain/codomain (spec §§6.1, 10.3).
var phase1Sorts = map[string]bool{
	"Expression": true, "Function": true, "FunctionApplication": true,
	"Functional": true, "FunctionalApplication": true, "Operator": true,
	"OperatorApplication": true, "Tensor": true, "Index": true, "Map": true,
	"Relation": true, "Collection": true, "Transformation": true,
	"Limit": true, "AsymptoticTruncation": true, "DerivationObligation": true,
}

// ValidPhase1Sort reports whether s is a registered Phase-1 sort identifier.
// Introducing a new sort via an unknown string is forbidden.
func ValidPhase1Sort(s string) bool { return phase1Sorts[s] }

// TargetCheck is one ArtifactRef-typed field with its closed allowed families
// (spec §5.7). Validation is exact: the resolved family must equal one entry.
type TargetCheck struct {
	Field   string
	Ref     artifact.ArtifactRef
	Allowed []FamilyID
}

// ValidateAgainst resolves every check against families (ref → its family).
// A field not present in families is unresolved and rejected.
func ValidateAgainst(checks []TargetCheck, families map[artifact.ArtifactRef]FamilyID) error {
	for _, c := range checks {
		got, ok := families[c.Ref]
		if !ok {
			return fmt.Errorf("physmath: %s references unresolved artifact", c.Field)
		}
		match := false
		for _, a := range c.Allowed {
			if got == a {
				match = true
				break
			}
		}
		if !match {
			return fmt.Errorf("physmath: %s targets %s/%s/%s, not in closed target set",
				c.Field, got.Namespace, got.ArtifactType, got.SchemaVersion)
		}
	}
	return nil
}

func encodeRefSequence(refs []artifact.ArtifactRef) []byte {
	elems := make([][]byte, len(refs))
	for i, r := range refs {
		elems[i] = r.Encode()
	}
	return artifact.EncodeSequence(elems)
}

func encodeRefSet(refs []artifact.ArtifactRef) ([]byte, error) {
	elems := make([][]byte, len(refs))
	for i, r := range refs {
		elems[i] = r.Encode()
	}
	return artifact.EncodeSet(elems)
}

func checkDuplicateRefs(name string, refs []artifact.ArtifactRef) error {
	seen := make(map[artifact.ArtifactRef]bool, len(refs))
	for _, r := range refs {
		if seen[r] {
			return fmt.Errorf("physmath: duplicate reference in %s", name)
		}
		seen[r] = true
	}
	return nil
}

// validateRefGrammar enforces namespace/type/version grammar on an embedded
// reference without resolving it. Digest bytes are opaque by construction.
func validateRefGrammar(field string, r artifact.ArtifactRef) error {
	if err := artifact.ValidateNamespace(r.Namespace); err != nil {
		return fmt.Errorf("physmath: %s: %w", field, err)
	}
	if err := artifact.ValidateSchemaVersion(r.SchemaVersion); err != nil {
		return fmt.Errorf("physmath: %s: %w", field, err)
	}
	if err := artifact.ValidateArtifactType(r.ArtifactType); err != nil {
		return fmt.Errorf("physmath: %s: %w", field, err)
	}
	return nil
}

// validateRefGrammars applies validateRefGrammar to every listed field.
func validateRefGrammars(fields map[string]artifact.ArtifactRef) error {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := validateRefGrammar(k, fields[k]); err != nil {
			return err
		}
	}
	return nil
}

// Function declares parameters and references its body expression
// (spec §10.1). Family physmath/function/1.
type Function struct {
	ParameterIDs []ParameterID // SEQUENCE, order is semantic
	BodyRef      artifact.ArtifactRef
}

// Family returns FamilyFunction.
func (Function) Family() FamilyID { return FamilyFunction }

// Encode returns the canonical payload: Sequence(ParameterIDs) || BodyRef.
func (f Function) Encode() ([]byte, error) {
	elems := make([][]byte, len(f.ParameterIDs))
	for i, id := range f.ParameterIDs {
		raw := id
		elems[i] = raw[:]
	}
	return append(artifact.EncodeSequence(elems), f.BodyRef.Encode()...), nil
}

// Validate checks ordinal consistency (§8.3) and the BodyRef target family.
func (f Function) Validate() error {
	return ValidateFunctionParameterIDs(f.ParameterIDs)
}

// TargetChecks for Function.BodyRef → physmath/expression/1.
func (f Function) TargetChecks() []TargetCheck {
	return []TargetCheck{{"Function.BodyRef", f.BodyRef, []FamilyID{FamilyExpression}}}
}

// FunctionApplication applies a Function to expression arguments
// (spec §10.2). Family physmath/function_application/1.
type FunctionApplication struct {
	FunctionRef artifact.ArtifactRef
	Args        []artifact.ArtifactRef // SEQUENCE
}

// Family returns FamilyFunctionApplication.
func (FunctionApplication) Family() FamilyID { return FamilyFunctionApplication }

// Encode returns the canonical payload.
func (a FunctionApplication) Encode() []byte {
	out := a.FunctionRef.Encode()
	return append(out, encodeRefSequence(a.Args)...)
}

// Validate enforces reference grammars. All remaining constraints are
// reference targets, checked via TargetChecks with a families map.
func (a FunctionApplication) Validate() error {
	fields := map[string]artifact.ArtifactRef{"FunctionApplication.FunctionRef": a.FunctionRef}
	for i, arg := range a.Args {
		fields[fmt.Sprintf("FunctionApplication.Args[%d]", i)] = arg
	}
	return validateRefGrammars(fields)
}

// TargetChecks: FunctionRef → function/1; Args → expression/1.
func (a FunctionApplication) TargetChecks() []TargetCheck {
	checks := []TargetCheck{{"FunctionApplication.FunctionRef", a.FunctionRef, []FamilyID{FamilyFunction}}}
	for i, arg := range a.Args {
		checks = append(checks, TargetCheck{
			Field:   fmt.Sprintf("FunctionApplication.Args[%d]", i),
			Ref:     arg,
			Allowed: []FamilyID{FamilyExpression},
		})
	}
	return checks
}

// Map binds a definition to domain/codomain sorts (spec §10.3).
// Family physmath/map/1.
type Map struct {
	DomainSort    string
	CodomainSort  string
	DefinitionRef artifact.ArtifactRef
}

// Family returns FamilyMap.
func (Map) Family() FamilyID { return FamilyMap }

// Encode returns the canonical payload.
func (m Map) Encode() ([]byte, error) {
	d, err := artifact.EncodeString(m.DomainSort)
	if err != nil {
		return nil, err
	}
	c, err := artifact.EncodeString(m.CodomainSort)
	if err != nil {
		return nil, err
	}
	out := append(d, c...)
	return append(out, m.DefinitionRef.Encode()...), nil
}

// Validate requires registered sort identifiers (§10.3: no new sorts by string).
func (m Map) Validate() error {
	if !ValidPhase1Sort(m.DomainSort) {
		return fmt.Errorf("physmath: unregistered Map DomainSort %q", m.DomainSort)
	}
	if !ValidPhase1Sort(m.CodomainSort) {
		return fmt.Errorf("physmath: unregistered Map CodomainSort %q", m.CodomainSort)
	}
	return nil
}

// TargetChecks: DefinitionRef → physmath/definition/1.
func (m Map) TargetChecks() []TargetCheck {
	return []TargetCheck{{"Map.DefinitionRef", m.DefinitionRef, []FamilyID{FamilyDefinition}}}
}

// IndexVariance is the closed v1 variance (spec §10.5). Variance is mathematical
// structure; it chooses no physical metric or sign convention.
type IndexVariance string

const (
	VarianceCovariant     IndexVariance = "COVARIANT"
	VarianceContravariant IndexVariance = "CONTRAVARIANT"
	VarianceUnspecified   IndexVariance = "UNSPECIFIED"
)

// ValidIndexVariance reports membership in the closed vocabulary.
func ValidIndexVariance(v IndexVariance) bool {
	switch v {
	case VarianceCovariant, VarianceContravariant, VarianceUnspecified:
		return true
	}
	return false
}

// Index is one tensor slot (spec §10.5). Family physmath/index/1.
type Index struct {
	Label    string
	Variance IndexVariance
	Space    *artifact.ArtifactRef // optional; when present → collection/1
}

// Family returns FamilyIndex.
func (Index) Family() FamilyID { return FamilyIndex }

// Encode returns the canonical payload.
func (in Index) Encode() ([]byte, error) {
	l, err := artifact.EncodeString(in.Label)
	if err != nil {
		return nil, err
	}
	v, err := artifact.EncodeEnum(string(in.Variance))
	if err != nil {
		return nil, err
	}
	out := append(l, v...)
	if in.Space == nil {
		return append(out, artifact.EncodeOptional(false, nil)...), nil
	}
	return append(out, artifact.EncodeOptional(true, in.Space.Encode())...), nil
}

// Validate checks the closed variance vocabulary.
func (in Index) Validate() error {
	if !ValidIndexVariance(in.Variance) {
		return fmt.Errorf("physmath: invalid Index variance %q", in.Variance)
	}
	return nil
}

// TargetChecks: SpaceRef → collection/1 when present.
func (in Index) TargetChecks() []TargetCheck {
	if in.Space == nil {
		return nil
	}
	return []TargetCheck{{"Index.SpaceRef", *in.Space, []FamilyID{FamilyCollection}}}
}

// Collection is a SET of expression references (spec §10.6).
// Family physmath/collection/1.
type Collection struct {
	ElementRefs []artifact.ArtifactRef // SET
}

// Family returns FamilyCollection.
func (Collection) Family() FamilyID { return FamilyCollection }

// Encode returns the canonical payload (SET: sorted, deduplicated).
func (c Collection) Encode() ([]byte, error) {
	return encodeRefSet(c.ElementRefs)
}

// Validate rejects duplicate canonical elements.
func (c Collection) Validate() error {
	return checkDuplicateRefs("Collection.ElementRefs", c.ElementRefs)
}

// TargetChecks: elements → expression/1.
func (c Collection) TargetChecks() []TargetCheck {
	checks := make([]TargetCheck, len(c.ElementRefs))
	for i, e := range c.ElementRefs {
		checks[i] = TargetCheck{
			Field:   fmt.Sprintf("Collection.ElementRefs[%d]", i),
			Ref:     e,
			Allowed: []FamilyID{FamilyExpression},
		}
	}
	return checks
}

// Tensor is a dense tensor in canonical row-major order (spec §10.4).
// Family physmath/tensor/1. General sparse semantics are deferred.
type Tensor struct {
	Rank          uint16
	Dimension     uint16
	IndexRefs     []artifact.ArtifactRef // SEQUENCE, slot order semantic
	ComponentRefs []artifact.ArtifactRef // SEQUENCE, canonical row-major order
}

// Family returns FamilyTensor.
func (Tensor) Family() FamilyID { return FamilyTensor }

// Encode returns the canonical payload.
func (t Tensor) Encode() []byte {
	out := artifact.EncodeUInt16(t.Rank)
	out = append(out, artifact.EncodeUInt16(t.Dimension)...)
	out = append(out, encodeRefSequence(t.IndexRefs)...)
	return append(out, encodeRefSequence(t.ComponentRefs)...)
}

// Validate enforces the dense-tensor shape: len(IndexRefs) = Rank,
// len(ComponentRefs) = Dimension^Rank (with overflow guard).
func (t Tensor) Validate() error {
	if uint64(len(t.IndexRefs)) != uint64(t.Rank) {
		return fmt.Errorf("physmath: Tensor IndexRefs %d != Rank %d", len(t.IndexRefs), t.Rank)
	}
	want := uint64(1)
	for i := uint64(0); i < uint64(t.Rank); i++ {
		want *= uint64(t.Dimension)
		if want > 1<<24 {
			return fmt.Errorf("physmath: Tensor shape exceeds Phase-1 bound")
		}
	}
	if uint64(len(t.ComponentRefs)) != want {
		return fmt.Errorf("physmath: Tensor ComponentRefs %d != Dimension^Rank %d",
			len(t.ComponentRefs), want)
	}
	return nil
}

// TargetChecks: IndexRefs → index/1; ComponentRefs → expression/1.
func (t Tensor) TargetChecks() []TargetCheck {
	var checks []TargetCheck
	for i, r := range t.IndexRefs {
		checks = append(checks, TargetCheck{
			Field:   fmt.Sprintf("Tensor.IndexRefs[%d]", i),
			Ref:     r,
			Allowed: []FamilyID{FamilyIndex},
		})
	}
	for i, r := range t.ComponentRefs {
		checks = append(checks, TargetCheck{
			Field:   fmt.Sprintf("Tensor.ComponentRefs[%d]", i),
			Ref:     r,
			Allowed: []FamilyID{FamilyExpression},
		})
	}
	return checks
}

// OperatorReservedID is the Phase-1 reserved mathematical operator identity.
const OperatorReservedID = "differentiate"

// OperatorApplication applies a mathematical operator (spec §10.7).
// Family physmath/operator_application/1.
//
// Phase-1: OperatorRef MUST be absent (non-absent is invalid); the executable
// vocabulary is exactly {differentiate}. For differentiate, Parameters =
// Encode(BoundParameterRef) || UInt32BE(Order), Order >= 1.
type OperatorApplication struct {
	OperatorID string
	Args       []artifact.ArtifactRef // SEQUENCE
	Parameters []byte
}

// Family returns FamilyOperatorApplication.
func (OperatorApplication) Family() FamilyID { return FamilyOperatorApplication }

// Encode returns the canonical payload (OperatorRef absent → 0x00).
func (o OperatorApplication) Encode() ([]byte, error) {
	id, err := artifact.EncodeString(o.OperatorID)
	if err != nil {
		return nil, err
	}
	out := append(id, artifact.EncodeOptional(false, nil)...)
	out = append(out, encodeRefSequence(o.Args)...)
	return append(out, artifact.EncodeBytes(o.Parameters)...), nil
}

// Validate enforces the Phase-1 operator contract.
func (o OperatorApplication) Validate() error {
	if !ValidOperatorID(o.OperatorID) {
		return fmt.Errorf("physmath: OperatorID %q not in reserved vocabulary", o.OperatorID)
	}
	if o.OperatorID == OperatorCovariantDerivative {
		// Slice 1: covariant_derivative carries no parameters; the target
		// arrives via Args (expression/1 or tensor/1 per TargetChecks).
		if len(o.Parameters) != 0 {
			return fmt.Errorf("physmath: covariant_derivative takes no parameters in slice 1")
		}
		for i, a := range o.Args {
			if err := validateRefGrammar(fmt.Sprintf("OperatorApplication.Args[%d]", i), a); err != nil {
				return err
			}
		}
		return nil
	}
	d := artifact.NewDecoder(o.Parameters)
	if _, err := artifact.DecodeArtifactRef(d); err != nil {
		return fmt.Errorf("physmath: differentiate Parameters missing BoundParameterRef: %w", err)
	}
	var pid [32]byte
	raw, err := d.Digest32()
	if err != nil {
		return fmt.Errorf("physmath: differentiate Parameters missing ParameterID: %w", err)
	}
	copy(pid[:], raw[:])
	order, err := d.UInt32()
	if err != nil {
		return fmt.Errorf("physmath: differentiate Parameters missing Order: %w", err)
	}
	if order < 1 {
		return fmt.Errorf("physmath: differentiate Order must be >= 1")
	}
	if !d.Exhausted() {
		return fmt.Errorf("physmath: trailing bytes in differentiate Parameters")
	}
	for i, a := range o.Args {
		if err := validateRefGrammar(fmt.Sprintf("OperatorApplication.Args[%d]", i), a); err != nil {
			return err
		}
	}
	return nil
}

// TargetChecks: differentiate Args → expression/1; covariant_derivative
// Args → expression/1 or tensor/1 (tensor target, never executed).
// (OperatorRef absent: no check.)
func (o OperatorApplication) TargetChecks() []TargetCheck {
	allowed := []FamilyID{FamilyExpression}
	if o.OperatorID == OperatorCovariantDerivative {
		allowed = []FamilyID{FamilyExpression, FamilyTensor}
	}
	checks := make([]TargetCheck, len(o.Args))
	for i, a := range o.Args {
		checks[i] = TargetCheck{
			Field:   fmt.Sprintf("OperatorApplication.Args[%d]", i),
			Ref:     a,
			Allowed: allowed,
		}
	}
	return checks
}

// Transformation records one isolated, durable, content-addressed mathematical
// operation (spec §10.8). Family physmath/transformation/1. For Phase-1
// PhysMath transformations: ProducerNamespace = "physmath", Output family =
// physmath/expression/1, Inputs per the exact rule contract (§17.2, Phase-3).
type Transformation struct {
	ProducerNamespace string
	RuleID            string
	Inputs            []artifact.ArtifactRef // SEQUENCE
	Output            artifact.ArtifactRef
	Parameters        []byte
}

// Family returns FamilyTransformation.
func (Transformation) Family() FamilyID { return FamilyTransformation }

// Encode returns the canonical payload in declaration order.
func (t Transformation) Encode() ([]byte, error) {
	p, err := artifact.EncodeString(t.ProducerNamespace)
	if err != nil {
		return nil, err
	}
	r, err := artifact.EncodeString(t.RuleID)
	if err != nil {
		return nil, err
	}
	out := append(p, r...)
	out = append(out, encodeRefSequence(t.Inputs)...)
	out = append(out, t.Output.Encode()...)
	return append(out, artifact.EncodeBytes(t.Parameters)...), nil
}

// Validate enforces the rule-pair lock plus reference grammars.
func (t Transformation) Validate() error {
	if err := t.ValidateRulePair(); err != nil {
		return err
	}
	fields := map[string]artifact.ArtifactRef{"Transformation.Output": t.Output}
	for i, r := range t.Inputs {
		fields[fmt.Sprintf("Transformation.Inputs[%d]", i)] = r
	}
	return validateRefGrammars(fields)
}

// ValidateRulePair rejects unknown (ProducerNamespace, RuleID) pairs for
// validation/deserialization (spec §§17.11, 24.5).
func (t Transformation) ValidateRulePair() error {
	if !ValidPhase1RulePair(t.ProducerNamespace, t.RuleID) {
		return fmt.Errorf("physmath: unknown rule pair (%q, %q)", t.ProducerNamespace, t.RuleID)
	}
	return nil
}

// Equation packages a durable relation for reference (spec §11.1).
// Family physmath/equation/1.
type Equation struct {
	RelationRef artifact.ArtifactRef
}

// Family returns FamilyEquation.
func (Equation) Family() FamilyID { return FamilyEquation }

// Encode returns the canonical payload.
func (e Equation) Encode() []byte { return e.RelationRef.Encode() }

// Validate enforces reference grammar. The target family is checked via
// TargetChecks with a families map.
func (e Equation) Validate() error {
	return validateRefGrammar("Equation.RelationRef", e.RelationRef)
}

// TargetChecks: RelationRef → physmath/relation/1.
func (e Equation) TargetChecks() []TargetCheck {
	return []TargetCheck{{"Equation.RelationRef", e.RelationRef, []FamilyID{FamilyRelation}}}
}

// EncodeRelationBody returns the durable physmath/relation/1 payload
// (plan9.3 D26): Enum(operator) || CanonicalExpression(left) ||
// CanonicalExpression(right) — NO outer Expression node tag.
func EncodeRelationBody(op ComparisonOp, left, right Expr) ([]byte, error) {
	if !ValidComparisonOp(op) {
		return nil, fmt.Errorf("physmath: invalid comparison operator %q", op)
	}
	en, err := artifact.EncodeEnum(string(op))
	if err != nil {
		return nil, err
	}
	l, err := left.Encode()
	if err != nil {
		return nil, err
	}
	r, err := right.Encode()
	if err != nil {
		return nil, err
	}
	out := append(en, l...)
	return append(out, r...), nil
}

// EquationSet is a SEQUENCE of distinct equation artifacts (spec §11.2).
// Order is semantic and canonical; duplicates are invalid (deliberate
// uniqueness on a SEQUENCE, never sorted as a SET).
// Family physmath/equation_set/1.
type EquationSet struct {
	EquationRefs []artifact.ArtifactRef // SEQUENCE
}

// Family returns FamilyEquationSet.
func (EquationSet) Family() FamilyID { return FamilyEquationSet }

// Encode returns the canonical payload (declaration order preserved).
func (s EquationSet) Encode() []byte { return encodeRefSequence(s.EquationRefs) }

// Validate rejects duplicate canonical references.
func (s EquationSet) Validate() error {
	return checkDuplicateRefs("EquationSet.EquationRefs", s.EquationRefs)
}

// TargetChecks: EquationRefs → physmath/equation/1.
func (s EquationSet) TargetChecks() []TargetCheck {
	checks := make([]TargetCheck, len(s.EquationRefs))
	for i, r := range s.EquationRefs {
		checks[i] = TargetCheck{
			Field:   fmt.Sprintf("EquationSet.EquationRefs[%d]", i),
			Ref:     r,
			Allowed: []FamilyID{FamilyEquation},
		}
	}
	return checks
}

// Definition binds a defined object to its defining expression (spec §11.3).
// Family physmath/definition/1. A Definition is not itself an executable
// transformation (application only via EXPR-APPLY-001, Phase 3).
type Definition struct {
	DefinedRef  artifact.ArtifactRef
	DefiningRef artifact.ArtifactRef
}

// Family returns FamilyDefinition.
func (Definition) Family() FamilyID { return FamilyDefinition }

// Encode returns the canonical payload.
func (d Definition) Encode() []byte {
	out := d.DefinedRef.Encode()
	return append(out, d.DefiningRef.Encode()...)
}

// TargetChecks: DefinedRef → function/1 OR expression/1; DefiningRef → expression/1.
func (d Definition) TargetChecks() []TargetCheck {
	return []TargetCheck{
		{"Definition.DefinedRef", d.DefinedRef, []FamilyID{FamilyFunction, FamilyExpression}},
		{"Definition.DefiningRef", d.DefiningRef, []FamilyID{FamilyExpression}},
	}
}

// MathCondition wraps a predicate relation (spec §11.4). No physical
// interpretation or trust metadata. Family physmath/condition/1.
type MathCondition struct {
	PredicateRef artifact.ArtifactRef
}

// Family returns FamilyCondition.
func (MathCondition) Family() FamilyID { return FamilyCondition }

// Encode returns the canonical payload.
func (c MathCondition) Encode() []byte { return c.PredicateRef.Encode() }

// Validate enforces reference grammar. The target family is checked via
// TargetChecks with a families map.
func (c MathCondition) Validate() error {
	return validateRefGrammar("MathCondition.PredicateRef", c.PredicateRef)
}

// TargetChecks: PredicateRef → physmath/relation/1.
func (c MathCondition) TargetChecks() []TargetCheck {
	return []TargetCheck{{"MathCondition.PredicateRef", c.PredicateRef, []FamilyID{FamilyRelation}}}
}

// relationExpressionFamilies is the closed input family set for derivation
// premises and steps (spec §§5.7, 12.3, 13.1).
var premiseFamilies = []FamilyID{
	FamilyExpression, FamilyFunction, FamilyRelation,
	FamilyEquation, FamilyDefinition, FamilyCondition,
	// K1 slice context families (validated data; never step outputs):
	FamilyTensor, FamilyIndex, FamilySymmetricBilinear, FamilyContraction,
	FamilyVolumeDensity, FamilyContractionEvaluation, FamilyVariationReduction,
	FamilyDomain, FamilyIntegral,
}

// resultFamilies is the closed result family set (spec §12.3): premises minus
// condition/1.
var resultFamilies = []FamilyID{
	FamilyExpression, FamilyFunction, FamilyRelation,
	FamilyEquation, FamilyDefinition,
	// Carried-only (steps produce expression/1; see checkStepFamilies):
	FamilyTensor, FamilyIndex, FamilySymmetricBilinear, FamilyContraction,
	FamilyVolumeDensity, FamilyContractionEvaluation, FamilyVariationReduction,
	FamilyDomain, FamilyIntegral,
}

// DerivationStep is inline data inside physmath/derivation/1 (spec §12.2). It has
// no independent Phase-1 ArtifactRef. Canonical field order is declaration order.
type DerivationStep struct {
	Ordinal           uint32
	ProducerNamespace string
	RuleID            string
	InputRefs         []artifact.ArtifactRef // SEQUENCE
	OutputRef         artifact.ArtifactRef
	ConditionRefs     []artifact.ArtifactRef // SET
	Parameters        []byte
}

// Encode returns the canonical step bytes in declaration order.
func (s DerivationStep) Encode() ([]byte, error) {
	out := artifact.EncodeUInt32(s.Ordinal)
	p, err := artifact.EncodeString(s.ProducerNamespace)
	if err != nil {
		return nil, err
	}
	out = append(out, p...)
	r, err := artifact.EncodeString(s.RuleID)
	if err != nil {
		return nil, err
	}
	out = append(out, r...)
	out = append(out, encodeRefSequence(s.InputRefs)...)
	out = append(out, s.OutputRef.Encode()...)
	set, err := encodeRefSet(s.ConditionRefs)
	if err != nil {
		return nil, err
	}
	out = append(out, set...)
	return append(out, artifact.EncodeBytes(s.Parameters)...), nil
}

// ValidateRulePair rejects unknown rule pairs for the step.
func (s DerivationStep) ValidateRulePair() error {
	if !ValidPhase1RulePair(s.ProducerNamespace, s.RuleID) {
		return fmt.Errorf("physmath: unknown step rule pair (%q, %q)", s.ProducerNamespace, s.RuleID)
	}
	return nil
}

// Derivation records an ordered transformation trace (spec §12.3).
// Family physmath/derivation/1.
type Derivation struct {
	PremiseRefs []artifact.ArtifactRef // SET
	Steps       []DerivationStep       // SEQUENCE, order preserved
	ResultRefs  []artifact.ArtifactRef // SET
}

// Family returns FamilyDerivation.
func (Derivation) Family() FamilyID { return FamilyDerivation }

// Encode returns the canonical payload.
func (d Derivation) Encode() ([]byte, error) {
	pre, err := encodeRefSet(d.PremiseRefs)
	if err != nil {
		return nil, err
	}
	stepElems := make([][]byte, len(d.Steps))
	for i, s := range d.Steps {
		enc, err := s.Encode()
		if err != nil {
			return nil, err
		}
		stepElems[i] = enc
	}
	out := append(pre, artifact.EncodeSequence(stepElems)...)
	res, err := encodeRefSet(d.ResultRefs)
	if err != nil {
		return nil, err
	}
	return append(out, res...), nil
}

// Validate is the uniform durable gate: structural validity now; same-run
// availability and rule-family contracts are replay concerns.
func (d Derivation) Validate() error {
	return d.ValidateStructure()
}

// ValidateStructure checks set uniqueness and contiguous ordinals from 0.
// Same-run availability and rule-family contracts are Phase-3 replay concerns.
func (d Derivation) ValidateStructure() error {
	if err := checkDuplicateRefs("Derivation.PremiseRefs", d.PremiseRefs); err != nil {
		return err
	}
	if err := checkDuplicateRefs("Derivation.ResultRefs", d.ResultRefs); err != nil {
		return err
	}
	seen := make(map[uint32]bool, len(d.Steps))
	for _, s := range d.Steps {
		if seen[s.Ordinal] {
			return fmt.Errorf("physmath: duplicate step ordinal %d", s.Ordinal)
		}
		seen[s.Ordinal] = true
		if err := s.ValidateRulePair(); err != nil {
			return err
		}
		// SET-typed ConditionRefs reject duplicates structurally, matching
		// the encoder's later SET rejection (M-1: one validation layer).
		if err := checkDuplicateRefs(
			fmt.Sprintf("DerivationStep[%d].ConditionRefs", s.Ordinal), s.ConditionRefs); err != nil {
			return err
		}
	}
	for i := 0; i < len(d.Steps); i++ {
		if !seen[uint32(i)] {
			return fmt.Errorf("physmath: step ordinals not contiguous from 0")
		}
	}
	return nil
}

// decodeDerivationStep decodes one inline DerivationStep from a shared
// cursor in canonical declaration order: UInt32(ordinal) ||
// String(producer) || String(rule) || Sequence(inputs) || output ||
// Set(conditions) || Bytes(parameters). All reference sequences are
// self-delimiting (length-prefixed strings, fixed digests).
func decodeDerivationStep(d *artifact.Decoder) (DerivationStep, error) {
	ord, err := d.UInt32()
	if err != nil {
		return DerivationStep{}, fmt.Errorf("physmath: step missing ordinal: %w", err)
	}
	prod, err := d.String()
	if err != nil {
		return DerivationStep{}, fmt.Errorf("physmath: step missing producer: %w", err)
	}
	rule, err := d.String()
	if err != nil {
		return DerivationStep{}, fmt.Errorf("physmath: step missing rule: %w", err)
	}
	nin, err := d.SequenceCount()
	if err != nil {
		return DerivationStep{}, fmt.Errorf("physmath: step missing inputs: %w", err)
	}
	ins := make([]artifact.ArtifactRef, 0, nin)
	for i := 0; i < nin; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return DerivationStep{}, fmt.Errorf("physmath: step bad input %d: %w", i, err)
		}
		ins = append(ins, r)
	}
	out, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return DerivationStep{}, fmt.Errorf("physmath: step missing output: %w", err)
	}
	nc, err := d.SequenceCount()
	if err != nil {
		return DerivationStep{}, fmt.Errorf("physmath: step missing conditions: %w", err)
	}
	conds := make([]artifact.ArtifactRef, 0, nc)
	for i := 0; i < nc; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return DerivationStep{}, fmt.Errorf("physmath: step bad condition %d: %w", i, err)
		}
		conds = append(conds, r)
	}
	params, err := d.Bytes()
	if err != nil {
		return DerivationStep{}, fmt.Errorf("physmath: step missing parameters: %w", err)
	}
	return DerivationStep{
		Ordinal: ord, ProducerNamespace: prod, RuleID: rule,
		InputRefs: ins, OutputRef: out, ConditionRefs: conds, Parameters: params,
	}, nil
}

// DecodeDerivation decodes a physmath/derivation/1 canonical payload:
// Set(premises) || Sequence(steps, declaration order) || Set(results).
// Steps are decoded inline from the shared cursor in declaration order.
// It is representation only: no store Lookup, no Apply, no Replay. The
// decoded value is validated with ValidateStructure (duplicate refs,
// contiguous ordinals, known rule pairs, duplicate conditions).
func DecodeDerivation(payload []byte) (Derivation, error) {
	d := artifact.NewDecoder(payload)
	np, err := d.SequenceCount()
	if err != nil {
		return Derivation{}, fmt.Errorf("physmath: derivation missing premises: %w", err)
	}
	pres := make([]artifact.ArtifactRef, 0, np)
	for i := 0; i < np; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return Derivation{}, fmt.Errorf("physmath: derivation bad premise %d: %w", i, err)
		}
		pres = append(pres, r)
	}
	ns, err := d.SequenceCount()
	if err != nil {
		return Derivation{}, fmt.Errorf("physmath: derivation missing steps: %w", err)
	}
	steps := make([]DerivationStep, 0, ns)
	for i := 0; i < ns; i++ {
		s, err := decodeDerivationStep(d)
		if err != nil {
			return Derivation{}, err
		}
		steps = append(steps, s)
	}
	nr, err := d.SequenceCount()
	if err != nil {
		return Derivation{}, fmt.Errorf("physmath: derivation missing results: %w", err)
	}
	res := make([]artifact.ArtifactRef, 0, nr)
	for i := 0; i < nr; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return Derivation{}, fmt.Errorf("physmath: derivation bad result %d: %w", i, err)
		}
		res = append(res, r)
	}
	if !d.Exhausted() {
		return Derivation{}, fmt.Errorf("physmath: trailing bytes in derivation payload")
	}
	dd := Derivation{PremiseRefs: pres, Steps: steps, ResultRefs: res}
	if err := dd.ValidateStructure(); err != nil {
		return Derivation{}, err
	}
	return dd, nil
}

// TargetChecks for premises/results against the closed family sets.
// Per-step InputRefs/OutputRef family validation against the selected rule
// contract (§17.2) is Phase-3 replay validation.
func (d Derivation) TargetChecks() []TargetCheck {
	var checks []TargetCheck
	for i, r := range d.PremiseRefs {
		checks = append(checks, TargetCheck{
			Field:   fmt.Sprintf("Derivation.PremiseRefs[%d]", i),
			Ref:     r,
			Allowed: premiseFamilies,
		})
	}
	for i, r := range d.ResultRefs {
		checks = append(checks, TargetCheck{
			Field:   fmt.Sprintf("Derivation.ResultRefs[%d]", i),
			Ref:     r,
			Allowed: resultFamilies,
		})
	}
	for i, s := range d.Steps {
		for j, r := range s.InputRefs {
			checks = append(checks, TargetCheck{
				Field:   fmt.Sprintf("Derivation.Steps[%d].InputRefs[%d]", i, j),
				Ref:     r,
				Allowed: premiseFamilies,
			})
		}
		checks = append(checks, TargetCheck{
			Field:   fmt.Sprintf("Derivation.Steps[%d].OutputRef", i),
			Ref:     s.OutputRef,
			Allowed: []FamilyID{FamilyExpression},
		})
		for j, r := range s.ConditionRefs {
			checks = append(checks, TargetCheck{
				Field:   fmt.Sprintf("Derivation.Steps[%d].ConditionRefs[%d]", i, j),
				Ref:     r,
				Allowed: []FamilyID{FamilyCondition},
			})
		}
	}
	return checks
}

// Theorem is a represented mathematical claim + derivation (spec §13.1).
// DerivationRef is mandatory in Phase 1. Family physmath/theorem/1.
type Theorem struct {
	PremiseRefs    []artifact.ArtifactRef // SET
	ConclusionRef  artifact.ArtifactRef
	AssumptionRefs []artifact.ArtifactRef // SET
	DerivationRef  artifact.ArtifactRef
}

// Family returns FamilyTheorem.
func (Theorem) Family() FamilyID { return FamilyTheorem }

// Encode returns the canonical payload in declaration order.
func (t Theorem) Encode() ([]byte, error) {
	pre, err := encodeRefSet(t.PremiseRefs)
	if err != nil {
		return nil, err
	}
	out := append(pre, t.ConclusionRef.Encode()...)
	asm, err := encodeRefSet(t.AssumptionRefs)
	if err != nil {
		return nil, err
	}
	out = append(out, asm...)
	return append(out, t.DerivationRef.Encode()...), nil
}

// Validate enforces structural validity without resolution: reference
// grammars plus SET uniqueness. Target families are checked via TargetChecks
// with a families map; derivation completeness via replay.
func (t Theorem) Validate() error {
	fields := map[string]artifact.ArtifactRef{
		"Theorem.ConclusionRef": t.ConclusionRef,
		"Theorem.DerivationRef": t.DerivationRef,
	}
	for i, r := range t.PremiseRefs {
		fields[fmt.Sprintf("Theorem.PremiseRefs[%d]", i)] = r
	}
	for i, r := range t.AssumptionRefs {
		fields[fmt.Sprintf("Theorem.AssumptionRefs[%d]", i)] = r
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	if err := checkDuplicateRefs("Theorem.PremiseRefs", t.PremiseRefs); err != nil {
		return err
	}
	return checkDuplicateRefs("Theorem.AssumptionRefs", t.AssumptionRefs)
}

// TargetChecks: premises → six families; ConclusionRef → relation/1;
// AssumptionRefs → condition/1; DerivationRef → derivation/1.
func (t Theorem) TargetChecks() []TargetCheck {
	checks := []TargetCheck{
		{"Theorem.ConclusionRef", t.ConclusionRef, []FamilyID{FamilyRelation}},
		{"Theorem.DerivationRef", t.DerivationRef, []FamilyID{FamilyDerivation}},
	}
	for i, r := range t.PremiseRefs {
		checks = append(checks, TargetCheck{
			Field:   fmt.Sprintf("Theorem.PremiseRefs[%d]", i),
			Ref:     r,
			Allowed: premiseFamilies,
		})
	}
	for i, r := range t.AssumptionRefs {
		checks = append(checks, TargetCheck{
			Field:   fmt.Sprintf("Theorem.AssumptionRefs[%d]", i),
			Ref:     r,
			Allowed: []FamilyID{FamilyCondition},
		})
	}
	return checks
}

// sortedRefBytes is a test-visible helper: canonical SET ordering proof.
func sortedRefBytes(refs []artifact.ArtifactRef) [][]byte {
	elems := make([][]byte, len(refs))
	for i, r := range refs {
		elems[i] = r.Encode()
	}
	sort.Slice(elems, func(i, j int) bool { return bytes.Compare(elems[i], elems[j]) < 0 })
	return elems
}
