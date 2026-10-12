package physmath

import (
	"errors"
	"math/big"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// evalExact is a TEST-ONLY exact rational evaluator for closed
// Rational/Add/Mul/Neg/Pow-int trees (big.Rat, no floats). It is not
// production semantics and claims no engine capability: it serves as the
// independent oracle for determinant/inverse values while symbolic goldens
// pin engine structure. Anything else errors.
func evalExact(e Expr) (*big.Rat, error) {
	switch e.Kind {
	case TagRational:
		return new(big.Rat).SetFrac(e.Numerator.ToBig(), e.Denominator.ToBig()), nil
	case TagAdd:
		acc := new(big.Rat)
		for _, c := range e.Children {
			v, err := evalExact(c)
			if err != nil {
				return nil, err
			}
			acc.Add(acc, v)
		}
		return acc, nil
	case TagMul:
		acc := new(big.Rat).SetInt64(1)
		for _, c := range e.Children {
			v, err := evalExact(c)
			if err != nil {
				return nil, err
			}
			acc.Mul(acc, v)
		}
		return acc, nil
	case TagNeg:
		v, err := evalExact(*e.Operand)
		if err != nil {
			return nil, err
		}
		return new(big.Rat).Neg(v), nil
	case TagPow:
		if e.Exp.Kind != TagRational {
			return nil, errors.New("non-rational exponent")
		}
		if e.Exp.Denominator.ToBig().Cmp(big.NewInt(1)) != 0 {
			return nil, errors.New("non-integer exponent")
		}
		n := e.Exp.Numerator.ToBig()
		b, err := evalExact(*e.Base)
		if err != nil {
			return nil, err
		}
		if n.Sign() == 0 {
			return new(big.Rat).SetInt64(1), nil
		}
		neg := n.Sign() < 0
		abs := new(big.Int).Abs(n)
		acc := new(big.Rat).SetInt64(1)
		one := new(big.Rat).SetInt64(1)
		for i := new(big.Int); i.Cmp(abs) < 0; i.Add(i, one.Num()) {
			acc.Mul(acc, b)
		}
		_ = one
		if neg {
			if acc.Sign() == 0 {
				return nil, errors.New("division by zero")
			}
			acc.Inv(acc)
		}
		return acc, nil
	default:
		return nil, errors.New("out-of-oracle scope")
	}
}

func mustEval(t *testing.T, s Store, ref artifact.ArtifactRef) *big.Rat {
	t.Helper()
	e, err := resolveExpression(s, ref)
	if err != nil {
		t.Fatal(err)
	}
	v, err := evalExact(e)
	if err != nil {
		t.Fatalf("oracle out of scope: %v", err)
	}
	return v
}

// matrixTensor mints a rank-2 tensor/1 from row-major rational ints.
func matrixTensor(t *testing.T, s Store, dim int, vals ...int64) artifact.ArtifactRef {
	t.Helper()
	if len(vals) != dim*dim {
		t.Fatalf("need %d values, got %d", dim*dim, len(vals))
	}
	comps := make([]Expr, len(vals))
	for i, v := range vals {
		comps[i] = MustRational(v, 1)
	}
	return tensorRef(t, s, 2, dim, idxRefShared(t, s), comps)
}

func idxRefShared(t *testing.T, s Store) []artifact.ArtifactRef {
	t.Helper()
	sp := spaceRef(t, s, "mat_space")
	return []artifact.ArtifactRef{
		idxRef(t, s, "i", VarianceCovariant, sp),
		idxRef(t, s, "j", VarianceCovariant, sp),
	}
}

// detCond mints the exact NEQ(det,0) condition for a determinant ref.
func detCond(t *testing.T, s Store, detRef artifact.ArtifactRef) artifact.ArtifactRef {
	t.Helper()
	det, err := resolveExpression(s, detRef)
	if err != nil {
		t.Fatal(err)
	}
	body, err := EncodeRelationBody(OpNEQ, det, MustRational(0, 1))
	if err != nil {
		t.Fatal(err)
	}
	rel, err := s.Put(FamilyRelation, body)
	if err != nil {
		t.Fatal(err)
	}
	cond, err := s.Put(FamilyCondition, rel.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return cond
}

func TestGeneralDeterminantRational(t *testing.T) {
	s := NewMemStore()
	m := matrixTensor(t, s, 2, 1, 2, 3, 4)
	det, d, err := GeneralDeterminant(s, m)
	if err != nil {
		t.Fatalf("det failed: %v", err)
	}
	// Structural golden: Leibniz terms with signs. The Neg(Rational)
	// fold lets the rational sum reach the all-Rational fold, so the
	// engine-canonical result is the bare rational -2.
	want, err := MustRational(-2, 1).Encode()
	if err != nil {
		t.Fatal(err)
	}
	_, got, err := s.Lookup(det)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("2x2 determinant is not hand-derived -2")
	}
	if v := mustEval(t, s, det); v.Cmp(big.NewRat(-2, 1)) != 0 {
		t.Fatalf("2x2 determinant value wrong: %v", v)
	}
	if len(d.Steps) != 2 || d.Steps[0].RuleID != RuleExprCanon || d.Steps[1].RuleID != RuleExprSimplify {
		t.Fatal("det derivation is not CANON+SIMPLIFY")
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("det derivation does not replay: %v", err)
	}
	det2, _, err := GeneralDeterminant(s, m)
	if err != nil {
		t.Fatal(err)
	}
	if det2 != det {
		t.Fatal("determinant recomputation nondeterministic")
	}
}

func TestGeneralDeterminant3x3(t *testing.T) {
	s := NewMemStore()
	// Leibniz by hand: +1·1·0, -1·4·6, -2·0·0, +2·4·5, +3·0·6, -3·1·5
	// = 0-24-0+40+0-15; value 1 via oracle. Zero factors annihilate in
	// SIMPLIFY; rational zeros strip from Add; Neg(0) residues stay.
	m := matrixTensor(t, s, 3, 1, 2, 3, 0, 1, 4, 5, 6, 0)
	det, d, err := GeneralDeterminant(s, m)
	if err != nil {
		t.Fatalf("3x3 det failed: %v", err)
	}
	// Leibniz by hand: 0-24-0+40+0-15; Neg folds discharge each
	// subtraction, zeros strip, and the rational sum folds to bare 1.
	want, err := MustRational(1, 1).Encode()
	if err != nil {
		t.Fatal(err)
	}
	_, got, err := s.Lookup(det)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("3x3 determinant is not hand-derived 1")
	}
	if v := mustEval(t, s, det); v.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatalf("3x3 determinant value wrong: %v", v)
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("3x3 det derivation does not replay: %v", err)
	}
}

func TestGeneralDeterminantIdentity4(t *testing.T) {
	s := NewMemStore()
	vals := make([]int64, 16)
	for i := 0; i < 4; i++ {
		vals[i*4+i] = 1
	}
	m := matrixTensor(t, s, 4, vals...)
	det, d, err := GeneralDeterminant(s, m)
	if err != nil {
		t.Fatalf("4x4 identity det failed: %v", err)
	}
	if v := mustEval(t, s, det); v.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatalf("4x4 identity determinant value wrong: %v", v)
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("4x4 det derivation does not replay: %v", err)
	}
}

func TestGeneralDeterminantDimOne(t *testing.T) {
	s := NewMemStore()
	m := matrixTensor(t, s, 1, 5)
	det, _, err := GeneralDeterminant(s, m)
	if err != nil {
		t.Fatalf("1x1 det failed: %v", err)
	}
	if v := mustEval(t, s, det); v.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatalf("1x1 determinant value wrong: %v", v)
	}
}

func TestGeneralDeterminantNegatives(t *testing.T) {
	s := NewMemStore()
	good := matrixTensor(t, s, 2, 1, 2, 3, 4)
	// Wrong rank.
	rank3idx := append(idxRefShared(t, s), idxRef(t, s, "k", VarianceCovariant, spaceRef(t, s, "rk3")))
	rank3 := tensorRef(t, s, 3, 2, rank3idx, rationalMatrix(1, 2, 3, 4, 5, 6, 7, 8))
	if _, _, err := GeneralDeterminant(s, rank3); err == nil {
		t.Fatal("rank-3 matrix accepted")
	}
	// Wrong family.
	one := mustStoreExpr(t, s, MustRational(1, 1))
	if _, _, err := GeneralDeterminant(s, one); err == nil {
		t.Fatal("expression matrix accepted")
	}
	// Unresolvable.
	ghost := ComputeTestRef(t, "physmath", "tensor/1")
	if _, _, err := GeneralDeterminant(s, ghost); err == nil {
		t.Fatal("unresolvable matrix accepted")
	}
	// Dimension bound: 5x5 with shared zero components (cheap mint).
	zero := mustStoreExpr(t, s, MustRational(0, 1))
	zrefs := make([]artifact.ArtifactRef, 25)
	for i := range zrefs {
		zrefs[i] = zero
	}
	big := Tensor{Rank: 2, Dimension: 5, IndexRefs: idxRefShared(t, s), ComponentRefs: zrefs}
	if err := big.Validate(); err != nil {
		t.Fatal(err)
	}
	bigRef, err := s.Put(FamilyTensor, big.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := GeneralDeterminant(s, bigRef); err == nil {
		t.Fatal("dimension 5 accepted despite bound")
	}
	if _, _, _, err := InverseMatrix(s, bigRef, nil); err == nil {
		t.Fatal("dimension-5 inverse accepted despite bound")
	}
	_ = good
}

// TestInverseMatrixRational2x2: A = [[1,2],[3,4]], det = -2.
// Cofactors: C00=4, C01=-3, C10=-2, C11=1; adjugate (transposed)
// [[4,-2],[-3,1]]; inv values [[-2,1],[3/2,-1/2]] via oracle. The
// off-diagonal swap is the cofactor-transposition trap.
func TestInverseMatrixRational2x2(t *testing.T) {
	s := NewMemStore()
	m := matrixTensor(t, s, 2, 1, 2, 3, 4)
	det, _, err := GeneralDeterminant(s, m)
	if err != nil {
		t.Fatal(err)
	}
	cond := detCond(t, s, det)
	gotDet, comps, d, err := InverseMatrix(s, m, []artifact.ArtifactRef{cond})
	if err != nil {
		t.Fatalf("inverse failed: %v", err)
	}
	if gotDet != det {
		t.Fatal("inverse determinant ref differs from recomputed det")
	}
	wantVals := []*big.Rat{big.NewRat(-2, 1), big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(-1, 2)}
	if len(comps) != 4 {
		t.Fatalf("inverse has %d components, want 4", len(comps))
	}
	for i, want := range wantVals {
		if got := mustEval(t, s, comps[i]); got.Cmp(want) != 0 {
			t.Fatalf("inverse component %d value wrong: %v", i, got)
		}
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("inverse derivation does not replay: %v", err)
	}
	det2, comps2, _, err := InverseMatrix(s, m, []artifact.ArtifactRef{cond})
	if err != nil {
		t.Fatal(err)
	}
	if det2 != det {
		t.Fatal("inverse recomputation nondeterministic (det)")
	}
	for i := range comps {
		if comps2[i] != comps[i] {
			t.Fatal("inverse recomputation nondeterministic (components)")
		}
	}
	// Missing condition fails closed.
	if _, _, _, err := InverseMatrix(s, m, nil); err == nil {
		t.Fatal("inverse without NEQ(det,0) accepted")
	} else if !errors.Is(err, ErrMissingCondition) {
		t.Fatalf("wrong failure for missing condition: %v", err)
	}
	// Wrong-variable condition fails closed.
	other := detCond(t, s, mustStoreExpr(t, s, MustRational(7, 1)))
	if _, _, _, err := InverseMatrix(s, m, []artifact.ArtifactRef{other}); err == nil {
		t.Fatal("inverse with unrelated condition accepted")
	} else if !errors.Is(err, ErrMissingCondition) {
		t.Fatalf("wrong failure for unrelated condition: %v", err)
	}
	// Singular rational matrix rejected even with conditions.
	sing := matrixTensor(t, s, 2, 1, 2, 2, 4)
	sdet, _, err := GeneralDeterminant(s, sing)
	if err != nil {
		t.Fatal(err)
	}
	if v := mustEval(t, s, sdet); v.Sign() != 0 {
		t.Fatalf("singular det value wrong: %v", v)
	}
	scond := detCond(t, s, sdet)
	if _, _, _, err := InverseMatrix(s, sing, []artifact.ArtifactRef{scond}); err == nil {
		t.Fatal("singular matrix inverse accepted")
	} else if !errors.Is(err, ErrBadInput) {
		t.Fatalf("wrong failure for singular matrix: %v", err)
	}
	// Symmetric-bilinear input takes the shared matrix path:
	// det(diag(-1,1)) = -1; inverse is self-inverse diag(-1,1).
	sb := SymmetricBilinear{Dimension: 2, ComponentRefs: []artifact.ArtifactRef{
		mustStoreExpr(t, s, MustRational(-1, 1)), mustStoreExpr(t, s, MustRational(0, 1)),
		mustStoreExpr(t, s, MustRational(0, 1)), mustStoreExpr(t, s, MustRational(1, 1)),
	}}
	if err := sb.Validate(); err != nil {
		t.Fatal(err)
	}
	sbRef, err := s.Put(FamilySymmetricBilinear, sb.Encode())
	if err != nil {
		t.Fatal(err)
	}
	sbDet, err := func() (artifact.ArtifactRef, error) {
		d, _, err := GeneralDeterminant(s, sbRef)
		return d, err
	}()
	if err != nil {
		t.Fatalf("symmetric-bilinear det failed: %v", err)
	}
	_, sbComps, _, err := InverseMatrix(s, sbRef, []artifact.ArtifactRef{detCond(t, s, sbDet)})
	if err != nil {
		t.Fatalf("symmetric-bilinear inverse failed: %v", err)
	}
	wantSB := []*big.Rat{big.NewRat(-1, 1), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}
	for i, want := range wantSB {
		if got := mustEval(t, s, sbComps[i]); got.Cmp(want) != 0 {
			t.Fatalf("symmetric-bilinear inverse component %d value wrong: %v", i, got)
		}
	}
}

// TestInverseMatrixProductIdentity verifies A·A^-1 = I by engine-executed
// rational folding on hand-built products (no new multiply operation).
func TestInverseMatrixProductIdentity(t *testing.T) {
	s := NewMemStore()
	// (AA^-1)[0][0] = 1*(-2)+2*(3/2) = 1.
	p00 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{
		mustStoreExpr(t, s, Add(
			Mul(MustRational(1, 1), MustRational(-2, 1)),
			Mul(MustRational(2, 1), MustRational(3, 2)),
		)),
	}, nil, nil)
	if got := mustResolve(t, s, p00.OutputRef); !got.Equal(MustRational(1, 1)) {
		t.Fatal("AA^-1[0][0] is not 1")
	}
	// (AA^-1)[0][1] = 1*1+2*(-1/2) = 0.
	p01 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{
		mustStoreExpr(t, s, Add(
			Mul(MustRational(1, 1), MustRational(1, 1)),
			Mul(MustRational(2, 1), MustRational(-1, 2)),
		)),
	}, nil, nil)
	if got := mustResolve(t, s, p01.OutputRef); !got.Equal(MustRational(0, 1)) {
		t.Fatal("AA^-1[0][1] is not 0")
	}
	// (AA^-1)[1][0] = 3*(-2)+4*(3/2) = 0.
	p10 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{
		mustStoreExpr(t, s, Add(
			Mul(MustRational(3, 1), MustRational(-2, 1)),
			Mul(MustRational(4, 1), MustRational(3, 2)),
		)),
	}, nil, nil)
	if got := mustResolve(t, s, p10.OutputRef); !got.Equal(MustRational(0, 1)) {
		t.Fatal("AA^-1[1][0] is not 0")
	}
	// (AA^-1)[1][1] = 3*1+4*(-1/2) = 1.
	p11 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{
		mustStoreExpr(t, s, Add(
			Mul(MustRational(3, 1), MustRational(1, 1)),
			Mul(MustRational(4, 1), MustRational(-1, 2)),
		)),
	}, nil, nil)
	if got := mustResolve(t, s, p11.OutputRef); !got.Equal(MustRational(1, 1)) {
		t.Fatal("AA^-1[1][1] is not 1")
	}
}
