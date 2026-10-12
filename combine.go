package physmath

import (
	"bytes"
	"fmt"
	"math/big"
)

// ruleCombine implements EXPR-COMBINE-001: monomial combination within one
// Mul root. Exactly one input; empty parameters; condition context threaded
// (combining down to exponent 0 requires an exact NEQ(base,0) condition,
// mirroring the Pow(x,0) gate).
//
// The rule repeatedly combines the first combinable factor pair found in
// canonical child order until fixpoint, then canonicalizes. A factor is
// combinable when it is a Pow node with an integer Rational exponent, or a
// non-Rational non-Pow expression (read as exponent 1). Rational factors
// are never combined here — exact rational folding belongs to
// EXPR-SIMPLIFY-001. Identity (0/1) handling and distributivity are
// likewise other rules' jobs; this rule only adds exponents of byte-equal
// bases. Non-Mul roots are rejected (narrow scope, not a general CAS).
//
// Deterministic and idempotent. It never fires inside old derivations
// unless explicitly invoked, so existing CANON+SIMPLIFY goldens (notably
// the FRW product form) are unaffected.
func ruleCombine(store Store, inv Invocation) (Expr, error) {
	if err := requireArity(inv, 1); err != nil {
		return Expr{}, err
	}
	if err := requireEmptyParams(inv); err != nil {
		return Expr{}, err
	}
	e, err := resolveExpression(store, inv.InputRefs[0])
	if err != nil {
		return Expr{}, err
	}
	var bodies [][]byte
	for _, c := range normalizeConditionRefs(inv.ConditionRefs) {
		body, err := resolveConditionBody(store, c)
		if err != nil {
			return Expr{}, err
		}
		bodies = append(bodies, body)
	}
	start, err := Canonicalize(e)
	if err != nil {
		return Expr{}, err
	}
	if start.Kind != TagMul {
		return Expr{}, fmt.Errorf("%w: EXPR-COMBINE-001 requires Mul root", ErrBadInput)
	}
	cur := start
	for {
		next, changed, err := combineStep(cur, bodies)
		if err != nil {
			return Expr{}, err
		}
		if !changed {
			return cur, nil
		}
		cur = sortCommutative(next)
	}
}

// factorKey reads a Mul child as (base bytes, integer exponent, ok).
// Rationals report not-combinable: folding is SIMPLIFY's domain.
func factorKey(c Expr) ([]byte, *big.Int, bool) {
	if c.Kind == TagRational {
		return nil, nil, false
	}
	if c.Kind == TagPow {
		if c.Exp.Kind != TagRational {
			return nil, nil, false
		}
		n := c.Exp.Numerator.ToBig()
		dn := c.Exp.Denominator.ToBig()
		if dn.Cmp(big.NewInt(1)) != 0 {
			return nil, nil, false
		}
		bb, err := c.Base.Encode()
		if err != nil {
			return nil, nil, false
		}
		return bb, n, true
	}
	bb, err := c.Encode()
	if err != nil {
		return nil, nil, false
	}
	return bb, big.NewInt(1), true
}

// combineStep merges the first byte-equal-base factor pair in child order.
func combineStep(e Expr, condBodies [][]byte) (Expr, bool, error) {
	kids := e.Children
	keys := make([][]byte, len(kids))
	exps := make([]*big.Int, len(kids))
	for i, c := range kids {
		bb, n, ok := factorKey(c)
		if !ok {
			continue
		}
		keys[i], exps[i] = bb, n
	}
	for i := 0; i < len(kids); i++ {
		if exps[i] == nil {
			continue
		}
		for j := i + 1; j < len(kids); j++ {
			if exps[j] == nil || !bytes.Equal(keys[i], keys[j]) {
				continue
			}
			sum := new(big.Int).Add(exps[i], exps[j])
			var rep Expr
			switch sum.Sign() {
			case 0:
				base := cloneBaseOf(kids[i])
				if !nonZeroConditionMet(base, condBodies) {
					return Expr{}, false, fmt.Errorf("%w: Pow(x,0) combination without exact x != 0 condition",
						ErrMissingCondition)
				}
				rep = MustRational(1, 1)
			case 1:
				if sum.Cmp(big.NewInt(1)) == 0 {
					rep = cloneBaseOf(kids[i])
				} else {
					nRat, err := NewRational(sum, big.NewInt(1))
					if err != nil {
						return Expr{}, false, err
					}
					rep = Pow(cloneBaseOf(kids[i]), nRat)
				}
			default:
				nRat, err := NewRational(sum, big.NewInt(1))
				if err != nil {
					return Expr{}, false, err
				}
				rep = Pow(cloneBaseOf(kids[i]), nRat)
			}
			kept := make([]Expr, 0, len(kids)-1)
			for k, c := range kids {
				if k == i || k == j {
					continue
				}
				kept = append(kept, c)
			}
			kept = append(kept, rep)
			if len(kept) == 1 {
				return kept[0], true, nil
			}
			return Expr{Kind: TagMul, Children: kept}, true, nil
		}
	}
	return e, false, nil
}

// cloneBaseOf returns the Pow base of a combined factor, or a clone of a
// bare-base factor.
func cloneBaseOf(c Expr) Expr {
	if c.Kind == TagPow {
		return cloneExpr(*c.Base)
	}
	return cloneExpr(c)
}
