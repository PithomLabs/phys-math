package physmath

import (
	"crypto/sha256"
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// ParameterID identifies a mathematical binding slot within a declaration
// context (spec §8.1). It is NOT a global provenance identifier and NOT an
// ArtifactRef digest.
type ParameterID [32]byte

// DeriveParameterID derives the slot identity for a Phase-1 Function
// parameter (spec §8.3). Phase-1 Function parameters always use depth zero:
//
//	ParameterID = SHA-256(String("physmath") || Bytes(ScopePathBytes) || UInt32BE(ordinal))
//	ScopePathBytes = UInt32BE(0)
//
// DisplayName is excluded (alpha-renaming preserves identity); the artifact's
// own ArtifactRef is excluded (no recursive identity dependence).
func DeriveParameterID(ordinal uint32) ParameterID {
	var scopePath []byte
	scopePath = append(scopePath, artifact.EncodeUInt32(0)...) // depth = 0, no path elements
	h := sha256.New()
	h.Write(artifact.MustEncodeString("physmath"))
	h.Write(artifact.EncodeBytes(scopePath))
	h.Write(artifact.EncodeUInt32(ordinal))
	var out ParameterID
	copy(out[:], h.Sum(nil))
	return out
}

// ValidateFunctionParameterIDs requires stored IDs to equal ordinal-derived
// IDs (spec §8.3, acceptance §25.30).
func ValidateFunctionParameterIDs(ids []ParameterID) error {
	for i, id := range ids {
		if want := DeriveParameterID(uint32(i)); id != want {
			return fmt.Errorf("physmath: ParameterIDs[%d] is not ordinal-derived", i)
		}
	}
	return nil
}

// BoundParameterRef disambiguates a ParameterID across declaring artifacts
// (spec §8.5). Canonical encoding (spec §§4.10, 8.5):
// EncodeArtifactRef(DeclaringArtifactRef) || Digest32(ParameterID) —
// no Bytes() wrapper around the ParameterID.
type BoundParameterRef struct {
	DeclaringArtifactRef artifact.ArtifactRef
	ParameterID          ParameterID
}

// Encode returns the canonical encoding.
func (b BoundParameterRef) Encode() []byte {
	out := b.DeclaringArtifactRef.Encode()
	return append(out, b.ParameterID[:]...)
}

// DecodeBoundParameterRef decodes one BoundParameterRef.
func DecodeBoundParameterRef(d *artifact.Decoder) (BoundParameterRef, error) {
	ref, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return BoundParameterRef{}, err
	}
	raw, err := d.Digest32()
	if err != nil {
		return BoundParameterRef{}, err
	}
	return BoundParameterRef{DeclaringArtifactRef: ref, ParameterID: ParameterID(raw)}, nil
}

// ValidateFunctionBody enforces the Function body binding rule (spec §8.6):
// every Symbol in the body must carry either a declared ParameterID or a
// ConstantID. Unbound symbols are rejected. Standalone schematic expressions
// are NOT validated by this function (different regime, spec §8.8).
func ValidateFunctionBody(fn Function, body Expr) error {
	if err := ValidateFunctionParameterIDs(fn.ParameterIDs); err != nil {
		return err
	}
	declared := make(map[ParameterID]bool, len(fn.ParameterIDs))
	for _, id := range fn.ParameterIDs {
		declared[id] = true
	}
	var walk func(e Expr) error
	walk = func(e Expr) error {
		switch e.Kind {
		case TagSymbol:
			switch {
			case e.ParamID != nil && e.ConstantID != nil:
				return fmt.Errorf("physmath: body Symbol has both ParameterID and ConstantID")
			case e.ParamID != nil:
				if !declared[*e.ParamID] {
					return fmt.Errorf("physmath: body Symbol has undeclared ParameterID")
				}
			case e.ConstantID != nil:
				if err := ValidateConstantID(*e.ConstantID); err != nil {
					return err
				}
			default:
				return fmt.Errorf("physmath: unbound Symbol in Function body")
			}
			return nil
		case TagAdd, TagMul:
			for _, c := range e.Children {
				if err := walk(c); err != nil {
					return err
				}
			}
			return nil
		case TagNeg:
			return walk(*e.Operand)
		case TagPow:
			if err := walk(*e.Base); err != nil {
				return err
			}
			return walk(*e.Exp)
		case TagSqrt:
			return walk(*e.Radicand)
		case TagCall:
			for _, a := range e.Args {
				if err := walk(a); err != nil {
					return err
				}
			}
			return nil
		case TagRelation:
			if err := walk(*e.Left); err != nil {
				return err
			}
			return walk(*e.Right)
		case TagBranch:
			for _, b := range e.Branches {
				if err := walk(b); err != nil {
					return err
				}
			}
			return nil
		case TagRational:
			return ValidateRationalNode(e)
		default:
			return fmt.Errorf("physmath: unknown expression tag 0x%02x in body", byte(e.Kind))
		}
	}
	return walk(body)
}

// ConstantID names a built-in mathematical constant (spec §9.1).
type ConstantID string

// Phase-1 built-in mathematical constants (closed in Phase 1).
const (
	ConstPi ConstantID = "math.const.pi"
	ConstE  ConstantID = "math.const.e"
	ConstI  ConstantID = "math.const.i"
)

// ValidateConstantID rejects unknown constant identifiers. Physical constants
// acquire meaning only through phys-* contracts; PhysMath never assigns it
// (spec §9.2, t/k rule §8.9).
func ValidateConstantID(c ConstantID) error {
	switch c {
	case ConstPi, ConstE, ConstI:
		return nil
	default:
		return fmt.Errorf("physmath: unknown ConstantID %q", c)
	}
}
