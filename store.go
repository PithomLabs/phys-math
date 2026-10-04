package physmath

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/PithomLabs/phys-artifact"
)

// Store resolves durable references to (family, canonical payload) and mints
// new content-addressed expression artifacts. It is execution infrastructure,
// not a global semantic registry: callers supply the store explicitly.
type Store interface {
	// Lookup resolves ref to its family and canonical payload bytes.
	Lookup(ref artifact.ArtifactRef) (FamilyID, []byte, error)
	// Put mints (or fetches, content-addressed) the artifact for family/payload.
	Put(family FamilyID, payload []byte) (artifact.ArtifactRef, error)
}

// memEntry is one stored artifact.
type memEntry struct {
	family  FamilyID
	payload []byte
}

// MemStore is an in-memory content-addressed Store.
type MemStore struct {
	items map[artifact.ArtifactRef]memEntry
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{items: make(map[artifact.ArtifactRef]memEntry)}
}

// Lookup implements Store.
func (m *MemStore) Lookup(ref artifact.ArtifactRef) (FamilyID, []byte, error) {
	e, ok := m.items[ref]
	if !ok {
		return FamilyID{}, nil, fmt.Errorf("physmath: unresolved artifact reference")
	}
	out := make([]byte, len(e.payload))
	copy(out, e.payload)
	return e.family, out, nil
}

// Put implements Store via the single artifact digest formula.
func (m *MemStore) Put(family FamilyID, payload []byte) (artifact.ArtifactRef, error) {
	ref, err := artifact.ComputeRef(family.Namespace, family.SchemaVersion, family.ArtifactType, payload)
	if err != nil {
		return artifact.ArtifactRef{}, err
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	m.items[ref] = memEntry{family: family, payload: cp}
	return ref, nil
}

// PutExpression encodes e and mints its physmath/expression/1 artifact.
func PutExpression(store Store, e Expr) (artifact.ArtifactRef, error) {
	payload, err := e.Encode()
	if err != nil {
		return artifact.ArtifactRef{}, err
	}
	return store.Put(FamilyExpression, payload)
}

// resolveExpression looks up an expression artifact and decodes it.
func resolveExpression(store Store, ref artifact.ArtifactRef) (Expr, error) {
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return Expr{}, err
	}
	if fam != FamilyExpression {
		return Expr{}, fmt.Errorf("physmath: expected expression/1, got %s/%s",
			fam.Namespace, fam.ArtifactType)
	}
	return DecodeExpr(artifact.NewDecoder(payload))
}

// resolveFunction looks up a function artifact and decodes it.
func resolveFunction(store Store, ref artifact.ArtifactRef) (Function, error) {
	fam, payload, err := store.Lookup(ref)
	if err != nil {
		return Function{}, err
	}
	if fam != FamilyFunction {
		return Function{}, fmt.Errorf("physmath: expected function/1, got %s/%s",
			fam.Namespace, fam.ArtifactType)
	}
	return DecodeFunction(artifact.NewDecoder(payload))
}

// DecodeFunction decodes a physmath/function/1 canonical payload:
// Sequence(ParameterIDs as raw 32-byte values) || BodyRef.
func DecodeFunction(d *artifact.Decoder) (Function, error) {
	n, err := d.SequenceCount()
	if err != nil {
		return Function{}, err
	}
	ids := make([]ParameterID, 0, n)
	for i := 0; i < n; i++ {
		raw, err := d.Digest32()
		if err != nil {
			return Function{}, err
		}
		ids = append(ids, ParameterID(raw))
	}
	body, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return Function{}, err
	}
	fn := Function{ParameterIDs: ids, BodyRef: body}
	if err := fn.Validate(); err != nil {
		return Function{}, err
	}
	return fn, nil
}

// resolveConditionBody resolves a condition reference to its durable relation
// payload bytes: condition/1 → embedded relation ref → relation/1 payload.
func resolveConditionBody(store Store, condRef artifact.ArtifactRef) ([]byte, error) {
	fam, payload, err := store.Lookup(condRef)
	if err != nil {
		return nil, err
	}
	if fam != FamilyCondition {
		return nil, fmt.Errorf("physmath: condition context is not condition/1")
	}
	relRef, err := artifact.DecodeArtifactRef(artifact.NewDecoder(payload))
	if err != nil {
		return nil, err
	}
	relFam, relPayload, err := store.Lookup(relRef)
	if err != nil {
		return nil, err
	}
	if relFam != FamilyRelation {
		return nil, fmt.Errorf("physmath: condition predicate is not relation/1")
	}
	return relPayload, nil
}

// normalizeConditionRefs deduplicates and sorts condition references by
// canonical ref bytes (SET semantics). Ordering has no semantic effect.
func normalizeConditionRefs(refs []artifact.ArtifactRef) []artifact.ArtifactRef {
	seen := make(map[artifact.ArtifactRef]bool, len(refs))
	var out []artifact.ArtifactRef
	for _, r := range refs {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].Encode(), out[j].Encode()) < 0
	})
	return out
}

// cloneExpr deep-copies an expression tree (substitution inserts copies so
// later mutation of one occurrence cannot alias another).
func cloneExpr(e Expr) Expr {
	switch e.Kind {
	case TagAdd, TagMul:
		cp := make([]Expr, len(e.Children))
		for i, c := range e.Children {
			cp[i] = cloneExpr(c)
		}
		return Expr{Kind: e.Kind, Children: cp}
	case TagNeg:
		o := cloneExpr(*e.Operand)
		return Expr{Kind: TagNeg, Operand: &o}
	case TagPow:
		b := cloneExpr(*e.Base)
		x := cloneExpr(*e.Exp)
		return Expr{Kind: TagPow, Base: &b, Exp: &x}
	case TagSqrt:
		r := cloneExpr(*e.Radicand)
		return Expr{Kind: TagSqrt, Radicand: &r}
	case TagCall:
		args := make([]Expr, len(e.Args))
		for i, a := range e.Args {
			args[i] = cloneExpr(a)
		}
		return Expr{Kind: TagCall, FuncRef: e.FuncRef, Args: args}
	case TagRelation:
		l := cloneExpr(*e.Left)
		r := cloneExpr(*e.Right)
		return Expr{Kind: TagRelation, RelOp: e.RelOp, Left: &l, Right: &r}
	case TagBranch:
		bs := make([]Expr, len(e.Branches))
		for i, b := range e.Branches {
			bs[i] = cloneExpr(b)
		}
		return Expr{Kind: TagBranch, Branches: bs}
	default:
		cp := e
		if e.ParamID != nil {
			p := *e.ParamID
			cp.ParamID = &p
		}
		if e.ConstantID != nil {
			c := *e.ConstantID
			cp.ConstantID = &c
		}
		// BigInt magnitudes are slices: deep-copy so the clone shares no
		// mutable backing store with the source (M-2 immutability).
		cp.Numerator = artifact.BigInt{
			Neg: e.Numerator.Neg,
			Mag: append([]byte(nil), e.Numerator.Mag...),
		}
		cp.Denominator = artifact.BigInt{
			Neg: e.Denominator.Neg,
			Mag: append([]byte(nil), e.Denominator.Mag...),
		}
		return cp
	}
}
