package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// ruleTotalDerivative implements TOTAL-DERIVATIVE-001: total differentiation
// of a scalar expression with respect to one time variable, expanding
// function applications through the chain rule.
//
// Exactly 2 inputs (expression + declaring time function); parameters
// Encode(BoundParameterRef) || UInt32BE(Order), Order >= 1. The BPR must
// reference the supplied declaring function and its time slot. Traversal is
// total: Call nodes are expanded by simultaneous substitution of the
// declared body (chain rule through applications) and differentiation
// recurses into the expansion. Non-Call nodes follow the exact closed
// per-node rules of DIFFERENTIATE-001 (including negative-integer Pow with
// an exact NEQ(base,0) condition, and Sqrt/Relation/BranchSet rejection).
// Non-time symbols (other parameters, constants, unbound names)
// differentiate to 0 as independent quantities.
//
// Two boundaries are explicit, not inferred:
//   - Partial vs total: DIFFERENTIATE-001 rejects Call roots; this rule
//     expands them. A bare time symbol differentiates identically under
//     both rules by construction.
//   - Expansion always terminates: artifact references are content-addressed,
//     so a reference cycle would require a hash-preimage cycle. No
//     re-entry guard is needed and none is kept.
//
// Order > 1 differentiates repeatedly in one invocation. The operation is
// theory-neutral: the "time" variable is whichever parameter slot the
// caller declares; no physical time semantics are encoded.
func ruleTotalDerivative(store Store, inv Invocation) (Expr, error) {
	if err := requireArity(inv, 2); err != nil {
		return Expr{}, err
	}
	// Parameters carry BPR + order (never empty); shape is decoded
	// explicitly below, mirroring ruleDifferentiate.
	target, err := resolveExpression(store, inv.InputRefs[0])
	if err != nil {
		return Expr{}, err
	}
	fn, err := resolveFunction(store, inv.InputRefs[1])
	if err != nil {
		return Expr{}, err
	}
	d := artifact.NewDecoder(inv.Parameters)
	bpr, err := DecodeBoundParameterRef(d)
	if err != nil {
		return Expr{}, fmt.Errorf("%w: bad BoundParameterRef: %w", ErrBadParameters, err)
	}
	order, err := d.UInt32()
	if err != nil {
		return Expr{}, fmt.Errorf("%w: missing Order: %w", ErrBadParameters, err)
	}
	if !d.Exhausted() {
		return Expr{}, fmt.Errorf("%w: trailing bytes in total-derivative params", ErrBadParameters)
	}
	if bpr.DeclaringArtifactRef != inv.InputRefs[1] {
		return Expr{}, fmt.Errorf("%w: BoundParameterRef does not match declaring Function", ErrBadInput)
	}
	declared := false
	for _, id := range fn.ParameterIDs {
		if id == bpr.ParameterID {
			declared = true
			break
		}
	}
	if !declared {
		return Expr{}, fmt.Errorf("%w: target not declared by Function", ErrBadInput)
	}
	if order < 1 {
		return Expr{}, fmt.Errorf("%w: Order must be >= 1", ErrBadParameters)
	}
	var bodies [][]byte
	for _, c := range normalizeConditionRefs(inv.ConditionRefs) {
		body, err := resolveConditionBody(store, c)
		if err != nil {
			return Expr{}, err
		}
		bodies = append(bodies, body)
	}
	out := target
	for i := uint32(0); i < order; i++ {
		out, err = totalDiffOnce(store, bpr.ParameterID, out, bodies)
		if err != nil {
			return Expr{}, err
		}
	}
	return Canonicalize(out)
}

// totalDiffOnce differentiates e totally with respect to the time variable
// identified by timeParam. It first expands every Call node through its
// declared body (chain rule through applications), then delegates the
// Call-free tree to the exact DIFFERENTIATE-001 per-node rules with the
// same conditions. Expansion always terminates: artifact references are
// content-addressed, so a reference cycle would require a hash-preimage
// cycle.
func totalDiffOnce(store Store, timeParam ParameterID, e Expr, conds [][]byte) (Expr, error) {
	expanded, err := expandCalls(store, e)
	if err != nil {
		return Expr{}, err
	}
	return differentiateOnce(timeParam, expanded, conds)
}

// expandCalls replaces every Call node by its declared body with arguments
// substituted, recursing into expansions and compound children. Arity,
// function resolution, and body validation fail closed per node.
func expandCalls(store Store, e Expr) (Expr, error) {
	switch e.Kind {
	case TagCall:
		fn, err := resolveFunction(store, e.FuncRef)
		if err != nil {
			return Expr{}, fmt.Errorf("%w: total-derivative Call function unresolvable: %w", ErrBadInput, err)
		}
		if len(fn.ParameterIDs) != len(e.Args) {
			return Expr{}, fmt.Errorf("%w: function arity %d != call args %d",
				ErrBadInput, len(fn.ParameterIDs), len(e.Args))
		}
		body, err := resolveExpression(store, fn.BodyRef)
		if err != nil {
			return Expr{}, err
		}
		if err := ValidateFunctionBody(fn, body); err != nil {
			return Expr{}, err
		}
		return expandCalls(store, substituteParams(fn.ParameterIDs, e.Args, body))
	case TagAdd, TagMul:
		cp := make([]Expr, len(e.Children))
		for i, c := range e.Children {
			xc, err := expandCalls(store, c)
			if err != nil {
				return Expr{}, err
			}
			cp[i] = xc
		}
		return Expr{Kind: e.Kind, Children: cp}, nil
	case TagNeg:
		o, err := expandCalls(store, *e.Operand)
		if err != nil {
			return Expr{}, err
		}
		return Neg(o), nil
	case TagPow:
		b, err := expandCalls(store, *e.Base)
		if err != nil {
			return Expr{}, err
		}
		x, err := expandCalls(store, *e.Exp)
		if err != nil {
			return Expr{}, err
		}
		return Pow(b, x), nil
	default:
		return cloneExpr(e), nil
	}
}
