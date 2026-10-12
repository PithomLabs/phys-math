package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// MatrixDimBound caps the dimension for general square-matrix operations
// (determinant, inverse). Leibniz expansion grows factorially; d ≤ 4 keeps
// every derivation small and auditable while covering polar (2), spatial
// (3), and spacetime (4) geometry. The bound is enforced after shape gates
// and before any artifact is minted.
const MatrixDimBound = 4

// squareMatrix is a decoded dense row-major component list with dimension.
type squareMatrix struct {
	dim   int
	exprs []Expr // row-major, dim*dim
}

// readSquareMatrix resolves one matrix operand through the shared component
// path: tensor/1 of rank 2, or symmetric_bilinear/1 (components only, no
// index metadata). Both encodings are row-major; no conversion loses
// component identity because components are decoded, never re-encoded,
// before validation. Dimension bounds and squareness are enforced here,
// before any mint.
func readSquareMatrix(store Store, field string, ref artifact.ArtifactRef) (*squareMatrix, error) {
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return nil, fmt.Errorf("physmath: %s unresolvable: %w", field, err)
	}
	var compRefs []artifact.ArtifactRef
	var dim int
	switch fam {
	case FamilyTensor:
		tn, err := DecodeTensor(payload)
		if err != nil {
			return nil, fmt.Errorf("physmath: %s undecodable: %w", field, err)
		}
		if tn.Rank != 2 {
			return nil, fmt.Errorf("physmath: %s rank %d != 2: %w", field, tn.Rank, ErrBadInput)
		}
		dim, compRefs = int(tn.Dimension), tn.ComponentRefs
	case FamilySymmetricBilinear:
		m, err := DecodeSymmetricBilinear(payload)
		if err != nil {
			return nil, fmt.Errorf("physmath: %s undecodable: %w", field, err)
		}
		dim, compRefs = int(m.Dimension), m.ComponentRefs
	default:
		return nil, fmt.Errorf("physmath: %s is not tensor/1 or symmetric_bilinear/1: %w", field, ErrBadInput)
	}
	if dim < 1 {
		return nil, fmt.Errorf("physmath: %s dimension must be >= 1: %w", field, ErrBadInput)
	}
	if dim > MatrixDimBound {
		return nil, fmt.Errorf("physmath: %s dimension %d exceeds bound %d: %w",
			field, dim, MatrixDimBound, ErrBadInput)
	}
	if len(compRefs) != dim*dim {
		return nil, fmt.Errorf("physmath: %s components %d != dim^2 %d: %w",
			field, len(compRefs), dim*dim, ErrBadInput)
	}
	exprs, err := resolveComponents(store, field+" matrix", compRefs)
	if err != nil {
		return nil, err
	}
	return &squareMatrix{dim: dim, exprs: exprs}, nil
}

// permutations returns all index permutations of size n with their signs
// (+1 even, -1 odd) via inversion parity. n ≤ 4 by the matrix bound.
func permutations(n int) ([][]int, []int) {
	var perms [][]int
	var signs []int
	cur := make([]int, n)
	used := make([]bool, n)
	var rec func(pos int)
	rec = func(pos int) {
		if pos == n {
			cp := append([]int{}, cur...)
			inv := 0
			for i := 0; i < n; i++ {
				for j := i + 1; j < n; j++ {
					if cp[i] > cp[j] {
						inv++
					}
				}
			}
			sign := 1
			if inv%2 == 1 {
				sign = -1
			}
			perms = append(perms, cp)
			signs = append(signs, sign)
			return
		}
		for i := 0; i < n; i++ {
			if used[i] {
				continue
			}
			used[i] = true
			cur[pos] = i
			rec(pos + 1)
			used[i] = false
		}
	}
	rec(0)
	return perms, signs
}

// leibnizExpr builds the determinant sum for a row-major component list:
// Σ_σ sign(σ) Π_i M[i,σ(i)]. Pure construction, no store interaction.
func leibnizExpr(comps []Expr, d int) Expr {
	perms, signs := permutations(d)
	terms := make([]Expr, 0, len(perms))
	for k, p := range perms {
		factors := make([]Expr, 0, d)
		for i := 0; i < d; i++ {
			factors = append(factors, cloneExpr(comps[i*d+p[i]]))
		}
		term := Mul(factors...)
		if signs[k] < 0 {
			term = Neg(term)
		}
		terms = append(terms, term)
	}
	if len(terms) == 1 {
		return terms[0]
	}
	return Add(terms...)
}

// minorExprs returns the (d-1)x(d-1) row-major list deleting row r and
// column c. Pure construction.
func minorExprs(comps []Expr, d, r, c int) []Expr {
	out := make([]Expr, 0, (d-1)*(d-1))
	for i := 0; i < d; i++ {
		if i == r {
			continue
		}
		for j := 0; j < d; j++ {
			if j == c {
				continue
			}
			out = append(out, cloneExpr(comps[i*d+j]))
		}
	}
	return out
}

// GeneralDeterminant computes det(M) for a square matrix through the
// existing executable path: Leibniz construction + EXPR-CANON-001 then
// EXPR-SIMPLIFY-001 replay. It returns the determinant reference plus the
// replayable two-step derivation, mirroring DiagonalDeterminant (which
// remains the preferred path for diagonal metrics and is untouched).
func GeneralDeterminant(store Store, matrixRef artifact.ArtifactRef) (artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, Derivation{}, err
	}
	m, err := readSquareMatrix(store, "determinant matrix", matrixRef)
	if err != nil {
		return fail(err)
	}
	prodRef, err := PutExpression(store, leibnizExpr(m.exprs, m.dim))
	if err != nil {
		return fail(err)
	}
	canon, err := Apply(store, Invocation{
		ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
		InputRefs: []artifact.ArtifactRef{prodRef},
	})
	if err != nil {
		return fail(fmt.Errorf("physmath: determinant canon failed: %w", err))
	}
	res, err := Apply(store, Invocation{
		ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
		InputRefs: []artifact.ArtifactRef{canon.OutputRef},
	})
	if err != nil {
		return fail(fmt.Errorf("physmath: determinant simplify failed: %w", err))
	}
	d := Derivation{
		PremiseRefs: []artifact.ArtifactRef{prodRef},
		Steps: []DerivationStep{
			{
				Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
				InputRefs: []artifact.ArtifactRef{prodRef}, OutputRef: canon.OutputRef,
			},
			{
				Ordinal: 1, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
				InputRefs: []artifact.ArtifactRef{canon.OutputRef}, OutputRef: res.OutputRef,
			},
		},
		ResultRefs: []artifact.ArtifactRef{res.OutputRef},
	}
	if err := Replay(store, d, nil); err != nil {
		return fail(fmt.Errorf("physmath: determinant derivation does not replay: %w", err))
	}
	return res.OutputRef, d, nil
}

// nonzeroDetCondition mints the exact NEQ(det,0) condition/1 artifact for a
// determinant expression. The condition is an explicitly recorded
// assumption: it licenses division, never proves nonzeroness.
func nonzeroDetCondition(store Store, det Expr) (artifact.ArtifactRef, error) {
	body, err := EncodeRelationBody(OpNEQ, det, MustRational(0, 1))
	if err != nil {
		return artifact.ArtifactRef{}, err
	}
	rel, err := store.Put(FamilyRelation, body)
	if err != nil {
		return artifact.ArtifactRef{}, err
	}
	return store.Put(FamilyCondition, rel.Encode())
}

// InverseMatrix computes A^-1 = adj(A)/det(A) through the existing
// executable path. It returns the determinant reference, the ordered
// row-major inverse component references, and one flat replayable
// derivation (determinant CANON+SIMPLIFY first, then per-component
// CANON+SIMPLIFY pairs, mirroring the contraction evaluator shape).
//
// Invertibility gate: a rational-zero determinant fails closed as
// singular, even with conditions. Otherwise an exact NEQ(det,0) condition
// over the determinant expression must be supplied in conds (fail closed
// without it); the condition is threaded into every component derivation.
// Index variance reversal (contravariant inverse of a covariant metric and
// vice versa) is the caller's assembly obligation: this operation returns
// ordered components, and LeviCivita mints index artifacts with flipped
// variance at assembly time.
func InverseMatrix(store Store, matrixRef artifact.ArtifactRef, conds []artifact.ArtifactRef) (artifact.ArtifactRef, []artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, []artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, nil, Derivation{}, err
	}
	m, err := readSquareMatrix(store, "inverse matrix", matrixRef)
	if err != nil {
		return fail(err)
	}
	detRef, detDeriv, err := GeneralDeterminant(store, matrixRef)
	if err != nil {
		return fail(err)
	}
	detExpr, err := resolveExpression(store, detRef)
	if err != nil {
		return fail(err)
	}
	if detExpr.Kind == TagRational && detExpr.Numerator.ToBig().Sign() == 0 {
		return fail(fmt.Errorf("physmath: singular rational matrix not invertible: %w", ErrBadInput))
	}
	wantCond, err := nonzeroDetCondition(store, detExpr)
	if err != nil {
		return fail(err)
	}
	_ = wantCond
	// Exact-condition gate: some supplied condition must byte-match
	// NEQ(det,0); a declared condition is an assumption, and its absence
	// fails closed before any component is minted.
	matched := false
	for _, c := range conds {
		if c == wantCond {
			matched = true
			break
		}
	}
	// Compare by payload bytes in case refs alias: resolve and compare.
	if !matched {
		fam, payload, err := store.Lookup(wantCond)
		if err != nil {
			return fail(err)
		}
		_ = fam
		for _, c := range conds {
			cfam, cpayload, err := store.Lookup(c)
			if err != nil || cfam != FamilyCondition {
				continue
			}
			if string(cpayload) == string(payload) {
				matched = true
				break
			}
		}
	}
	if !matched {
		return fail(fmt.Errorf("physmath: inverse requires exact NEQ(det,0) condition: %w", ErrMissingCondition))
	}
	var premises []artifact.ArtifactRef
	var steps []DerivationStep
	var results []artifact.ArtifactRef
	premises = append(premises, detDeriv.PremiseRefs...)
	steps = append(steps, detDeriv.Steps...)
	results = append(results, detDeriv.ResultRefs...)
	invComps := make([]artifact.ArtifactRef, 0, m.dim*m.dim)
	for i := 0; i < m.dim; i++ {
		for j := 0; j < m.dim; j++ {
			// adj[i][j] = cofactor(j,i): delete row j, column i.
			// The 1x1 cofactor is defined as 1 (empty Leibniz product).
			var cof Expr
			if m.dim == 1 {
				cof = MustRational(1, 1)
			} else {
				cof = leibnizExpr(minorExprs(m.exprs, m.dim, j, i), m.dim-1)
			}
			if m.dim > 1 && (i+j)%2 == 1 {
				cof = Neg(cof)
			}
			premise := Mul(cof, Pow(cloneExpr(detExpr), MustRational(-1, 1)))
			prodRef, err := PutExpression(store, premise)
			if err != nil {
				return fail(err)
			}
			canon, err := Apply(store, Invocation{
				ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
				InputRefs: []artifact.ArtifactRef{prodRef}, ConditionRefs: conds,
			})
			if err != nil {
				return fail(fmt.Errorf("physmath: inverse component (%d,%d) canon failed: %w", i, j, err))
			}
			res, err := Apply(store, Invocation{
				ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
				InputRefs: []artifact.ArtifactRef{canon.OutputRef}, ConditionRefs: conds,
			})
			if err != nil {
				return fail(fmt.Errorf("physmath: inverse component (%d,%d) simplify failed: %w", i, j, err))
			}
			base := uint32(len(steps))
			premises = append(premises, prodRef)
			results = append(results, res.OutputRef)
			invComps = append(invComps, res.OutputRef)
			steps = append(steps,
				DerivationStep{
					Ordinal: base, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
					InputRefs: []artifact.ArtifactRef{prodRef}, OutputRef: canon.OutputRef,
					ConditionRefs: normalizeConditionRefs(conds),
				},
				DerivationStep{
					Ordinal: base + 1, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
					InputRefs: []artifact.ArtifactRef{canon.OutputRef}, OutputRef: res.OutputRef,
					ConditionRefs: normalizeConditionRefs(conds),
				},
			)
		}
	}
	d := Derivation{
		PremiseRefs: uniqueRefs(premises),
		Steps:       steps,
		ResultRefs:  uniqueRefs(results),
	}
	if err := Replay(store, d, nil); err != nil {
		return fail(fmt.Errorf("physmath: inverse derivation does not replay: %w", err))
	}
	return detRef, invComps, d, nil
}
