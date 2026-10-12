package physmath

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"

	"github.com/PithomLabs/phys-artifact"
)

// Invocation is the explicit in-memory execution context for one rule
// invocation (spec §17.1). ConditionRefs are context inputs and do NOT count
// toward rule input arity. Only TransformResult immutability is normative;
// Invocation is a plain consumed input struct (plan9.3 D21).
type Invocation struct {
	ProducerNamespace string
	RuleID            string
	InputRefs         []artifact.ArtifactRef // SEQUENCE
	ConditionRefs     []artifact.ArtifactRef // SET semantics
	Parameters        []byte
}

// TransformResult is the immutable in-memory rule outcome (spec §17.1).
// ConditionRefs is exactly the normalized supplied condition set: Phase-1
// rules generate no new mathematical conditions and discharge none.
type TransformResult struct {
	OutputRef     artifact.ArtifactRef
	ConditionRefs []artifact.ArtifactRef // SET semantics
}

// Apply executes one Phase-1 rule invocation against store. On failure it
// returns a non-nil error and no usable output reference.
func Apply(store Store, inv Invocation) (TransformResult, error) {
	if !ValidPhase1RulePair(inv.ProducerNamespace, inv.RuleID) {
		return TransformResult{}, fmt.Errorf("physmath: unknown rule pair (%q, %q): %w",
			inv.ProducerNamespace, inv.RuleID, ErrUnknownRuleID)
	}
	conds := normalizeConditionRefs(inv.ConditionRefs)
	var out Expr
	var err error
	switch inv.RuleID {
	case RuleExprCanon:
		out, err = ruleCanon(store, inv)
	case RuleExprSimplify:
		out, err = ruleSimplify(store, inv)
	case RuleRatNormalize:
		out, err = ruleRatNormalize(store, inv)
	case RuleSelectBranch:
		out, err = ruleSelectBranch(store, inv)
	case RuleExprApply:
		out, err = ruleApply(store, inv)
	case RuleExprSubstitute:
		out, err = ruleSubstitute(store, inv)
	case RuleDifferentiate:
		out, err = ruleDifferentiate(store, inv)
	case RuleTotalDerivative:
		out, err = ruleTotalDerivative(store, inv)
	case RuleExprCombine:
		out, err = ruleCombine(store, inv)
	default:
		return TransformResult{}, fmt.Errorf("physmath: unhandled rule %q: %w", inv.RuleID, ErrUnknownRuleID)
	}
	if err != nil {
		return TransformResult{}, err
	}
	ref, err := PutExpression(store, out)
	if err != nil {
		return TransformResult{}, err
	}
	return TransformResult{OutputRef: ref, ConditionRefs: conds}, nil
}

// requireArity enforces exact input arity: mismatches fail even when the
// supplied artifacts could otherwise be interpreted meaningfully.
func requireArity(inv Invocation, n int) error {
	if len(inv.InputRefs) != n {
		return fmt.Errorf("%w: %s wants %d inputs, got %d",
			ErrArityMismatch, inv.RuleID, n, len(inv.InputRefs))
	}
	return nil
}

// requireEmptyParams enforces empty parameter payloads.
func requireEmptyParams(inv Invocation) error {
	if len(inv.Parameters) != 0 {
		return fmt.Errorf("%w: %s requires empty parameters", ErrBadParameters, inv.RuleID)
	}
	return nil
}

// ruleCanon implements EXPR-CANON-001 (§17.3): post-order traversal,
// canonicalize children before parents, flatten Add/Mul, sort commutative
// children, arity-normalize. Idempotent. No theorem proving.
func ruleCanon(store Store, inv Invocation) (Expr, error) {
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
	return Canonicalize(e)
}

// ruleRatNormalize implements RAT-NORMALIZE-001 (§17.5): the single input
// root MUST be Rational; output is the normalized Rational node. Idempotent.
func ruleRatNormalize(store Store, inv Invocation) (Expr, error) {
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
	if e.Kind != TagRational {
		return Expr{}, fmt.Errorf("%w: RAT-NORMALIZE-001 requires Rational root", ErrBadInput)
	}
	// Construction already guarantees reduced/positive/zero=0-1; re-derive to
	// make normalization explicit rather than assumed.
	return NewRational(e.Numerator.ToBig(), e.Denominator.ToBig())
}

// ruleSelectBranch implements SELECT-BRANCH-001 (§17.6): root MUST be
// BranchSet; parameters are UInt32BE(BranchOrdinal), zero-based, in range.
// Output is exactly the selected branch. No condition is generated.
func ruleSelectBranch(store Store, inv Invocation) (Expr, error) {
	if err := requireArity(inv, 1); err != nil {
		return Expr{}, err
	}
	if len(inv.Parameters) != 4 {
		return Expr{}, fmt.Errorf("%w: SELECT-BRANCH-001 requires UInt32BE ordinal", ErrBadParameters)
	}
	ordinal := uint32(inv.Parameters[0])<<24 | uint32(inv.Parameters[1])<<16 |
		uint32(inv.Parameters[2])<<8 | uint32(inv.Parameters[3])
	e, err := resolveExpression(store, inv.InputRefs[0])
	if err != nil {
		return Expr{}, err
	}
	if e.Kind != TagBranch {
		return Expr{}, fmt.Errorf("%w: SELECT-BRANCH-001 requires BranchSet root", ErrBadInput)
	}
	if uint64(ordinal) >= uint64(len(e.Branches)) {
		return Expr{}, fmt.Errorf("%w: branch ordinal %d out of range", ErrBadInput, ordinal)
	}
	return cloneExpr(e.Branches[ordinal]), nil
}

// ruleSimplify implements EXPR-SIMPLIFY-001 (§17.4): recursive post-order
// traversal; at each node, repeatedly apply the closed rewrite list in
// priority order until locally stable, restarting at the top after each
// rewrite. Newly formed Add/Mul keep canonical child order. Deterministic
// and idempotent. Closed list ONLY — no distributivity, factorization,
// x+x→2x, trig/calculus identities, or theorem proving.
func ruleSimplify(store Store, inv Invocation) (Expr, error) {
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
	return simplifyExpr(e, bodies)
}

func simplifyExpr(e Expr, condBodies [][]byte) (Expr, error) {
	var err error
	switch e.Kind {
	case TagAdd, TagMul:
		children := make([]Expr, len(e.Children))
		for i, c := range e.Children {
			children[i], err = simplifyExpr(c, condBodies)
			if err != nil {
				return Expr{}, err
			}
		}
		e = Expr{Kind: e.Kind, Children: children}
	case TagNeg:
		o, err := simplifyExpr(*e.Operand, condBodies)
		if err != nil {
			return Expr{}, err
		}
		e = Neg(o)
	case TagPow:
		b, err := simplifyExpr(*e.Base, condBodies)
		if err != nil {
			return Expr{}, err
		}
		x, err := simplifyExpr(*e.Exp, condBodies)
		if err != nil {
			return Expr{}, err
		}
		e = Pow(b, x)
	case TagSqrt:
		r, err := simplifyExpr(*e.Radicand, condBodies)
		if err != nil {
			return Expr{}, err
		}
		e = Sqrt(r)
	case TagCall:
		args := make([]Expr, len(e.Args))
		for i, a := range e.Args {
			args[i], err = simplifyExpr(a, condBodies)
			if err != nil {
				return Expr{}, err
			}
		}
		e = Call(e.FuncRef, args...)
	case TagRelation:
		l, err := simplifyExpr(*e.Left, condBodies)
		if err != nil {
			return Expr{}, err
		}
		r, err := simplifyExpr(*e.Right, condBodies)
		if err != nil {
			return Expr{}, err
		}
		e, err = Relation(e.RelOp, l, r)
		if err != nil {
			return Expr{}, err
		}
	case TagBranch:
		bs := make([]Expr, len(e.Branches))
		for i, b := range e.Branches {
			bs[i], err = simplifyExpr(b, condBodies)
			if err != nil {
				return Expr{}, err
			}
		}
		e = BranchSet(bs...)
	}
	for {
		next, changed, err := simplifyStep(e, condBodies)
		if err != nil {
			return Expr{}, err
		}
		if !changed {
			return e, nil
		}
		// Newly formed Add/Mul nodes keep canonical child order (§17.4).
		e = sortCommutative(next)
	}
}

// sortCommutative sorts Add/Mul children by canonical expression bytes.
func sortCommutative(e Expr) Expr {
	if e.Kind != TagAdd && e.Kind != TagMul {
		return e
	}
	type keyed struct {
		enc []byte
		exp Expr
	}
	ks := make([]keyed, len(e.Children))
	for i, c := range e.Children {
		enc, err := c.Encode()
		if err != nil {
			return e
		}
		ks[i] = keyed{enc, c}
	}
	sort.SliceStable(ks, func(i, j int) bool { return bytes.Compare(ks[i].enc, ks[j].enc) < 0 })
	out := make([]Expr, len(ks))
	for i, k := range ks {
		out[i] = k.exp
	}
	return Expr{Kind: e.Kind, Children: out}
}

// isRationalValue reports whether e is the exact Rational constant v (canonical
// comparison — children are already simplified).
func isRationalValue(e Expr, num, den int64) bool {
	return e.Kind == TagRational && e.Equal(MustRational(num, den))
}

// foldRationals evaluates an all-Rational Add/Mul node with exact arithmetic.
func foldRationals(kind Tag, children []Expr) (Expr, error) {
	acc := new(big.Rat)
	if kind == TagMul {
		acc.SetInt64(1)
	}
	for _, c := range children {
		q := new(big.Rat).SetFrac(c.Numerator.ToBig(), c.Denominator.ToBig())
		if kind == TagAdd {
			acc.Add(acc, q)
		} else {
			acc.Mul(acc, q)
		}
	}
	return NewRational(acc.Num(), acc.Denom())
}

// allRational reports whether every child is a Rational node.
func allRational(children []Expr) bool {
	if len(children) == 0 {
		return false
	}
	for _, c := range children {
		if c.Kind != TagRational {
			return false
		}
	}
	return true
}

// nonZeroConditionMet checks the fail-closed Pow(x,0) condition: an exact
// canonical MathCondition predicate NEQ(canonical-x, 0/1) must be present in
// the supplied context. No equivalence checking, proving, or synthesis.
func nonZeroConditionMet(operand Expr, condBodies [][]byte) bool {
	opBytes, err := operand.Encode()
	if err != nil {
		return false
	}
	_ = opBytes
	want, err := EncodeRelationBody(OpNEQ, operand, MustRational(0, 1))
	if err != nil {
		return false
	}
	for _, body := range condBodies {
		if bytes.Equal(body, want) {
			return true
		}
	}
	return false
}

// simplifyStep applies the first matching closed rewrite in priority order.
func simplifyStep(e Expr, condBodies [][]byte) (Expr, bool, error) {
	// 1. Neg(Neg(x)) -> x
	if e.Kind == TagNeg && e.Operand.Kind == TagNeg {
		return cloneExpr(*e.Operand.Operand), true, nil
	}
	// 1b. Neg(Rational r) -> Rational(-r): exact rational negation, the
	// additive-inverse closure of the existing exact-rational folds. It
	// lets rational Leibniz sums (e.g. Add(4, Neg(6))) reach the
	// all-Rational fold. Non-rational operands are untouched.
	if e.Kind == TagNeg && e.Operand.Kind == TagRational {
		n, err := NewRational(new(big.Int).Neg(e.Operand.Numerator.ToBig()), e.Operand.Denominator.ToBig())
		if err != nil {
			return Expr{}, false, err
		}
		return n, true, nil
	}
	// 2/3. Add/Mul identity removal + 4. Mul zero annihilation + 5. folding.
	if e.Kind == TagAdd || e.Kind == TagMul {
		identity := MustRational(1, 1)
		if e.Kind == TagAdd {
			identity = MustRational(0, 1)
		}
		// 4. Mul(..., 0, ...) -> 0 (checked before identity removal per list order:
		// list order is Add-zero, Mul-one, Mul-zero — but Mul-zero dominates, so
		// check it first within Mul to avoid one-removal changing the outcome;
		// both orders converge; priority follows the spec list textually below).
		if e.Kind == TagMul {
			for _, c := range e.Children {
				if isRationalValue(c, 0, 1) {
					return MustRational(0, 1), true, nil
				}
			}
		}
		// 2/3. Remove identity children, then §7.6 arity normalization.
		kept := make([]Expr, 0, len(e.Children))
		for _, c := range e.Children {
			if c.Equal(identity) {
				continue
			}
			kept = append(kept, c)
		}
		if len(kept) != len(e.Children) {
			switch len(kept) {
			case 0:
				return identity, true, nil
			case 1:
				return kept[0], true, nil
			default:
				// Keep canonical child order (children already canonical).
				return Expr{Kind: e.Kind, Children: kept}, true, nil
			}
		}
		// 5. Exact rational arithmetic only when the whole node is Rational.
		if allRational(e.Children) {
			folded, err := foldRationals(e.Kind, e.Children)
			if err != nil {
				return Expr{}, false, err
			}
			if !folded.Equal(e) || len(e.Children) > 1 {
				// A single Rational child would have been arity-normalized
				// already; folding fires for 2+ children or value change.
				if len(e.Children) > 1 {
					return folded, true, nil
				}
			}
		}
		return e, false, nil
	}
	// 6. Pow(x, 1) -> x
	if e.Kind == TagPow && isRationalValue(*e.Exp, 1, 1) {
		return cloneExpr(*e.Base), true, nil
	}
	// 7. Pow(x, 0) -> 1 only with explicit x != 0; otherwise fail closed.
	if e.Kind == TagPow && isRationalValue(*e.Exp, 0, 1) {
		if !nonZeroConditionMet(*e.Base, condBodies) {
			return Expr{}, false, fmt.Errorf("%w: Pow(x,0) without exact x != 0 condition",
				ErrMissingCondition)
		}
		return MustRational(1, 1), true, nil
	}
	return e, false, nil
}
