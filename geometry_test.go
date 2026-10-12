package physmath

import (
	"bytes"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// mintIndexRef mints an index/1 artifact with the given variance.
func mintIndexRef(t *testing.T, s Store, label string, v IndexVariance) artifact.ArtifactRef {
	t.Helper()
	idx := Index{Label: label, Variance: v}
	payload, err := idx.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyIndex, payload)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// mintTensorRef mints a rank-2 dim-2 tensor with opposing-variance slots.
func mintTensorRef(t *testing.T, s Store, v0, v1 IndexVariance) artifact.ArtifactRef {
	t.Helper()
	i0 := mintIndexRef(t, s, "a", v0)
	i1 := mintIndexRef(t, s, "b", v1)
	comps := make([]artifact.ArtifactRef, 4)
	for i := range comps {
		comps[i] = mustStoreExpr(t, s, MustRational(int64(i+1), 1))
	}
	tn := Tensor{Rank: 2, Dimension: 2, IndexRefs: []artifact.ArtifactRef{i0, i1}, ComponentRefs: comps}
	if err := tn.Validate(); err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyTensor, tn.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func familiesMap(t *testing.T, s *MemStore, refs ...artifact.ArtifactRef) map[artifact.ArtifactRef]FamilyID {
	t.Helper()
	m := map[artifact.ArtifactRef]FamilyID{}
	for _, r := range refs {
		fam, _, err := s.Lookup(r)
		if err != nil {
			t.Fatal(err)
		}
		m[r] = fam
	}
	return m
}

func TestSymmetricBilinearShape(t *testing.T) {
	s := NewMemStore()
	comps := make([]artifact.ArtifactRef, 4)
	for i := range comps {
		comps[i] = mustStoreExpr(t, s, MustRational(int64(i+1), 1))
	}
	// InverseRef grammar accepts any well-formed ref; family is enforced
	// via TargetChecks. A same-shape metric ref is used here.
	oneRef := mustStoreExpr(t, s, MustRational(1, 1))
	invMetric := SymmetricBilinear{Dimension: 1,
		ComponentRefs: []artifact.ArtifactRef{oneRef}, InverseRef: nil} // self-inverse claim
	if err := invMetric.Validate(); err != nil {
		t.Fatalf("1x1 metric rejected: %v", err)
	}
	invRef, err := s.Put(FamilySymmetricBilinear, invMetric.Encode())
	if err != nil {
		t.Fatal(err)
	}
	m := SymmetricBilinear{Dimension: 2, ComponentRefs: comps, InverseRef: &invRef}
	if err := m.Validate(); err != nil {
		t.Fatalf("valid metric rejected: %v", err)
	}
	// Deterministic encoding.
	a, b := m.Encode(), m.Encode()
	if !bytes.Equal(a, b) {
		t.Fatal("metric encoding nondeterministic")
	}
	// TargetChecks resolve.
	fams := familiesMap(t, s, append(comps, invRef)...)
	if err := ValidateAgainst(m.TargetChecks(), fams); err != nil {
		t.Fatalf("metric targets rejected: %v", err)
	}
}

func TestSymmetricBilinearNegatives(t *testing.T) {
	s := NewMemStore()
	one := mustStoreExpr(t, s, MustRational(1, 1))
	badInv := artifact.ArtifactRef{Namespace: "BAD", SchemaVersion: "1", ArtifactType: "x/1"}
	good := SymmetricBilinear{Dimension: 1, ComponentRefs: []artifact.ArtifactRef{one}, InverseRef: nil}
	if err := good.Validate(); err != nil {
		t.Fatalf("1x1 metric rejected: %v", err)
	}
	for name, m := range map[string]SymmetricBilinear{
		"zero dimension":   {Dimension: 0, ComponentRefs: nil, InverseRef: nil},
		"over bound":       {Dimension: SymmetricBilinearDimensionBound + 1, InverseRef: nil},
		"count mismatch":   {Dimension: 2, ComponentRefs: []artifact.ArtifactRef{one}, InverseRef: nil},
		"bad inverse gram": {Dimension: 1, ComponentRefs: []artifact.ArtifactRef{one}, InverseRef: &badInv},
	} {
		_ = name
		if err := m.Validate(); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestCheckSymmetricBilinear(t *testing.T) {
	s := NewMemStore()
	r := func(n, d int64) artifact.ArtifactRef { return mustStoreExpr(t, s, MustRational(n, d)) }
	// Symmetric [[1,2],[2,3]].
	sym := SymmetricBilinear{Dimension: 2,
		ComponentRefs: []artifact.ArtifactRef{r(1, 1), r(2, 1), r(2, 1), r(3, 1)}, InverseRef: nil}
	if err := CheckSymmetricBilinear(s, sym); err != nil {
		t.Fatalf("symmetric metric rejected: %v", err)
	}
	// Asymmetric [[1,2],[3,3]].
	asym := SymmetricBilinear{Dimension: 2,
		ComponentRefs: []artifact.ArtifactRef{r(1, 1), r(2, 1), r(3, 1), r(3, 1)}, InverseRef: nil}
	if err := CheckSymmetricBilinear(s, asym); err == nil {
		t.Fatal("asymmetric metric certified symmetric")
	}
	// Unresolvable component.
	ghost := sym
	ghost.ComponentRefs[0] = ComputeTestRef(t, "physmath", "expression/1")
	if err := CheckSymmetricBilinear(s, ghost); err == nil {
		t.Fatal("unresolvable component accepted")
	}
}

func TestContractionCertificate(t *testing.T) {
	s := NewMemStore()
	tn := mintTensorRef(t, s, VarianceCovariant, VarianceContravariant)
	res := mustStoreExpr(t, s, NewEphemeralSymbol("contracted"))
	c := Contraction{TensorRef: tn, First: 0, Second: 1, ResultRef: res}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid contraction rejected: %v", err)
	}
	if err := CheckContraction(s, c); err != nil {
		t.Fatalf("valid contraction failed resolution: %v", err)
	}
	fams := familiesMap(t, s, tn, res)
	if err := ValidateAgainst(c.TargetChecks(), fams); err != nil {
		t.Fatalf("contraction targets rejected: %v", err)
	}
}

func TestContractionNegatives(t *testing.T) {
	s := NewMemStore()
	tn := mintTensorRef(t, s, VarianceCovariant, VarianceContravariant)
	res := mustStoreExpr(t, s, NewEphemeralSymbol("c"))
	same := Contraction{TensorRef: tn, First: 1, Second: 1, ResultRef: res}
	if err := same.Validate(); err == nil {
		t.Fatal("self-pairing accepted")
	}
	oor := Contraction{TensorRef: tn, First: 0, Second: 5, ResultRef: res}
	if err := CheckContraction(s, oor); err == nil {
		t.Fatal("out-of-rank position accepted")
	}
	like := mintTensorRef(t, s, VarianceCovariant, VarianceCovariant)
	if err := CheckContraction(s, Contraction{TensorRef: like, First: 0, Second: 1, ResultRef: res}); err == nil {
		t.Fatal("like-variance pairing certified")
	}
	unspec := mintTensorRef(t, s, VarianceUnspecified, VarianceContravariant)
	if err := CheckContraction(s, Contraction{TensorRef: unspec, First: 0, Second: 1, ResultRef: res}); err == nil {
		t.Fatal("UNSPECIFIED variance pairing certified")
	}
	notExpr := Contraction{TensorRef: tn, First: 0, Second: 1, ResultRef: tn}
	if err := CheckContraction(s, notExpr); err == nil {
		t.Fatal("non-expression result accepted")
	}
	notTensor := Contraction{TensorRef: res, First: 0, Second: 1, ResultRef: res}
	if err := CheckContraction(s, notTensor); err == nil {
		t.Fatal("non-tensor target accepted")
	}
}

func TestDomainKinds(t *testing.T) {
	for _, k := range []string{DomainHypersurface, DomainRegion, DomainInterval, DomainPoint} {
		if !ValidDomainKind(k) {
			t.Fatalf("closed kind %q rejected", k)
		}
	}
	if ValidDomainKind("MANIFOLD") {
		t.Fatal("open kind accepted")
	}
	s := NewMemStore()
	e := mustStoreExpr(t, s, NewEphemeralSymbol("x"))
	dm := Domain{Kind: DomainHypersurface, DefiningRefs: []artifact.ArtifactRef{e}}
	if err := dm.Validate(); err != nil {
		t.Fatalf("valid domain rejected: %v", err)
	}
	bad := Domain{Kind: "MANIFOLD", DefiningRefs: []artifact.ArtifactRef{e}}
	if err := bad.Validate(); err == nil {
		t.Fatal("open domain kind accepted")
	}
	dup := Domain{Kind: DomainRegion, DefiningRefs: []artifact.ArtifactRef{e, e}}
	if err := dup.Validate(); err == nil {
		t.Fatal("duplicate defining refs accepted")
	}
	fams := familiesMap(t, s, e)
	if err := ValidateAgainst(dm.TargetChecks(), fams); err != nil {
		t.Fatalf("domain targets rejected: %v", err)
	}
}

func TestIntegralRepresentation(t *testing.T) {
	s := NewMemStore()
	e := mustStoreExpr(t, s, NewEphemeralSymbol("T"))
	dm := Domain{Kind: DomainHypersurface, DefiningRefs: []artifact.ArtifactRef{e}}
	dmPayload, err := dm.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dmRef, err := s.Put(FamilyDomain, dmPayload)
	if err != nil {
		t.Fatal(err)
	}
	w := mustStoreExpr(t, s, MustRational(1, 1))
	in := Integral{DomainRef: dmRef, IntegrandRefs: []artifact.ArtifactRef{e}, WeightRef: &w}
	if err := in.Validate(); err != nil {
		t.Fatalf("valid integral rejected: %v", err)
	}
	p1, err := in.Encode()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := in.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p1, p2) {
		t.Fatal("integral encoding nondeterministic")
	}
	plain := Integral{DomainRef: dmRef, IntegrandRefs: []artifact.ArtifactRef{e}}
	if _, err := plain.Encode(); err != nil {
		t.Fatalf("weightless integral rejected: %v", err)
	}
	fams := familiesMap(t, s, dmRef, e, w)
	if err := ValidateAgainst(in.TargetChecks(), fams); err != nil {
		t.Fatalf("integral targets rejected: %v", err)
	}
	empty := Integral{DomainRef: dmRef}
	if err := empty.Validate(); err == nil {
		t.Fatal("integrand-free integral accepted")
	}
	dup := Integral{DomainRef: dmRef, IntegrandRefs: []artifact.ArtifactRef{e, e}}
	if err := dup.Validate(); err == nil {
		t.Fatal("duplicate integrands accepted")
	}
	// Integral is data: no Phase-1 rule consumes integral/1 — assert Apply
	// has no such path by checking the rule registry is unchanged.
	for _, r := range Phase1Rules {
		if r == "EXPR-INTEGRATE-001" {
			t.Fatal("integration execution rule exists in slice 1")
		}
	}
}

func TestNewFamiliesThreading(t *testing.T) {
	s := NewMemStore()
	e := mustStoreExpr(t, s, MustRational(2, 1))
	dm := Domain{Kind: DomainInterval, DefiningRefs: []artifact.ArtifactRef{e}}
	dmPayload, err := dm.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dmRef, err := s.Put(FamilyDomain, dmPayload)
	if err != nil {
		t.Fatal(err)
	}
	in := Integral{DomainRef: dmRef, IntegrandRefs: []artifact.ArtifactRef{e}}
	inPayload, err := in.Encode()
	if err != nil {
		t.Fatal(err)
	}
	inRef, err := s.Put(FamilyIntegral, inPayload)
	if err != nil {
		t.Fatal(err)
	}
	d := Derivation{PremiseRefs: []artifact.ArtifactRef{e, dmRef, inRef}, Steps: nil, ResultRefs: []artifact.ArtifactRef{e}}
	fams := familiesMap(t, s, e, dmRef, inRef)
	if err := ValidateAgainst(d.TargetChecks(), fams); err != nil {
		t.Fatalf("new families rejected as premises: %v", err)
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("premise-only replay with new families failed: %v", err)
	}
}

// MINOR-02: every domain kind, including POINT, requires at least one
// identity-bearing defining reference. Structural validation only.
func TestDomainDefiningRefsRequired(t *testing.T) {
	s := NewMemStore()
	e := mustStoreExpr(t, s, NewEphemeralSymbol("x"))
	kinds := []string{DomainHypersurface, DomainRegion, DomainInterval, DomainPoint}
	for _, k := range kinds {
		if err := (Domain{Kind: k, DefiningRefs: []artifact.ArtifactRef{e}}).Validate(); err != nil {
			t.Fatalf("kind %s with defining ref rejected: %v", k, err)
		}
		if err := (Domain{Kind: k}).Validate(); err == nil {
			t.Fatalf("kind %s accepted empty DefiningRefs", k)
		}
		if err := (Domain{Kind: k, DefiningRefs: nil}).Validate(); err == nil {
			t.Fatalf("kind %s accepted nil DefiningRefs", k)
		}
	}
	// Malformed reference rejected regardless of kind.
	bad := artifact.ArtifactRef{Namespace: "BAD", SchemaVersion: "1", ArtifactType: "x/1"}
	if err := (Domain{Kind: DomainPoint, DefiningRefs: []artifact.ArtifactRef{bad}}).Validate(); err == nil {
		t.Fatal("malformed defining ref accepted")
	}
	// Well-formed but unresolvable reference passes structural validation
	// and fails at the resolution boundary (TargetChecks), not silently.
	ghost := ComputeTestRef(t, "physmath", "expression/1")
	dm := Domain{Kind: DomainRegion, DefiningRefs: []artifact.ArtifactRef{ghost}}
	if err := dm.Validate(); err != nil {
		t.Fatalf("structurally valid domain rejected: %v", err)
	}
	if err := ValidateAgainst(dm.TargetChecks(), map[artifact.ArtifactRef]FamilyID{}); err == nil {
		t.Fatal("unresolvable defining ref passed resolution")
	}
	// Integral over a domain with refs still validates; resolution of the
	// domain itself is enforced through the same boundary.
	dmPayload, err := dm.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dmRef, err := s.Put(FamilyDomain, dmPayload)
	if err != nil {
		t.Fatal(err)
	}
	in := Integral{DomainRef: dmRef, IntegrandRefs: []artifact.ArtifactRef{e}}
	if err := in.Validate(); err != nil {
		t.Fatalf("integral over referenced domain rejected: %v", err)
	}
}
