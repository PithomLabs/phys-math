package physmath

import (
	"bytes"
	"testing"
)

// M-2: clones are structurally independent — mutating a clone's BigInt
// magnitude must not affect the source. Golden bytes stay identical.
func TestCloneDeepCopiesMagnitudes(t *testing.T) {
	orig := Mul(MustRational(3, 2), NewEphemeralSymbol("x"))
	before, err := orig.Encode()
	if err != nil {
		t.Fatal(err)
	}
	cp := cloneExpr(orig)
	after, err := cp.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("clone bytes differ from source")
	}
	// Mutate the clone's magnitude through the (exported, mutable) struct path.
	cp.Children[0].Numerator.Mag[0] = 0xFF
	if orig.Children[0].Numerator.Mag[0] == 0xFF {
		t.Fatal("clone shares BigInt.Mag backing store with source")
	}
	// Denominator path as well.
	cp2 := cloneExpr(orig)
	cp2.Children[0].Denominator.Mag[0] = 0xFF
	if orig.Children[0].Denominator.Mag[0] == 0xFF {
		t.Fatal("clone shares denominator backing store with source")
	}
	// Source still encodes to the golden bytes.
	again, err := orig.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, again) {
		t.Fatal("source bytes changed")
	}
}
