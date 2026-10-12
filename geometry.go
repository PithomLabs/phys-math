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
	// InverseRef optionally links a separately stored inverse metric/1
	// artifact. Mutual inversion is never verified here. Nil makes NO
	// inverse claim at all — neither self-inversion nor absence: it only
	// records that no inverse artifact is linked in this slice. Any
	// self-inversion fact (e.g. diag(-1,1,1,1)) is established by
	// construction review of the components, not by this field.
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

// DecodeDomain decodes a physmath/domain/1 canonical payload. Needed by
// cross-artifact consistency checks (e.g. integral-domain digest equality).
func DecodeDomain(payload []byte) (Domain, error) {
	d := artifact.NewDecoder(payload)
	kind, err := d.String()
	if err != nil {
		return Domain{}, fmt.Errorf("physmath: domain missing kind: %w", err)
	}
	if !ValidDomainKind(kind) {
		return Domain{}, fmt.Errorf("physmath: invalid Domain kind %q", kind)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return Domain{}, fmt.Errorf("physmath: domain missing defining set: %w", err)
	}
	refs := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return Domain{}, fmt.Errorf("physmath: domain bad defining ref %d: %w", i, err)
		}
		refs = append(refs, r)
	}
	if !d.Exhausted() {
		return Domain{}, fmt.Errorf("physmath: trailing bytes in domain payload")
	}
	dm := Domain{Kind: kind, DefiningRefs: refs}
	if err := dm.Validate(); err != nil {
		return Domain{}, err
	}
	return dm, nil
}

// DecodeIntegral decodes a physmath/integral/1 canonical payload.
func DecodeIntegral(payload []byte) (Integral, error) {
	d := artifact.NewDecoder(payload)
	domain, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return Integral{}, fmt.Errorf("physmath: integral missing domain: %w", err)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return Integral{}, fmt.Errorf("physmath: integral missing integrand set: %w", err)
	}
	integrands := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return Integral{}, fmt.Errorf("physmath: integral bad integrand %d: %w", i, err)
		}
		integrands = append(integrands, r)
	}
	present, err := d.OptionalPresence()
	if err != nil {
		return Integral{}, fmt.Errorf("physmath: integral missing weight presence: %w", err)
	}
	var weight *artifact.ArtifactRef
	if present {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return Integral{}, fmt.Errorf("physmath: integral bad weight: %w", err)
		}
		weight = &r
	}
	if !d.Exhausted() {
		return Integral{}, fmt.Errorf("physmath: trailing bytes in integral payload")
	}
	in := Integral{DomainRef: domain, IntegrandRefs: integrands, WeightRef: weight}
	if err := in.Validate(); err != nil {
		return Integral{}, err
	}
	return in, nil
}

// --- R2/R3: diagonal determinant, signature classes, volume density ---
//
// The determinant of a diagonal metric is computed through the EXISTING
// executable path (Mul construction + EXPR-CANON-001 replay). No new rule,
// no registry change. What is new here: diagonality enforcement,
// signature-class validation, and volume-density representation with
// assumption-gated reduction. Square-root / absolute-value evaluation is
// NOT implemented anywhere: FactorRef linkages are represented and
// assumption-gated, never executed.

// Signature classes: closed mathematical vocabulary (Sylvester level).
// Positive-definite vs Lorentzian determines absolute-value handling in
// volume density. Physical readings of coordinates stay framework-owned.
const (
	SignaturePositiveDefinite = "POSITIVE_DEFINITE"
	SignatureLorentzian       = "LORENTZIAN"
)

// ValidSignatureClass reports membership in the closed signature vocabulary.
func ValidSignatureClass(s string) bool {
	return s == SignaturePositiveDefinite || s == SignatureLorentzian
}

// DecodeSymmetricBilinear decodes a physmath/symmetric_bilinear/1 payload.
func DecodeSymmetricBilinear(payload []byte) (SymmetricBilinear, error) {
	d := artifact.NewDecoder(payload)
	dim, err := d.UInt16()
	if err != nil {
		return SymmetricBilinear{}, fmt.Errorf("physmath: metric missing dimension: %w", err)
	}
	n, err := d.SequenceCount()
	if err != nil {
		return SymmetricBilinear{}, fmt.Errorf("physmath: metric missing components: %w", err)
	}
	comps := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return SymmetricBilinear{}, fmt.Errorf("physmath: metric bad component %d: %w", i, err)
		}
		comps = append(comps, r)
	}
	present, err := d.OptionalPresence()
	if err != nil {
		return SymmetricBilinear{}, fmt.Errorf("physmath: metric missing inverse presence: %w", err)
	}
	var inv *artifact.ArtifactRef
	if present {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return SymmetricBilinear{}, fmt.Errorf("physmath: metric bad inverse: %w", err)
		}
		inv = &r
	}
	if !d.Exhausted() {
		return SymmetricBilinear{}, fmt.Errorf("physmath: trailing bytes in metric payload")
	}
	m := SymmetricBilinear{Dimension: dim, ComponentRefs: comps, InverseRef: inv}
	if err := m.Validate(); err != nil {
		return SymmetricBilinear{}, err
	}
	return m, nil
}

// zeroPayload is the canonical encoding of rational 0 (0/1) for
// diagonality comparison.
func zeroPayload() []byte {
	p, err := MustRational(0, 1).Encode()
	if err != nil {
		panic(err)
	}
	return p
}

// DiagonalDeterminant computes det(diag) = product of diagonal components
// through the existing executable path: it mints the Mul premise, replays
// EXPR-CANON-001 then EXPR-SIMPLIFY-001 (constant folding to the evaluated
// value), and returns the determinant reference plus the replayable
// two-step derivation. Off-diagonal entries must be canonical zero;
// anything else fails closed (non-diagonal input rejected, never coerced).
func DiagonalDeterminant(store Store, metricRef artifact.ArtifactRef) (artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, Derivation{}, err
	}
	fam, payload, err := store.Lookup(metricRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: determinant metric unresolvable: %w", err))
	}
	if fam != FamilySymmetricBilinear {
		return fail(fmt.Errorf("physmath: determinant target is not symmetric_bilinear/1"))
	}
	m, err := DecodeSymmetricBilinear(payload)
	if err != nil {
		return fail(err)
	}
	n := int(m.Dimension)
	diag := make([]Expr, 0, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			r := m.ComponentRefs[i*n+j]
			cfam, cpayload, err := store.Lookup(r)
			if err != nil {
				return fail(fmt.Errorf("physmath: metric component (%d,%d) unresolvable: %w", i, j, err))
			}
			if cfam != FamilyExpression {
				return fail(fmt.Errorf("physmath: metric component (%d,%d) is not expression/1", i, j))
			}
			if i == j {
				e, err := DecodeExpr(artifact.NewDecoder(cpayload))
				if err != nil {
					return fail(err)
				}
				diag = append(diag, e)
				continue
			}
			if string(cpayload) != string(zeroPayload()) {
				return fail(fmt.Errorf("physmath: metric not diagonal at (%d,%d): %w", i, j, ErrBadInput))
			}
		}
	}
	prodRef, err := PutExpression(store, Mul(diag...))
	if err != nil {
		return fail(err)
	}
	canon, err := Apply(store, Invocation{
		ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
		InputRefs: []artifact.ArtifactRef{prodRef},
	})
	if err != nil {
		return fail(err)
	}
	res, err := Apply(store, Invocation{
		ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
		InputRefs: []artifact.ArtifactRef{canon.OutputRef},
	})
	if err != nil {
		return fail(err)
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

// CheckSignature validates a declared signature class against a diagonal
// metric's rational diagonal entries: POSITIVE_DEFINITE needs all > 0;
// LORENTZIAN needs exactly one < 0 with the rest > 0 and none zero.
// Any non-rational diagonal entry (e.g. symbolic -xbar^2) yields
// ErrSignatureUnverifiable: uncertainty preserved, never asserted.
// Symmetry alone never implies a class (checked separately).
func CheckSignature(store Store, metricRef artifact.ArtifactRef, declared string) error {
	if !ValidSignatureClass(declared) {
		return fmt.Errorf("physmath: invalid signature class %q", declared)
	}
	fam, payload, err := store.Lookup(metricRef)
	if err != nil {
		return fmt.Errorf("physmath: signature metric unresolvable: %w", err)
	}
	if fam != FamilySymmetricBilinear {
		return fmt.Errorf("physmath: signature target is not symmetric_bilinear/1")
	}
	m, err := DecodeSymmetricBilinear(payload)
	if err != nil {
		return err
	}
	n := int(m.Dimension)
	neg, pos := 0, 0
	for i := 0; i < n; i++ {
		r := m.ComponentRefs[i*n+i]
		cfam, cpayload, err := store.Lookup(r)
		if err != nil {
			return fmt.Errorf("physmath: diagonal entry %d unresolvable: %w", i, err)
		}
		if cfam != FamilyExpression {
			return fmt.Errorf("physmath: diagonal entry %d is not expression/1", i)
		}
		e, err := DecodeExpr(artifact.NewDecoder(cpayload))
		if err != nil {
			return err
		}
		if e.Kind != TagRational {
			return fmt.Errorf("physmath: diagonal entry %d not rationally decidable: %w",
				i, ErrSignatureUnverifiable)
		}
		switch e.Numerator.ToBig().Sign() {
		case 0:
			return fmt.Errorf("physmath: degenerate zero diagonal at %d", i)
		case -1:
			neg++
		default:
			pos++
		}
	}
	switch declared {
	case SignaturePositiveDefinite:
		if neg != 0 || pos != n {
			return fmt.Errorf("physmath: not positive-definite (%d negative of %d)", neg, n)
		}
	case SignatureLorentzian:
		if neg != 1 || pos != n-1 {
			return fmt.Errorf("physmath: not Lorentzian (neg=%d of %d)", neg, n)
		}
	}
	return nil
}

// FamilyVolumeDensity is the metric-derived volume-density factor
// mu = sqrt(|det|), represented with its exact sources and assumptions.
// The square-root / absolute-value evaluation is NOT executed anywhere:
// FactorRef is a represented expression, and any reduction (e.g. |xbar|
// to xbar) requires cited reduction conditions. An unlicensed reduction
// (ReductionRef present without conditions) fails validation.
// Family physmath/volume_density/1.
var FamilyVolumeDensity = FamilyID{NamespacePhysMath, "volume_density/1", "1"}

// VolumeDensity links a metric, its computed determinant value, the
// replayable determinant derivation, a signature class, and a represented
// volume factor, with assumption-gated reduction. Determinant-value
// consistency with the metric is established by CheckVolumeDensity
// (store-aware: resolves the derivation, replays the exact CANON+SIMPLIFY
// path, and matches the diagonal-expression premise and determinant output),
// not by Validate: a bare structural check here verifies presence and
// grammar of the derivation reference but never resolves or replays it.
//
// Provenance limit (MINOR-04 correction 2): premise matching binds the
// derivation to the metric's determinant-relevant diagonal-expression
// content, not to the exact metric artifact identity. Two metric artifacts
// with byte-identical diagonal expressions share the same premise and
// correctly share validity under the provenance check.
type VolumeDensity struct {
	MetricRef                artifact.ArtifactRef   // → symmetric_bilinear/1
	DeterminantRef           artifact.ArtifactRef   // → expression/1 (computed det value)
	DeterminantDerivationRef artifact.ArtifactRef   // required → derivation/1 (exact CANON+SIMPLIFY det evidence)
	SignatureClass           string                 // closed vocabulary
	FactorRef                artifact.ArtifactRef   // → expression/1 (mu expression, represented)
	ReductionRef             *artifact.ArtifactRef  // optional → expression/1 (reduced factor)
	ReductionConditionRefs   []artifact.ArtifactRef // SET → condition/1; REQUIRED iff ReductionRef present
	DomainAssumptionRefs     []artifact.ArtifactRef // SET → condition/1 (coordinate-domain assumptions, e.g. xbar>0)
}

// Family returns FamilyVolumeDensity.
func (VolumeDensity) Family() FamilyID { return FamilyVolumeDensity }

// Encode returns the canonical payload in declaration order. The
// derivation reference is identity-bearing: changing the evidence changes
// the volume-density digest.
func (v VolumeDensity) Encode() ([]byte, error) {
	out := v.MetricRef.Encode()
	out = append(out, v.DeterminantRef.Encode()...)
	out = append(out, v.DeterminantDerivationRef.Encode()...)
	sc, err := artifact.EncodeEnum(v.SignatureClass)
	if err != nil {
		return nil, err
	}
	out = append(out, sc...)
	out = append(out, v.FactorRef.Encode()...)
	if v.ReductionRef == nil {
		out = append(out, artifact.EncodeOptional(false, nil)...)
	} else {
		out = append(out, artifact.EncodeOptional(true, v.ReductionRef.Encode())...)
	}
	elems := make([][]byte, len(v.ReductionConditionRefs))
	for i, r := range v.ReductionConditionRefs {
		elems[i] = r.Encode()
	}
	set, err := artifact.EncodeSet(elems)
	if err != nil {
		return nil, err
	}
	out = append(out, set...)
	delems := make([][]byte, len(v.DomainAssumptionRefs))
	for i, r := range v.DomainAssumptionRefs {
		delems[i] = r.Encode()
	}
	dset, err := artifact.EncodeSet(delems)
	if err != nil {
		return nil, err
	}
	return append(out, dset...), nil
}

// Validate enforces presence and grammar of the required derivation
// reference, the closed signature vocabulary, SET uniqueness, and the key
// gate: a claimed reduction without cited reduction conditions is rejected
// (no unlicensed absolute-value reduction). It performs no store Lookup,
// no family resolution, and no replay: a missing/zero or grammatically
// invalid derivation reference fails here, but resolvability, derivation
// shape, replay success, and premise/result matching belong exclusively to
// CheckVolumeDensity. It never evaluates square roots or absolute
// values.
func (v VolumeDensity) Validate() error {
	if !ValidSignatureClass(v.SignatureClass) {
		return fmt.Errorf("physmath: invalid VolumeDensity signature class %q", v.SignatureClass)
	}
	if v.ReductionRef != nil && len(v.ReductionConditionRefs) == 0 {
		return fmt.Errorf("physmath: volume reduction without cited conditions fails closed")
	}
	fields := map[string]artifact.ArtifactRef{
		"VolumeDensity.MetricRef": v.MetricRef, "VolumeDensity.DeterminantRef": v.DeterminantRef,
		"VolumeDensity.DeterminantDerivationRef": v.DeterminantDerivationRef,
		"VolumeDensity.FactorRef":                v.FactorRef,
	}
	if v.ReductionRef != nil {
		fields["VolumeDensity.ReductionRef"] = *v.ReductionRef
	}
	for i, r := range v.ReductionConditionRefs {
		fields[getRefFieldName("VolumeDensity.ReductionConditionRefs", i)] = r
	}
	for i, r := range v.DomainAssumptionRefs {
		fields[getRefFieldName("VolumeDensity.DomainAssumptionRefs", i)] = r
	}
	if err := validateRefGrammars(fields); err != nil {
		return err
	}
	if err := checkDuplicateRefs("VolumeDensity.ReductionConditionRefs", v.ReductionConditionRefs); err != nil {
		return err
	}
	return checkDuplicateRefs("VolumeDensity.DomainAssumptionRefs", v.DomainAssumptionRefs)
}

func getRefFieldName(base string, i int) string {
	return fmt.Sprintf("%s[%d]", base, i)
}

// TargetChecks: MetricRef → symmetric_bilinear/1; DeterminantRef,
// FactorRef → expression/1; DeterminantDerivationRef → derivation/1;
// ReductionRef → expression/1 when present; condition sets → condition/1.
// These expectations are resolved only by the store-aware provenance check,
// never by Validate alone.
func (v VolumeDensity) TargetChecks() []TargetCheck {
	checks := []TargetCheck{
		{Field: "VolumeDensity.MetricRef", Ref: v.MetricRef, Allowed: []FamilyID{FamilySymmetricBilinear}},
		{Field: "VolumeDensity.DeterminantRef", Ref: v.DeterminantRef, Allowed: []FamilyID{FamilyExpression}},
		{Field: "VolumeDensity.DeterminantDerivationRef", Ref: v.DeterminantDerivationRef, Allowed: []FamilyID{FamilyDerivation}},
		{Field: "VolumeDensity.FactorRef", Ref: v.FactorRef, Allowed: []FamilyID{FamilyExpression}},
	}
	if v.ReductionRef != nil {
		checks = append(checks, TargetCheck{
			Field: "VolumeDensity.ReductionRef", Ref: *v.ReductionRef, Allowed: []FamilyID{FamilyExpression},
		})
	}
	for i, r := range v.ReductionConditionRefs {
		checks = append(checks, TargetCheck{
			Field: getRefFieldName("VolumeDensity.ReductionConditionRefs", i),
			Ref:   r, Allowed: []FamilyID{FamilyCondition},
		})
	}
	for i, r := range v.DomainAssumptionRefs {
		checks = append(checks, TargetCheck{
			Field: getRefFieldName("VolumeDensity.DomainAssumptionRefs", i),
			Ref:   r, Allowed: []FamilyID{FamilyCondition},
		})
	}
	return checks
}

// DecodeVolumeDensity decodes a physmath/volume_density/1 canonical payload
// in declaration order. It is representation only: it checks field order,
// grammar (via Validate), and exhaustion, but performs no store Lookup and
// no replay. Provenance must still be established with
// CheckVolumeDensity.
func DecodeVolumeDensity(payload []byte) (VolumeDensity, error) {
	d := artifact.NewDecoder(payload)
	metric, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return VolumeDensity{}, fmt.Errorf("physmath: volume missing metric: %w", err)
	}
	det, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return VolumeDensity{}, fmt.Errorf("physmath: volume missing determinant: %w", err)
	}
	deriv, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return VolumeDensity{}, fmt.Errorf("physmath: volume missing determinant derivation: %w", err)
	}
	sig, err := d.String()
	if err != nil {
		return VolumeDensity{}, fmt.Errorf("physmath: volume missing signature class: %w", err)
	}
	factor, err := artifact.DecodeArtifactRef(d)
	if err != nil {
		return VolumeDensity{}, fmt.Errorf("physmath: volume missing factor: %w", err)
	}
	present, err := d.OptionalPresence()
	if err != nil {
		return VolumeDensity{}, fmt.Errorf("physmath: volume missing reduction presence: %w", err)
	}
	var red *artifact.ArtifactRef
	if present {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return VolumeDensity{}, fmt.Errorf("physmath: volume bad reduction: %w", err)
		}
		red = &r
	}
	n, err := d.SequenceCount()
	if err != nil {
		return VolumeDensity{}, fmt.Errorf("physmath: volume missing reduction conditions: %w", err)
	}
	rconds := make([]artifact.ArtifactRef, 0, n)
	for i := 0; i < n; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return VolumeDensity{}, fmt.Errorf("physmath: volume bad reduction condition %d: %w", i, err)
		}
		rconds = append(rconds, r)
	}
	m, err := d.SequenceCount()
	if err != nil {
		return VolumeDensity{}, fmt.Errorf("physmath: volume missing domain assumptions: %w", err)
	}
	dass := make([]artifact.ArtifactRef, 0, m)
	for i := 0; i < m; i++ {
		r, err := artifact.DecodeArtifactRef(d)
		if err != nil {
			return VolumeDensity{}, fmt.Errorf("physmath: volume bad domain assumption %d: %w", i, err)
		}
		dass = append(dass, r)
	}
	if !d.Exhausted() {
		return VolumeDensity{}, fmt.Errorf("physmath: trailing bytes in volume-density payload")
	}
	v := VolumeDensity{
		MetricRef: metric, DeterminantRef: det, DeterminantDerivationRef: deriv,
		SignatureClass: sig, FactorRef: factor, ReductionRef: red,
		ReductionConditionRefs: rconds, DomainAssumptionRefs: dass,
	}
	if err := v.Validate(); err != nil {
		return VolumeDensity{}, err
	}
	return v, nil
}

// diagonalProductRef recomputes the expected determinant-multiplication
// premise for a stored diagonal metric: it decodes the metric, enforces
// diagonality with the same zeroPayload gate as DiagonalDeterminant, builds
// Mul(diag...) and mints it with PutExpression. Content-addressing makes the
// recomputed reference byte-comparable to the derivation premise recorded
// by DiagonalDeterminant. It executes no rewrite rule: Apply/Replay are
// never invoked here.
func diagonalProductRef(store Store, metricRef artifact.ArtifactRef) (artifact.ArtifactRef, error) {
	fail := func(err error) (artifact.ArtifactRef, error) {
		return artifact.ArtifactRef{}, err
	}
	fam, payload, err := store.Lookup(metricRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: provenance metric unresolvable: %w", err))
	}
	if fam != FamilySymmetricBilinear {
		return fail(fmt.Errorf("physmath: provenance metric is not symmetric_bilinear/1"))
	}
	m, err := DecodeSymmetricBilinear(payload)
	if err != nil {
		return fail(err)
	}
	n := int(m.Dimension)
	diag := make([]Expr, 0, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			r := m.ComponentRefs[i*n+j]
			cfam, cpayload, err := store.Lookup(r)
			if err != nil {
				return fail(fmt.Errorf("physmath: provenance component (%d,%d) unresolvable: %w", i, j, err))
			}
			if cfam != FamilyExpression {
				return fail(fmt.Errorf("physmath: provenance component (%d,%d) is not expression/1", i, j))
			}
			if i == j {
				e, err := DecodeExpr(artifact.NewDecoder(cpayload))
				if err != nil {
					return fail(err)
				}
				diag = append(diag, e)
				continue
			}
			if string(cpayload) != string(zeroPayload()) {
				return fail(fmt.Errorf("physmath: provenance metric not diagonal at (%d,%d): %w", i, j, ErrBadInput))
			}
		}
	}
	return PutExpression(store, Mul(diag...))
}

// CheckVolumeDensity is the store-aware provenance gate for
// volume-density records (MINOR-04 correction). Validate checks structure
// only; this operation resolves and verifies the actual evidence:
//
//  1. Every TargetChecks entry resolves to its closed allowed family
//     (metric → symmetric_bilinear/1, determinant/factor → expression/1,
//     derivation → derivation/1, conditions → condition/1).
//  2. The derivation artifact decodes and has exactly the two-step
//     DiagonalDeterminant shape: ordinal 0 (physmath, EXPR-CANON-001),
//     ordinal 1 (physmath, EXPR-SIMPLIFY-001), in declaration order, with
//     empty condition contexts and empty parameters, a single premise, and
//     a single result equal to VolumeDensity.DeterminantRef (full
//     ArtifactRef equality, not digest-only).
//  3. Step linkage is exact: step 0 inputs are the derivation premise,
//     step 1 inputs are step 0's output, step 1's output is the result.
//  4. The derivation replays through the existing Replay interface.
//  5. The derivation premise equals the recomputed Mul premise of the
//     declared metric's diagonal expressions (diagonalProductRef).
//
// Guarantee limit: step 5 binds determinant-relevant diagonal-expression
// content, not the exact metric artifact identity — two metric artifacts
// with byte-identical diagonal expressions share the same premise and
// correctly share validity here. Cross-metric (diagonal-content-changing),
// substituted-output, missing/corrupted-dependency, and unresolvable-evidence
// cases fail closed below.
func CheckVolumeDensity(store Store, v VolumeDensity) error {
	if err := v.Validate(); err != nil {
		return err
	}
	for _, c := range v.TargetChecks() {
		fam, _, err := store.Lookup(c.Ref)
		if err != nil {
			return fmt.Errorf("physmath: %s unresolvable: %w", c.Field, err)
		}
		match := false
		for _, a := range c.Allowed {
			if fam == a {
				match = true
				break
			}
		}
		if !match {
			return fmt.Errorf("physmath: %s targets %s/%s/%s, not in closed target set",
				c.Field, fam.Namespace, fam.ArtifactType, fam.SchemaVersion)
		}
	}
	dfam, dpayload, err := store.Lookup(v.DeterminantDerivationRef)
	if err != nil {
		return fmt.Errorf("physmath: determinant derivation unresolvable: %w", err)
	}
	if dfam != FamilyDerivation {
		return fmt.Errorf("physmath: determinant derivation is not derivation/1")
	}
	d, err := DecodeDerivation(dpayload)
	if err != nil {
		return err
	}
	if len(d.PremiseRefs) != 1 {
		return fmt.Errorf("physmath: determinant derivation must carry exactly one premise")
	}
	if len(d.ResultRefs) != 1 || d.ResultRefs[0] != v.DeterminantRef {
		return fmt.Errorf("physmath: determinant derivation result does not match VolumeDensity.DeterminantRef")
	}
	if len(d.Steps) != 2 {
		return fmt.Errorf("physmath: determinant derivation must have exactly two steps")
	}
	s0, s1 := d.Steps[0], d.Steps[1]
	if s0.Ordinal != 0 || s1.Ordinal != 1 {
		return fmt.Errorf("physmath: determinant derivation steps must be ordinals 0,1 in order")
	}
	if s0.ProducerNamespace != NamespacePhysMath || s0.RuleID != RuleExprCanon {
		return fmt.Errorf("physmath: determinant derivation step 0 must be (physmath, EXPR-CANON-001)")
	}
	if s1.ProducerNamespace != NamespacePhysMath || s1.RuleID != RuleExprSimplify {
		return fmt.Errorf("physmath: determinant derivation step 1 must be (physmath, EXPR-SIMPLIFY-001)")
	}
	if len(s0.ConditionRefs) != 0 || len(s1.ConditionRefs) != 0 {
		return fmt.Errorf("physmath: determinant derivation steps must carry no conditions")
	}
	if len(s0.Parameters) != 0 || len(s1.Parameters) != 0 {
		return fmt.Errorf("physmath: determinant derivation steps must carry no parameters")
	}
	if len(s0.InputRefs) != 1 || s0.InputRefs[0] != d.PremiseRefs[0] {
		return fmt.Errorf("physmath: determinant derivation step 0 must consume the derivation premise")
	}
	if len(s1.InputRefs) != 1 || s1.InputRefs[0] != s0.OutputRef {
		return fmt.Errorf("physmath: determinant derivation step 1 must consume step 0 output")
	}
	if s1.OutputRef != d.ResultRefs[0] {
		return fmt.Errorf("physmath: determinant derivation step 1 must produce the derivation result")
	}
	if err := Replay(store, d, nil); err != nil {
		return fmt.Errorf("physmath: determinant derivation does not replay: %w", err)
	}
	expected, err := diagonalProductRef(store, v.MetricRef)
	if err != nil {
		return err
	}
	if d.PremiseRefs[0] != expected {
		return fmt.Errorf("physmath: determinant derivation premise does not match the declared metric diagonals")
	}
	return nil
}
