package physmath

import (
	"errors"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// TestDifferentiateFirstPower: d(u^1)/dx = du/dx with no manufactured
// Pow(u,0) node and no condition requirement.
func TestDifferentiateFirstPower(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	fnRef, _ := mustStoreFunction(t, s, 1, NewBoundSymbol("a", p0))
	params := func() []byte {
		return append(BoundParameterRef{fnRef, p0}.Encode(), artifact.EncodeUInt32(1)...)
	}
	// Variable base: d(x^1)/dx = 1 directly, unconditionally.
	x := NewBoundSymbol("x", p0)
	in := mustStoreExpr(t, s, Pow(x, MustRational(1, 1)))
	res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in, fnRef}, nil, params())
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(MustRational(1, 1)) {
		t.Fatal("d(x^1)/dx is not exactly 1")
	}
	// Composite base: d((x*y)^1)/dx = d(x*y), chain preserved.
	p1 := DeriveParameterID(1)
	fn2, _ := mustStoreFunction(t, s, 2,
		Mul(NewBoundSymbol("a", p0), NewBoundSymbol("b", p1)))
	y := NewBoundSymbol("y", p1)
	base := Mul(x, y)
	in2 := mustStoreExpr(t, s, Pow(base, MustRational(1, 1)))
	params2 := append(BoundParameterRef{fn2, p0}.Encode(), artifact.EncodeUInt32(1)...)
	res2 := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in2, fn2}, nil, params2)
	want2, _ := Canonicalize(Add(Mul(MustRational(1, 1), y), Mul(x, MustRational(0, 1))))
	if got := mustResolve(t, s, res2.OutputRef); !got.Equal(want2) {
		t.Fatal("composite first-power derivative wrong")
	}
	// Constant base: d(2^1)/dx = 0.
	in3 := mustStoreExpr(t, s, Pow(MustRational(2, 1), MustRational(1, 1)))
	res3 := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in3, fnRef}, nil, params())
	if got := mustResolve(t, s, res3.OutputRef); !got.Equal(MustRational(0, 1)) {
		t.Fatal("constant-base first power did not differentiate to 0")
	}
	// Replay through the rule with no conditions attached.
	if err := Replay(s, Derivation{
		PremiseRefs: []artifact.ArtifactRef{in, fnRef},
		Steps: []DerivationStep{{
			Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleDifferentiate,
			InputRefs: []artifact.ArtifactRef{in, fnRef}, OutputRef: res.OutputRef,
			Parameters: params(),
		}},
		ResultRefs: []artifact.ArtifactRef{res.OutputRef},
	}, nil); err != nil {
		t.Fatalf("first-power derivation does not replay: %v", err)
	}
	// General u^0 safety intact: Pow(y,0) still needs its condition.
	pow0 := mustStoreExpr(t, s, Pow(y, MustRational(0, 1)))
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleExprSimplify, InputRefs: []artifact.ArtifactRef{pow0}}); !errors.Is(err, ErrMissingCondition) {
		t.Fatalf("generic Pow(u,0) gate weakened: %v", err)
	}
}
