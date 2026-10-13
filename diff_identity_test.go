package physmath

import (
	"errors"
	"strings"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// These tests pin the differentiation-variable identity contract added to
// DIFFERENTIATE-001 and TOTAL-DERIVATIVE-001 (shared checkDiffTarget):
// trusted differentiation requires the declaring Function to record display
// names, and every target-PID occurrence in the (expanded) target tree must
// display-match the declared name. Vacuous differentiation of an absent
// variable stays valid; body membership is never consulted; the residual
// (display, PID) indistinguishability under different declaring functions is
// documented behavior, tested as such.

// diffApply runs DIFFERENTIATE-001 and returns its error (nil on success).
func diffApply(s Store, inputs []artifact.ArtifactRef, params []byte) error {
	_, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
		RuleID: RuleDifferentiate, InputRefs: inputs, Parameters: params})
	return err
}

// wrongVarReason reports whether err is the wrong-variable rejection from
// checkDiffTarget (as opposed to an unrelated failure with the same class).
func wrongVarReason(err error) bool {
	return errors.Is(err, ErrBadInput) && strings.Contains(err.Error(), "does not denote declared variable")
}

// legacyFunction mints a nameless (legacy-layout) function/1 artifact.
func legacyFunction(t *testing.T, s Store, body Expr) artifact.ArtifactRef {
	t.Helper()
	ids := []ParameterID{DeriveParameterID(0)}
	bodyRef := mustStoreExpr(t, s, body)
	fn := Function{ParameterIDs: ids, BodyRef: bodyRef}
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

// TestDifferentiateIdentityRejections: wrong-variable requests fail closed at
// the rule boundary with the intended reason.
func TestDifferentiateIdentityRejections(t *testing.T) {
	t.Run("wrong-display-root", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		fn, _ := mustStoreFunction(t, s, []string{"x"}, NewBoundSymbol("x", p0))
		target := mustStoreExpr(t, s, NewBoundSymbol("q", p0))
		err := diffApply(s, []artifact.ArtifactRef{target, fn}, diffInvoke(t, s, fn, p0, 1))
		if !wrongVarReason(err) {
			t.Fatalf("same-PID foreign display accepted (or wrong reason): %v", err)
		}
	})

	t.Run("wrong-display-nested", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		fn, _ := mustStoreFunction(t, s, []string{"x"}, NewBoundSymbol("x", p0))
		bad := NewBoundSymbol("q", p0)
		rel, err := Relation(OpEQ, bad, MustRational(1, 1))
		if err != nil {
			t.Fatal(err)
		}
		cases := map[string]Expr{
			"add-mul-pow": Add(Mul(Pow(bad, MustRational(2, 1)), MustRational(3, 1)), MustRational(1, 1)),
			"sqrt":        Sqrt(bad),
			"neg":         Neg(bad),
			"relation":    rel,
			"branch":      BranchSet(bad, NewBoundSymbol("w", p0)),
		}
		for name, e := range cases {
			if err := diffApply(s, []artifact.ArtifactRef{mustStoreExpr(t, s, e), fn},
				diffInvoke(t, s, fn, p0, 1)); !wrongVarReason(err) {
				t.Fatalf("%s: nested wrong display accepted (or wrong reason): %v", name, err)
			}
		}
	})

	t.Run("missing-param-names", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		legacy := legacyFunction(t, s, NewBoundSymbol("x", p0))
		target := mustStoreExpr(t, s, NewBoundSymbol("x", p0))
		err := diffApply(s, []artifact.ArtifactRef{target, legacy}, diffInvoke(t, s, legacy, p0, 1))
		if !errors.Is(err, ErrMissingIdentity) {
			t.Fatalf("nameless Function accepted at trusted boundary: %v", err)
		}
	})

	t.Run("undeclared-target-pid", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		p1 := DeriveParameterID(1)
		fn, _ := mustStoreFunction(t, s, []string{"x"}, NewBoundSymbol("x", p0))
		target := mustStoreExpr(t, s, NewBoundSymbol("x", p0))
		params := append(BoundParameterRef{fn, p1}.Encode(), artifact.EncodeUInt32(1)...)
		if err := diffApply(s, []artifact.ArtifactRef{target, fn}, params); !errors.Is(err, ErrBadInput) {
			t.Fatalf("undeclared target PID accepted: %v", err)
		}
	})

	t.Run("pid-reuse-foreign-vocabulary", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		// Same ordinal PID, two declaring functions, two vocabularies.
		fnX, _ := mustStoreFunction(t, s, []string{"x"}, NewBoundSymbol("x", p0))
		fnT, _ := mustStoreFunction(t, s, []string{"t"}, NewBoundSymbol("t", p0))
		target := mustStoreExpr(t, s, NewBoundSymbol("x", p0))
		err := diffApply(s, []artifact.ArtifactRef{target, fnT}, diffInvoke(t, s, fnT, p0, 1))
		if !wrongVarReason(err) {
			t.Fatalf("x(pid0) differentiated under t-vocabulary (or wrong reason): %v", err)
		}
		res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{target, fnX}, nil,
			diffInvoke(t, s, fnX, p0, 1))
		if got := mustResolve(t, s, res.OutputRef); !got.Equal(MustRational(1, 1)) {
			t.Fatalf("x(pid0) under its own x-vocabulary wrong: %v", got)
		}
	})

	t.Run("name-count-mismatch-on-decode", func(t *testing.T) {
		s := NewMemStore()
		bodyRef := mustStoreExpr(t, s, NewBoundSymbol("x", DeriveParameterID(0)))
		pid := DeriveParameterID(0)
		raw := pid
		payload := append(artifact.EncodeSequence([][]byte{raw[:]}), bodyRef.Encode()...)
		xn, err := artifact.EncodeString("x")
		if err != nil {
			t.Fatal(err)
		}
		yn, err := artifact.EncodeString("y")
		if err != nil {
			t.Fatal(err)
		}
		payload = append(payload, artifact.EncodeSequence([][]byte{xn, yn})...)
		if _, err := DecodeFunction(artifact.NewDecoder(payload)); err == nil ||
			!strings.Contains(err.Error(), "ParamNames") {
			t.Fatalf("name count mismatch decoded: %v", err)
		}
		_ = s
	})

	t.Run("total-derivative-substituted-collision", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		calFn, _ := mustStoreFunction(t, s, []string{"q"},
			Pow(NewBoundSymbol("q", p0), MustRational(2, 1)))
		timeFn, _ := mustStoreFunction(t, s, []string{"t"}, NewBoundSymbol("t", p0))
		// Call argument carries the time PID under a foreign display; the
		// conflict appears only after expansion substitutes it into the body.
		in := mustStoreExpr(t, s, Call(calFn, NewBoundSymbol("u", p0)))
		_, err := Apply(s, Invocation{ProducerNamespace: NamespacePhysMath,
			RuleID: RuleTotalDerivative, InputRefs: []artifact.ArtifactRef{in, timeFn},
			Parameters: totalParams(timeFn, p0, 1)})
		if !wrongVarReason(err) {
			t.Fatalf("substituted foreign display accepted (or wrong reason): %v", err)
		}
	})
}

// TestDifferentiateIdentityAcceptances: the check narrows only the
// wrong-variable class; vacuous and independent-quantity cases stay valid.
func TestDifferentiateIdentityAcceptances(t *testing.T) {
	t.Run("vacuous-zero-regression", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		fn, _ := mustStoreFunction(t, s, []string{"x"}, NewBoundSymbol("x", p0))
		// Variable-independent target: no target-PID occurrence at all.
		in := mustStoreExpr(t, s, Add(NewEphemeralSymbol("omega"), MustRational(5, 1)))
		res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in, fn}, nil,
			diffInvoke(t, s, fn, p0, 1))
		if len(res.ConditionRefs) != 0 {
			t.Fatalf("vacuous differentiation manufactured conditions: %d", len(res.ConditionRefs))
		}
		simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
		if got := mustResolve(t, s, simp.OutputRef); !got.Equal(MustRational(0, 1)) {
			t.Fatalf("variable-independent target did not differentiate to 0: %v", got)
		}
	})

	t.Run("off-coordinate-polar-zero", func(t *testing.T) {
		s := NewMemStore()
		// Polar vocabulary: metric components depending only on r, asked
		// for the theta derivative.
		polar, _ := mustStoreFunction(t, s, []string{"r", "theta"}, NewBoundSymbol("r", DeriveParameterID(0)))
		r := NewBoundSymbol("r", DeriveParameterID(0))
		thetaPID := DeriveParameterID(1)
		gth := mustStoreExpr(t, s, Pow(r, MustRational(2, 1)))
		res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{gth, polar}, nil,
			diffInvoke(t, s, polar, thetaPID, 1))
		simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
		if got := mustResolve(t, s, simp.OutputRef); !got.Equal(MustRational(0, 1)) {
			t.Fatalf("off-coordinate component d(theta) is not 0: %v", got)
		}
		// Constant component (g_rr = 1) likewise.
		grr := mustStoreExpr(t, s, MustRational(1, 1))
		res2 := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{grr, polar}, nil,
			diffInvoke(t, s, polar, thetaPID, 1))
		simp2 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res2.OutputRef}, nil, nil)
		if got := mustResolve(t, s, simp2.OutputRef); !got.Equal(MustRational(0, 1)) {
			t.Fatalf("constant component d(theta) is not 0: %v", got)
		}
	})

	t.Run("legitimate-shared-identity-zero", func(t *testing.T) {
		s := NewMemStore()
		p0, p1 := DeriveParameterID(0), DeriveParameterID(1)
		fn, _ := mustStoreFunction(t, s, []string{"x", "y"},
			Mul(NewBoundSymbol("x", p0), NewBoundSymbol("y", p1)))
		// A non-target PID under a foreign display: an independent
		// quantity. The check never examines non-target PIDs.
		in := mustStoreExpr(t, s, Add(NewBoundSymbol("z", p1), MustRational(4, 1)))
		res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in, fn}, nil,
			diffInvoke(t, s, fn, p0, 1))
		simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
		if got := mustResolve(t, s, simp.OutputRef); !got.Equal(MustRational(0, 1)) {
			t.Fatalf("independent quantity did not differentiate to 0: %v", got)
		}
	})

	t.Run("total-derivative-matching-display", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		calFn, _ := mustStoreFunction(t, s, []string{"q"},
			Pow(NewBoundSymbol("q", p0), MustRational(2, 1)))
		timeFn, _ := mustStoreFunction(t, s, []string{"t"}, NewBoundSymbol("t", p0))
		tsym := NewBoundSymbol("t", p0)
		in := mustStoreExpr(t, s, Call(calFn, tsym))
		res := applyRule(t, s, RuleTotalDerivative, []artifact.ArtifactRef{in, timeFn}, nil,
			totalParams(timeFn, p0, 1))
		simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
		want, err := Canonicalize(Mul(MustRational(2, 1), tsym))
		if err != nil {
			t.Fatal(err)
		}
		if got := mustResolve(t, s, simp.OutputRef); !got.Equal(want) {
			t.Fatalf("matching-display total derivative wrong: %v", got)
		}
	})
}

// TestDifferentiateIdentityReplay: replay re-executes the rule, so the same
// contract holds on verification, with the intended failure reason.
func TestDifferentiateIdentityReplay(t *testing.T) {
	mkDerivation := func(in, fn artifact.ArtifactRef, out artifact.ArtifactRef, params []byte) Derivation {
		return Derivation{
			PremiseRefs: []artifact.ArtifactRef{in, fn},
			Steps: []DerivationStep{{
				Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleDifferentiate,
				InputRefs: []artifact.ArtifactRef{in, fn}, OutputRef: out, Parameters: params,
			}},
			ResultRefs: []artifact.ArtifactRef{out},
		}
	}

	t.Run("missing-identity-fn", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		legacy := legacyFunction(t, s, NewBoundSymbol("x", p0))
		in := mustStoreExpr(t, s, NewBoundSymbol("x", p0))
		err := Replay(s, mkDerivation(in, legacy, in, diffInvoke(t, s, legacy, p0, 1)), nil)
		if !errors.Is(err, ErrMissingIdentity) {
			t.Fatalf("replay accepted nameless declaring Function: %v", err)
		}
	})

	t.Run("wrong-variable-step", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		fn, _ := mustStoreFunction(t, s, []string{"x"}, NewBoundSymbol("x", p0))
		in := mustStoreExpr(t, s, NewBoundSymbol("q", p0))
		err := Replay(s, mkDerivation(in, fn, in, diffInvoke(t, s, fn, p0, 1)), nil)
		if !wrongVarReason(err) {
			t.Fatalf("replay accepted wrong-variable step (or wrong reason): %v", err)
		}
	})

	t.Run("tampered-output-step", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		fn, _ := mustStoreFunction(t, s, []string{"x"}, NewBoundSymbol("x", p0))
		in := mustStoreExpr(t, s, Pow(NewBoundSymbol("x", p0), MustRational(2, 1)))
		params := diffInvoke(t, s, fn, p0, 1)
		res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{in, fn}, nil, params)
		if res.OutputRef == in {
			t.Fatal("fixture: derivative unexpectedly equals input")
		}
		// Valid rule invocation, declared output replaced by another
		// available expression: only the output-equality check catches it.
		err := Replay(s, mkDerivation(in, fn, in, params), nil)
		if err == nil || !strings.Contains(err.Error(), "output mismatch") {
			t.Fatalf("tampered differentiation output accepted: %v", err)
		}
		_ = res
	})
}

// TestDifferentiateIdentityResidualLimitation pins the DOCUMENTED boundary:
// (display, PID) bytes are ordinal-per-function and globally reused, so two
// distinct declaring functions that both record the same name for the same
// ordinal PID are indistinguishable to this check. This is accepted
// behavior, not an oversight; scope-enriching symbol identity is out of
// scope for this contract.
func TestDifferentiateIdentityResidualLimitation(t *testing.T) {
	t.Run("identical-display-pid-bytes-indistinguishable", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		refA, _ := mustStoreFunction(t, s, []string{"x"},
			Add(NewBoundSymbol("x", p0), MustRational(1, 1)))
		refB, _ := mustStoreFunction(t, s, []string{"x"},
			Pow(NewBoundSymbol("x", p0), MustRational(2, 1)))
		if refA == refB {
			t.Fatal("fixture: distinct declaring functions collided")
		}
		target := mustStoreExpr(t, s, NewBoundSymbol("x", p0))
		// The very same target bytes are accepted under both functions.
		for name, fnRef := range map[string]artifact.ArtifactRef{"fnA": refA, "fnB": refB} {
			res := applyRule(t, s, RuleDifferentiate, []artifact.ArtifactRef{target, fnRef}, nil,
				diffInvoke(t, s, fnRef, p0, 1))
			if got := mustResolve(t, s, res.OutputRef); !got.Equal(MustRational(1, 1)) {
				t.Fatalf("%s: identical (display, PID) bytes not accepted: %v", name, got)
			}
		}
	})
}

// TestFunctionParamNamesCodec: optional trailing name sequence round-trips;
// legacy layout stays decodable; malformed name data fails closed.
func TestFunctionParamNamesCodec(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		s := NewMemStore()
		p0, p1 := DeriveParameterID(0), DeriveParameterID(1)
		bodyRef := mustStoreExpr(t, s, Mul(NewBoundSymbol("x", p0), NewBoundSymbol("y", p1)))
		fn := Function{ParameterIDs: []ParameterID{p0, p1},
			ParamNames: []string{"x", "y"}, BodyRef: bodyRef}
		payload, err := fn.Encode()
		if err != nil {
			t.Fatal(err)
		}
		ref, err := s.Put(FamilyFunction, payload)
		if err != nil {
			t.Fatal(err)
		}
		_, stored, err := s.Lookup(ref)
		if err != nil {
			t.Fatal(err)
		}
		dec, err := DecodeFunction(artifact.NewDecoder(stored))
		if err != nil {
			t.Fatalf("named function does not decode: %v", err)
		}
		if len(dec.ParamNames) != 2 || dec.ParamNames[0] != "x" || dec.ParamNames[1] != "y" {
			t.Fatalf("ParamNames changed in round-trip: %v", dec.ParamNames)
		}
		if dec.ParameterIDs[0] != p0 || dec.ParameterIDs[1] != p1 || dec.BodyRef != bodyRef {
			t.Fatal("function identity changed in round-trip")
		}
	})

	t.Run("legacy-absent-sequence", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		bodyRef := mustStoreExpr(t, s, NewBoundSymbol("x", p0))
		fn := Function{ParameterIDs: []ParameterID{p0}, BodyRef: bodyRef}
		payload, err := fn.Encode()
		if err != nil {
			t.Fatal(err)
		}
		dec, err := DecodeFunction(artifact.NewDecoder(payload))
		if err != nil {
			t.Fatalf("legacy layout does not decode: %v", err)
		}
		if dec.ParamNames != nil {
			t.Fatalf("legacy payload gained names: %v", dec.ParamNames)
		}
		if err := dec.Validate(); err != nil {
			t.Fatalf("legacy payload no longer validates: %v", err)
		}
	})

	t.Run("encode-length-mismatch", func(t *testing.T) {
		p0, p1 := DeriveParameterID(0), DeriveParameterID(1)
		fn := Function{ParameterIDs: []ParameterID{p0, p1}, ParamNames: []string{"x"}}
		if _, err := fn.Encode(); err == nil || !strings.Contains(err.Error(), "ParamNames") {
			t.Fatalf("mismatched name count encoded: %v", err)
		}
	})

	t.Run("encode-and-decode-empty-name", func(t *testing.T) {
		s := NewMemStore()
		p0 := DeriveParameterID(0)
		bodyRef := mustStoreExpr(t, s, NewBoundSymbol("x", p0))
		if _, err := (Function{ParameterIDs: []ParameterID{p0},
			ParamNames: []string{""}, BodyRef: bodyRef}).Encode(); err == nil ||
			!strings.Contains(err.Error(), "empty ParamNames") {
			t.Fatalf("empty declared name encoded: %v", err)
		}
		raw := p0
		payload := append(artifact.EncodeSequence([][]byte{raw[:]}), bodyRef.Encode()...)
		empty, err := artifact.EncodeString("")
		if err != nil {
			t.Fatal(err)
		}
		payload = append(payload, artifact.EncodeSequence([][]byte{empty})...)
		if _, err := DecodeFunction(artifact.NewDecoder(payload)); err == nil ||
			!strings.Contains(err.Error(), "empty ParamNames") {
			t.Fatalf("empty declared name decoded: %v", err)
		}
	})
}
