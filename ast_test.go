package physmath

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// §25.8: nested Add/Mul with different insertion order → one canonical order.
func TestAddMulNormalization(t *testing.T) {
	x := NewEphemeralSymbol("x")
	y := NewEphemeralSymbol("y")
	z := NewEphemeralSymbol("z")
	a := Add(x, Add(y, z))
	b := Add(Add(z, y), x)
	ca, err := Canonicalize(a)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := Canonicalize(b)
	if err != nil {
		t.Fatal(err)
	}
	if !ca.Equal(cb) {
		t.Fatal("commutative structures lack one canonical child order")
	}
	ea, _ := ca.Encode()
	eb, _ := cb.Encode()
	if !bytes.Equal(ea, eb) {
		t.Fatal("canonical bytes differ")
	}
	// Flatten check: no nested Add remains.
	for _, c := range ca.Children {
		if c.Kind == TagAdd {
			t.Fatal("nested Add not flattened")
		}
	}
	// Arity normalization §7.6.
	empty, err := Canonicalize(Add())
	if err != nil {
		t.Fatal(err)
	}
	if !empty.Equal(MustRational(0, 1)) {
		t.Fatal("Add([]) != 0/1")
	}
	emptyM, err := Canonicalize(Mul())
	if err != nil {
		t.Fatal(err)
	}
	if !emptyM.Equal(MustRational(1, 1)) {
		t.Fatal("Mul([]) != 1/1")
	}
	single, err := Canonicalize(Add(x))
	if err != nil {
		t.Fatal(err)
	}
	if !single.Equal(x) {
		t.Fatal("Add([x]) != x")
	}
}

// §7.9: Sqrt is a distinct node; identity preserves the kind.
func TestSqrtDistinctness(t *testing.T) {
	x := NewEphemeralSymbol("x")
	s := Sqrt(x)
	p := Pow(x, MustRational(1, 2))
	if s.Equal(p) {
		t.Fatal("Sqrt collapsed into Pow(x,1/2)")
	}
	se, _ := s.Encode()
	if se[0] != TagSqrt {
		t.Fatalf("Sqrt tag = 0x%02x", se[0])
	}
}

// Durable symbol rule: exactly one of ParameterID/ConstantID.
func TestSymbolBindingRegimes(t *testing.T) {
	pid := DeriveParameterID(0)
	if err := ValidateSymbolBinding(NewBoundSymbol("g", pid)); err != nil {
		t.Fatalf("bound symbol rejected: %v", err)
	}
	if err := ValidateSymbolBinding(NewConstantSymbol("pi", ConstPi)); err != nil {
		t.Fatalf("constant symbol rejected: %v", err)
	}
	both := Expr{Kind: TagSymbol, DisplayName: "x", ParamID: &pid}
	c := ConstPi
	both.ConstantID = &c
	if err := ValidateSymbolBinding(both); err == nil {
		t.Fatal("both-present symbol accepted")
	}
	if err := ValidateSymbolBinding(NewEphemeralSymbol("x")); err == nil {
		t.Fatal("neither-present symbol accepted as durable")
	}
	// DisplayName is identity-bearing: same PID, different display → different bytes.
	a := NewBoundSymbol("t", pid)
	b := NewBoundSymbol("tau", pid)
	if a.Equal(b) {
		t.Fatal("DisplayName change did not alter identity")
	}
}

func TestRationalConstruction(t *testing.T) {
	r, err := NewRational(big.NewInt(2), big.NewInt(4))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Equal(MustRational(1, 2)) {
		t.Fatal("2/4 not reduced to 1/2")
	}
	neg, err := NewRational(big.NewInt(1), big.NewInt(-3))
	if err != nil {
		t.Fatal(err)
	}
	if !neg.Equal(MustRational(-1, 3)) {
		t.Fatal("denominator not normalized positive")
	}
	zero, err := NewRational(big.NewInt(0), big.NewInt(5))
	if err != nil {
		t.Fatal(err)
	}
	if !zero.Equal(MustRational(0, 1)) {
		t.Fatal("zero not 0/1")
	}
	if _, err := NewRational(big.NewInt(1), big.NewInt(0)); err == nil {
		t.Fatal("zero denominator accepted")
	}
	// Encoded non-reduced rational rejected on decode.
	bad := Expr{Kind: TagRational,
		Numerator:   artifact.BigIntFromInt64(2),
		Denominator: artifact.BigIntFromInt64(4)}
	enc, _ := bad.Encode()
	if _, err := DecodeExpr(artifact.NewDecoder(enc)); err == nil {
		t.Fatal("non-reduced Rational decoded without error")
	}
}

// §25.16: durable relation payload has no outer tag; sides byte-identical to
// standalone expression artifacts.
func TestRelationSerialization(t *testing.T) {
	l := Add(NewEphemeralSymbol("x"), MustRational(1, 2))
	r := NewEphemeralSymbol("y")
	body, err := EncodeRelationBody(OpEQ, l, r)
	if err != nil {
		t.Fatal(err)
	}
	le, _ := l.Encode()
	re, _ := r.Encode()
	op, _ := artifact.EncodeEnum("EQ")
	want := append(append(op, le...), re...)
	if !bytes.Equal(body, want) {
		t.Fatal("relation body is not Enum||Left||Right")
	}
	if body[0] == TagRelation {
		t.Fatal("durable relation carries the outer Expression tag")
	}
	if _, err := Relation("APPROX", l, r); err == nil {
		t.Fatal("open comparison vocabulary accepted")
	}
}

// D27: unknown tags rejected on decode.
func TestUnknownTagRejected(t *testing.T) {
	if _, err := DecodeExpr(artifact.NewDecoder([]byte{0x0B})); err == nil {
		t.Fatal("unknown tag 0x0B accepted")
	}
	if _, err := DecodeExpr(artifact.NewDecoder([]byte{0x00})); err == nil {
		t.Fatal("tag 0x00 accepted")
	}
}

// Round-trip every node kind through canonical bytes.
func TestNodeRoundTrips(t *testing.T) {
	pid := DeriveParameterID(1)
	fnRef := ComputeTestRef(t, "physmath", "function/1")
	nodes := []Expr{
		NewBoundSymbol("g", pid),
		NewConstantSymbol("e", ConstE),
		MustRational(-3, 7),
		Add(NewEphemeralSymbol("a"), MustRational(1, 1)),
		Mul(NewEphemeralSymbol("a"), NewEphemeralSymbol("b")),
		Neg(NewEphemeralSymbol("a")),
		Pow(NewEphemeralSymbol("a"), MustRational(2, 1)),
		Sqrt(NewEphemeralSymbol("a")),
		Call(fnRef, NewEphemeralSymbol("a")),
		mustRelation(t, OpLT, NewEphemeralSymbol("a"), MustRational(0, 1)),
		BranchSet(NewEphemeralSymbol("a"), MustRational(0, 1)),
	}
	for i, n := range nodes {
		enc, err := n.Encode()
		if err != nil {
			t.Fatalf("node %d encode: %v", i, err)
		}
		got, err := DecodeExpr(artifact.NewDecoder(enc))
		if err != nil {
			t.Fatalf("node %d decode: %v", i, err)
		}
		if !got.Equal(n) {
			t.Fatalf("node %d changed through round trip", i)
		}
	}
	// Truncated input rejected.
	enc, _ := nodes[3].Encode()
	if _, err := DecodeExpr(artifact.NewDecoder(enc[:len(enc)-1])); err == nil {
		t.Fatal("truncated input accepted")
	}
}

func mustRelation(t *testing.T, op ComparisonOp, l, r Expr) Expr {
	t.Helper()
	e, err := Relation(op, l, r)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
