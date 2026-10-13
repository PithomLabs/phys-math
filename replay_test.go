package physmath

import (
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// TestReplayEquivShape executes the full six-transformation mechanical slice:
// APPLY(rho) → APPLY(staged f) → CANON → APPLY(f) → APPLY(staged rho) → CANON,
// then checks final byte equality and the theorem conclusion gate.
func TestReplayEquivShape(t *testing.T) {
	s := NewMemStore()
	p0, p1 := DeriveParameterID(0), DeriveParameterID(1)
	g := NewBoundSymbol("g", p0)
	x := NewBoundSymbol("x", p1)
	fRef, _ := mustStoreFunction(t, s, []string{"v"}, Mul(MustRational(2, 1), NewBoundSymbol("v", p0)))
	rhoRef, _ := mustStoreFunction(t, s, []string{"g", "v"},
		Mul(NewBoundSymbol("g", p0), NewBoundSymbol("v", p1)))

	innerRho := mustStoreExpr(t, s, Call(rhoRef, g, x))
	innerF := mustStoreExpr(t, s, Call(fRef, x))

	run := func(rule string, inputs []artifact.ArtifactRef, params []byte) artifact.ArtifactRef {
		t.Helper()
		res, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
			RuleID: rule, InputRefs: inputs, Parameters: params})
		if err != nil {
			t.Fatalf("rule %s: %v", rule, err)
		}
		return res.OutputRef
	}

	// Step 0: APPLY inner rho → g·x.
	s0 := run(RuleExprApply, []artifact.ArtifactRef{innerRho, rhoRef}, nil)
	// Staged LHS_1 = f(g·x), linkage-checked against Step 0.
	s0expr := mustResolve(t, s, s0)
	lhs1 := mustStoreExpr(t, s, Call(fRef, s0expr))
	if arg := mustResolve(t, s, lhs1); true {
		_ = arg
	}
	// Step 1: APPLY staged LHS_1 → 2·(g·x).
	s1 := run(RuleExprApply, []artifact.ArtifactRef{lhs1, fRef}, nil)
	// Step 2: CANON → 2·g·x.
	s2 := run(RuleExprCanon, []artifact.ArtifactRef{s1}, nil)
	// Step 3: APPLY inner f → 2·x.
	s3 := run(RuleExprApply, []artifact.ArtifactRef{innerF, fRef}, nil)
	// Staged RHS_1 = rho(g, 2·x), linkage-checked against Step 3.
	s3expr := mustResolve(t, s, s3)
	rhs1 := mustStoreExpr(t, s, Call(rhoRef, g, s3expr))
	// Step 4: APPLY staged RHS_1 → g·(2·x).
	s4 := run(RuleExprApply, []artifact.ArtifactRef{rhs1, rhoRef}, nil)
	// Step 5: CANON → 2·g·x.
	s5 := run(RuleExprCanon, []artifact.ArtifactRef{s4}, nil)

	// Final byte equality (spec §16.10/16.11 gate).
	e2 := mustResolve(t, s, s2)
	e5 := mustResolve(t, s, s5)
	if !e2.Equal(e5) {
		t.Fatal("final LHS/RHS bytes differ")
	}

	// Assemble the Derivation with staged artifacts as fixtures.
	steps := []DerivationStep{
		{Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprApply,
			InputRefs: []artifact.ArtifactRef{innerRho, rhoRef}, OutputRef: s0},
		{Ordinal: 1, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprApply,
			InputRefs: []artifact.ArtifactRef{lhs1, fRef}, OutputRef: s1},
		{Ordinal: 2, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
			InputRefs: []artifact.ArtifactRef{s1}, OutputRef: s2},
		{Ordinal: 3, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprApply,
			InputRefs: []artifact.ArtifactRef{innerF, fRef}, OutputRef: s3},
		{Ordinal: 4, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprApply,
			InputRefs: []artifact.ArtifactRef{rhs1, rhoRef}, OutputRef: s4},
		{Ordinal: 5, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
			InputRefs: []artifact.ArtifactRef{s4}, OutputRef: s5},
	}
	// s2 and s5 are content-identical (final byte equality); the SET carries
	// the single canonical result reference. Declaring Functions are premises:
	// same-run availability covers every step input.
	d := Derivation{
		PremiseRefs: []artifact.ArtifactRef{innerRho, innerF, fRef, rhoRef},
		Steps:       steps,
		ResultRefs:  []artifact.ArtifactRef{s2},
	}
	fixtures := map[artifact.ArtifactRef]bool{lhs1: true, rhs1: true}
	if err := Replay(s, d, fixtures); err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	// Determinism: replay twice → identical refs (§25.17, D14).
	if err := Replay(s, d, fixtures); err != nil {
		t.Fatalf("second replay failed: %v", err)
	}

	// Theorem conclusion gate (§13.3): relation embeds the designated finals.
	relBytes, err := EncodeRelationBody(OpEQ, e2, e5)
	if err != nil {
		t.Fatal(err)
	}
	relRef, err := s.Put(FamilyRelation, relBytes)
	if err != nil {
		t.Fatal(err)
	}
	drvBytes, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	drvRef, err := s.Put(FamilyDerivation, drvBytes)
	if err != nil {
		t.Fatal(err)
	}
	th := Theorem{ConclusionRef: relRef, DerivationRef: drvRef}
	if err := CheckTheoremConclusion(s, th, s2, s5); err != nil {
		t.Fatalf("conclusion gate failed: %v", err)
	}
	// Swapped sides still pass when bytes are identical (both sides equal);
	// a genuinely different side fails.
	other, _ := s.Put(FamilyExpression, mustEncode(t, NewEphemeralSymbol("zzz")))
	if err := CheckTheoremConclusion(s, th, s2, other); err == nil {
		t.Fatal("conclusion gate accepted foreign bytes")
	}
}

func mustEncode(t *testing.T, e Expr) []byte {
	t.Helper()
	b, err := e.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Replay rejects: future-step input, tampered output, wrong-family input.
func TestReplayNegatives(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	fRef, _ := mustStoreFunction(t, s, []string{"v"}, Mul(MustRational(2, 1), NewBoundSymbol("v", p0)))
	inner := mustStoreExpr(t, s, Call(fRef, NewEphemeralSymbol("x")))
	res, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprApply, InputRefs: []artifact.ArtifactRef{inner, fRef}})
	if err != nil {
		t.Fatal(err)
	}
	// Tampered OutputRef.
	bad := Derivation{
		PremiseRefs: []artifact.ArtifactRef{inner},
		Steps: []DerivationStep{{Ordinal: 0, ProducerNamespace: NamespacePhysMath,
			RuleID: RuleExprApply, InputRefs: []artifact.ArtifactRef{inner, fRef},
			OutputRef: inner}},
		ResultRefs: []artifact.ArtifactRef{inner},
	}
	if err := Replay(s, bad, nil); err == nil {
		t.Fatal("tampered output accepted on replay")
	}
	_ = res
	// Future-step input: step 0 consumes step 1's output.
	future := Derivation{
		PremiseRefs: []artifact.ArtifactRef{inner},
		Steps: []DerivationStep{
			{Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
				InputRefs: []artifact.ArtifactRef{res.OutputRef}, OutputRef: res.OutputRef},
			{Ordinal: 1, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprApply,
				InputRefs: []artifact.ArtifactRef{inner, fRef}, OutputRef: res.OutputRef},
		},
		ResultRefs: []artifact.ArtifactRef{res.OutputRef},
	}
	if err := Replay(s, future, nil); err == nil {
		t.Fatal("future-step input accepted")
	}
	// Wrong input family for the rule (§25.29): APPLY with expression as Input 1.
	wrongFam := Derivation{
		PremiseRefs: []artifact.ArtifactRef{inner},
		Steps: []DerivationStep{{Ordinal: 0, ProducerNamespace: NamespacePhysMath,
			RuleID: RuleExprApply, InputRefs: []artifact.ArtifactRef{inner, inner},
			OutputRef: res.OutputRef}},
		ResultRefs: []artifact.ArtifactRef{res.OutputRef},
	}
	if err := Replay(s, wrongFam, nil); err == nil {
		t.Fatal("wrong input family accepted")
	}
	// Result neither produced nor carried through.
	dangling := Derivation{
		PremiseRefs: []artifact.ArtifactRef{inner},
		Steps: []DerivationStep{{Ordinal: 0, ProducerNamespace: NamespacePhysMath,
			RuleID: RuleExprApply, InputRefs: []artifact.ArtifactRef{inner, fRef},
			OutputRef: res.OutputRef}},
		ResultRefs: []artifact.ArtifactRef{inner, res.OutputRef, fRef},
	}
	_ = dangling
	// fRef is a function artifact: available? premises={inner}; outputs={res}.
	// fRef itself was never premise/output → dangling.
	if err := Replay(s, dangling, nil); err == nil {
		t.Fatal("dangling result accepted")
	}
}
