package physmath

import (
	"bytes"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// ComputeTestRef builds a deterministic ArtifactRef for fixture wiring.
func ComputeTestRef(t *testing.T, ns, typ string) artifact.ArtifactRef {
	t.Helper()
	ref, err := artifact.ComputeRef(ns, "1", typ, []byte("fixture:"+ns+"/"+typ))
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func refFamily(ns, typ string) FamilyID { return FamilyID{ns, typ, "1"} }

func TestRegistryCompleteness(t *testing.T) {
	fams := AllPhysMathFamilies()
	// Plan 1.17 grows the registry 30 -> 31 (riemann_trace_reverse/1 record; no new executable rule).
	if len(fams) != 31 {
		t.Fatalf("registry has %d families, want 31", len(fams))
	}
	seen := map[FamilyID]bool{}
	for _, f := range fams {
		if seen[f] {
			t.Fatalf("duplicate family %+v", f)
		}
		seen[f] = true
		if f.Namespace != NamespacePhysMath || f.SchemaVersion != "1" {
			t.Fatalf("family %+v violates namespace/version pin", f)
		}
	}
}

func TestFunctionFamily(t *testing.T) {
	body := ComputeTestRef(t, "physmath", "expression/1")
	fn := Function{ParameterIDs: []ParameterID{DeriveParameterID(0)}, BodyRef: body}
	if err := fn.Validate(); err != nil {
		t.Fatal(err)
	}
	if fn.Family() != FamilyFunction {
		t.Fatal("wrong family")
	}
	if _, err := fn.Encode(); err != nil {
		t.Fatal(err)
	}
	fams := map[artifact.ArtifactRef]FamilyID{body: FamilyExpression}
	if err := ValidateAgainst(fn.TargetChecks(), fams); err != nil {
		t.Fatal(err)
	}
	wrong := ComputeTestRef(t, "physmath", "relation/1")
	famsWrong := map[artifact.ArtifactRef]FamilyID{wrong: FamilyRelation}
	bad := Function{ParameterIDs: []ParameterID{DeriveParameterID(0)}, BodyRef: wrong}
	if err := ValidateAgainst(bad.TargetChecks(), famsWrong); err == nil {
		t.Fatal("BodyRef → relation/1 accepted")
	}
	// Unresolved reference rejected.
	if err := ValidateAgainst(fn.TargetChecks(), map[artifact.ArtifactRef]FamilyID{}); err == nil {
		t.Fatal("unresolved reference accepted")
	}
}

func TestMapSortClosure(t *testing.T) {
	def := ComputeTestRef(t, "physmath", "definition/1")
	m := Map{DomainSort: "Expression", CodomainSort: "Expression", DefinitionRef: def}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := Map{DomainSort: "GroupAction", CodomainSort: "Expression", DefinitionRef: def}
	if err := bad.Validate(); err == nil {
		t.Fatal("unregistered sort accepted — new sort by string")
	}
}

func TestTensorShape(t *testing.T) {
	idx := ComputeTestRef(t, "physmath", "index/1")
	comp := ComputeTestRef(t, "physmath", "expression/1")
	comps := []artifact.ArtifactRef{comp, comp, comp, comp}
	tn := Tensor{Rank: 1, Dimension: 4,
		IndexRefs:     []artifact.ArtifactRef{idx},
		ComponentRefs: comps}
	if err := tn.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := tn.Validate(); err != nil {
		t.Fatal(err)
	}
	fams := map[artifact.ArtifactRef]FamilyID{idx: FamilyIndex, comp: FamilyExpression}
	if err := ValidateAgainst(tn.TargetChecks(), fams); err != nil {
		t.Fatal(err)
	}
	badRank := Tensor{Rank: 2, Dimension: 4, IndexRefs: []artifact.ArtifactRef{idx}, ComponentRefs: comps}
	if err := badRank.Validate(); err == nil {
		t.Fatal("IndexRefs != Rank accepted")
	}
	badCount := Tensor{Rank: 1, Dimension: 4, IndexRefs: []artifact.ArtifactRef{idx}, ComponentRefs: comps[:3]}
	if err := badCount.Validate(); err == nil {
		t.Fatal("ComponentRefs != Dim^Rank accepted")
	}
}

func TestIndexVarianceAndSpace(t *testing.T) {
	space := ComputeTestRef(t, "physmath", "collection/1")
	in := Index{Label: "mu", Variance: VarianceContravariant, Space: &space}
	if err := in.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Encode(); err != nil {
		t.Fatal(err)
	}
	bad := Index{Label: "mu", Variance: "MIXED"}
	if err := bad.Validate(); err == nil {
		t.Fatal("open variance accepted")
	}
	fams := map[artifact.ArtifactRef]FamilyID{space: FamilyCollection}
	if err := ValidateAgainst(in.TargetChecks(), fams); err != nil {
		t.Fatal(err)
	}
	plain := Index{Label: "mu", Variance: VarianceUnspecified}
	if len(plain.TargetChecks()) != 0 {
		t.Fatal("absent SpaceRef produced checks")
	}
}

func TestCollectionSetSemantics(t *testing.T) {
	a := ComputeTestRef(t, "physmath", "expression/1")
	dup := Collection{ElementRefs: []artifact.ArtifactRef{a, a}}
	if err := dup.Validate(); err == nil {
		t.Fatal("duplicate SET elements accepted")
	}
	if _, err := dup.Encode(); err == nil {
		t.Fatal("duplicate SET encoded without error")
	}
	// Order-independent encoding.
	b, _ := artifact.ComputeRef("physmath", "1", "expression/1", []byte("other"))
	c1 := Collection{ElementRefs: []artifact.ArtifactRef{a, b}}
	c2 := Collection{ElementRefs: []artifact.ArtifactRef{b, a}}
	e1, _ := c1.Encode()
	e2, _ := c2.Encode()
	if !bytes.Equal(e1, e2) {
		t.Fatal("Collection encoding depends on traversal order")
	}
}

func TestOperatorApplicationContract(t *testing.T) {
	arg := ComputeTestRef(t, "physmath", "expression/1")
	decl := ComputeTestRef(t, "physmath", "function/1")
	params := append(BoundParameterRef{decl, DeriveParameterID(0)}.Encode(),
		artifact.EncodeUInt32(2)...)
	op := OperatorApplication{OperatorID: "differentiate", Args: []artifact.ArtifactRef{arg}, Parameters: params}
	if err := op.Validate(); err != nil {
		t.Fatalf("valid differentiate rejected: %v", err)
	}
	// A4 spec change (K1 slice): covariant_derivative is reserved
	// representational vocabulary, never executable. Empty parameters
	// validate; non-empty parameters and unknown IDs fail closed.
	reserved := OperatorApplication{OperatorID: "covariant_derivative", Args: []artifact.ArtifactRef{arg}}
	if err := reserved.Validate(); err != nil {
		t.Fatalf("reserved covariant_derivative rejected: %v", err)
	}
	if reserved.Executable() {
		t.Fatal("covariant_derivative reports executable")
	}
	withParams := OperatorApplication{OperatorID: "covariant_derivative", Args: []artifact.ArtifactRef{arg}, Parameters: params}
	if err := withParams.Validate(); err == nil {
		t.Fatal("covariant_derivative with slice-1 parameters accepted")
	}
	badID := OperatorApplication{OperatorID: "integrate", Args: []artifact.ArtifactRef{arg}}
	if err := badID.Validate(); err == nil {
		t.Fatal("non-vocabulary OperatorID accepted")
	}
	zeroOrder := append(BoundParameterRef{decl, DeriveParameterID(0)}.Encode(), artifact.EncodeUInt32(0)...)
	if err := (OperatorApplication{OperatorID: "differentiate", Args: []artifact.ArtifactRef{arg}, Parameters: zeroOrder}).Validate(); err == nil {
		t.Fatal("Order 0 accepted")
	}
	trailing := append(params, 0x00)
	if err := (OperatorApplication{OperatorID: "differentiate", Args: []artifact.ArtifactRef{arg}, Parameters: trailing}).Validate(); err == nil {
		t.Fatal("trailing parameter bytes accepted")
	}
}

func TestTransformationRuleLock(t *testing.T) {
	out := ComputeTestRef(t, "physmath", "expression/1")
	tr := Transformation{ProducerNamespace: "physmath", RuleID: RuleExprCanon,
		Inputs: []artifact.ArtifactRef{out}, Output: out}
	if err := tr.ValidateRulePair(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []Transformation{
		{ProducerNamespace: "physmath", RuleID: "PROJECT-STRICT-001", Output: out},
		{ProducerNamespace: "phys-gr", RuleID: RuleExprCanon, Output: out},
		{ProducerNamespace: "physmath", RuleID: "EXPR-DISTRIBUTE-001", Output: out},
	} {
		if err := bad.ValidateRulePair(); err == nil {
			t.Fatalf("unknown pair (%q,%q) accepted", bad.ProducerNamespace, bad.RuleID)
		}
	}
	if _, err := tr.Encode(); err != nil {
		t.Fatal(err)
	}
	step := DerivationStep{Ordinal: 0, ProducerNamespace: "physmath", RuleID: "NOPE",
		OutputRef: out}
	if err := step.ValidateRulePair(); err == nil {
		t.Fatal("unknown step pair accepted")
	}
}

func TestEquationSetSequenceUniqueness(t *testing.T) {
	e1 := ComputeTestRef(t, "physmath", "equation/1")
	e2, _ := artifact.ComputeRef("physmath", "1", "equation/1", []byte("second"))
	s := EquationSet{EquationRefs: []artifact.ArtifactRef{e1, e2}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	enc := s.Encode()
	rev := EquationSet{EquationRefs: []artifact.ArtifactRef{e2, e1}}
	encRev := rev.Encode()
	if bytes.Equal(enc, encRev) {
		t.Fatal("EquationSet order not canonical (SEQUENCE flattened)")
	}
	dup := EquationSet{EquationRefs: []artifact.ArtifactRef{e1, e1}}
	if err := dup.Validate(); err == nil {
		t.Fatal("duplicate EquationSet refs accepted")
	}
}

func TestDefinitionAndConditionTargets(t *testing.T) {
	fnRef := ComputeTestRef(t, "physmath", "function/1")
	exprRef := ComputeTestRef(t, "physmath", "expression/1")
	relRef := ComputeTestRef(t, "physmath", "relation/1")
	def := Definition{DefinedRef: fnRef, DefiningRef: exprRef}
	fams := map[artifact.ArtifactRef]FamilyID{fnRef: FamilyFunction, exprRef: FamilyExpression}
	if err := ValidateAgainst(def.TargetChecks(), fams); err != nil {
		t.Fatal(err)
	}
	badDef := Definition{DefinedRef: relRef, DefiningRef: exprRef}
	famsBad := map[artifact.ArtifactRef]FamilyID{relRef: FamilyRelation, exprRef: FamilyExpression}
	if err := ValidateAgainst(badDef.TargetChecks(), famsBad); err == nil {
		t.Fatal("DefinedRef → relation/1 accepted")
	}
	cond := MathCondition{PredicateRef: relRef}
	if err := ValidateAgainst(cond.TargetChecks(), famsBad); err != nil {
		t.Fatal(err)
	}
}

func TestDerivationStructure(t *testing.T) {
	expr := ComputeTestRef(t, "physmath", "expression/1")
	out, _ := artifact.ComputeRef("physmath", "1", "expression/1", []byte("out"))
	step := DerivationStep{Ordinal: 0, ProducerNamespace: "physmath", RuleID: RuleExprCanon,
		InputRefs: []artifact.ArtifactRef{expr}, OutputRef: out}
	d := Derivation{PremiseRefs: []artifact.ArtifactRef{expr}, Steps: []DerivationStep{step},
		ResultRefs: []artifact.ArtifactRef{out}}
	if err := d.ValidateStructure(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Encode(); err != nil {
		t.Fatal(err)
	}
	gap := Derivation{Steps: []DerivationStep{step,
		{Ordinal: 2, ProducerNamespace: "physmath", RuleID: RuleExprCanon, OutputRef: out}}}
	if err := gap.ValidateStructure(); err == nil {
		t.Fatal("non-contiguous ordinals accepted")
	}
	dup := Derivation{Steps: []DerivationStep{step, step}}
	if err := dup.ValidateStructure(); err == nil {
		t.Fatal("duplicate ordinals accepted")
	}
	dupPre := Derivation{PremiseRefs: []artifact.ArtifactRef{expr, expr}}
	if err := dupPre.ValidateStructure(); err == nil {
		t.Fatal("duplicate PremiseRefs accepted")
	}
}

func TestTheoremTargets(t *testing.T) {
	rel := ComputeTestRef(t, "physmath", "relation/1")
	drv, _ := artifact.ComputeRef("physmath", "1", "derivation/1", []byte("d"))
	cond, _ := artifact.ComputeRef("physmath", "1", "condition/1", []byte("c"))
	th := Theorem{ConclusionRef: rel, AssumptionRefs: []artifact.ArtifactRef{cond}, DerivationRef: drv}
	fams := map[artifact.ArtifactRef]FamilyID{rel: FamilyRelation, drv: FamilyDerivation, cond: FamilyCondition}
	if err := ValidateAgainst(th.TargetChecks(), fams); err != nil {
		t.Fatal(err)
	}
	if _, err := th.Encode(); err != nil {
		t.Fatal(err)
	}
	bad := Theorem{ConclusionRef: cond, DerivationRef: drv}
	if err := ValidateAgainst(bad.TargetChecks(), fams); err == nil {
		t.Fatal("ConclusionRef → condition/1 accepted")
	}
}
