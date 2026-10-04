package physmath

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"

	"github.com/PithomLabs/phys-artifact"
)

// Tag is an Expression node tag byte.
type Tag byte

// Closed Phase-1 Expression node tags (plan9.3 D27; spec §7.1).
const (
	TagSymbol   = 0x01
	TagRational = 0x02
	TagAdd      = 0x03
	TagMul      = 0x04
	TagNeg      = 0x05
	TagPow      = 0x06
	TagSqrt     = 0x07
	TagCall     = 0x08
	TagRelation = 0x09
	TagBranch   = 0x0A
)

// ComparisonOp is the closed comparison vocabulary (spec §7.11).
// These are mathematical operators; the "no conventions" purity rule bans
// physical/theory conventions, not mathematical operators.
type ComparisonOp string

const (
	OpEQ  ComparisonOp = "EQ"
	OpNEQ ComparisonOp = "NEQ"
	OpLT  ComparisonOp = "LT"
	OpLTE ComparisonOp = "LTE"
	OpGT  ComparisonOp = "GT"
	OpGTE ComparisonOp = "GTE"
)

// ValidComparisonOp reports whether op is in the closed vocabulary.
func ValidComparisonOp(op ComparisonOp) bool {
	switch op {
	case OpEQ, OpNEQ, OpLT, OpLTE, OpGT, OpGTE:
		return true
	}
	return false
}

// Expr is an immutable Phase-1 mathematical expression tree. Only the fields
// belonging to Kind participate in canonical bytes; use the constructors.
type Expr struct {
	Kind Tag

	// Symbol fields.
	DisplayName string
	ParamID     *ParameterID
	ConstantID  *ConstantID

	// Rational fields (always reduced, denominator positive, zero = 0/1).
	Numerator   artifact.BigInt
	Denominator artifact.BigInt

	// Add / Mul children (sequence at construction; canonical order sorted).
	Children []Expr

	// Neg operand.
	Operand *Expr

	// Pow base and exponent.
	Base *Expr
	Exp  *Expr

	// Sqrt radicand.
	Radicand *Expr

	// Call target and arguments (argument order is semantic).
	FuncRef artifact.ArtifactRef
	Args    []Expr

	// Relation node fields.
	RelOp ComparisonOp
	Left  *Expr
	Right *Expr

	// BranchSet alternatives (order is semantic).
	Branches []Expr
}

// NewEphemeralSymbol builds an unbound symbol (both IDs absent). Ephemeral
// symbols are NOT permitted in durable Function bodies (§8.6).
func NewEphemeralSymbol(display string) Expr {
	return Expr{Kind: TagSymbol, DisplayName: display}
}

// NewBoundSymbol builds a ParameterID-bearing schematic/bound symbol.
func NewBoundSymbol(display string, pid ParameterID) Expr {
	return Expr{Kind: TagSymbol, DisplayName: display, ParamID: &pid}
}

// NewConstantSymbol builds a ConstantID-bearing symbol.
func NewConstantSymbol(display string, cid ConstantID) Expr {
	return Expr{Kind: TagSymbol, DisplayName: display, ConstantID: &cid}
}

// ValidateSymbolBinding enforces the durable-symbol rule: exactly one of
// ParameterID or ConstantID is present. Both present is always invalid.
func ValidateSymbolBinding(e Expr) error {
	if e.Kind != TagSymbol {
		return fmt.Errorf("physmath: not a Symbol node")
	}
	hasPID := e.ParamID != nil
	hasCID := e.ConstantID != nil
	if hasPID == hasCID {
		return fmt.Errorf("physmath: durable Symbol requires exactly one of ParameterID, ConstantID")
	}
	return nil
}

// NewRational builds a reduced Rational node (denominator positive, zero = 0/1).
func NewRational(num, den *big.Int) (Expr, error) {
	if den == nil || den.Sign() == 0 {
		return Expr{}, fmt.Errorf("physmath: zero Rational denominator")
	}
	n := new(big.Int).Set(num)
	d := new(big.Int).Set(den)
	if d.Sign() < 0 {
		n.Neg(n)
		d.Neg(d)
	}
	g := new(big.Int).GCD(nil, nil, n, d)
	if g.Sign() != 0 {
		n.Quo(n, g)
		d.Quo(d, g)
	}
	if n.Sign() == 0 {
		d.SetInt64(1)
	}
	return Expr{
		Kind:        TagRational,
		Numerator:   artifact.BigIntFromBig(n),
		Denominator: artifact.BigIntFromBig(d),
	}, nil
}

// MustRational panics on error; for test/fixture constants only.
func MustRational(num, den int64) Expr {
	e, err := NewRational(big.NewInt(num), big.NewInt(den))
	if err != nil {
		panic(err)
	}
	return e
}

// Add builds an Add node (children kept in given order; Canonicalize sorts).
func Add(children ...Expr) Expr { return Expr{Kind: TagAdd, Children: append([]Expr{}, children...)} }

// Mul builds a Mul node.
func Mul(children ...Expr) Expr { return Expr{Kind: TagMul, Children: append([]Expr{}, children...)} }

// Neg builds a Neg node (construction preserves the node; Neg(Neg(x))→x is a
// Phase-3 EXPR-SIMPLIFY-001 rewrite, not construction).
func Neg(operand Expr) Expr { e := operand; return Expr{Kind: TagNeg, Operand: &e} }

// Pow builds a Pow node with arbitrary Expression exponent.
func Pow(base, exp Expr) Expr {
	b, x := base, exp
	return Expr{Kind: TagPow, Base: &b, Exp: &x}
}

// Sqrt builds a Sqrt node. Sqrt is a distinct node kind; identity preserves it
// (no silent rewriting to Pow(_,1/2)).
func Sqrt(radicand Expr) Expr { r := radicand; return Expr{Kind: TagSqrt, Radicand: &r} }

// Call builds a Call node referencing a physmath/function/1 artifact.
func Call(fn artifact.ArtifactRef, args ...Expr) Expr {
	return Expr{Kind: TagCall, FuncRef: fn, Args: append([]Expr{}, args...)}
}

// Relation builds an Expression-level relation node.
func Relation(op ComparisonOp, left, right Expr) (Expr, error) {
	if !ValidComparisonOp(op) {
		return Expr{}, fmt.Errorf("physmath: invalid comparison operator %q", op)
	}
	l, r := left, right
	return Expr{Kind: TagRelation, RelOp: op, Left: &l, Right: &r}, nil
}

// BranchSet builds a BranchSet node (branch order is semantic).
func BranchSet(branches ...Expr) Expr {
	return Expr{Kind: TagBranch, Branches: append([]Expr{}, branches...)}
}

// Encode returns the canonical expression bytes:
// UInt8(tag) || node-specific fields, children recursive (spec §7.3).
func (e Expr) Encode() ([]byte, error) {
	out := []byte{byte(e.Kind)}
	body, err := e.encodeBody()
	if err != nil {
		return nil, err
	}
	return append(out, body...), nil
}

func encodeChild(c Expr) ([]byte, error) { return c.Encode() }

func (e Expr) encodeBody() ([]byte, error) {
	switch e.Kind {
	case TagSymbol:
		s, err := artifact.EncodeString(e.DisplayName)
		if err != nil {
			return nil, err
		}
		out := s
		if e.ParamID == nil {
			out = append(out, artifact.EncodeOptional(false, nil)...)
		} else {
			out = append(out, artifact.EncodeOptional(true, (*e.ParamID)[:])...)
		}
		if e.ConstantID == nil {
			out = append(out, artifact.EncodeOptional(false, nil)...)
		} else {
			cs, err := artifact.EncodeString(string(*e.ConstantID))
			if err != nil {
				return nil, err
			}
			out = append(out, artifact.EncodeOptional(true, cs)...)
		}
		return out, nil
	case TagRational:
		n, err := e.Numerator.Encode()
		if err != nil {
			return nil, err
		}
		d, err := e.Denominator.Encode()
		if err != nil {
			return nil, err
		}
		return append(n, d...), nil
	case TagAdd, TagMul:
		elems := make([][]byte, len(e.Children))
		for i, c := range e.Children {
			enc, err := encodeChild(c)
			if err != nil {
				return nil, err
			}
			elems[i] = enc
		}
		return artifact.EncodeSequence(elems), nil
	case TagNeg:
		if e.Operand == nil {
			return nil, fmt.Errorf("physmath: Neg without operand")
		}
		return encodeChild(*e.Operand)
	case TagPow:
		if e.Base == nil || e.Exp == nil {
			return nil, fmt.Errorf("physmath: Pow without base/exponent")
		}
		b, err := encodeChild(*e.Base)
		if err != nil {
			return nil, err
		}
		x, err := encodeChild(*e.Exp)
		if err != nil {
			return nil, err
		}
		return append(b, x...), nil
	case TagSqrt:
		if e.Radicand == nil {
			return nil, fmt.Errorf("physmath: Sqrt without radicand")
		}
		return encodeChild(*e.Radicand)
	case TagCall:
		out := e.FuncRef.Encode()
		elems := make([][]byte, len(e.Args))
		for i, a := range e.Args {
			enc, err := encodeChild(a)
			if err != nil {
				return nil, err
			}
			elems[i] = enc
		}
		return append(out, artifact.EncodeSequence(elems)...), nil
	case TagRelation:
		if !ValidComparisonOp(e.RelOp) {
			return nil, fmt.Errorf("physmath: invalid comparison operator %q", e.RelOp)
		}
		if e.Left == nil || e.Right == nil {
			return nil, fmt.Errorf("physmath: Relation without sides")
		}
		op, err := artifact.EncodeEnum(string(e.RelOp))
		if err != nil {
			return nil, err
		}
		l, err := encodeChild(*e.Left)
		if err != nil {
			return nil, err
		}
		r, err := encodeChild(*e.Right)
		if err != nil {
			return nil, err
		}
		out := append(op, l...)
		return append(out, r...), nil
	case TagBranch:
		elems := make([][]byte, len(e.Branches))
		for i, b := range e.Branches {
			enc, err := encodeChild(b)
			if err != nil {
				return nil, err
			}
			elems[i] = enc
		}
		return artifact.EncodeSequence(elems), nil
	default:
		return nil, fmt.Errorf("physmath: unknown expression tag 0x%02x", byte(e.Kind))
	}
}

// Equal reports canonical byte equality (spec §19.1).
func (e Expr) Equal(o Expr) bool {
	a, err := e.Encode()
	if err != nil {
		return false
	}
	b, err := o.Encode()
	if err != nil {
		return false
	}
	return bytes.Equal(a, b)
}

// Canonicalize normalizes Add/Mul construction: recursively canonicalize
// children, flatten nested same-kind nodes, sort commutative children by
// canonical bytes, and apply §7.6 arity normalization. All other nodes keep
// their explicit kind (no theorem proving, no distributivity).
func Canonicalize(e Expr) (Expr, error) {
	switch e.Kind {
	case TagAdd, TagMul:
		var flat []Expr
		for _, c := range e.Children {
			cc, err := Canonicalize(c)
			if err != nil {
				return Expr{}, err
			}
			if cc.Kind == e.Kind {
				flat = append(flat, cc.Children...)
			} else {
				flat = append(flat, cc)
			}
		}
		type keyed struct {
			enc []byte
			exp Expr
		}
		ks := make([]keyed, len(flat))
		for i, f := range flat {
			enc, err := f.Encode()
			if err != nil {
				return Expr{}, err
			}
			ks[i] = keyed{enc, f}
		}
		sort.Slice(ks, func(i, j int) bool { return bytes.Compare(ks[i].enc, ks[j].enc) < 0 })
		sorted := make([]Expr, len(ks))
		for i, k := range ks {
			sorted[i] = k.exp
		}
		switch len(sorted) {
		case 0:
			if e.Kind == TagAdd {
				return MustRational(0, 1), nil
			}
			return MustRational(1, 1), nil
		case 1:
			return sorted[0], nil
		}
		return Expr{Kind: e.Kind, Children: sorted}, nil
	case TagNeg:
		if e.Operand == nil {
			return Expr{}, fmt.Errorf("physmath: Neg without operand")
		}
		o, err := Canonicalize(*e.Operand)
		if err != nil {
			return Expr{}, err
		}
		return Neg(o), nil
	case TagPow:
		b, err := Canonicalize(*e.Base)
		if err != nil {
			return Expr{}, err
		}
		x, err := Canonicalize(*e.Exp)
		if err != nil {
			return Expr{}, err
		}
		return Pow(b, x), nil
	case TagSqrt:
		r, err := Canonicalize(*e.Radicand)
		if err != nil {
			return Expr{}, err
		}
		return Sqrt(r), nil
	case TagCall:
		args := make([]Expr, len(e.Args))
		for i, a := range e.Args {
			c, err := Canonicalize(a)
			if err != nil {
				return Expr{}, err
			}
			args[i] = c
		}
		return Call(e.FuncRef, args...), nil
	case TagRelation:
		l, err := Canonicalize(*e.Left)
		if err != nil {
			return Expr{}, err
		}
		r, err := Canonicalize(*e.Right)
		if err != nil {
			return Expr{}, err
		}
		return Relation(e.RelOp, l, r)
	case TagBranch:
		bs := make([]Expr, len(e.Branches))
		for i, b := range e.Branches {
			c, err := Canonicalize(b)
			if err != nil {
				return Expr{}, err
			}
			bs[i] = c
		}
		return BranchSet(bs...), nil
	default:
		return e, nil
	}
}

// DecodeExpr decodes one canonical expression and enforces node validation
// (unknown tags rejected per D27; Rational normalization; closed operators).
func DecodeExpr(d *artifact.Decoder) (Expr, error) {
	tag, err := d.UInt8()
	if err != nil {
		return Expr{}, err
	}
	switch tag {
	case TagSymbol:
		display, err := d.String()
		if err != nil {
			return Expr{}, err
		}
		var pid *ParameterID
		present, err := d.OptionalPresence()
		if err != nil {
			return Expr{}, err
		}
		if present {
			raw, err := d.Digest32()
			if err != nil {
				return Expr{}, err
			}
			p := ParameterID(raw)
			pid = &p
		}
		var cid *ConstantID
		present, err = d.OptionalPresence()
		if err != nil {
			return Expr{}, err
		}
		if present {
			s, err := d.String()
			if err != nil {
				return Expr{}, err
			}
			c := ConstantID(s)
			cid = &c
		}
		return Expr{Kind: TagSymbol, DisplayName: display, ParamID: pid, ConstantID: cid}, nil
	case TagRational:
		n, err := artifact.DecodeBigInt(d)
		if err != nil {
			return Expr{}, err
		}
		den, err := artifact.DecodeBigInt(d)
		if err != nil {
			return Expr{}, err
		}
		e := Expr{Kind: TagRational, Numerator: n, Denominator: den}
		if err := ValidateRationalNode(e); err != nil {
			return Expr{}, err
		}
		return e, nil
	case TagAdd, TagMul:
		n, err := d.SequenceCount()
		if err != nil {
			return Expr{}, err
		}
		children := make([]Expr, 0, n)
		for i := 0; i < n; i++ {
			c, err := DecodeExpr(d)
			if err != nil {
				return Expr{}, err
			}
			children = append(children, c)
		}
		return Expr{Kind: Tag(tag), Children: children}, nil
	case TagNeg:
		o, err := DecodeExpr(d)
		if err != nil {
			return Expr{}, err
		}
		return Neg(o), nil
	case TagPow:
		b, err := DecodeExpr(d)
		if err != nil {
			return Expr{}, err
		}
		x, err := DecodeExpr(d)
		if err != nil {
			return Expr{}, err
		}
		return Pow(b, x), nil
	case TagSqrt:
		r, err := DecodeExpr(d)
		if err != nil {
			return Expr{}, err
		}
		return Sqrt(r), nil
	case TagCall:
		ref, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return Expr{}, err
		}
		n, err := d.SequenceCount()
		if err != nil {
			return Expr{}, err
		}
		args := make([]Expr, 0, n)
		for i := 0; i < n; i++ {
			a, err := DecodeExpr(d)
			if err != nil {
				return Expr{}, err
			}
			args = append(args, a)
		}
		return Call(ref, args...), nil
	case TagRelation:
		op, err := d.String()
		if err != nil {
			return Expr{}, err
		}
		if !ValidComparisonOp(ComparisonOp(op)) {
			return Expr{}, fmt.Errorf("physmath: invalid comparison operator %q", op)
		}
		l, err := DecodeExpr(d)
		if err != nil {
			return Expr{}, err
		}
		r, err := DecodeExpr(d)
		if err != nil {
			return Expr{}, err
		}
		return Relation(ComparisonOp(op), l, r)
	case TagBranch:
		n, err := d.SequenceCount()
		if err != nil {
			return Expr{}, err
		}
		bs := make([]Expr, 0, n)
		for i := 0; i < n; i++ {
			b, err := DecodeExpr(d)
			if err != nil {
				return Expr{}, err
			}
			bs = append(bs, b)
		}
		return BranchSet(bs...), nil
	default:
		return Expr{}, fmt.Errorf("physmath: unknown expression tag 0x%02x", tag)
	}
}

// ValidateRationalNode enforces reduced form, positive denominator, zero = 0/1.
func ValidateRationalNode(e Expr) error {
	if e.Kind != TagRational {
		return fmt.Errorf("physmath: not a Rational node")
	}
	n := e.Numerator.ToBig()
	d := e.Denominator.ToBig()
	if d.Sign() <= 0 {
		return fmt.Errorf("physmath: Rational denominator not positive")
	}
	if n.Sign() == 0 {
		if d.Cmp(big.NewInt(1)) != 0 {
			return fmt.Errorf("physmath: zero Rational must be 0/1")
		}
		return nil
	}
	g := new(big.Int).GCD(nil, nil, n, d)
	if g.Cmp(big.NewInt(1)) != 0 {
		return fmt.Errorf("physmath: Rational not reduced")
	}
	return nil
}
