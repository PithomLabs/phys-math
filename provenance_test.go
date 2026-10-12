package physmath

import (
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// mustStoreDerivation encodes d and mints its physmath/derivation/1 artifact.
func mustStoreDerivation(t *testing.T, s Store, d Derivation) artifact.ArtifactRef {
	t.Helper()
	raw, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := s.Put(FamilyDerivation, raw)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// provenVolume builds a fully provenance-valid VolumeDensity for metric mref:
// computes the determinant, stores the derivation, and wires the reference.
func provenVolume(t *testing.T, s Store, mref artifact.ArtifactRef, sig string, factor artifact.ArtifactRef) (VolumeDensity, artifact.ArtifactRef, Derivation) {
	t.Helper()
	detRef, d, err := DiagonalDeterminant(s, mref)
	if err != nil {
		t.Fatal(err)
	}
	dref := mustStoreDerivation(t, s, d)
	v := VolumeDensity{MetricRef: mref, DeterminantRef: detRef,
		DeterminantDerivationRef: dref, SignatureClass: sig, FactorRef: factor}
	if err := v.Validate(); err != nil {
		t.Fatal(err)
	}
	return v, detRef, d
}

func TestVolumeProvenancePositive(t *testing.T) {
	s := NewMemStore()
	// Minkowski: exact -1, deterministic recomputation.
	mref, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	one := mustStoreExpr(t, s, MustRational(1, 1))
	mink, detRef, d := provenVolume(t, s, mref, SignatureLorentzian, one)
	if err := CheckVolumeDensity(s, mink); err != nil {
		t.Fatalf("Minkowski provenance failed: %v", err)
	}
	// Determinism: recomputation yields the identical determinant reference.
	detRef2, d2, err := DiagonalDeterminant(s, mref)
	if err != nil {
		t.Fatal(err)
	}
	if detRef2 != detRef {
		t.Fatal("determinant recomputation nondeterministic")
	}
	_ = d2
	_ = d
	// Stored derivation decodes and replays independently of the checker.
	fam, raw, err := s.Lookup(mink.DeterminantDerivationRef)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyDerivation {
		t.Fatalf("derivation ref family = %v, want derivation/1", fam)
	}
	dec, err := DecodeDerivation(raw)
	if err != nil {
		t.Fatalf("stored derivation does not decode: %v", err)
	}
	if err := Replay(s, dec, nil); err != nil {
		t.Fatalf("decoded derivation does not replay: %v", err)
	}
	// Stored volume round-trips through Encode/DecodeVolumeDensity.
	vraw, err := mink.Encode()
	if err != nil {
		t.Fatal(err)
	}
	vref, err := s.Put(FamilyVolumeDensity, vraw)
	if err != nil {
		t.Fatal(err)
	}
	_, got, err := s.Lookup(vref)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := DecodeVolumeDensity(got)
	if err != nil {
		t.Fatalf("stored volume does not decode: %v", err)
	}
	rtRaw, err := rt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(rtRaw) != string(vraw) {
		t.Fatal("volume round-trip changed identity")
	}
	if err := CheckVolumeDensity(s, rt); err != nil {
		t.Fatalf("decoded volume provenance failed: %v", err)
	}
}

func TestVolumeProvenanceRindler(t *testing.T) {
	s := NewMemStore()
	xbar := NewEphemeralSymbol("xbar")
	rref, _ := diagMetric(t, s, Neg(Pow(xbar, MustRational(2, 1))),
		MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	absx := mustStoreExpr(t, s, NewEphemeralSymbol("abs_xbar"))
	rvol, _, _ := provenVolume(t, s, rref, SignatureLorentzian, absx)
	if err := CheckVolumeDensity(s, rvol); err != nil {
		t.Fatalf("Rindler provenance failed: %v", err)
	}
}

func TestVolumeProvenanceCrossMetric(t *testing.T) {
	s := NewMemStore()
	// Metric A: Minkowski. Metric B: 2x2 positive-definite.
	aref, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	bref, _ := diagMetric(t, s, MustRational(2, 1), MustRational(3, 1))
	one := mustStoreExpr(t, s, MustRational(1, 1))
	// Valid evidence for A.
	adet, aDeriv, err := DiagonalDeterminant(s, aref)
	if err != nil {
		t.Fatal(err)
	}
	aderivRef := mustStoreDerivation(t, s, aDeriv)
	_ = adet
	// Pair metric B with A's determinant + A's derivation: premise mismatch.
	bdet, _, err := DiagonalDeterminant(s, bref)
	if err != nil {
		t.Fatal(err)
	}
	_ = bdet
	cross := VolumeDensity{MetricRef: bref, DeterminantRef: adet,
		DeterminantDerivationRef: aderivRef,
		SignatureClass:           SignaturePositiveDefinite, FactorRef: one}
	if err := cross.Validate(); err != nil {
		t.Fatalf("cross-metric record should be structurally valid: %v", err)
	}
	if err := CheckVolumeDensity(s, cross); err == nil {
		t.Fatal("determinant from another metric accepted by provenance check")
	}
}

func TestVolumeProvenanceUnrelatedDet(t *testing.T) {
	s := NewMemStore()
	mref, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	one := mustStoreExpr(t, s, MustRational(1, 1))
	// Unrelated determinant with the correct expression family (7, never computed).
	stray := mustStoreExpr(t, s, MustRational(7, 1))
	_, d, err := DiagonalDeterminant(s, mref)
	if err != nil {
		t.Fatal(err)
	}
	// Rewire the derivation result to the stray ref and re-store: simulates a
	// caller claiming an unrelated determinant through a tampered derivation.
	d.ResultRefs = []artifact.ArtifactRef{stray}
	d.Steps[1].OutputRef = stray
	tamperedRef := mustStoreDerivation(t, s, d)
	v := VolumeDensity{MetricRef: mref, DeterminantRef: stray,
		DeterminantDerivationRef: tamperedRef,
		SignatureClass:           SignatureLorentzian, FactorRef: one}
	if err := v.Validate(); err != nil {
		t.Fatalf("tampered record should be structurally valid: %v", err)
	}
	// Fails at replay (recomputed step output != declared tampered output).
	if err := CheckVolumeDensity(s, v); err == nil {
		t.Fatal("unrelated determinant accepted by provenance check")
	}
}

func TestVolumeProvenanceSubstitutedOutput(t *testing.T) {
	s := NewMemStore()
	mref, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	one := mustStoreExpr(t, s, MustRational(1, 1))
	vol, _, _ := provenVolume(t, s, mref, SignatureLorentzian, one)
	// Substitute the output reference after the derivation was produced.
	sub := vol
	sub.DeterminantRef = one
	if err := sub.Validate(); err != nil {
		t.Fatalf("substituted record should stay structurally valid: %v", err)
	}
	if err := CheckVolumeDensity(s, sub); err == nil {
		t.Fatal("substituted determinant reference accepted by provenance check")
	}
}

func TestVolumeProvenanceMissingEvidence(t *testing.T) {
	s := NewMemStore()
	mref, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	one := mustStoreExpr(t, s, MustRational(1, 1))
	detRef, _, err := DiagonalDeterminant(s, mref)
	if err != nil {
		t.Fatal(err)
	}
	// Zero derivation reference: structural presence gate must fail.
	v := VolumeDensity{MetricRef: mref, DeterminantRef: detRef,
		SignatureClass: SignatureLorentzian, FactorRef: one}
	if err := v.Validate(); err == nil {
		t.Fatal("missing derivation reference accepted by Validate")
	}
	// Unresolvable derivation reference: grammatically valid but absent.
	ghost := ComputeTestRef(t, "physmath", "derivation/1")
	v2 := VolumeDensity{MetricRef: mref, DeterminantRef: detRef,
		DeterminantDerivationRef: ghost,
		SignatureClass:           SignatureLorentzian, FactorRef: one}
	if err := v2.Validate(); err != nil {
		t.Fatalf("unresolvable-but-grammatical ref should pass Validate: %v", err)
	}
	if err := CheckVolumeDensity(s, v2); err == nil {
		t.Fatal("unresolvable derivation evidence accepted by provenance check")
	}
}

func TestVolumeProvenanceWrongDerivationFamily(t *testing.T) {
	s := NewMemStore()
	mref, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	one := mustStoreExpr(t, s, MustRational(1, 1))
	detRef, _, err := DiagonalDeterminant(s, mref)
	if err != nil {
		t.Fatal(err)
	}
	// Point the derivation reference at an expression/1 artifact instead.
	v := VolumeDensity{MetricRef: mref, DeterminantRef: detRef,
		DeterminantDerivationRef: one,
		SignatureClass:           SignatureLorentzian, FactorRef: one}
	if err := v.Validate(); err != nil {
		t.Fatalf("wrong-family ref should pass grammar Validate: %v", err)
	}
	if err := CheckVolumeDensity(s, v); err == nil {
		t.Fatal("wrong-family derivation evidence accepted by provenance check")
	}
}

func TestVolumeProvenanceReorderedSteps(t *testing.T) {
	s := NewMemStore()
	mref, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	one := mustStoreExpr(t, s, MustRational(1, 1))
	detRef, d, err := DiagonalDeterminant(s, mref)
	if err != nil {
		t.Fatal(err)
	}
	_ = detRef
	// Reorder steps: SIMPLIFY first, CANON second (ordinals swapped).
	d.Steps[0], d.Steps[1] = d.Steps[1], d.Steps[0]
	d.Steps[0].Ordinal, d.Steps[1].Ordinal = 0, 1
	reorderedRef := mustStoreDerivation(t, s, d)
	v := VolumeDensity{MetricRef: mref, DeterminantRef: detRef,
		DeterminantDerivationRef: reorderedRef,
		SignatureClass:           SignatureLorentzian, FactorRef: one}
	if err := CheckVolumeDensity(s, v); err == nil {
		t.Fatal("reordered derivation steps accepted by provenance check")
	}
}

func TestVolumeProvenanceDroppedPremise(t *testing.T) {
	s := NewMemStore()
	mref, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	one := mustStoreExpr(t, s, MustRational(1, 1))
	detRef, d, err := DiagonalDeterminant(s, mref)
	if err != nil {
		t.Fatal(err)
	}
	_ = detRef
	d.PremiseRefs = nil
	droppedRef := mustStoreDerivation(t, s, d)
	v := VolumeDensity{MetricRef: mref, DeterminantRef: detRef,
		DeterminantDerivationRef: droppedRef,
		SignatureClass:           SignatureLorentzian, FactorRef: one}
	if err := CheckVolumeDensity(s, v); err == nil {
		t.Fatal("derivation with dropped premise accepted by provenance check")
	}
}

func TestVolumeProvenanceCorruptedDependency(t *testing.T) {
	// Corrupted store: metric component made unresolvable after the
	// derivation was honestly produced. A fresh store keeps the refs but
	// drops one component payload.
	// diag(2,3): diagonals {2,3}, determinant 6 — no payload collisions
	// between diagonal, determinant, and factor expressions, so dropping
	// one diagonal payload is a genuine corruption.
	full := NewMemStore()
	mref, _ := diagMetric(t, full, MustRational(2, 1), MustRational(3, 1))
	one := mustStoreExpr(t, full, MustRational(1, 1))
	detRef, d, err := DiagonalDeterminant(full, mref)
	if err != nil {
		t.Fatal(err)
	}
	derivRef := mustStoreDerivation(t, full, d)
	vol := VolumeDensity{MetricRef: mref, DeterminantRef: detRef,
		DeterminantDerivationRef: derivRef,
		SignatureClass:           SignaturePositiveDefinite, FactorRef: one}

	// Rebuild a partial store: copy everything except one metric component.
	partial := NewMemStore()
	mfam, mpayload, err := full.Lookup(mref)
	if err != nil {
		t.Fatal(err)
	}
	m, err := DecodeSymmetricBilinear(mpayload)
	if err != nil {
		t.Fatal(err)
	}
	// Copy all component payloads except ComponentRefs[0].
	for i, r := range m.ComponentRefs {
		if i == 0 {
			continue
		}
		fam, payload, err := full.Lookup(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := partial.Put(fam, payload); err != nil {
			t.Fatal(err)
		}
	}
	// Copy metric, determinant, derivation, factor payloads.
	for _, r := range []artifact.ArtifactRef{mref, detRef, derivRef, one} {
		fam, payload, err := full.Lookup(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := partial.Put(fam, payload); err != nil {
			t.Fatal(err)
		}
	}
	// Copy derivation-internal refs (premise + step outputs) so the failure
	// is specifically the missing metric component, not the derivation.
	for _, r := range append(append([]artifact.ArtifactRef{}, d.PremiseRefs...), d.ResultRefs...) {
		fam, payload, err := full.Lookup(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := partial.Put(fam, payload); err != nil {
			t.Fatal(err)
		}
	}
	for _, st := range d.Steps {
		for _, r := range append(append([]artifact.ArtifactRef{}, st.InputRefs...), st.OutputRef) {
			fam, payload, err := full.Lookup(r)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := partial.Put(fam, payload); err != nil {
				t.Fatal(err)
			}
		}
	}
	_ = mfam
	if err := CheckVolumeDensity(partial, vol); err == nil {
		t.Fatal("corrupted metric dependency accepted by provenance check")
	}
}

func TestVolumeProvenanceRevisionChangesDiagonals(t *testing.T) {
	s := NewMemStore()
	one := mustStoreExpr(t, s, MustRational(1, 1))
	two := mustStoreExpr(t, s, MustRational(2, 1))
	// Revision 1: diag(-1,1,1,1). Revision 2: diag(-1,1,1,2).
	rev1, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(1, 1))
	rev2, _ := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1), MustRational(1, 1), MustRational(2, 1))
	_ = two
	det1, d1, err := DiagonalDeterminant(s, rev1)
	if err != nil {
		t.Fatal(err)
	}
	_ = det1
	d1ref := mustStoreDerivation(t, s, d1)
	det2, _, err := DiagonalDeterminant(s, rev2)
	if err != nil {
		t.Fatal(err)
	}
	// Reuse rev1 evidence with rev2's determinant: premise mismatch.
	reused := VolumeDensity{MetricRef: rev2, DeterminantRef: det2,
		DeterminantDerivationRef: d1ref,
		SignatureClass:           SignatureLorentzian, FactorRef: one}
	if err := CheckVolumeDensity(s, reused); err == nil {
		t.Fatal("stale-revision evidence accepted by provenance check")
	}
}

func TestVolumeProvenanceIdenticalDiagonalsShared(t *testing.T) {
	s := NewMemStore()
	one := mustStoreExpr(t, s, MustRational(1, 1))
	// Two distinct metric artifacts with byte-identical diagonal content,
	// differing only in InverseRef linkage (nil vs linked). Documented
	// limit: they share the same determinant premise and correctly share
	// validity under the provenance check, which binds diagonal-expression
	// content rather than exact artifact identity.
	m1, m1struct := diagMetric(t, s, MustRational(-1, 1), MustRational(1, 1))
	m2struct := SymmetricBilinear{Dimension: m1struct.Dimension, ComponentRefs: m1struct.ComponentRefs, InverseRef: &m1}
	if err := m2struct.Validate(); err != nil {
		t.Fatal(err)
	}
	m2, err := s.Put(FamilySymmetricBilinear, m2struct.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if m1 == m2 {
		t.Fatal("expected distinct metric artifacts")
	}
	det1, d1, err := DiagonalDeterminant(s, m1)
	if err != nil {
		t.Fatal(err)
	}
	d1ref := mustStoreDerivation(t, s, d1)
	shared := VolumeDensity{MetricRef: m2, DeterminantRef: det1,
		DeterminantDerivationRef: d1ref,
		SignatureClass:           SignatureLorentzian, FactorRef: one}
	if err := CheckVolumeDensity(s, shared); err != nil {
		t.Fatalf("identical-diagonal evidence wrongly rejected: %v", err)
	}
}

func TestDecodeDerivationRoundTrip(t *testing.T) {
	s := NewMemStore()
	mref, _ := diagMetric(t, s, MustRational(2, 1), MustRational(3, 1))
	_, d, err := DiagonalDeterminant(s, mref)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dec, err := DecodeDerivation(raw)
	if err != nil {
		t.Fatalf("round-trip decode failed: %v", err)
	}
	if len(dec.Steps) != 2 || len(dec.PremiseRefs) != 1 || len(dec.ResultRefs) != 1 {
		t.Fatal("round-trip changed derivation shape")
	}
	// Truncated payload fails.
	if _, err := DecodeDerivation(raw[:len(raw)/2]); err == nil {
		t.Fatal("truncated derivation accepted")
	}
	// Trailing bytes fail.
	if _, err := DecodeDerivation(append(append([]byte{}, raw...), 0x00)); err == nil {
		t.Fatal("trailing bytes accepted")
	}
}

func TestDecodeVolumeDensityMalformed(t *testing.T) {
	s := NewMemStore()
	mref, _ := diagMetric(t, s, MustRational(1, 1), MustRational(1, 1))
	one := mustStoreExpr(t, s, MustRational(1, 1))
	vol, _, _ := provenVolume(t, s, mref, SignaturePositiveDefinite, one)
	raw, err := vol.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeVolumeDensity(raw[:len(raw)/2]); err == nil {
		t.Fatal("truncated volume accepted")
	}
	if _, err := DecodeVolumeDensity(append(append([]byte{}, raw...), 0xFF)); err == nil {
		t.Fatal("trailing bytes accepted")
	}
	// Unknown rule in derivation payload fails decode.
	_, d, err := DiagonalDeterminant(s, mref)
	if err != nil {
		t.Fatal(err)
	}
	d.Steps[0].RuleID = "NOPE"
	badRaw, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeDerivation(badRaw); err == nil {
		t.Fatal("unknown rule pair accepted by derivation decode")
	}
}
