package physmath

import (
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

func badRef() artifact.ArtifactRef {
	return artifact.ArtifactRef{Namespace: "Bad Name", SchemaVersion: "1", ArtifactType: "expression/1"}
}

func goodRef(t *testing.T, fam FamilyID) artifact.ArtifactRef {
	t.Helper()
	ref, err := artifact.ComputeRef(fam.Namespace, fam.SchemaVersion, fam.ArtifactType, []byte("v"))
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// H-2: every durable Validate gate accepts valid values and rejects malformed
// references without needing resolution.
func TestDurableValidators(t *testing.T) {
	expr := goodRef(t, FamilyExpression)
	rel := goodRef(t, FamilyRelation)
	drv := goodRef(t, FamilyDerivation)
	cond := goodRef(t, FamilyCondition)
	fn := goodRef(t, FamilyFunction)

	if err := (Equation{RelationRef: rel}).Validate(); err != nil {
		t.Fatalf("Equation: %v", err)
	}
	if err := (Equation{RelationRef: badRef()}).Validate(); err == nil {
		t.Fatal("Equation with malformed ref accepted")
	}
	if err := (MathCondition{PredicateRef: rel}).Validate(); err != nil {
		t.Fatalf("MathCondition: %v", err)
	}
	if err := (MathCondition{PredicateRef: expr}).Validate(); err != nil {
		// Grammar-valid: family targeting is checked via TargetChecks.
		t.Fatalf("MathCondition grammar gate over-rejected: %v", err)
	}
	if err := (FunctionApplication{FunctionRef: fn, Args: []artifact.ArtifactRef{expr}}).Validate(); err != nil {
		t.Fatalf("FunctionApplication: %v", err)
	}
	if err := (FunctionApplication{FunctionRef: badRef()}).Validate(); err == nil {
		t.Fatal("FunctionApplication with malformed ref accepted")
	}
	th := Theorem{ConclusionRef: rel, DerivationRef: drv,
		PremiseRefs: []artifact.ArtifactRef{expr}, AssumptionRefs: []artifact.ArtifactRef{cond}}
	if err := th.Validate(); err != nil {
		t.Fatalf("Theorem: %v", err)
	}
	dupPremise := Theorem{ConclusionRef: rel, DerivationRef: drv,
		PremiseRefs: []artifact.ArtifactRef{expr, expr}}
	if err := dupPremise.Validate(); err == nil {
		t.Fatal("Theorem duplicate premises accepted")
	}
	badTh := Theorem{ConclusionRef: badRef(), DerivationRef: drv}
	if err := badTh.Validate(); err == nil {
		t.Fatal("Theorem with malformed ConclusionRef accepted")
	}
	// ConclusionRef → expression/1 is structurally encodable but must fail
	// the family gate (spec §13.1: conclusion is relation/1).
	fams := map[artifact.ArtifactRef]FamilyID{expr: FamilyExpression, drv: FamilyDerivation}
	wrongFam := Theorem{ConclusionRef: expr, DerivationRef: drv}
	if err := ValidateAgainst(wrongFam.TargetChecks(), fams); err == nil {
		t.Fatal("Theorem conclusion → expression/1 passed family gate")
	}
	tr := Transformation{ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
		Inputs: []artifact.ArtifactRef{expr}, Output: expr}
	if err := tr.Validate(); err != nil {
		t.Fatalf("Transformation: %v", err)
	}
	badRule := Transformation{ProducerNamespace: NamespacePhysMath, RuleID: "NOPE", Output: expr}
	if err := badRule.Validate(); err == nil {
		t.Fatal("Transformation unknown rule accepted")
	}
}

// M-1: duplicate step ConditionRefs rejected structurally; reordered unique
// conditions behave identically.
func TestConditionRefsDuplicates(t *testing.T) {
	expr := goodRef(t, FamilyExpression)
	condA := goodRef(t, FamilyCondition)
	condB, _ := artifact.ComputeRef("physmath", "1", "condition/1", []byte("b"))
	mkStep := func(conds []artifact.ArtifactRef) Derivation {
		return Derivation{
			PremiseRefs: []artifact.ArtifactRef{expr},
			Steps: []DerivationStep{{Ordinal: 0, ProducerNamespace: NamespacePhysMath,
				RuleID: RuleExprCanon, InputRefs: []artifact.ArtifactRef{expr},
				OutputRef: expr, ConditionRefs: conds}},
			ResultRefs: []artifact.ArtifactRef{expr},
		}
	}
	if err := mkStep([]artifact.ArtifactRef{condA, condB}).ValidateStructure(); err != nil {
		t.Fatalf("unique conditions rejected: %v", err)
	}
	if err := mkStep([]artifact.ArtifactRef{condB, condA}).ValidateStructure(); err != nil {
		t.Fatalf("reordered conditions rejected: %v", err)
	}
	if err := mkStep([]artifact.ArtifactRef{condA, condA}).ValidateStructure(); err == nil {
		t.Fatal("duplicate ConditionRefs accepted structurally")
	}
}
