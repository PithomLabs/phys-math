package physmath

import (
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// §25.4: alpha-renaming preserves ParameterID.
func TestParameterAlphaRenaming(t *testing.T) {
	a := DeriveParameterID(0)
	b := DeriveParameterID(0)
	if a != b {
		t.Fatal("ParameterID unstable across calls")
	}
	// DisplayName excluded: t vs tau with same declaration position.
	s1 := NewBoundSymbol("t", a)
	s2 := NewBoundSymbol("tau", a)
	if s1.ParamID == nil || s2.ParamID == nil || *s1.ParamID != *s2.ParamID {
		t.Fatal("alpha-renaming altered ParameterID")
	}
}

// §25.5: declaration positions separate.
func TestParameterSeparation(t *testing.T) {
	if DeriveParameterID(0) == DeriveParameterID(1) {
		t.Fatal("ordinal 0 and 1 share ParameterID")
	}
}

// §25.30: stored IDs must equal ordinal-derived IDs.
func TestParameterOrdinalConsistency(t *testing.T) {
	good := []ParameterID{DeriveParameterID(0), DeriveParameterID(1)}
	if err := ValidateFunctionParameterIDs(good); err != nil {
		t.Fatal(err)
	}
	tampered := []ParameterID{DeriveParameterID(0), DeriveParameterID(0)}
	if err := ValidateFunctionParameterIDs(tampered); err == nil {
		t.Fatal("non-ordinal ParameterIDs accepted")
	}
	if err := ValidateFunctionParameterIDs([]ParameterID{DeriveParameterID(1)}); err == nil {
		t.Fatal("wrong-ordinal ParameterID accepted")
	}
}

// §25.6: Function body closure — foreign ParameterID rejected.
func TestFunctionBodyClosure(t *testing.T) {
	bodyRef := ComputeTestRef(t, "physmath", "expression/1")
	fn := Function{ParameterIDs: []ParameterID{DeriveParameterID(0)}, BodyRef: bodyRef}
	good := Mul(NewBoundSymbol("v", DeriveParameterID(0)), MustRational(2, 1))
	if err := ValidateFunctionBody(fn, good); err != nil {
		t.Fatalf("valid body rejected: %v", err)
	}
	foreign := NewBoundSymbol("g", DeriveParameterID(1))
	if err := ValidateFunctionBody(fn, foreign); err == nil {
		t.Fatal("foreign ParameterID accepted in body")
	}
	if err := ValidateFunctionBody(fn, NewEphemeralSymbol("v")); err == nil {
		t.Fatal("unbound symbol accepted in durable body")
	}
	both := NewBoundSymbol("v", DeriveParameterID(0))
	c := ConstPi
	both.ConstantID = &c
	if err := ValidateFunctionBody(fn, both); err == nil {
		t.Fatal("dual-bound symbol accepted in body")
	}
	badConst := NewConstantSymbol("c", ConstantID("phys.c"))
	if err := ValidateFunctionBody(Function{ParameterIDs: nil, BodyRef: bodyRef}, badConst); err == nil {
		t.Fatal("unknown ConstantID accepted in body")
	}
	// Standalone schematic regime is separate: schematic ParameterID-bearing
	// symbols are legal WITHOUT a declaring artifact outside Function bodies.
	schematic := NewBoundSymbol("x", DeriveParameterID(1))
	if _, err := schematic.Encode(); err != nil {
		t.Fatalf("schematic symbol not encodable: %v", err)
	}
}

// §25.7: symbol physical neutrality — PhysMath attaches no role from spelling.
func TestSymbolPhysicalNeutrality(t *testing.T) {
	for _, name := range []string{"t", "k", "c", "G", "psi"} {
		s := NewEphemeralSymbol(name)
		c, err := Canonicalize(s)
		if err != nil {
			t.Fatal(err)
		}
		if !c.Equal(s) {
			t.Fatalf("canonicalization altered symbol %q", name)
		}
		enc, _ := s.Encode()
		dec, err := DecodeExpr(artifact.NewDecoder(enc))
		if err != nil {
			t.Fatal(err)
		}
		if dec.DisplayName != name || dec.ParamID != nil || dec.ConstantID != nil {
			t.Fatalf("symbol %q gained meaning through codec", name)
		}
	}
}

// BoundParameterRef encoding: ref || raw 32 bytes, no Bytes() wrapper (D28).
func TestBoundParameterRefEncoding(t *testing.T) {
	decl := ComputeTestRef(t, "physmath", "function/1")
	b := BoundParameterRef{DeclaringArtifactRef: decl, ParameterID: DeriveParameterID(3)}
	enc := b.Encode()
	if len(enc) != len(decl.Encode())+32 {
		t.Fatalf("BoundParameterRef length %d, want ref+32", len(enc))
	}
	got, err := DecodeBoundParameterRef(artifact.NewDecoder(enc))
	if err != nil {
		t.Fatal(err)
	}
	if got != b {
		t.Fatal("BoundParameterRef round trip changed value")
	}
}

// ConstantID closed vocabulary.
func TestConstantIDClosed(t *testing.T) {
	for _, c := range []ConstantID{ConstPi, ConstE, ConstI} {
		if err := ValidateConstantID(c); err != nil {
			t.Fatalf("%q rejected: %v", c, err)
		}
	}
	if err := ValidateConstantID("math.const.hbar"); err == nil {
		t.Fatal("undeclared constant accepted")
	}
	if err := ValidateConstantID("G"); err == nil {
		t.Fatal("physical constant name accepted as math ConstantID")
	}
}
