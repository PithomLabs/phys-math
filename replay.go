package physmath

import (
	"bytes"
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// Replay deterministically re-executes a Derivation (spec §§12.4, 20.4).
// fixtures are explicitly supplied protocol artifacts available alongside
// premises (spec §12.6). For each step, in ordinal order, Replay checks
// same-run availability, validates input/output families against the selected
// rule contract (§17.2, acceptance §25.29), executes the rule, and requires
// the recomputed output reference to equal the declared OutputRef exactly.
// ResultRefs must be produced by a step or carried through from premises.
// Structural validity is representation success, never scientific truth.
func Replay(store Store, d Derivation, fixtures map[artifact.ArtifactRef]bool) error {
	if err := d.ValidateStructure(); err != nil {
		return err
	}
	available := make(map[artifact.ArtifactRef]bool, len(d.PremiseRefs))
	for _, r := range d.PremiseRefs {
		if _, _, err := store.Lookup(r); err != nil {
			return fmt.Errorf("physmath: premise unresolvable: %w", err)
		}
		available[r] = true
	}
	for r := range fixtures {
		if _, _, err := store.Lookup(r); err != nil {
			return fmt.Errorf("physmath: fixture unresolvable: %w", err)
		}
		available[r] = true
	}
	ordered := orderedSteps(d.Steps)
	for _, s := range ordered {
		for _, in := range s.InputRefs {
			if !available[in] {
				return fmt.Errorf("physmath: step %d input not available in same run", s.Ordinal)
			}
		}
		if err := checkStepFamilies(store, s); err != nil {
			return err
		}
		res, err := Apply(store, Invocation{
			ProducerNamespace: s.ProducerNamespace,
			RuleID:            s.RuleID,
			InputRefs:         s.InputRefs,
			ConditionRefs:     s.ConditionRefs,
			Parameters:        s.Parameters,
		})
		if err != nil {
			return fmt.Errorf("physmath: step %d replay failed: %w", s.Ordinal, err)
		}
		if res.OutputRef != s.OutputRef {
			return fmt.Errorf("physmath: step %d output mismatch on replay", s.Ordinal)
		}
		if !equalRefSets(res.ConditionRefs, s.ConditionRefs) {
			return fmt.Errorf("physmath: step %d condition context mismatch on replay", s.Ordinal)
		}
		available[s.OutputRef] = true
	}
	for _, r := range d.ResultRefs {
		if !available[r] {
			return fmt.Errorf("physmath: result not produced or carried through")
		}
	}
	return nil
}

// orderedSteps returns steps sorted by ordinal (encoding preserves declaration
// order; replay follows ordinal order explicitly).
func orderedSteps(steps []DerivationStep) []DerivationStep {
	out := make([]DerivationStep, len(steps))
	copy(out, steps)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Ordinal < out[j-1].Ordinal; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func equalRefSets(a, b []artifact.ArtifactRef) bool {
	na := normalizeConditionRefs(a)
	nb := normalizeConditionRefs(b)
	if len(na) != len(nb) {
		return false
	}
	for i := range na {
		if na[i] != nb[i] {
			return false
		}
	}
	return true
}

// checkStepFamilies validates step input/output families against the exact
// rule contract (§17.2). ConditionRefs must target condition/1.
func checkStepFamilies(store Store, s DerivationStep) error {
	familyOf := func(ref artifact.ArtifactRef) (FamilyID, error) {
		fam, _, err := store.Lookup(ref)
		return fam, err
	}
	exprOnly := func(refs []artifact.ArtifactRef, what string) error {
		for _, r := range refs {
			fam, err := familyOf(r)
			if err != nil {
				return err
			}
			if fam != FamilyExpression {
				return fmt.Errorf("physmath: %s of rule %s must be expression/1", what, s.RuleID)
			}
		}
		return nil
	}
	for _, c := range s.ConditionRefs {
		fam, err := familyOf(c)
		if err != nil {
			return err
		}
		if fam != FamilyCondition {
			return fmt.Errorf("physmath: step ConditionRefs must target condition/1")
		}
	}
	outFam, err := familyOf(s.OutputRef)
	if err != nil {
		return err
	}
	if outFam != FamilyExpression {
		return fmt.Errorf("physmath: rule %s output must be expression/1", s.RuleID)
	}
	switch s.RuleID {
	case RuleExprCanon, RuleExprSimplify, RuleRatNormalize, RuleSelectBranch:
		if len(s.InputRefs) != 1 {
			return fmt.Errorf("physmath: rule %s arity // families: want 1 input", s.RuleID)
		}
		return exprOnly(s.InputRefs, "input")
	case RuleExprApply, RuleDifferentiate:
		if len(s.InputRefs) != 2 {
			return fmt.Errorf("physmath: rule %s arity // families: want 2 inputs", s.RuleID)
		}
		if err := exprOnly(s.InputRefs[:1], "input 0"); err != nil {
			return err
		}
		fam, err := familyOf(s.InputRefs[1])
		if err != nil {
			return err
		}
		if fam != FamilyFunction {
			return fmt.Errorf("physmath: rule %s input 1 must be function/1", s.RuleID)
		}
		return nil
	case RuleExprSubstitute:
		if len(s.InputRefs) < 2 {
			return fmt.Errorf("physmath: rule %s needs 1+D inputs", s.RuleID)
		}
		if err := exprOnly(s.InputRefs[:1], "input 0"); err != nil {
			return err
		}
		for _, r := range s.InputRefs[1:] {
			fam, err := familyOf(r)
			if err != nil {
				return err
			}
			if fam != FamilyFunction {
				return fmt.Errorf("physmath: rule %s declaring inputs must be function/1", s.RuleID)
			}
		}
		return nil
	default:
		return fmt.Errorf("physmath: unknown rule %s in family check: %w", s.RuleID, ErrUnknownRuleID)
	}
}

// CheckTheoremConclusion verifies the theorem-conclusion relation rule
// (spec §13.3, plan9.3 D26): the durable relation embeds exactly the
// canonical bytes of the designated final derivation-result expressions.
// leftRef/rightRef are the first/second designated final results. This is
// construction-time validation, not repository-wide inference.
func CheckTheoremConclusion(store Store, th Theorem, leftRef, rightRef artifact.ArtifactRef) error {
	relFam, relPayload, err := store.Lookup(th.ConclusionRef)
	if err != nil {
		return err
	}
	if relFam != FamilyRelation {
		return fmt.Errorf("physmath: Theorem ConclusionRef is not relation/1")
	}
	left, err := resolveExpression(store, leftRef)
	if err != nil {
		return err
	}
	right, err := resolveExpression(store, rightRef)
	if err != nil {
		return err
	}
	want, err := EncodeRelationBody(OpEQ, left, right)
	if err != nil {
		return err
	}
	if !bytes.Equal(relPayload, want) {
		return fmt.Errorf("physmath: conclusion relation does not embed the designated final bytes")
	}
	return nil
}
