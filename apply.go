package physmath

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/PithomLabs/phys-artifact"
)

// ruleApply implements EXPR-APPLY-001 (§17.7): the only Phase-1
// definitional/function-body expansion rule.
//
// Exactly 2 inputs: Input[0] = expression rooted at Call, Input[1] = the
// referenced function. The root node MUST be the Call; nested Calls are not
// selected (callers isolate the subtree as its own artifact first — no
// NodePath selector exists in Phase 1). The body expands by simultaneous
// substitution of all declared parameter slots. No recursion into Calls
// appearing in the result; no alpha-freshening (capture-free by construction,
// plan9.3 D23). Empty parameters.
func ruleApply(store Store, inv Invocation) (Expr, error) {
	if err := requireArity(inv, 2); err != nil {
		return Expr{}, err
	}
	if err := requireEmptyParams(inv); err != nil {
		return Expr{}, err
	}
	target, err := resolveExpression(store, inv.InputRefs[0])
	if err != nil {
		return Expr{}, err
	}
	if target.Kind != TagCall {
		return Expr{}, fmt.Errorf("%w: EXPR-APPLY-001 requires root Call", ErrBadInput)
	}
	fn, err := resolveFunction(store, inv.InputRefs[1])
	if err != nil {
		return Expr{}, err
	}
	if target.FuncRef != inv.InputRefs[1] {
		return Expr{}, fmt.Errorf("%w: Call FunctionRef does not match supplied Function", ErrBadInput)
	}
	if len(fn.ParameterIDs) != len(target.Args) {
		return Expr{}, fmt.Errorf("%w: function arity %d != call args %d",
			ErrBadInput, len(fn.ParameterIDs), len(target.Args))
	}
	body, err := resolveExpression(store, fn.BodyRef)
	if err != nil {
		return Expr{}, err
	}
	if err := ValidateFunctionBody(fn, body); err != nil {
		return Expr{}, err
	}
	return substituteParams(fn.ParameterIDs, target.Args, body), nil
}

// substituteParams replaces body Symbols carrying ParameterIDs[i] with
// clone(args[i]) throughout the tree. Constants are preserved.
func substituteParams(ids []ParameterID, args []Expr, body Expr) Expr {
	slot := make(map[ParameterID]int, len(ids))
	for i, id := range ids {
		slot[id] = i
	}
	var walk func(e Expr) Expr
	walk = func(e Expr) Expr {
		switch e.Kind {
		case TagSymbol:
			if e.ParamID != nil {
				if i, ok := slot[*e.ParamID]; ok {
					return cloneExpr(args[i])
				}
			}
			return cloneExpr(e)
		case TagAdd, TagMul:
			cp := make([]Expr, len(e.Children))
			for i, c := range e.Children {
				cp[i] = walk(c)
			}
			return Expr{Kind: e.Kind, Children: cp}
		case TagNeg:
			o := walk(*e.Operand)
			return Expr{Kind: TagNeg, Operand: &o}
		case TagPow:
			b := walk(*e.Base)
			x := walk(*e.Exp)
			return Expr{Kind: TagPow, Base: &b, Exp: &x}
		case TagSqrt:
			r := walk(*e.Radicand)
			return Expr{Kind: TagSqrt, Radicand: &r}
		case TagCall:
			cp := make([]Expr, len(e.Args))
			for i, a := range e.Args {
				cp[i] = walk(a)
			}
			return Expr{Kind: TagCall, FuncRef: e.FuncRef, Args: cp}
		case TagRelation:
			l := walk(*e.Left)
			r := walk(*e.Right)
			out, _ := Relation(e.RelOp, l, r)
			return out
		case TagBranch:
			bs := make([]Expr, len(e.Branches))
			for i, b := range e.Branches {
				bs[i] = walk(b)
			}
			return Expr{Kind: TagBranch, Branches: bs}
		default:
			return cloneExpr(e)
		}
	}
	return walk(body)
}

// substitutionEntry is one decoded EXPR-SUBSTITUTE-001 parameter entry:
// Encode(BoundParameterRef) || EncodeArtifactRef(ReplacementExpression).
type substitutionEntry struct {
	target      BoundParameterRef
	replacement artifact.ArtifactRef
	raw         []byte // exact canonical entry bytes (for sort validation)
}

// ruleSubstitute implements EXPR-SUBSTITUTE-001 (§17.8): simultaneous,
// recursive substitution over the complete target tree.
//
// Input[0] = target expression; Inputs[1..D] (D >= 1) = distinct declaring
// functions, sorted by canonical ArtifactRef bytes. Parameters:
// UInt32BE(N) || N entries sorted lexicographically by exact canonical entry
// bytes. Matching is by ParameterID. Replacements are not re-substituted.
// Duplicate target ParameterIDs are invalid REGARDLESS of DeclaringArtifactRef.
func ruleSubstitute(store Store, inv Invocation) (Expr, error) {
	if len(inv.InputRefs) < 2 {
		return Expr{}, fmt.Errorf("%w: EXPR-SUBSTITUTE-001 needs 1+D inputs, D >= 1", ErrArityMismatch)
	}
	target, err := resolveExpression(store, inv.InputRefs[0])
	if err != nil {
		return Expr{}, err
	}
	// Declaring refs sorted by canonical bytes (strictly increasing = distinct).
	decls := inv.InputRefs[1:]
	for i := 1; i < len(decls); i++ {
		if bytes.Compare(decls[i-1].Encode(), decls[i].Encode()) >= 0 {
			return Expr{}, fmt.Errorf("%w: declaring Function refs not strictly sorted", ErrBadInput)
		}
	}
	// Resolve declaring functions and index their parameter slots.
	declSet := make(map[artifact.ArtifactRef]bool, len(decls))
	declares := make(map[artifact.ArtifactRef]map[ParameterID]bool, len(decls))
	for _, ref := range decls {
		fn, err := resolveFunction(store, ref)
		if err != nil {
			return Expr{}, err
		}
		declSet[ref] = true
		slots := make(map[ParameterID]bool, len(fn.ParameterIDs))
		for _, id := range fn.ParameterIDs {
			slots[id] = true
		}
		declares[ref] = slots
	}
	// Decode and validate the parameter entries.
	d := artifact.NewDecoder(inv.Parameters)
	n, err := d.SequenceCount()
	if err != nil {
		return Expr{}, fmt.Errorf("%w: substitute params missing N: %w", ErrBadParameters, err)
	}
	if n < 1 {
		return Expr{}, fmt.Errorf("%w: substitute needs N >= 1", ErrBadParameters)
	}
	entries := make([]substitutionEntry, 0, n)
	for i := 0; i < n; i++ {
		start := d.Offset()
		bpr, err := DecodeBoundParameterRef(d)
		if err != nil {
			return Expr{}, fmt.Errorf("%w: bad substitution target: %w", ErrBadParameters, err)
		}
		rep, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return Expr{}, fmt.Errorf("%w: bad replacement ref: %w", ErrBadParameters, err)
		}
		_ = start
		_ = rep
		entries = append(entries, substitutionEntry{target: bpr, replacement: rep})
	}
	if !d.Exhausted() {
		return Expr{}, fmt.Errorf("%w: trailing bytes in substitute params", ErrBadParameters)
	}
	// Recompute exact canonical entry bytes for sort validation.
	for i := range entries {
		entries[i].raw = append(entries[i].target.Encode(), entries[i].replacement.Encode()...)
	}
	for i := 1; i < len(entries); i++ {
		if bytes.Compare(entries[i-1].raw, entries[i].raw) >= 0 {
			return Expr{}, fmt.Errorf("%w: substitution entries not strictly sorted", ErrBadParameters)
		}
	}
	// Validate targets and resolve replacements.
	seen := make(map[ParameterID]bool, len(entries))
	reps := make(map[ParameterID]Expr, len(entries))
	for _, e := range entries {
		if !declSet[e.target.DeclaringArtifactRef] {
			return Expr{}, fmt.Errorf("%w: declaring context missing from inputs", ErrBadInput)
		}
		if !declares[e.target.DeclaringArtifactRef][e.target.ParameterID] {
			return Expr{}, fmt.Errorf("%w: target not declared by its Function", ErrBadInput)
		}
		if seen[e.target.ParameterID] {
			return Expr{}, fmt.Errorf("%w: duplicate substitution target ParameterID", ErrBadInput)
		}
		seen[e.target.ParameterID] = true
		rep, err := resolveExpression(store, e.replacement)
		if err != nil {
			return Expr{}, err
		}
		reps[e.target.ParameterID] = rep
	}
	// Single simultaneous pass; replacements are inserted as-is.
	var walk func(e Expr) Expr
	walk = func(e Expr) Expr {
		switch e.Kind {
		case TagSymbol:
			if e.ParamID != nil {
				if rep, ok := reps[*e.ParamID]; ok {
					return cloneExpr(rep)
				}
			}
			return cloneExpr(e)
		case TagAdd, TagMul:
			cp := make([]Expr, len(e.Children))
			for i, c := range e.Children {
				cp[i] = walk(c)
			}
			return Expr{Kind: e.Kind, Children: cp}
		case TagNeg:
			o := walk(*e.Operand)
			return Expr{Kind: TagNeg, Operand: &o}
		case TagPow:
			b := walk(*e.Base)
			x := walk(*e.Exp)
			return Expr{Kind: TagPow, Base: &b, Exp: &x}
		case TagSqrt:
			r := walk(*e.Radicand)
			return Expr{Kind: TagSqrt, Radicand: &r}
		case TagCall:
			cp := make([]Expr, len(e.Args))
			for i, a := range e.Args {
				cp[i] = walk(a)
			}
			return Expr{Kind: TagCall, FuncRef: e.FuncRef, Args: cp}
		case TagRelation:
			l := walk(*e.Left)
			r := walk(*e.Right)
			out, _ := Relation(e.RelOp, l, r)
			return out
		case TagBranch:
			bs := make([]Expr, len(e.Branches))
			for i, b := range e.Branches {
				bs[i] = walk(b)
			}
			return Expr{Kind: TagBranch, Branches: bs}
		default:
			return cloneExpr(e)
		}
	}
	return walk(target), nil
}

// ruleDifferentiate implements DIFFERENTIATE-001 (§17.9): ordinary partial
// differentiation of scalar expressions with respect to one target ParameterID.
//
// Exactly 2 inputs (expression + declaring function); parameters
// Encode(BoundParameterRef) || UInt32BE(Order), Order >= 1. The BPR must
// reference the supplied declaring function and a declared slot. Closed
// derivative rules only; Call/Relation/BranchSet/Sqrt differentiation is out
// of scope and fails. Pow requires an integer Rational exponent: positive
// exponents differentiate unconditionally, while negative exponents
// additionally require an exact NEQ(base, 0) condition in context (fail
// closed without it; the denominator is never assumed nonzero). Order > 1
// differentiates repeatedly in one invocation. Non-target symbols
// (other parameters, constants, unbound names) differentiate to 0 as
// independent quantities; this is ordinary partial differentiation, not a new
// semantic rule.
func ruleDifferentiate(store Store, inv Invocation) (Expr, error) {
	if err := requireArity(inv, 2); err != nil {
		return Expr{}, err
	}
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
		return Expr{}, fmt.Errorf("%w: trailing bytes in differentiate params", ErrBadParameters)
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
		out, err = differentiateOnce(bpr.ParameterID, out, bodies)
		if err != nil {
			return Expr{}, err
		}
	}
	return Canonicalize(out)
}

func differentiateOnce(target ParameterID, e Expr, conds [][]byte) (Expr, error) {
	one := MustRational(1, 1)
	zero := MustRational(0, 1)
	switch e.Kind {
	case TagRational:
		return zero, nil
	case TagSymbol:
		if e.ParamID != nil && *e.ParamID == target {
			return one, nil
		}
		return zero, nil
	case TagAdd:
		ds := make([]Expr, len(e.Children))
		for i, c := range e.Children {
			dc, err := differentiateOnce(target, c, conds)
			if err != nil {
				return Expr{}, err
			}
			ds[i] = dc
		}
		return Add(ds...), nil
	case TagMul:
		if len(e.Children) == 0 {
			return zero, nil
		}
		if len(e.Children) == 1 {
			return differentiateOnce(target, e.Children[0], conds)
		}
		terms := make([]Expr, len(e.Children))
		for i, c := range e.Children {
			dc, err := differentiateOnce(target, c, conds)
			if err != nil {
				return Expr{}, err
			}
			factors := make([]Expr, 0, len(e.Children))
			for j, o := range e.Children {
				if j != i {
					factors = append(factors, cloneExpr(o))
				}
			}
			factors = append(factors, dc)
			terms[i] = Mul(factors...)
		}
		return Add(terms...), nil
	case TagNeg:
		dc, err := differentiateOnce(target, *e.Operand, conds)
		if err != nil {
			return Expr{}, err
		}
		return Neg(dc), nil
	case TagPow:
		if e.Exp.Kind != TagRational {
			return Expr{}, fmt.Errorf("%w: Pow exponent must be Rational", ErrBadInput)
		}
		n := e.Exp.Numerator.ToBig()
		dn := e.Exp.Denominator.ToBig()
		if dn.Cmp(big.NewInt(1)) != 0 {
			return Expr{}, fmt.Errorf("%w: Pow exponent must be integer", ErrBadInput)
		}
		if n.Sign() == 0 {
			return zero, nil
		}
		if n.Sign() < 0 && !nonZeroConditionMet(*e.Base, conds) {
			return Expr{}, fmt.Errorf("%w: Pow negative exponent without exact base != 0 condition",
				ErrMissingCondition)
		}
		nRat, _ := NewRational(n, big.NewInt(1))
		nMinus, _ := NewRational(new(big.Int).Sub(n, big.NewInt(1)), big.NewInt(1))
		db, err := differentiateOnce(target, *e.Base, conds)
		if err != nil {
			return Expr{}, err
		}
		return Mul(nRat, Pow(cloneExpr(*e.Base), nMinus), db), nil
	default:
		return Expr{}, fmt.Errorf("%w: %s differentiation out of scope in Phase 1", ErrBadInput, kindName(e.Kind))
	}
}

func kindName(k Tag) string {
	switch k {
	case TagCall:
		return "Call"
	case TagRelation:
		return "Relation"
	case TagBranch:
		return "BranchSet"
	case TagSqrt:
		return "Sqrt"
	default:
		return "node"
	}
}
