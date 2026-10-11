package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// K1 foundation slice, representation + structural validation only.
// Nothing in this file executes mathematics: there are no rewrite rules
// here, Apply is untouched, and replay treats these artifacts as
// validated data. Non-executability is a documented property, not a gap
// in validation.

// New Phase-1 durable families (spec §5.6 extended; registry grows 16→20).
var (
	// FamilySymmetricBilinear is a symmetric rank-2 tensor as a pure bilinear form.
	// It carries NO signature or convention metadata: those are
	// framework-owned assumptions (phys-lib/gr), never mathematics.
	FamilySymmetricBilinear = FamilyID{NamespacePhysMath, "symmetric_bilinear/1", "1"}
	// FamilyContraction is a validated index-contraction certificate:
	// structural proof that two tensor slots may pair. It evaluates
	// nothing; the contracted result is supplied, never computed.
	FamilyContraction = FamilyID{NamespacePhysMath, "contraction/1", "1"}
	// FamilyDomain is an identity-bearing integration domain (hypersurface,
	// region, interval, point) defined by references, never prose.
	FamilyDomain = FamilyID{NamespacePhysMath, "domain/1", "1"}
	// FamilyIntegral is a represented (never executed) integral of
	// integrand references over a domain, with an optional weight.
	FamilyIntegral = FamilyID{NamespacePhysMath, "integral/1", "1"}
)

// SymmetricBilinearDimensionBound caps stored metric dimensions (dim² components must
// stay far below the tensor shape bound; this is a storage bound, not a
// claim about physical spacetime dimensionality).
const SymmetricBilinearDimensionBound = 64

// SymmetricBilinear is a dense symmetric rank-2 tensor in canonical row-major order.
// Family physmath/metric/1. Symmetry is a documented property verified by
// CheckSymmetricBilinear against a store; Validate enforces shape only, so
// an asymmetric payload is structurally storable but symmetrically
// unverified — callers must check before relying on symmetry.
type SymmetricBilinear struct {
	Dimension     uint16
	ComponentRefs []artifact.ArtifactRef // SEQUENCE, row-major dim*dim → expression/1
	// InverseRef points at the inverse metric/1 artifact, or is nil to
	// assert self-inversion (e.g. diag(-1,1,1,1)). Mutual inversion is
	// never verified here: the reference is structural linkage, and nil
	// is an explicit, auditable claim rather than a missing field.
	InverseRef *artifact.ArtifactRef // optional → symmetric_bilinear/1
}

// Family returns FamilySymmetricBilinear.
func (SymmetricBilinear) Family() FamilyID { return FamilySymmetricBilinear }

// Encode returns the canonical payload in declaration order.
func (m SymmetricBilinear) Encode() []byte {
	out := artifact.EncodeUInt16(m.Dimension)
	out = append(out, encodeRefSequence(m.ComponentRefs)...)
	if m.InverseRef == nil {
		return append(out, artifact.EncodeOptional(false, nil)...)
	}
	return append(out, artifact.EncodeOptional(true, m.InverseRef.Encode())...)
}

// Validate enforces shape and reference grammar. It does NOT verify
// symmetry, invertibility, or signature: see CheckSymmetricBilinear and the
// framework layer for those obligations.
func (m SymmetricBilinear) Validate() error {
	if m.Dimension < 1 {
		return fmt.Errorf("physmath: symmetric bilinear dimension must be >= 1")
	}
	if m.Dimension > SymmetricBilinearDimensionBound {
		return fmt.Errorf("physmath: symmetric bilinear dimension exceeds bound %d", SymmetricBilinearDimensionBound)
	}
	want := uint64(m.Dimension) * uint64(m.Dimension)
	if uint64(len(m.ComponentRefs)) != want {
		return fmt.Errorf("physmath: symmetric bilinear ComponentRefs %d != dim^2 %d",
			len(m.ComponentRefs), want)
	}
	fields := map[string]artifact.ArtifactRef{}
	if m.InverseRef != nil {
		fields["SymmetricBilinear.InverseRef"] = *m.InverseRef
	}
	for i, r := range m.ComponentRefs {
		fields[fmt.Sprintf("SymmetricBilinear.ComponentRefs[%d]", i)] = r
	}
	return validateRefGrammars(fields)
}

// TargetChecks: ComponentRefs → expression/1; InverseRef → metric/1.
func (m SymmetricBilinear) TargetChecks() []TargetCheck {
	checks := make([]TargetCheck, 0, len(m.ComponentRefs)+1)
	for i, r := range m.ComponentRefs {
		checks = append(checks, TargetCheck{
			Field:   fmt.Sprintf("SymmetricBilinear.ComponentRefs[%d]", i),
			Ref:     r,
			Allowed: []FamilyID{FamilyExpression},
		})
	}
	if m.InverseRef != nil {
		checks = append(checks, TargetCheck{
			Field: "SymmetricBilinear.InverseRef", Ref: *m.InverseRef, Allowed: []FamilyID{FamilySymmetricBilinear},
		})
	}
	return checks
}

// CheckSymmetricBilinear resolves every component payload and requires
// pairwise canonical-byte equality of (i,j) and (j,i). It proves stored
// symmetry, not invertibility or physical meaning.
func CheckSymmetricBilinear(store Store, m SymmetricBilinear) error {
	n := int(m.Dimension)
	payloads := make([][]byte, len(m.ComponentRefs))
	for i, r := range m.ComponentRefs {
		fam, p, err := store.Lookup(r)
		if err != nil {
			return fmt.Errorf("physmath: symmetric-bilinear component %d unresolvable: %w", i, err)
		}
		if fam != FamilyExpression {
			return fmt.Errorf("physmath: symmetric-bilinear component %d is not expression/1", i)
		}
		payloads[i] = p
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			a := payloads[i*n+j]
			b := payloads[j*n+i]
			if len(a) != len(b) {
				return fmt.Errorf("physmath: symmetric-bilinear asymmetric at (%d,%d)", i, j)
			}
			for k := range a {
				if a[k] != b[k] {
					return fmt.Errorf("physmath: symmetric-bilinear asymmetric at (%d,%d)", i, j)
				}
			}
		}
	}
	return nil
}

// Contraction certifies that two slots of one stored tensor may pair:
// same tensor, distinct positions, opposing variances. The contracted
// ResultRef is supplied by the caller and never computed here.
// Family physmath/contraction/1.
type Contraction struct {
	TensorRef artifact.ArtifactRef // → tensor/1
	First     uint32               // slot position
	Second    uint32               // slot position, must differ from First
	ResultRef artifact.ArtifactRef // → expression/1 (given, not evaluated)
}

// Family returns FamilyContraction.
func (Contraction) Family() FamilyID { return FamilyContraction }

// Encode returns the canonical payload in declaration order.
func (c Contraction) Encode() []byte {
	out := c.TensorRef.Encode()
	out = append(out, artifact.EncodeUInt32(c.First)...)
	out = append(out, artifact.EncodeUInt32(c.Second)...)
	return append(out, c.ResultRef.Encode()...)
}

// Validate enforces distinct slots and reference grammar. Variance
// opposition and rank bounds require resolution: see CheckContraction.
func (c Contraction) Validate() error {
	if c.First == c.Second {
		return fmt.Errorf("physmath: Contraction slots must differ (self-pairing forbidden)")
	}
	return validateRefGrammars(map[string]artifact.ArtifactRef{
		"Contraction.TensorRef": c.TensorRef,
		"Contraction.ResultRef": c.ResultRef,
	})
}

// TargetChecks: TensorRef → tensor/1; ResultRef → expression/1.
func (c Contraction) TargetChecks() []TargetCheck {
	return []TargetCheck{
		{Field: "Contraction.TensorRef", Ref: c.TensorRef, Allowed: []FamilyID{FamilyTensor}},
		{Field: "Contraction.ResultRef", Ref: c.ResultRef, Allowed: []FamilyID{FamilyExpression}},
	}
}

// DecodeIndex decodes a physmath/index/1 canonical payload. Needed to
// validate contraction variance without trusting caller claims.
func DecodeIndex(payload []byte) (Index, error) {
	d := artifact.NewDecoder(payload)
	label, err := d.String()
	if err != nil {
		return Index{}, fmt.Errorf("physmath: index missing label: %w", err)
	}
	variance, err := d.String()
	if err != nil {
		return Index{}, fmt.Errorf("physmath: index missing variance: %w", err)
	}
	if !ValidIndexVariance(IndexVariance(variance)) {
		return Index{}, fmt.Errorf("physmath: invalid index variance %q", variance)
	}
	present, err := d.OptionalPresence()
	if err != nil {
		return Index{}, fmt.Errorf("physmath: index missing space presence: %w", err)
	}
	var space *artifact.ArtifactRef
	if present {
		ref, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return Index{}, fmt.Errorf("physmath: index bad space ref: %w", err)
		}
		space = &ref
	}
	if !d.Exhausted() {
		return Index{}, fmt.Errorf("physmath: trailing bytes in index payload")
	}
	return Index{Label: label, Variance: IndexVariance(variance), Space: space}, nil
}

// CheckContraction resolves the tensor, decodes both slot indices, and
// requires in-rank positions with strictly opposing variances
// (COVARIANT vs CONTRAVARIANT). UNSPECIFIED variance fails closed: an
// unspecified slot cannot certify pairing. ResultRef family is expression/1.
func CheckContraction(store Store, c Contraction) error {
	fam, payload, err := store.Lookup(c.TensorRef)
	if err != nil {
		return fmt.Errorf("physmath: contraction tensor unresolvable: %w", err)
	}
	if fam != FamilyTensor {
		return fmt.Errorf("physmath: contraction target is not tensor/1")
	}
	d := artifact.NewDecoder(payload)
	rank, err := d.UInt16()
	if err != nil {
		return fmt.Errorf("physmath: tensor missing rank: %w", err)
	}
	if _, err := d.UInt16(); err != nil {
		return fmt.Errorf("physmath: tensor missing dimension: %w", err)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return fmt.Errorf("physmath: tensor missing index sequence: %w", err)
	}
	if uint64(n) != uint64(rank) {
		return fmt.Errorf("physmath: tensor index count %d != rank %d", n, rank)
	}
	if uint64(c.First) >= uint64(rank) || uint64(c.Second) >= uint64(rank) {
		return fmt.Errorf("physmath: contraction position out of rank %d", rank)
	}
	indexRefs := make([]artifact.ArtifactRef, n)
	for i := range indexRefs {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return fmt.Errorf("physmath: tensor bad index ref %d: %w", i, err)
		}
		indexRefs[i] = r
	}
	variances := make([]IndexVariance, n)
	for i, r := range indexRefs {
		ifam, ipayload, err := store.Lookup(r)
		if err != nil {
			return fmt.Errorf("physmath: index %d unresolvable: %w", i, err)
		}
		if ifam != FamilyIndex {
			return fmt.Errorf("physmath: tensor slot %d is not index/1", i)
		}
		idx, err := DecodeIndex(ipayload)
		if err != nil {
			return err
		}
		variances[i] = idx.Variance
	}
	a, b := variances[c.First], variances[c.Second]
	if a == VarianceUnspecified || b == VarianceUnspecified {
		return fmt.Errorf("physmath: contraction with UNSPECIFIED variance fails closed")
	}
	if a == b {
		return fmt.Errorf("physmath: contraction requires opposing variances, got %s/%s", a, b)
	}
	rfam, _, err := store.Lookup(c.ResultRef)
	if err != nil {
		return fmt.Errorf("physmath: contraction result unresolvable: %w", err)
	}
	if rfam != FamilyExpression {
		return fmt.Errorf("physmath: contraction result is not expression/1")
	}
	return nil
}

// Domain kinds: closed vocabulary for integral domains. The domain is
// identified by kind plus defining references, never by prose.
var (
	DomainHypersurface = "HYPERSURFACE"
	DomainRegion       = "REGION"
	DomainInterval     = "INTERVAL"
	DomainPoint        = "POINT"
)

// ValidDomainKind reports membership in the closed domain vocabulary.
func ValidDomainKind(k string) bool {
	switch k {
	case DomainHypersurface, DomainRegion, DomainInterval, DomainPoint:
		return true
	}
	return false
}

// Domain is an identity-bearing integral domain. Family physmath/domain/1.
type Domain struct {
	Kind         string
	DefiningRefs []artifact.ArtifactRef // SET → expression/1 or equation/1
}

// Family returns FamilyDomain.
func (Domain) Family() FamilyID { return FamilyDomain }

// Encode returns the canonical payload.
func (dm Domain) Encode() ([]byte, error) {
	k, err := artifact.EncodeEnum(dm.Kind)
	if err != nil {
		return nil, err
	}
	elems := make([][]byte, len(dm.DefiningRefs))
	for i, r := range dm.DefiningRefs {
		elems[i] = r.Encode()
	}
	set, err := artifact.EncodeSet(elems)
	if err != nil {
		return nil, err
	}
	return append(k, set...), nil
}

// Validate enforces the closed kind vocabulary, grammar, and SET uniqueness.
// MINOR-02 invariant: every domain kind, including POINT, requires at least
// one defining reference. The kind names a type of domain, never which
// particular domain is meant; identity comes from the references.
func (dm Domain) Validate() error {
	if !ValidDomainKind(dm.Kind) {
		return fmt.Errorf("physmath: invalid Domain kind %q", dm.Kind)
	}
	if len(dm.DefiningRefs) == 0 {
		return fmt.Errorf("physmath: Domain kind %q needs at least one defining reference", dm.Kind)
	}
	fields := map[string]artifact.ArtifactRef{}
	for i, r := range dm.DefiningRefs {
		fields[fmt.Sprintf("Domain.DefiningRefs[%d]", i)] = r
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	return checkDuplicateRefs("Domain.DefiningRefs", dm.DefiningRefs)
}

// TargetChecks: DefiningRefs → expression/1 or equation/1.
func (dm Domain) TargetChecks() []TargetCheck {
	checks := make([]TargetCheck, len(dm.DefiningRefs))
	for i, r := range dm.DefiningRefs {
		checks[i] = TargetCheck{
			Field:   fmt.Sprintf("Domain.DefiningRefs[%d]", i),
			Ref:     r,
			Allowed: []FamilyID{FamilyExpression, FamilyEquation},
		}
	}
	return checks
}

// Integral represents (never executes) an integral of integrand references
// over a domain, with an optional weight expression (e.g. a measure
// factor). There is no Phase-1 rule that consumes integral/1: Apply and
// Replay can validate it as data but can never execute it.
// Family physmath/integral/1.
type Integral struct {
	DomainRef     artifact.ArtifactRef   // → domain/1 (explicit artifact ref, never a label)
	IntegrandRefs []artifact.ArtifactRef // SET → expression/1 or tensor/1
	WeightRef     *artifact.ArtifactRef  // optional → expression/1
}

// Family returns FamilyIntegral.
func (Integral) Family() FamilyID { return FamilyIntegral }

// Encode returns the canonical payload in declaration order.
func (in Integral) Encode() ([]byte, error) {
	out := in.DomainRef.Encode()
	elems := make([][]byte, len(in.IntegrandRefs))
	for i, r := range in.IntegrandRefs {
		elems[i] = r.Encode()
	}
	set, err := artifact.EncodeSet(elems)
	if err != nil {
		return nil, err
	}
	out = append(out, set...)
	if in.WeightRef == nil {
		return append(out, artifact.EncodeOptional(false, nil)...), nil
	}
	return append(out, artifact.EncodeOptional(true, in.WeightRef.Encode())...), nil
}

// Validate enforces a non-empty integrand set, grammar, and SET uniqueness.
func (in Integral) Validate() error {
	if len(in.IntegrandRefs) == 0 {
		return fmt.Errorf("physmath: Integral needs at least one integrand")
	}
	fields := map[string]artifact.ArtifactRef{"Integral.DomainRef": in.DomainRef}
	for i, r := range in.IntegrandRefs {
		fields[fmt.Sprintf("Integral.IntegrandRefs[%d]", i)] = r
	}
	if in.WeightRef != nil {
		fields["Integral.WeightRef"] = *in.WeightRef
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	return checkDuplicateRefs("Integral.IntegrandRefs", in.IntegrandRefs)
}

// TargetChecks: DomainRef → domain/1; IntegrandRefs → expression/1 or
// tensor/1; WeightRef → expression/1 when present.
func (in Integral) TargetChecks() []TargetCheck {
	checks := []TargetCheck{
		{Field: "Integral.DomainRef", Ref: in.DomainRef, Allowed: []FamilyID{FamilyDomain}},
	}
	for i, r := range in.IntegrandRefs {
		checks = append(checks, TargetCheck{
			Field:   fmt.Sprintf("Integral.IntegrandRefs[%d]", i),
			Ref:     r,
			Allowed: []FamilyID{FamilyExpression, FamilyTensor},
		})
	}
	if in.WeightRef != nil {
		checks = append(checks, TargetCheck{
			Field: "Integral.WeightRef", Ref: *in.WeightRef, Allowed: []FamilyID{FamilyExpression},
		})
	}
	return checks
}
