package physmath

import (
	"bytes"
	"errors"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

func mustStoreExpr(t *testing.T, s Store, e Expr) artifact.ArtifactRef {
	t.Helper()
	ref, err := PutExpression(s, e)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// mustStoreFunction mints a Function with ordinal PIDs, explicit parameter
// names (the authoritative display vocabulary for differentiation identity),
// and a validated body. names may be nil only in legacy-decode fixtures.
func mustStoreFunction(t *testing.T, s Store, names []string, body Expr) (artifact.ArtifactRef, Function) {
	t.Helper()
	ids := make([]ParameterID, len(names))
	for i := range ids {
		ids[i] = DeriveParameterID(uint32(i))
	}
	bodyBytes, err := body.Encode()
	if err != nil {
		t.Fatal(err)
	}
	bodyRef, err := s.Put(FamilyExpression, bodyBytes)
	if err != nil {
		t.Fatal(err)
	}
	fn := Function{ParameterIDs: ids, ParamNames: names, BodyRef: bodyRef}
	if err := ValidateFunctionBody(fn, body); err != nil {
		t.Fatalf("fixture body invalid: %v", err)
	}
	fnBytes, err := fn.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyFunction, fnBytes)
	if err != nil {
		t.Fatal(err)
	}
	return ref, fn
}

func applyRule(t *testing.T, s Store, rule string, inputs []artifact.ArtifactRef, conds []artifact.ArtifactRef, params []byte) TransformResult {
	t.Helper()
	res, err := Apply(s, Invocation{
		ProducerNamespace: NamespacePhysMath, RuleID: rule,
		InputRefs: inputs, ConditionRefs: conds, Parameters: params,
	})
	if err != nil {
		t.Fatalf("rule %s failed: %v", rule, err)
	}
	return res
}

func mustResolve(t *testing.T, s Store, ref artifact.ArtifactRef) Expr {
	t.Helper()
	e, err := resolveExpression(s, ref)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// §25.11: unknown rules, wrong arity, wrong families rejected.
func TestRuleRegistryAndArity(t *testing.T) {
	s := NewMemStore()
	x := mustStoreExpr(t, s, NewEphemeralSymbol("x"))
	for _, inv := range []Invocation{
		{ProducerNamespace: "physmath", RuleID: "PROJECT-STRICT-001", InputRefs: []artifact.ArtifactRef{x}},
		{ProducerNamespace: "phys-gr", RuleID: RuleExprCanon, InputRefs: []artifact.ArtifactRef{x}},
		{ProducerNamespace: "physmath", RuleID: RuleExprCanon, InputRefs: []artifact.ArtifactRef{x, x}},
		{ProducerNamespace: "physmath", RuleID: RuleExprApply, InputRefs: []artifact.ArtifactRef{x}},
	} {
		if _, err := Apply(s, inv); err == nil {
			t.Fatalf("accepted %+v", inv)
		} else if !errors.Is(err, ErrUnknownRuleID) && !errors.Is(err, ErrArityMismatch) {
			t.Fatalf("wrong error class: %v", err)
		}
	}
	if _, err := Apply(s, Invocation{ProducerNamespace: "physmath", RuleID: RuleExprCanon,
		InputRefs: []artifact.ArtifactRef{x}, Parameters: []byte{0x01}}); !errors.Is(err, ErrBadParameters) {
		t.Fatalf("non-empty CANON params: %v", err)
	}
}

func TestCanonIdempotence(t *testing.T) {
	s := NewMemStore()
	x, y := NewEphemeralSymbol("x"), NewEphemeralSymbol("y")
	in := mustStoreExpr(t, s, Add(y, Add(x, y)))
	r1 := applyRule(t, s, RuleExprCanon, []artifact.ArtifactRef{in}, nil, nil)
	r2 := applyRule(t, s, RuleExprCanon, []artifact.ArtifactRef{r1.OutputRef}, nil, nil)
	if r1.OutputRef != r2.OutputRef {
		t.Fatal("EXPR-CANON-001 not idempotent")
	}
	if e := mustResolve(t, s, r1.OutputRef); len(e.Children) != 3 {
		t.Fatal("flatten failed")
	}
}

// §25.9 + closed rewrite list.
func TestSimplifyRewrites(t *testing.T) {
	s := NewMemStore()
	x := NewEphemeralSymbol("x")
	cases := []struct {
		name string
		in   Expr
		want Expr
	}{
		{"negneg", Neg(Neg(x)), x},
		{"add-zero", Add(x, MustRational(0, 1)), x},
		{"mul-one", Mul(x, MustRational(1, 1)), x},
		{"mul-zero", Mul(x, MustRational(0, 1)), MustRational(0, 1)},
		{"fold-add", Add(MustRational(1, 2), MustRational(1, 3)), MustRational(5, 6)},
		{"fold-mul", Mul(MustRational(2, 1), MustRational(3, 4)), MustRational(3, 2)},
		{"pow-one", Pow(x, MustRational(1, 1)), x},
	}
	for _, c := range cases {
		in := mustStoreExpr(t, s, c.in)
		res := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{in}, nil, nil)
		if got := mustResolve(t, s, res.OutputRef); !got.Equal(c.want) {
			t.Fatalf("%s: wrong result", c.name)
		}
		// Idempotence: second application reproduces the ref.
		res2 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
		if res2.OutputRef != res.OutputRef {
			t.Fatalf("%s: not idempotent", c.name)
		}
	}
}

// §25.27: no partial folding in mixed symbolic nodes.
func TestSimplifyNoPartialFolding(t *testing.T) {
	s := NewMemStore()
	x := NewEphemeralSymbol("x")
	in := mustStoreExpr(t, s, Add(MustRational(1, 2), x, MustRational(1, 3)))
	res := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{in}, nil, nil)
	got := mustResolve(t, s, res.OutputRef)
	if got.Kind != TagAdd || len(got.Children) != 3 {
		t.Fatal("partial rational folding occurred in mixed Add")
	}
}

// §25.26: Pow(x,0) fail-closed without exact condition; succeeds with it.
func TestSimplifyPowZeroCondition(t *testing.T) {
	s := NewMemStore()
	x := NewEphemeralSymbol("x")
	in := mustStoreExpr(t, s, Pow(x, MustRational(0, 1)))
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprSimplify, InputRefs: []artifact.ArtifactRef{in}}); !errors.Is(err, ErrMissingCondition) {
		t.Fatalf("Pow(x,0) without condition: %v", err)
	}
	// L-4 SPEC-DERIVED expectation: the exact NEQ(x,0) canonical relation
	// body, hand-derived from the §4.5/§7 framing rules (NOT produced by the
	// encoder under test):
	//
	//	Enum("NEQ") || Symbol("x",no IDs) || Rational(0/1)
	//	= String(3,"NEQ") || 0x01||String(1,"x")||00||00
	//	  || 0x02||BigInt(0)||BigInt(1)
	specDerivedNEQ := []byte{
		0x00, 0x00, 0x00, 0x03, 'N', 'E', 'Q',
		0x01, 0x00, 0x00, 0x00, 0x01, 'x', 0x00, 0x00,
		0x02,
		0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x01, 0x01,
	}
	// Cross-check documents the derivation; the hardcoded bytes govern.
	relBytes, err := EncodeRelationBody(OpNEQ, x, MustRational(0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(specDerivedNEQ, relBytes) {
		t.Fatal("SPEC-DERIVED condition bytes diverge from encoder")
	}
	relRef, err := s.Put(FamilyRelation, specDerivedNEQ)
	if err != nil {
		t.Fatal(err)
	}
	condBytes := relRef.Encode()
	condRef, err := s.Put(FamilyCondition, condBytes)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprSimplify, InputRefs: []artifact.ArtifactRef{in},
		ConditionRefs: []artifact.ArtifactRef{condRef}})
	if err != nil {
		t.Fatalf("Pow(x,0) with exact condition failed: %v", err)
	}
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(MustRational(1, 1)) {
		t.Fatal("Pow(x,0) did not reduce to 1")
	}
	// Condition preserved in result, not discharged (§25.10).
	if len(res.ConditionRefs) != 1 || res.ConditionRefs[0] != condRef {
		t.Fatal("condition not threaded through TransformResult")
	}
	// A NEQ(y,0) condition for a different y does NOT satisfy Pow(x,0).
	y := NewEphemeralSymbol("y")
	relBytes2, _ := EncodeRelationBody(OpNEQ, y, MustRational(0, 1))
	relRef2, _ := s.Put(FamilyRelation, relBytes2)
	condRef2, _ := s.Put(FamilyCondition, relRef2.Encode())
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprSimplify, InputRefs: []artifact.ArtifactRef{in},
		ConditionRefs: []artifact.ArtifactRef{condRef2}}); !errors.Is(err, ErrMissingCondition) {
		t.Fatalf("wrong-variable condition discharged: %v", err)
	}
}

// Neg(Rational) folds to the negated rational (additive-inverse closure
// of the exact-rational folds). Non-rational operands are untouched, and
// rational sums then reach the all-Rational fold: Add(4, Neg(6)) -> -2.
func TestSimplifyNegRational(t *testing.T) {
	s := NewMemStore()
	in := mustStoreExpr(t, s, Neg(MustRational(6, 1)))
	res := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{in}, nil, nil)
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(MustRational(-6, 1)) {
		t.Fatal("Neg(6) did not fold to -6")
	}
	sum := mustStoreExpr(t, s, Add(MustRational(4, 1), Neg(MustRational(6, 1))))
	res2 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{sum}, nil, nil)
	if got := mustResolve(t, s, res2.OutputRef); !got.Equal(MustRational(-2, 1)) {
		t.Fatal("Add(4, Neg(6)) did not fold to -2")
	}
	// Non-rational negation is not folded.
	x := NewEphemeralSymbol("x")
	sym := mustStoreExpr(t, s, Neg(x))
	res3 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{sym}, nil, nil)
	want3, _ := Canonicalize(Neg(x))
	if got := mustResolve(t, s, res3.OutputRef); !got.Equal(want3) {
		t.Fatal("Neg(symbol) altered")
	}
	if err := Replay(s, Derivation{
		PremiseRefs: []artifact.ArtifactRef{sum},
		Steps: []DerivationStep{{
			Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
			InputRefs: []artifact.ArtifactRef{sum}, OutputRef: res2.OutputRef,
		}},
		ResultRefs: []artifact.ArtifactRef{res2.OutputRef},
	}, nil); err != nil {
		t.Fatalf("neg-fold derivation does not replay: %v", err)
	}
}

// §25.15: branch selection.
func TestSelectBranch(t *testing.T) {
	s := NewMemStore()
	a, b := NewEphemeralSymbol("a"), NewEphemeralSymbol("b")
	in := mustStoreExpr(t, s, BranchSet(a, b))
	res := applyRule(t, s, RuleSelectBranch, []artifact.ArtifactRef{in}, nil, []byte{0, 0, 0, 1})
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(b) {
		t.Fatal("wrong branch selected")
	}
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleSelectBranch, InputRefs: []artifact.ArtifactRef{in},
		Parameters: []byte{0, 0, 0, 7}}); err == nil {
		t.Fatal("out-of-range ordinal accepted")
	}
	notBranch := mustStoreExpr(t, s, a)
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleSelectBranch, InputRefs: []artifact.ArtifactRef{notBranch},
		Parameters: []byte{0, 0, 0, 0}}); err == nil {
		t.Fatal("non-BranchSet root accepted")
	}
}

// RAT-NORMALIZE root constraint.
func TestRatNormalize(t *testing.T) {
	s := NewMemStore()
	r := mustStoreExpr(t, s, MustRational(3, 2))
	res := applyRule(t, s, RuleRatNormalize, []artifact.ArtifactRef{r}, nil, nil)
	if res.OutputRef != r {
		t.Fatal("normalized rational changed identity")
	}
	notRat := mustStoreExpr(t, s, NewEphemeralSymbol("x"))
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleRatNormalize, InputRefs: []artifact.ArtifactRef{notRat}}); err == nil {
		t.Fatal("non-Rational root accepted")
	}
}

// §25.12: EXPR-APPLY boundary.
func TestApplyBoundary(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	fnRef, _ := mustStoreFunction(t, s, []string{"v"}, Mul(MustRational(2, 1), NewBoundSymbol("v", p0)))
	inner := mustStoreExpr(t, s, Call(fnRef, NewEphemeralSymbol("x")))
	res := applyRule(t, s, RuleExprApply, []artifact.ArtifactRef{inner, fnRef}, nil, nil)
	want := Mul(MustRational(2, 1), NewEphemeralSymbol("x"))
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(want) {
		t.Fatal("APPLY expansion wrong")
	}
	// Nested call not selected.
	nested := mustStoreExpr(t, s, Add(Call(fnRef, NewEphemeralSymbol("x")), MustRational(1, 1)))
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprApply, InputRefs: []artifact.ArtifactRef{nested, fnRef}}); err == nil {
		t.Fatal("nested Call selected without isolation")
	}
	// Mismatched FunctionRef.
	otherFn, _ := mustStoreFunction(t, s, []string{"v"}, NewBoundSymbol("v", p0))
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprApply, InputRefs: []artifact.ArtifactRef{inner, otherFn}}); err == nil {
		t.Fatal("mismatched FunctionRef accepted")
	}
	// Arity mismatch.
	twoFn, _ := mustStoreFunction(t, s, []string{"a", "b"},
		Mul(NewBoundSymbol("a", DeriveParameterID(0)), NewBoundSymbol("b", DeriveParameterID(1))))
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprApply, InputRefs: []artifact.ArtifactRef{inner, twoFn}}); err == nil {
		t.Fatal("arity mismatch accepted")
	}
}

// §25.13 + §25.28: SUBSTITUTE declaring context + duplicate targets.
func TestSubstitute(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	fnRef, _ := mustStoreFunction(t, s, []string{"v"}, NewBoundSymbol("v", p0))
	target := mustStoreExpr(t, s, Add(NewBoundSymbol("g", p0), MustRational(1, 1)))
	rep := mustStoreExpr(t, s, Mul(NewEphemeralSymbol("y"), MustRational(3, 1)))
	entry := append(BoundParameterRef{fnRef, p0}.Encode(), rep.Encode()...)
	params := append(artifact.EncodeUInt32(1), entry...)
	res := applyRule(t, s, RuleExprSubstitute,
		[]artifact.ArtifactRef{target, fnRef}, nil, params)
	want := Add(Mul(NewEphemeralSymbol("y"), MustRational(3, 1)), MustRational(1, 1))
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(want) {
		t.Fatal("SUBSTITUTE result wrong")
	}
	// Missing declaring context.
	otherFn, _ := mustStoreFunction(t, s, []string{"v"}, Mul(MustRational(3, 1), NewBoundSymbol("v", p0)))
	entry2 := append(BoundParameterRef{otherFn, p0}.Encode(), rep.Encode()...)
	params2 := append(artifact.EncodeUInt32(1), entry2...)
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprSubstitute, InputRefs: []artifact.ArtifactRef{target, fnRef},
		Parameters: params2}); err == nil {
		t.Fatal("absent declaring Function accepted")
	}
	// Duplicate ParameterID across different declaring artifacts rejected.
	e1 := append(BoundParameterRef{fnRef, p0}.Encode(), rep.Encode()...)
	e2 := append(BoundParameterRef{otherFn, p0}.Encode(), rep.Encode()...)
	dup := append(artifact.EncodeUInt32(2), append(e1, e2...)...)
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprSubstitute, InputRefs: []artifact.ArtifactRef{target, fnRef, otherFn},
		Parameters: dup}); err == nil {
		t.Fatal("duplicate target ParameterID accepted")
	}
	// N = 0 rejected.
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprSubstitute, InputRefs: []artifact.ArtifactRef{target, fnRef},
		Parameters: artifact.EncodeUInt32(0)}); err == nil {
		t.Fatal("N=0 accepted")
	}
}

// §25.14 + closed derivative rules.
func TestDifferentiate(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	p1 := DeriveParameterID(1)
	fnRef, _ := mustStoreFunction(t, s, []string{"x", "y"},
		Mul(NewBoundSymbol("x", p0), NewBoundSymbol("y", p1)))
	diffParams := func(pid ParameterID, order uint32) []byte {
		return append(BoundParameterRef{fnRef, pid}.Encode(), artifact.EncodeUInt32(order)...)
	}
	// d(r+r)/dr-ish: d(x+x)/dx with schematic PID.
	x := NewBoundSymbol("x", p0)
	in := mustStoreExpr(t, s, Add(x, x))
	res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in, fnRef}, nil, diffParams(p0, 1))
	// d(x+x) = 1+1 exactly; folding to 2 is EXPR-SIMPLIFY-001's job.
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(Add(MustRational(1, 1), MustRational(1, 1))) {
		t.Fatal("linearity wrong")
	}
	folded := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
	if got := mustResolve(t, s, folded.OutputRef); !got.Equal(MustRational(2, 1)) {
		t.Fatal("simplify-after-differentiate wrong")
	}
	// Leibniz: d(x*x) = x*1 + 1*x → canon 2*x.
	in2 := mustStoreExpr(t, s, Mul(x, x))
	res2 := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in2, fnRef}, nil, diffParams(p0, 1))
	rawWant, _ := Canonicalize(Add(Mul(x, MustRational(1, 1)), Mul(x, MustRational(1, 1))))
	if got := mustResolve(t, s, res2.OutputRef); !got.Equal(rawWant) {
		t.Fatal("Leibniz wrong")
	}
	simp2 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res2.OutputRef}, nil, nil)
	if got := mustResolve(t, s, simp2.OutputRef); !got.Equal(Add(x, x)) {
		t.Fatal("leibniz+simplify wrong (x+x→2x must NOT fire)")
	}
	// Other parameter differentiates to 0.
	y := NewBoundSymbol("y", p1)
	in3 := mustStoreExpr(t, s, y)
	res3 := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in3, fnRef}, nil, diffParams(p0, 1))
	if got := mustResolve(t, s, res3.OutputRef); !got.Equal(MustRational(0, 1)) {
		t.Fatal("other parameter did not differentiate to 0")
	}
	// Mismatched declaring function rejected.
	otherFn, _ := mustStoreFunction(t, s, []string{"v"}, NewBoundSymbol("v", p0))
	badBPR := append(BoundParameterRef{otherFn, p0}.Encode(), artifact.EncodeUInt32(1)...)
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleDifferentiate, InputRefs: []artifact.ArtifactRef{in, fnRef},
		Parameters: badBPR}); err == nil {
		t.Fatal("mismatched BPR accepted")
	}
	// Call differentiation out of scope.
	callIn := mustStoreExpr(t, s, Call(fnRef, x, y))
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleDifferentiate, InputRefs: []artifact.ArtifactRef{callIn, fnRef},
		Parameters: diffParams(p0, 1)}); err == nil {
		t.Fatal("Call differentiation accepted")
	}
	// Non-integer Pow exponent rejected.
	powIn := mustStoreExpr(t, s, Pow(x, NewEphemeralSymbol("n")))
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleDifferentiate, InputRefs: []artifact.ArtifactRef{powIn, fnRef},
		Parameters: diffParams(p0, 1)}); err == nil {
		t.Fatal("non-integer Pow exponent accepted")
	}
	// Pow rule: d(x^2) = 2*x.
	pow2 := mustStoreExpr(t, s, Pow(x, MustRational(2, 1)))
	resP := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{pow2, fnRef}, nil, diffParams(p0, 1))
	rawP, _ := Canonicalize(Mul(MustRational(2, 1), Pow(x, MustRational(1, 1)), MustRational(1, 1)))
	if got := mustResolve(t, s, resP.OutputRef); !got.Equal(rawP) {
		t.Fatal("Pow rule wrong")
	}
	simpP := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{resP.OutputRef}, nil, nil)
	wantP, _ := Canonicalize(Mul(MustRational(2, 1), x))
	if got := mustResolve(t, s, simpP.OutputRef); !got.Equal(wantP) {
		t.Fatal("pow+simplify wrong")
	}
	// Order 2 on x^3, then simplify: d2(x^3) = 6x. Canonicalize flattens
	// nested Muls, so the exact form is Mul(x,2,3).
	pow3 := mustStoreExpr(t, s, Pow(x, MustRational(3, 1)))
	resP2 := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{pow3, fnRef}, nil, diffParams(p0, 2))
	simpP2 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{resP2.OutputRef}, nil, nil)
	wantP2 := Mul(x, MustRational(2, 1), MustRational(3, 1))
	if got := mustResolve(t, s, simpP2.OutputRef); !got.Equal(wantP2) {
		t.Fatal("Order-2 differentiation wrong")
	}
}
