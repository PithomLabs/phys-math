package physmath

import (
	"errors"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// TestCombineBasic: a*a -> a^2, a^2*a^3 -> a^5 by hand.
func TestCombineBasic(t *testing.T) {
	s := NewMemStore()
	x := NewEphemeralSymbol("x")
	in := mustStoreExpr(t, s, Mul(x, x))
	res := applyRule(t, s, RuleExprCombine, []artifact.ArtifactRef{in}, nil, nil)
	want, _ := Canonicalize(Pow(x, MustRational(2, 1)))
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(want) {
		t.Fatal("x*x did not combine to x^2")
	}
	in2 := mustStoreExpr(t, s, Mul(Pow(x, MustRational(2, 1)), Pow(x, MustRational(3, 1))))
	res2 := applyRule(t, s, RuleExprCombine, []artifact.ArtifactRef{in2}, nil, nil)
	want2, _ := Canonicalize(Pow(x, MustRational(5, 1)))
	if got := mustResolve(t, s, res2.OutputRef); !got.Equal(want2) {
		t.Fatal("x^2*x^3 did not combine to x^5")
	}
	// Coefficients untouched, pairs combined: 2*2*a*a -> 2*2*a^2.
	a := NewEphemeralSymbol("a")
	in3 := mustStoreExpr(t, s, Mul(MustRational(2, 1), MustRational(2, 1), a, a))
	res3 := applyRule(t, s, RuleExprCombine, []artifact.ArtifactRef{in3}, nil, nil)
	want3, _ := Canonicalize(Mul(MustRational(2, 1), MustRational(2, 1), Pow(a, MustRational(2, 1))))
	if got := mustResolve(t, s, res3.OutputRef); !got.Equal(want3) {
		t.Fatal("coefficient-preserving combination wrong")
	}
	// Rationals never fold here: 2*3 stays (SIMPLIFY's job).
	in4 := mustStoreExpr(t, s, Mul(MustRational(2, 1), MustRational(3, 1)))
	res4 := applyRule(t, s, RuleExprCombine, []artifact.ArtifactRef{in4}, nil, nil)
	want4, _ := Canonicalize(Mul(MustRational(2, 1), MustRational(3, 1)))
	if got := mustResolve(t, s, res4.OutputRef); !got.Equal(want4) {
		t.Fatal("rationals folded by combine rule")
	}
	// Distinct bases untouched.
	in5 := mustStoreExpr(t, s, Mul(x, a))
	res5 := applyRule(t, s, RuleExprCombine, []artifact.ArtifactRef{in5}, nil, nil)
	want5, _ := Canonicalize(Mul(x, a))
	if got := mustResolve(t, s, res5.OutputRef); !got.Equal(want5) {
		t.Fatal("distinct bases altered")
	}
	// Non-integer exponents untouched.
	in6 := mustStoreExpr(t, s, Mul(Pow(x, MustRational(1, 2)), Pow(x, MustRational(1, 2))))
	res6 := applyRule(t, s, RuleExprCombine, []artifact.ArtifactRef{in6}, nil, nil)
	want6, _ := Canonicalize(Mul(Pow(x, MustRational(1, 2)), Pow(x, MustRational(1, 2))))
	if got := mustResolve(t, s, res6.OutputRef); !got.Equal(want6) {
		t.Fatal("non-integer exponents combined")
	}
	// Determinism.
	res7, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprCombine, InputRefs: []artifact.ArtifactRef{in}})
	if err != nil {
		t.Fatal(err)
	}
	if res7.OutputRef != res.OutputRef {
		t.Fatal("combine recomputation nondeterministic")
	}
}

// TestCombineNegative: x^-1*x^-2 -> x^-3; x*x^-1 -> 1 iff NEQ(x,0).
func TestCombineNegative(t *testing.T) {
	s := NewMemStore()
	x := NewEphemeralSymbol("x")
	in := mustStoreExpr(t, s, Mul(Pow(x, MustRational(-1, 1)), Pow(x, MustRational(-2, 1))))
	res := applyRule(t, s, RuleExprCombine, []artifact.ArtifactRef{in}, nil, nil)
	want, _ := Canonicalize(Pow(x, MustRational(-3, 1)))
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(want) {
		t.Fatal("negative exponents did not combine to x^-3")
	}
	cancel := mustStoreExpr(t, s, Mul(x, Pow(x, MustRational(-1, 1))))
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprCombine, InputRefs: []artifact.ArtifactRef{cancel}}); !errors.Is(err, ErrMissingCondition) {
		t.Fatalf("x*x^-1 without NEQ condition did not fail closed: %v", err)
	}
	cond := neqCond(t, s, x)
	resC := applyRule(t, s, RuleExprCombine, []artifact.ArtifactRef{cancel},
		[]artifact.ArtifactRef{cond}, nil)
	if got := mustResolve(t, s, resC.OutputRef); !got.Equal(MustRational(1, 1)) {
		t.Fatal("x*x^-1 with NEQ did not combine to 1")
	}
	if len(resC.ConditionRefs) != 1 || resC.ConditionRefs[0] != cond {
		t.Fatal("condition not threaded through combine result")
	}
	if err := Replay(s, Derivation{
		PremiseRefs: []artifact.ArtifactRef{cancel},
		Steps: []DerivationStep{{
			Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCombine,
			InputRefs: []artifact.ArtifactRef{cancel}, OutputRef: resC.OutputRef,
			ConditionRefs: []artifact.ArtifactRef{cond},
		}},
		ResultRefs: []artifact.ArtifactRef{resC.OutputRef},
	}, nil); err != nil {
		t.Fatalf("combine derivation does not replay: %v", err)
	}
}

// TestCombineRejectsNonMul: narrow scope — Add and bare symbols rejected.
func TestCombineRejectsNonMul(t *testing.T) {
	s := NewMemStore()
	x := NewEphemeralSymbol("x")
	add := mustStoreExpr(t, s, Add(x, x))
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprCombine, InputRefs: []artifact.ArtifactRef{add}}); err == nil {
		t.Fatal("Add root accepted by combine rule")
	}
	sym := mustStoreExpr(t, s, x)
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprCombine, InputRefs: []artifact.ArtifactRef{sym}}); err == nil {
		t.Fatal("bare symbol accepted by combine rule")
	}
}
