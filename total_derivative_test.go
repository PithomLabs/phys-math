package physmath

import (
	"errors"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// mintFunction mints a function/1 artifact with explicit parameter IDs.
func mintFunction(t *testing.T, s Store, ids []ParameterID, body Expr) artifact.ArtifactRef {
	t.Helper()
	bodyRef := mustStoreExpr(t, s, body)
	fn := Function{ParameterIDs: ids, BodyRef: bodyRef}
	if err := ValidateFunctionBody(fn, body); err != nil {
		t.Fatal(err)
	}
	payload, err := fn.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyFunction, payload)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// timeSetup returns a declaring time function for t plus the bound symbol.
func timeSetup(t *testing.T, s Store, pid ParameterID) (artifact.ArtifactRef, Expr) {
	t.Helper()
	tsym := NewBoundSymbol("t", pid)
	return mintFunction(t, s, []ParameterID{pid}, tsym), tsym
}

func totalParams(fnRef artifact.ArtifactRef, pid ParameterID, order uint32) []byte {
	return append(BoundParameterRef{fnRef, pid}.Encode(), artifact.EncodeUInt32(order)...)
}

// TestTotalDerivativeChain: D_t[A(t)] with A(p) = p^2 gives 2t by hand.
func TestTotalDerivativeChain(t *testing.T) {
	s := NewMemStore()
	pt := DeriveParameterID(0)
	pa := DeriveParameterID(0)
	timeFn, tsym := timeSetup(t, s, pt)
	asym := NewBoundSymbol("a", pa)
	afn := mintFunction(t, s, []ParameterID{pa}, Pow(asym, MustRational(2, 1)))
	call := Call(afn, tsym)
	in := mustStoreExpr(t, s, call)
	res := applyRule(t, s, RuleTotalDerivative, []artifact.ArtifactRef{in, timeFn}, nil,
		totalParams(timeFn, pt, 1))
	// Hand-derived: expand A(t) -> t^2, then d/dt -> 2*t^1*1 (CANON
	// preserves Pow-one/Mul-one identities for SIMPLIFY).
	want, _ := Canonicalize(Mul(MustRational(2, 1), Pow(cloneExpr(tsym), MustRational(1, 1)), MustRational(1, 1)))
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(want) {
		t.Fatal("total-derivative chain wrong")
	}
	// Determinism across recomputation.
	res2, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleTotalDerivative, InputRefs: []artifact.ArtifactRef{in, timeFn},
		Parameters: totalParams(timeFn, pt, 1)})
	if err != nil {
		t.Fatal(err)
	}
	if res2.OutputRef != res.OutputRef {
		t.Fatal("total-derivative recomputation nondeterministic")
	}
	if err := Replay(s, Derivation{
		PremiseRefs: []artifact.ArtifactRef{in, timeFn},
		Steps: []DerivationStep{{
			Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleTotalDerivative,
			InputRefs: []artifact.ArtifactRef{in, timeFn}, OutputRef: res.OutputRef,
			Parameters: totalParams(timeFn, pt, 1),
		}},
		ResultRefs: []artifact.ArtifactRef{res.OutputRef},
	}, nil); err != nil {
		t.Fatalf("total-derivative derivation does not replay: %v", err)
	}
}

// TestTotalDerivativeNested: D_t[C(A(t))] with C(p) = -p gives -(2t).
func TestTotalDerivativeNested(t *testing.T) {
	s := NewMemStore()
	pt := DeriveParameterID(0)
	pa := DeriveParameterID(0)
	pc := DeriveParameterID(0)
	timeFn, tsym := timeSetup(t, s, pt)
	asym := NewBoundSymbol("a", pa)
	afn := mintFunction(t, s, []ParameterID{pa}, Pow(asym, MustRational(2, 1)))
	csym := NewBoundSymbol("c", pc)
	cfn := mintFunction(t, s, []ParameterID{pc}, Neg(csym))
	inner := Call(afn, tsym)
	outer := Call(cfn, inner)
	in := mustStoreExpr(t, s, outer)
	res := applyRule(t, s, RuleTotalDerivative, []artifact.ArtifactRef{in, timeFn}, nil,
		totalParams(timeFn, pt, 1))
	// Hand-derived: expand C -> -(t^2), then d/dt -> -(2*t^1*1). CANON
	// keeps the Mul-one and Pow-one identities (SIMPLIFY's job to strip).
	inner2, _ := Canonicalize(Mul(MustRational(2, 1), Pow(cloneExpr(tsym), MustRational(1, 1)), MustRational(1, 1)))
	want, _ := Canonicalize(Neg(inner2))
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(want) {
		t.Fatal("nested total-derivative wrong")
	}
}

// TestTotalDerivativeOrderTwo: D_t^2[A(t)] with A(p) = p^2 gives 2.
func TestTotalDerivativeOrderTwo(t *testing.T) {
	s := NewMemStore()
	pt := DeriveParameterID(0)
	pa := DeriveParameterID(0)
	timeFn, tsym := timeSetup(t, s, pt)
	asym := NewBoundSymbol("a", pa)
	afn := mintFunction(t, s, []ParameterID{pa}, Pow(asym, MustRational(2, 1)))
	in := mustStoreExpr(t, s, Call(afn, tsym))
	// Higher orders manufacture Pow(t,0) intermediates (d(t^1)/dt), so the
	// exact NEQ(t,0) condition is required for full simplification.
	cond := neqCond(t, s, tsym)
	res := applyRule(t, s, RuleTotalDerivative, []artifact.ArtifactRef{in, timeFn},
		[]artifact.ArtifactRef{cond}, totalParams(timeFn, pt, 2))
	simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef},
		[]artifact.ArtifactRef{cond}, nil)
	if got := mustResolve(t, s, simp.OutputRef); !got.Equal(MustRational(2, 1)) {
		t.Fatal("second total derivative wrong")
	}
}

// TestTotalDerivativeQuotient: D_t[A(t)^3 / N(t)] with identity-shaped
// A, N exercises product + negative-power chain with exact conditions.
func TestTotalDerivativeQuotient(t *testing.T) {
	s := NewMemStore()
	pt := DeriveParameterID(0)
	pa := DeriveParameterID(0)
	pn := DeriveParameterID(0)
	timeFn, tsym := timeSetup(t, s, pt)
	asym := NewBoundSymbol("a", pa)
	afn := mintFunction(t, s, []ParameterID{pa}, asym)
	nsym := NewBoundSymbol("n", pn)
	nfn := mintFunction(t, s, []ParameterID{pn}, nsym)
	aCall := Call(afn, tsym)
	nCall := Call(nfn, tsym)
	q := Mul(Pow(aCall, MustRational(3, 1)), Pow(nCall, MustRational(-1, 1)))
	in := mustStoreExpr(t, s, q)
	// Conditions apply to the expanded tree: the Call-form NEQ(N(t),0) does
	// NOT satisfy the post-expansion base t (exactness boundary preserved);
	// the honest requirement is NEQ(t,0).
	callForm := neqCond(t, s, nCall)
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleTotalDerivative, InputRefs: []artifact.ArtifactRef{in, timeFn},
		ConditionRefs: []artifact.ArtifactRef{callForm},
		Parameters:    totalParams(timeFn, pt, 1)}); !errors.Is(err, ErrMissingCondition) {
		t.Fatalf("pre-expansion condition discharged: %v", err)
	}
	cond := neqCond(t, s, tsym)
	res := applyRule(t, s, RuleTotalDerivative, []artifact.ArtifactRef{in, timeFn},
		[]artifact.ArtifactRef{cond}, totalParams(timeFn, pt, 1))
	// Hand-derived over the expanded tree (identity A, N expand Calls to t):
	// D[t^3]*t^-1 + t^3*D[t^-1] = (3*t^2*1)*t^-1 + t^3*(-1*t^-2*1).
	dA, _ := Canonicalize(Mul(MustRational(3, 1), Pow(cloneExpr(tsym), MustRational(2, 1)), MustRational(1, 1)))
	dN, _ := Canonicalize(Mul(MustRational(-1, 1), Pow(cloneExpr(tsym), MustRational(-2, 1)), MustRational(1, 1)))
	want, _ := Canonicalize(Add(
		Mul(dA, Pow(cloneExpr(tsym), MustRational(-1, 1))),
		Mul(Pow(cloneExpr(tsym), MustRational(3, 1)), dN),
	))
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(want) {
		t.Fatal("quotient total-derivative wrong")
	}
	if len(res.ConditionRefs) != 1 || res.ConditionRefs[0] != cond {
		t.Fatal("condition not threaded through total-derivative result")
	}
	if err := Replay(s, Derivation{
		PremiseRefs: []artifact.ArtifactRef{in, timeFn},
		Steps: []DerivationStep{{
			Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleTotalDerivative,
			InputRefs: []artifact.ArtifactRef{in, timeFn}, OutputRef: res.OutputRef,
			ConditionRefs: []artifact.ArtifactRef{cond}, Parameters: totalParams(timeFn, pt, 1),
		}},
		ResultRefs: []artifact.ArtifactRef{res.OutputRef},
	}, nil); err != nil {
		t.Fatalf("quotient total-derivative does not replay: %v", err)
	}
	// Without the condition: fail closed at the negative power.
	if _, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleTotalDerivative, InputRefs: []artifact.ArtifactRef{in, timeFn},
		Parameters: totalParams(timeFn, pt, 1)}); !errors.Is(err, ErrMissingCondition) {
		t.Fatalf("quotient without NEQ condition did not fail closed: %v", err)
	}
}

// TestTotalDerivativeNonCallRoot: bare expressions differentiate exactly
// like the partial rule (boundary: total == partial off applications).
func TestTotalDerivativeNonCallRoot(t *testing.T) {
	s := NewMemStore()
	pt := DeriveParameterID(0)
	timeFn, tsym := timeSetup(t, s, pt)
	in := mustStoreExpr(t, s, Add(tsym, tsym))
	res := applyRule(t, s, RuleTotalDerivative, []artifact.ArtifactRef{in, timeFn}, nil,
		totalParams(timeFn, pt, 1))
	if got := mustResolve(t, s, res.OutputRef); !got.Equal(Add(MustRational(1, 1), MustRational(1, 1))) {
		t.Fatal("non-Call total derivative diverges from partial rule")
	}
}

// TestTotalDerivativeNegatives pins each rejection boundary.
func TestTotalDerivativeNegatives(t *testing.T) {
	s := NewMemStore()
	pt := DeriveParameterID(0)
	pa := DeriveParameterID(0)
	timeFn, tsym := timeSetup(t, s, pt)
	asym := NewBoundSymbol("a", pa)
	afn := mintFunction(t, s, []ParameterID{pa}, Pow(asym, MustRational(2, 1)))
	in := mustStoreExpr(t, s, Call(afn, tsym))
	bad := func(params []byte, inputs []artifact.ArtifactRef, conds []artifact.ArtifactRef) error {
		_, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
			RuleID: RuleTotalDerivative, InputRefs: inputs,
			ConditionRefs: conds, Parameters: params})
		return err
	}
	// BPR bound to another function.
	otherFn, _ := mustStoreFunction(t, s, 1, NewBoundSymbol("v", pt))
	otherBPR := append(BoundParameterRef{otherFn, pt}.Encode(), artifact.EncodeUInt32(1)...)
	if err := bad(otherBPR, []artifact.ArtifactRef{in, timeFn}, nil); err == nil {
		t.Fatal("mismatched BPR accepted")
	}
	// Non-function second input.
	if err := bad(totalParams(timeFn, pt, 1), []artifact.ArtifactRef{in, in}, nil); err == nil {
		t.Fatal("non-function declaring input accepted")
	}
	// Order 0.
	zeroOrder := append(BoundParameterRef{timeFn, pt}.Encode(), artifact.EncodeUInt32(0)...)
	if err := bad(zeroOrder, []artifact.ArtifactRef{in, timeFn}, nil); err == nil {
		t.Fatal("order 0 accepted")
	}
	// Trailing parameter bytes.
	trailing := append(totalParams(timeFn, pt, 1), 0x00)
	if err := bad(trailing, []artifact.ArtifactRef{in, timeFn}, nil); err == nil {
		t.Fatal("trailing parameter bytes accepted")
	}
	// Wrong arity.
	if err := bad(totalParams(timeFn, pt, 1), []artifact.ArtifactRef{in}, nil); err == nil {
		t.Fatal("wrong arity accepted")
	}
	// Sqrt of a Call: expansion succeeds, Sqrt differentiation fails.
	sqIn := mustStoreExpr(t, s, Sqrt(Call(afn, tsym)))
	if err := bad(totalParams(timeFn, pt, 1), []artifact.ArtifactRef{sqIn, timeFn}, nil); err == nil {
		t.Fatal("Sqrt total differentiation accepted")
	}
	// Recursive expansion is unrepresentable: content-addressed refs form a
	// DAG (a reference cycle would require a hash-preimage cycle), so no
	// re-entry guard exists and none is tested. Arity mismatch still fails.
	arityBad := Call(afn, tsym, tsym)
	arityIn := mustStoreExpr(t, s, arityBad)
	if err := bad(totalParams(timeFn, pt, 1), []artifact.ArtifactRef{arityIn, timeFn}, nil); err == nil {
		t.Fatal("arity-mismatched Call accepted")
	}
}

// TestTotalDerivativeConservationShape builds the paper-flavored scalar
// conservation combination (1/N)·D_t[a³/N]-style from Call nodes: the
// quotient executes with replay while the opaque-potential term stays
// represented. U as a bare ephemeral symbol differentiates to 0 as an
// independent quantity — which is precisely why U(φ) must remain an
// opaque assumption rather than a differentiated object.
func TestTotalDerivativeConservationShape(t *testing.T) {
	s := NewMemStore()
	pt := DeriveParameterID(0)
	pa := DeriveParameterID(0)
	pn := DeriveParameterID(0)
	timeFn, tsym := timeSetup(t, s, pt)
	asym := NewBoundSymbol("a", pa)
	afn := mintFunction(t, s, []ParameterID{pa}, asym)
	nsym := NewBoundSymbol("n", pn)
	nfn := mintFunction(t, s, []ParameterID{pn}, nsym)
	aCall := Call(afn, tsym)
	nCall := Call(nfn, tsym)
	inner := Mul(Pow(aCall, MustRational(3, 1)), Pow(nCall, MustRational(-1, 1)))
	innerRef := mustStoreExpr(t, s, inner)
	cond := neqCond(t, s, tsym)
	dInner := applyRule(t, s, RuleTotalDerivative, []artifact.ArtifactRef{innerRef, timeFn},
		[]artifact.ArtifactRef{cond}, totalParams(timeFn, pt, 1))
	// Outer (1/N)·D_t[·] construction from the executed output reference.
	outer := Mul(Pow(nCall, MustRational(-1, 1)), mustResolve(t, s, dInner.OutputRef))
	outerRef := mustStoreExpr(t, s, outer)
	dec, err := DecodeExpr(artifact.NewDecoder(mustPayloadBytes(t, s, outerRef)))
	if err != nil {
		t.Fatal(err)
	}
	if dec.Kind != TagMul || len(dec.Children) != 2 {
		t.Fatal("conservation combination is not a two-factor Mul")
	}
	if err := Replay(s, Derivation{
		PremiseRefs: []artifact.ArtifactRef{innerRef, timeFn},
		Steps: []DerivationStep{{
			Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleTotalDerivative,
			InputRefs: []artifact.ArtifactRef{innerRef, timeFn}, OutputRef: dInner.OutputRef,
			ConditionRefs: []artifact.ArtifactRef{cond}, Parameters: totalParams(timeFn, pt, 1),
		}},
		ResultRefs: []artifact.ArtifactRef{dInner.OutputRef},
	}, nil); err != nil {
		t.Fatalf("inner quotient derivation does not replay: %v", err)
	}
	// Opaque potential term: ephemeral U differentiates to 0 as an
	// independent quantity — recorded boundary, not a computed ∂U/∂φ.
	uIn := mustStoreExpr(t, s, NewEphemeralSymbol("Uphi"))
	uRes := applyRule(t, s, RuleTotalDerivative, []artifact.ArtifactRef{uIn, timeFn}, nil,
		totalParams(timeFn, pt, 1))
	if got := mustResolve(t, s, uRes.OutputRef); !got.Equal(MustRational(0, 1)) {
		t.Fatal("opaque symbol did not differentiate to 0")
	}
}

func mustPayloadBytes(t *testing.T, s Store, ref artifact.ArtifactRef) []byte {
	t.Helper()
	_, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
