package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// KroneckerSelector mints a rank-1 coordinate-basis tensor with components
// (1,0,...,0) — the 1 at the declared position — for δ-type selectors such
// as δ^μ_0 in a comoving frame. It is data, not a new rule or family: the
// result is an ordinary tensor/1 artifact carrying the caller's index
// artifact (variance, label, and space are the caller's claims, resolved at
// check time).
//
// A rank-2 identity δ^μ_ν is a different object and is deliberately not
// constructed here; it gets a constructor only when a use justifies it.
func KroneckerSelector(s Store, dim uint16, position uint16, indexRef artifact.ArtifactRef) (artifact.ArtifactRef, error) {
	fail := func(err error) (artifact.ArtifactRef, error) { return artifact.ArtifactRef{}, err }
	if dim < 1 {
		return fail(fmt.Errorf("physmath: kronecker dimension must be >= 1: %w", ErrBadInput))
	}
	if dim > ContractEvalDimensionBound {
		return fail(fmt.Errorf("physmath: kronecker dimension %d exceeds bound %d: %w",
			dim, ContractEvalDimensionBound, ErrBadInput))
	}
	if position >= dim {
		return fail(fmt.Errorf("physmath: kronecker position %d out of dimension %d: %w",
			position, dim, ErrBadInput))
	}
	one, err := PutExpression(s, MustRational(1, 1))
	if err != nil {
		return fail(err)
	}
	zero, err := PutExpression(s, MustRational(0, 1))
	if err != nil {
		return fail(err)
	}
	comps := make([]artifact.ArtifactRef, dim)
	for i := range comps {
		comps[i] = zero
		if uint16(i) == position {
			comps[i] = one
		}
	}
	tn := Tensor{Rank: 1, Dimension: dim, IndexRefs: []artifact.ArtifactRef{indexRef}, ComponentRefs: comps}
	if err := tn.Validate(); err != nil {
		return fail(err)
	}
	// Construction-time family gate: the index must already resolve to
	// index/1 (components are freshly minted expression/1 by construction).
	ifam, _, err := s.Lookup(indexRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: kronecker index unresolvable: %w", err))
	}
	if ifam != FamilyIndex {
		return fail(fmt.Errorf("physmath: kronecker slot is not index/1: %w", ErrBadInput))
	}
	return s.Put(FamilyTensor, tn.Encode())
}

// ValidateKroneckerSelector verifies that tensorRef is a rank-1
// coordinate-basis selector with the 1 at exactly position: rank 1,
// in-bound dimension, one index/1 slot, rational components with a single
// 1 at position and 0 elsewhere (canonical-byte equality). It establishes
// the component pattern, not the physical meaning of the basis.
func ValidateKroneckerSelector(store Store, tensorRef artifact.ArtifactRef, position uint16) error {
	fam, payload, err := store.Lookup(tensorRef)
	if err != nil {
		return fmt.Errorf("physmath: kronecker tensor unresolvable: %w", err)
	}
	if fam != FamilyTensor {
		return fmt.Errorf("physmath: kronecker target is not tensor/1: %w", ErrBadInput)
	}
	tn, err := DecodeTensor(payload)
	if err != nil {
		return err
	}
	if tn.Rank != 1 {
		return fmt.Errorf("physmath: kronecker rank %d != 1: %w", tn.Rank, ErrBadInput)
	}
	if position >= tn.Dimension {
		return fmt.Errorf("physmath: kronecker position %d out of dimension %d: %w",
			position, tn.Dimension, ErrBadInput)
	}
	if len(tn.IndexRefs) != 1 {
		return fmt.Errorf("physmath: kronecker must carry one index: %w", ErrBadInput)
	}
	ifam, _, err := store.Lookup(tn.IndexRefs[0])
	if err != nil {
		return fmt.Errorf("physmath: kronecker index unresolvable: %w", err)
	}
	if ifam != FamilyIndex {
		return fmt.Errorf("physmath: kronecker slot is not index/1: %w", ErrBadInput)
	}
	one, err := MustRational(1, 1).Encode()
	if err != nil {
		return err
	}
	zero, err := MustRational(0, 1).Encode()
	if err != nil {
		return err
	}
	for i, r := range tn.ComponentRefs {
		cfam, cpayload, err := store.Lookup(r)
		if err != nil {
			return fmt.Errorf("physmath: kronecker component %d unresolvable: %w", i, err)
		}
		if cfam != FamilyExpression {
			return fmt.Errorf("physmath: kronecker component %d is not expression/1: %w", i, ErrBadInput)
		}
		want := zero
		if uint16(i) == position {
			want = one
		}
		if string(cpayload) != string(want) {
			return fmt.Errorf("physmath: kronecker component %d is not the selector pattern: %w", i, ErrBadInput)
		}
	}
	return nil
}
