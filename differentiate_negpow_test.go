package physmath

import (
	"errors"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// neqCond mints an exact NEQ(e,0) condition/1 artifact for e.
func neqCond(t *testing.T, s Store, e Expr) artifact.ArtifactRef {
	t.Helper()
	body, err := EncodeRelationBody(OpNEQ, e, MustRational(0, 1))
	if err != nil {
		t.Fatal(err)
	}
	relRef, err := s.Put(FamilyRelation, body)
	if err != nil {
		t.Fatal(err)
	}
	condRef, err := s.Put(FamilyCondition, relRef.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return condRef
}

func diffInvoke(t *testing.T, s Store, fnRef artifact.ArtifactRef, pid ParameterID, order uint32) []byte {
	t.Helper()
	return append(BoundParameterRef{fnRef, pid}.Encode(), artifact.EncodeUInt32(order)...)
}

// TestDifferentiateNegativePow: d(x^-2) requires NEQ(x,0); with it the
// hand-derived result is Mul(-2, Pow(x,-3), 1) up to canonical order.
func TestDifferentiateNegativePow(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	fnRef, _ := mustStoreFunction(t, s, 1, NewBoundSymbol("a", p0))
	x := NewBoundSymbol("x", p0)
	in := mustStoreExpr(t, s, Pow(x, MustRational(-2, 1)))

	// Without the condition: fail closed with the missing-condition sentinel.
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleDifferentiate, InputRefs: []artifact.ArtifactRef{in, fnRef},
		Parameters: diffInvoke(t, s, fnRef, p0, 1)}); !errors.Is(err, ErrMissingCondition) {
		t.Fatalf("negative Pow without NEQ condition did not fail closed: %v", err)
	}
	// Wrong-variable condition does not satisfy.
	y := NewBoundSymbol("y", DeriveParameterID(1))
	_ = y
	yEphem := NewEphemeralSymbol("y")
	wrongCond := neqCond(t, s, yEphem)
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleDifferentiate, InputRefs: []artifact.ArtifactRef{in, fnRef},
		ConditionRefs: []artifact.ArtifactRef{wrongCond},
		Parameters:    diffInvoke(t, s, fnRef, p0, 1)}); !errors.Is(err, ErrMissingCondition) {
		t.Fatalf("wrong-variable condition discharged negative Pow: %v", err)
	}
	// With the exact condition: hand-derived -2*x^-3 (times derivative 1).
	cond := neqCond(t, s, x)
	res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in, fnRef},
		[]artifact.ArtifactRef{cond}, diffInvoke(t, s, fnRef, p0, 1))
	want, _ := Canonicalize(Mul(MustRational(-2, 1), Pow(x, MustRational(-3, 1)), MustRational(1, 1)))
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(want) {
		t.Fatal("negative Pow rule wrong")
	}
	// Condition threaded, not discharged.
	if len(res.ConditionRefs) != 1 || res.ConditionRefs[0] != cond {
		t.Fatal("condition not threaded through TransformResult")
	}
	// Simplify removes the Mul-one identity: -2*x^-3.
	simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
	wantS, _ := Canonicalize(Mul(MustRational(-2, 1), Pow(x, MustRational(-3, 1))))
	if got := mustResolve(t, s, simp.OutputRef); !got.Equal(wantS) {
		t.Fatal("negative-pow simplify wrong")
	}
	if err := Replay(s, Derivation{
		PremiseRefs: []artifact.ArtifactRef{in, fnRef},
		Steps: []DerivationStep{{
			Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleDifferentiate,
			InputRefs: []artifact.ArtifactRef{in, fnRef}, OutputRef: res.OutputRef,
			ConditionRefs: []artifact.ArtifactRef{cond}, Parameters: diffInvoke(t, s, fnRef, p0, 1),
		}},
		ResultRefs: []artifact.ArtifactRef{res.OutputRef},
	}, nil); err != nil {
		t.Fatalf("negative-pow derivation does not replay: %v", err)
	}
}

// TestDifferentiateReciprocal: d(1/x) is Pow(x,-1) by construction;
// hand-derived -1*x^-2.
func TestDifferentiateReciprocal(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	fnRef, _ := mustStoreFunction(t, s, 1, NewBoundSymbol("a", p0))
	x := NewBoundSymbol("x", p0)
	in := mustStoreExpr(t, s, Pow(x, MustRational(-1, 1)))
	cond := neqCond(t, s, x)
	res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in, fnRef},
		[]artifact.ArtifactRef{cond}, diffInvoke(t, s, fnRef, p0, 1))
	simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
	want, _ := Canonicalize(Mul(MustRational(-1, 1), Pow(x, MustRational(-2, 1))))
	if got := mustResolve(t, s, simp.OutputRef); !got.Equal(want) {
		t.Fatal("reciprocal derivative wrong")
	}
}

// TestDifferentiateNegativePowComposite: d((x*y)^-1) with an exact
// NEQ(x*y,0) condition exercises the chain through a composite base.
func TestDifferentiateNegativePowComposite(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	p1 := DeriveParameterID(1)
	fnRef, _ := mustStoreFunction(t, s, 2,
		Mul(NewBoundSymbol("a", p0), NewBoundSymbol("b", p1)))
	x := NewBoundSymbol("x", p0)
	y := NewBoundSymbol("y", p1)
	base := Mul(x, y)
	in := mustStoreExpr(t, s, Pow(base, MustRational(-1, 1)))
	cond := neqCond(t, s, base)
	res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in, fnRef},
		[]artifact.ArtifactRef{cond}, diffInvoke(t, s, fnRef, p0, 1))
	// Hand-derived: -1 * base^-2 * d(base)/dx, with d(x*y)/dx = x*0+1*y
	// in canonical construction order before rule-level canonicalization.
	dBase, _ := Canonicalize(Add(Mul(x, MustRational(0, 1)), Mul(MustRational(1, 1), y)))
	want, _ := Canonicalize(Mul(MustRational(-1, 1), Pow(cloneExpr(base), MustRational(-2, 1)), dBase))
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(want) {
		t.Fatal("composite-base negative Pow wrong")
	}
	if err := Replay(s, Derivation{
		PremiseRefs: []artifact.ArtifactRef{in, fnRef},
		Steps: []DerivationStep{{
			Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleDifferentiate,
			InputRefs: []artifact.ArtifactRef{in, fnRef}, OutputRef: res.OutputRef,
			ConditionRefs: []artifact.ArtifactRef{cond}, Parameters: diffInvoke(t, s, fnRef, p0, 1),
		}},
		ResultRefs: []artifact.ArtifactRef{res.OutputRef},
	}, nil); err != nil {
		t.Fatalf("composite negative-pow derivation does not replay: %v", err)
	}
}

// TestDifferentiateNegativePowConstantBase: d(2^-3) = 0 via the constant
// base derivative, after satisfying the NEQ(2,0) condition.
func TestDifferentiateNegativePowConstantBase(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	fnRef, _ := mustStoreFunction(t, s, 1, NewBoundSymbol("a", p0))
	two := MustRational(2, 1)
	in := mustStoreExpr(t, s, Pow(two, MustRational(-3, 1)))
	cond := neqCond(t, s, two)
	res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in, fnRef},
		[]artifact.ArtifactRef{cond}, diffInvoke(t, s, fnRef, p0, 1))
	simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
	if got := mustResolve(t, s, simp.OutputRef); !got.Equal(MustRational(0, 1)) {
		t.Fatal("constant-base negative Pow did not differentiate to 0")
	}
}

// TestDifferentiateNegativePowOrderTwo: d2(x^-1) = 2*x^-3 by hand.
func TestDifferentiateNegativePowOrderTwo(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	fnRef, _ := mustStoreFunction(t, s, 1, NewBoundSymbol("a", p0))
	x := NewBoundSymbol("x", p0)
	in := mustStoreExpr(t, s, Pow(x, MustRational(-1, 1)))
	cond := neqCond(t, s, x)
	res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in, fnRef},
		[]artifact.ArtifactRef{cond}, diffInvoke(t, s, fnRef, p0, 2))
	simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
	// Hand-derived: d(-1*x^-2) = -1*(-2*x^-3) up to order; SIMPLIFY strips
	// identities but does NOT gather -1*-2 into 2 (Pow child blocks the
	// all-rational fold) — an explicit non-goal, pinned here.
	want, _ := Canonicalize(Mul(MustRational(-1, 1), MustRational(-2, 1), Pow(x, MustRational(-3, 1))))
	if got := mustResolve(t, s, simp.OutputRef); !got.Equal(want) {
		t.Fatal("order-2 negative Pow wrong")
	}
}

// TestDifferentiatePowStillRejectsNonInteger: 1/2 exponents stay out of scope.
func TestDifferentiatePowStillRejectsNonInteger(t *testing.T) {
	s := NewMemStore()
	p0 := DeriveParameterID(0)
	fnRef, _ := mustStoreFunction(t, s, 1, NewBoundSymbol("a", p0))
	x := NewBoundSymbol("x", p0)
	half := mustStoreExpr(t, s, Pow(x, MustRational(1, 2)))
	cond := neqCond(t, s, x)
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleDifferentiate, InputRefs: []artifact.ArtifactRef{half, fnRef},
		ConditionRefs: []artifact.ArtifactRef{cond},
		Parameters:    diffInvoke(t, s, fnRef, p0, 1)}); !errors.Is(err, ErrBadInput) {
		t.Fatalf("non-integer Pow exponent accepted: %v", err)
	}
}
