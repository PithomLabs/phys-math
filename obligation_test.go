package physmath

import (
	"strings"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// obligationInstances builds the warped and product divergence chains in
// one store and returns their evaluated instance references. Sharing one
// store proves the instances coexist as distinct artifacts.
func obligationInstances(t *testing.T, s Store) (warped, product artifact.ArtifactRef) {
	t.Helper()
	wchart, wmetric, wfn, wmapping, _, _, _, wcond := warpedSetup(t, s)
	_, wriem, _, err := EvaluateRiemann(s, wmetric, wchart, wmapping, wfn, []artifact.ArtifactRef{wcond})
	if err != nil {
		t.Fatal(err)
	}
	_, wcrec, _, err := EvaluateRiemannContraction(s, wriem)
	if err != nil {
		t.Fatal(err)
	}
	_, wsrec, _, err := EvaluateRiemannScalar(s, wcrec)
	if err != nil {
		t.Fatal(err)
	}
	_, wtrec, _, err := EvaluateRiemannTraceReverse(s, wcrec, wsrec)
	if err != nil {
		t.Fatal(err)
	}
	_, warped, _, err = EvaluateRiemannTraceReverseDivergence(s, wtrec)
	if err != nil {
		t.Fatal(err)
	}
	pchart, pmetric, pfn, pmap, _, _, _, pcond := prodSetup(t, s)
	_, priem, _, err := EvaluateRiemann(s, pmetric, pchart, pmap, pfn, []artifact.ArtifactRef{pcond})
	if err != nil {
		t.Fatal(err)
	}
	_, pcrec, _, err := EvaluateRiemannContraction(s, priem)
	if err != nil {
		t.Fatal(err)
	}
	_, psrec, _, err := EvaluateRiemannScalar(s, pcrec)
	if err != nil {
		t.Fatal(err)
	}
	_, ptrec, _, err := EvaluateRiemannTraceReverse(s, pcrec, psrec)
	if err != nil {
		t.Fatal(err)
	}
	_, product, _, err = EvaluateRiemannTraceReverseDivergence(s, ptrec)
	if err != nil {
		t.Fatal(err)
	}
	if warped == product {
		t.Fatal("warped and product divergence instances coincide")
	}
	return warped, product
}

// obligationRecord resolves an obligation record for inspection.
func obligationRecord(t *testing.T, s Store, ref artifact.ArtifactRef) RiemannTraceReverseDivergenceObligation {
	t.Helper()
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyRiemannTraceReverseDivergenceObligation {
		t.Fatalf("record is %s/%s, not obligation", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeRiemannTraceReverseDivergenceObligation(payload)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// tamperObligationRecord decodes an obligation record, applies mutate, and
// re-mints it as a new artifact.
func tamperObligationRecord(t *testing.T, s Store, ref artifact.ArtifactRef, mutate func(*RiemannTraceReverseDivergenceObligation)) artifact.ArtifactRef {
	t.Helper()
	rec := obligationRecord(t, s, ref)
	mutate(&rec)
	raw, err := rec.Encode()
	if err != nil {
		t.Fatalf("tampered record does not encode: %v", err)
	}
	bad, err := s.Put(FamilyRiemannTraceReverseDivergenceObligation, raw)
	if err != nil {
		t.Fatal(err)
	}
	return bad
}

// obligationFixture prebuilds both divergence instances plus the
// two-instance and warped-only obligation records once per parent test.
// Subtests clone the store and tamper their copy. TestObligationEvaluated
// keeps its own fresh build as the construction control.
type obligationFixture struct {
	store     *MemStore
	warped    artifact.ArtifactRef
	product   artifact.ArtifactRef
	recBoth   artifact.ArtifactRef
	recWarped artifact.ArtifactRef
}

func buildObligationFixture(t *testing.T) *obligationFixture {
	t.Helper()
	s := NewMemStore()
	warped, product := obligationInstances(t, s)
	recBoth, _, err := EvaluateRiemannTraceReverseDivergenceObligation(s, []artifact.ArtifactRef{warped, product})
	if err != nil {
		t.Fatal(err)
	}
	recWarped, _, err := EvaluateRiemannTraceReverseDivergenceObligation(s, []artifact.ArtifactRef{warped})
	if err != nil {
		t.Fatal(err)
	}
	return &obligationFixture{store: s, warped: warped, product: product, recBoth: recBoth, recWarped: recWarped}
}

// TestObligationEvaluated pins the valid two-geometry obligation: claim,
// scope, support, instance linkage, evidence classification per component,
// and acceptance by both gates.
func TestObligationEvaluated(t *testing.T) {
	s := NewMemStore()
	warped, product := obligationInstances(t, s)
	recRef, dd, err := EvaluateRiemannTraceReverseDivergenceObligation(s, []artifact.ArtifactRef{warped, product})
	if err != nil {
		t.Fatalf("obligation evaluation failed: %v", err)
	}
	rec := obligationRecord(t, s, recRef)
	if rec.Statement != ObligationStatementDivergenceVanishes {
		t.Fatalf("statement is %q", rec.Statement)
	}
	if rec.Scope != ObligationScopeEvaluatedInstances {
		t.Fatalf("scope is %q", rec.Scope)
	}
	if rec.Support != ObligationPartiallySupported {
		t.Fatalf("support is %q, want PARTIALLY_SUPPORTED", rec.Support)
	}
	if len(rec.DivergenceRefs) != 2 || rec.DivergenceRefs[0] != warped || rec.DivergenceRefs[1] != product {
		t.Fatal("obligation does not preserve ordered instance references")
	}
	// Composite derivation concatenates both instance derivations.
	wrec := divergenceRecord(t, s, warped)
	_, wdp, err := s.Lookup(wrec.DerivationRef)
	if err != nil {
		t.Fatal(err)
	}
	wdd, err := DecodeDerivation(wdp)
	if err != nil {
		t.Fatal(err)
	}
	prec := divergenceRecord(t, s, product)
	_, pdp, err := s.Lookup(prec.DerivationRef)
	if err != nil {
		t.Fatal(err)
	}
	pdd, err := DecodeDerivation(pdp)
	if err != nil {
		t.Fatal(err)
	}
	if len(dd.Steps) != len(wdd.Steps)+len(pdd.Steps) {
		t.Fatalf("composite has %d steps, instances cover %d", len(dd.Steps), len(wdd.Steps)+len(pdd.Steps))
	}
	// Evidence classification per component, preserving the evidence
	// type each geometry was established with:
	// - warped D_x: factored three-term Add (hand-verified nonzero
	//   cancellation), D_y/D_z: literal zero (structural identity).
	// - product D_x: factored two-term Add (structural drop-out),
	//   D_y/D_z: literal zero (structural identity).
	zero := MustRational(0, 1)
	if len(rec.ResultRefs) != 2 {
		t.Fatalf("obligation has %d results, want 2", len(rec.ResultRefs))
	}
	wcomps := divergenceComps(t, s, rec.ResultRefs[0])
	if len(wcomps) != 3 || wcomps[0].Kind != TagAdd || len(wcomps[0].Children) != 3 {
		t.Fatalf("warped obligation result is not a three-term factored sum:\n%s", showExpr(wcomps[0]))
	}
	for _, i := range []int{1, 2} {
		if !wcomps[i].Equal(zero) {
			t.Fatalf("warped obligation D[%d] is not exactly zero:\n%s", i, showExpr(wcomps[i]))
		}
	}
	pcomps := divergenceComps(t, s, rec.ResultRefs[1])
	if len(pcomps) != 3 || pcomps[0].Kind != TagAdd || len(pcomps[0].Children) != 2 {
		t.Fatalf("product obligation result is not a two-term factored sum:\n%s", showExpr(pcomps[0]))
	}
	for _, i := range []int{1, 2} {
		if !pcomps[i].Equal(zero) {
			t.Fatalf("product obligation D[%d] is not exactly zero:\n%s", i, showExpr(pcomps[i]))
		}
	}
	// The stored results are the instances' own evaluated results.
	if rec.ResultRefs[0] != wrec.ResultRef || rec.ResultRefs[1] != prec.ResultRef {
		t.Fatal("obligation results are not the instances' evaluated results")
	}
	if err := CheckRiemannTraceReverseDivergenceObligation(s, recRef); err != nil {
		t.Fatalf("valid obligation rejected by Check: %v", err)
	}
	if err := ReplayRiemannTraceReverseDivergenceObligation(s, recRef); err != nil {
		t.Fatalf("valid obligation rejected by artifact replay: %v", err)
	}
}

// TestObligationAdversarial proves corrupted obligation records fail Check
// and artifact Replay at the intended boundary. Both instances and both
// obligation records are built once in the fixture; each row clones the
// store and tampers its own copy.
func TestObligationAdversarial(t *testing.T) {
	bothFail := func(t *testing.T, s *MemStore, bad artifact.ArtifactRef, reason string) {
		t.Helper()
		if err := CheckRiemannTraceReverseDivergenceObligation(s, bad); err == nil {
			t.Fatal("corrupted record accepted by Check")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("Check wrong boundary (want %q): %v", reason, err)
		}
		if err := ReplayRiemannTraceReverseDivergenceObligation(s, bad); err == nil {
			t.Fatal("corrupted record accepted by artifact replay")
		} else if !strings.Contains(err.Error(), reason) {
			t.Fatalf("replay wrong boundary (want %q): %v", reason, err)
		}
	}
	fix := buildObligationFixture(t)

	t.Run("swapped-instances", func(t *testing.T) {
		s := cloneMemStore(t, fix.store)
		// Swap instance order while results stay put: each stored result
		// no longer matches its position's evaluated instance.
		bad := tamperObligationRecord(t, s, fix.recBoth, func(r *RiemannTraceReverseDivergenceObligation) {
			r.DivergenceRefs[0], r.DivergenceRefs[1] = r.DivergenceRefs[1], r.DivergenceRefs[0]
		})
		bothFail(t, s, bad, "does not match evaluated instance")
	})

	t.Run("substituted-result", func(t *testing.T) {
		s := cloneMemStore(t, fix.store)
		other := mustStoreExpr(t, s, MustRational(13, 1))
		bad := tamperObligationRecord(t, s, fix.recBoth, func(r *RiemannTraceReverseDivergenceObligation) { r.ResultRefs[0] = other })
		bothFail(t, s, bad, "not in closed target set")
	})

	t.Run("dropped-condition", func(t *testing.T) {
		s := cloneMemStore(t, fix.store)
		bad := tamperObligationRecord(t, s, fix.recBoth, func(r *RiemannTraceReverseDivergenceObligation) { r.ConditionRefs = r.ConditionRefs[:1] })
		bothFail(t, s, bad, "conditions do not match evaluated instances")
	})

	t.Run("foreign-condition", func(t *testing.T) {
		s := cloneMemStore(t, fix.store)
		// A valid condition from the product instance resolves in the
		// closed target set, so only the union gate can reject it.
		prec := divergenceRecord(t, s, fix.product)
		if len(prec.ConditionRefs) == 0 {
			t.Fatal("product instance has no conditions")
		}
		foreign := prec.ConditionRefs[0]
		bad := tamperObligationRecord(t, s, fix.recWarped, func(r *RiemannTraceReverseDivergenceObligation) {
			r.ConditionRefs = append(append([]artifact.ArtifactRef{}, r.ConditionRefs...), foreign)
		})
		bothFail(t, s, bad, "conditions do not match evaluated instances")
	})

	t.Run("unsupported-support-status", func(t *testing.T) {
		s := cloneMemStore(t, fix.store)
		// Promoting the status to a general-proof claim must not pass:
		// decoding rejects the unsupported value at both gates.
		bad := tamperObligationRecord(t, s, fix.recBoth, func(r *RiemannTraceReverseDivergenceObligation) { r.Support = "PROVED" })
		bothFail(t, s, bad, "support")
	})

	t.Run("tampered-claim-scope", func(t *testing.T) {
		s := cloneMemStore(t, fix.store)
		badClaim := tamperObligationRecord(t, s, fix.recBoth, func(r *RiemannTraceReverseDivergenceObligation) {
			r.Statement = "divergence vanishes everywhere by proof"
		})
		bothFail(t, s, badClaim, "statement mismatch")
		badScope := tamperObligationRecord(t, s, fix.recBoth, func(r *RiemannTraceReverseDivergenceObligation) {
			r.Scope = "all geometries"
		})
		bothFail(t, s, badScope, "scope mismatch")
	})

	t.Run("truncated-derivation", func(t *testing.T) {
		s := cloneMemStore(t, fix.store)
		rec := obligationRecord(t, s, fix.recBoth)
		_, dpayload, err := s.Lookup(rec.DerivationRef)
		if err != nil {
			t.Fatal(err)
		}
		d, err := DecodeDerivation(dpayload)
		if err != nil {
			t.Fatal(err)
		}
		short := Derivation{PremiseRefs: d.PremiseRefs, Steps: d.Steps[:len(d.Steps)-1], ResultRefs: d.ResultRefs}
		shortBytes, err := short.Encode()
		if err != nil {
			t.Fatal(err)
		}
		altDeriv, err := s.Put(FamilyDerivation, shortBytes)
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperObligationRecord(t, s, fix.recBoth, func(r *RiemannTraceReverseDivergenceObligation) { r.DerivationRef = altDeriv })
		bothFail(t, s, bad, "shorter than instance")
	})

	t.Run("ghost-instance", func(t *testing.T) {
		// No fixture needed: ghost references resolve nowhere.
		s := NewMemStore()
		ghost := ComputeTestRef(t, "physmath", "riemann_trace_reverse_divergence/1")
		if err := CheckRiemannTraceReverseDivergenceObligation(s, ghost); err == nil {
			t.Fatal("unresolvable record accepted")
		}
		if err := ReplayRiemannTraceReverseDivergenceObligation(s, ghost); err == nil {
			t.Fatal("unresolvable record replayed")
		}
		exprRef := mustStoreExpr(t, s, MustRational(1, 1))
		if err := CheckRiemannTraceReverseDivergenceObligation(s, exprRef); err == nil {
			t.Fatal("expression target accepted")
		}
	})
}

// TestObligationCodec pins the representation contract: round-trip,
// truncation/trailing rejection, and structural validation of empty,
// length-mismatched, duplicated, and mis-supported records.
func TestObligationCodec(t *testing.T) {
	fix := buildObligationFixture(t)
	s := cloneMemStore(t, fix.store)
	rec := obligationRecord(t, s, fix.recBoth)
	raw, err := rec.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeRiemannTraceReverseDivergenceObligation(raw); err != nil {
		t.Fatalf("record does not round-trip: %v", err)
	}
	if _, err := DecodeRiemannTraceReverseDivergenceObligation(raw[:len(raw)/2]); err == nil {
		t.Fatal("truncated record accepted")
	}
	if _, err := DecodeRiemannTraceReverseDivergenceObligation(append(append([]byte{}, raw...), 0x00)); err == nil {
		t.Fatal("trailing bytes accepted")
	}
	empty := rec
	empty.DivergenceRefs = nil
	empty.ResultRefs = nil
	if err := empty.Validate(); err == nil {
		t.Fatal("empty-instance record validated")
	}
	mismatched := rec
	mismatched.ResultRefs = mismatched.ResultRefs[:1]
	if err := mismatched.Validate(); err == nil {
		t.Fatal("length-mismatched record validated")
	}
	dup := rec
	dup.DivergenceRefs = []artifact.ArtifactRef{fix.warped, fix.warped}
	dup.ResultRefs = []artifact.ArtifactRef{rec.ResultRefs[0], rec.ResultRefs[0]}
	if err := dup.Validate(); err == nil {
		t.Fatal("duplicate-instance record validated")
	}
	badSupport := rec
	badSupport.Support = "PROVED"
	if err := badSupport.Validate(); err == nil {
		t.Fatal("general-proof status validated")
	}
}
