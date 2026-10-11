package physmath

import (
	"errors"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// A4 hardening: the reserved covariant_derivative operator must fail
// closed at every execution entry point, even with perfect parameters.
// Representation admission must never decay into silent success.

// differentiateFixture builds target x^2+1 with declaring function f(x).
func differentiateFixture(t *testing.T, s Store) (arg, fnRef artifact.ArtifactRef, params []byte, targetID ParameterID) {
	t.Helper()
	p0 := DeriveParameterID(0)
	targetID = p0
	fn, _ := mustStoreFunction(t, s, 1, NewBoundSymbol("x", p0))
	target := mustStoreExpr(t, s, Add(
		Pow(NewBoundSymbol("x", p0), MustRational(2, 1)),
		MustRational(1, 1)))
	params = append(BoundParameterRef{fn, p0}.Encode(), artifact.EncodeUInt32(1)...)
	return target, fn, params, p0
}

func TestDifferentiateExecutesViaOperator(t *testing.T) {
	s := NewMemStore()
	arg, fnRef, params, targetID := differentiateFixture(t, s)
	op := OperatorApplication{OperatorID: "differentiate", Args: []artifact.ArtifactRef{arg}, Parameters: params}
	if !op.Executable() {
		t.Fatal("differentiate reports non-executable")
	}
	res, err := ExecuteOperator(s, op, fnRef)
	if err != nil {
		t.Fatalf("differentiate execution failed: %v", err)
	}
	// d/dx(x^2+1) yields Add(Mul(2,Pow(x,1),1),0) at the differentiate
	// layer (Canonicalize only); SIMPLIFY reduces it to 2*x. Composing
	// both rules is the honest executable path.
	simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{res.OutputRef}, nil, nil)
	got := mustResolve(t, s, simp.OutputRef)
	want := Mul(MustRational(2, 1), NewBoundSymbol("x", targetID))
	gotCanon, err := Canonicalize(got)
	if err != nil {
		t.Fatal(err)
	}
	wantCanon, err := Canonicalize(want)
	if err != nil {
		t.Fatal(err)
	}
	if !gotCanon.Equal(wantCanon) {
		t.Fatalf("differentiate result wrong: %v", got)
	}
}

func TestCovariantDerivativeSentinel(t *testing.T) {
	s := NewMemStore()
	arg, fnRef, _, _ := differentiateFixture(t, s)
	// Perfect shape: valid arg, empty params per slice-1 contract.
	op := OperatorApplication{OperatorID: "covariant_derivative", Args: []artifact.ArtifactRef{arg}}
	if err := op.Validate(); err != nil {
		t.Fatalf("reserved operator shape rejected: %v", err)
	}
	if op.Executable() {
		t.Fatal("covariant_derivative reports executable")
	}
	if _, err := ExecuteOperator(s, op, fnRef); !errors.Is(err, ErrOperatorNotExecutable) {
		t.Fatalf("reserved operator did not fail with sentinel: %v", err)
	}
	// Tensor target: representable, still never executable.
	tn := mintTensorRef(t, s, VarianceCovariant, VarianceContravariant)
	top := OperatorApplication{OperatorID: "covariant_derivative", Args: []artifact.ArtifactRef{tn}}
	if err := top.Validate(); err != nil {
		t.Fatalf("tensor-target operator shape rejected: %v", err)
	}
	fams := familiesMap(t, s, tn)
	if err := ValidateAgainst(top.TargetChecks(), fams); err != nil {
		t.Fatalf("tensor-target operator refs rejected: %v", err)
	}
	if _, err := ExecuteOperator(s, top, fnRef); !errors.Is(err, ErrOperatorNotExecutable) {
		t.Fatalf("tensor-target operator did not fail with sentinel: %v", err)
	}
	// Malformed shape still fails closed at validation (not sentinel).
	bad := OperatorApplication{OperatorID: "covariant_derivative",
		Args: []artifact.ArtifactRef{arg}, Parameters: []byte{0x01}}
	if err := bad.Validate(); err == nil {
		t.Fatal("parameterized covariant_derivative accepted")
	}
	if _, err := ExecuteOperator(s, bad, fnRef); errors.Is(err, ErrOperatorNotExecutable) {
		t.Fatal("malformed operator returned sentinel instead of validation error")
	}
	// Unknown operator IDs fail closed everywhere.
	unknown := OperatorApplication{OperatorID: "integrate", Args: []artifact.ArtifactRef{arg}}
	if err := unknown.Validate(); err == nil {
		t.Fatal("unknown operator validated")
	}
	if unknown.Executable() {
		t.Fatal("unknown operator reports executable")
	}
}

func TestOperatorCannotEnterDerivationSteps(t *testing.T) {
	s := NewMemStore()
	arg, _, _, _ := differentiateFixture(t, s)
	op := OperatorApplication{OperatorID: "covariant_derivative", Args: []artifact.ArtifactRef{arg}}
	opPayload, err := op.Encode()
	if err != nil {
		t.Fatal(err)
	}
	opRef, err := s.Put(FamilyOperatorApplication, opPayload)
	if err != nil {
		t.Fatal(err)
	}
	// Operator artifacts travel as premises (validated data). A step
	// invoking a non-rule pair is rejected by the rule lock: replay can
	// never execute an operator.
	d := Derivation{
		PremiseRefs: []artifact.ArtifactRef{arg, opRef},
		Steps: []DerivationStep{{
			Ordinal:           0,
			ProducerNamespace: NamespacePhysMath,
			RuleID:            "covariant_derivative",
			InputRefs:         []artifact.ArtifactRef{arg},
			OutputRef:         arg,
		}},
		ResultRefs: []artifact.ArtifactRef{arg},
	}
	if err := Replay(s, d, nil); err == nil {
		t.Fatal("replay executed a non-rule operator step")
	}
	// Zero-step derivation over operator premises replays structurally
	// (availability only) and executes nothing: inert-data semantics.
	// A premise-only derivation with no steps performs zero rule
	// invocations by construction; replay success here proves availability,
	// never verification of the reserved operator.
	inert := Derivation{
		PremiseRefs: []artifact.ArtifactRef{arg, opRef},
		ResultRefs:  []artifact.ArtifactRef{arg},
	}
	if err := Replay(s, inert, nil); err != nil {
		t.Fatalf("premise-only replay failed: %v", err)
	}
}
