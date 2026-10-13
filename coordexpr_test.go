package physmath

import (
	"errors"
	"strings"
	"testing"

	"github.com/PithomLabs/phys-artifact"
)

// coordRecord resolves a coordinate_differentiation/1 record for inspection.
func coordRecord(t *testing.T, s Store, ref artifact.ArtifactRef) CoordinateDifferentiation {
	t.Helper()
	fam, payload, err := s.Lookup(ref)
	if err != nil {
		t.Fatal(err)
	}
	if fam != FamilyCoordinateDifferentiation {
		t.Fatalf("record is %s/%s, not coordinate_differentiation/1", fam.Namespace, fam.ArtifactType)
	}
	rec, err := DecodeCoordinateDifferentiation(payload)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// TestCoordDiffRoundTrip pins the record codec and structural gates.
func TestCoordDiffRoundTrip(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	src := mustStoreExpr(t, s, Pow(syms[0], MustRational(2, 1)))
	out, recRef, _, err := DifferentiateCoordinate(s, src, chart, 0, mapping, declFn, 1, nil)
	if err != nil {
		t.Fatalf("guarded differentiation failed: %v", err)
	}
	rec := coordRecord(t, s, recRef)
	if rec.SourceRef != src || rec.ChartRef != chart || rec.CoordIndex != 0 ||
		rec.DeclaringFnRef != declFn || rec.ResultRef != out || rec.Order != 1 {
		t.Fatal("record does not bind the construction inputs")
	}
	raw, err := rec.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dec, err := DecodeCoordinateDifferentiation(raw)
	if err != nil {
		t.Fatalf("record does not round-trip: %v", err)
	}
	if len(dec.Mapping.Entries) != 2 || dec.Mapping.Entries[0].ParameterID != DeriveParameterID(0) {
		t.Fatal("mapping changed in round-trip")
	}
	if _, err := DecodeCoordinateDifferentiation(raw[:len(raw)/2]); err == nil {
		t.Fatal("truncated record accepted")
	}
	if _, err := DecodeCoordinateDifferentiation(append(append([]byte{}, raw...), 0x00)); err == nil {
		t.Fatal("trailing bytes accepted")
	}
	bad := rec
	bad.Order = 0
	if err := bad.Validate(); err == nil {
		t.Fatal("order-0 record validated")
	}
	empty := rec
	empty.Mapping = CoordParamMap{}
	if err := empty.Validate(); err == nil {
		t.Fatal("empty-mapping record validated")
	}
	dupMap := rec
	dupMap.Mapping = CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: declFn, ParameterID: DeriveParameterID(0)},
		{DeclaringArtifactRef: declFn, ParameterID: DeriveParameterID(0)},
	}}
	if err := dupMap.Validate(); err == nil {
		t.Fatal("duplicate-PID mapping record validated")
	}
	if _, _, _, err := DifferentiateCoordinate(s, src, chart, 0, mapping, declFn, 0, nil); err == nil {
		t.Fatal("order-0 differentiation accepted")
	} else if !errors.Is(err, ErrBadParameters) {
		t.Fatalf("order-0 wrong reason: %v", err)
	}
	if _, _, _, err := DifferentiateCoordinate(s, src, chart, 5, mapping, declFn, 1, nil); err == nil {
		t.Fatal("out-of-range coordinate index accepted")
	}
}

// TestDifferentiateCoordinateValid uses independently derived expectations:
// d/dx(x^2) = 2x and d/dx(x^2+y^2) = 2x, in canonical simplified form.
func TestDifferentiateCoordinateValid(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	x, y := syms[0], syms[1]

	src := mustStoreExpr(t, s, Pow(x, MustRational(2, 1)))
	out, recRef, d, err := DifferentiateCoordinate(s, src, chart, 0, mapping, declFn, 1, nil)
	if err != nil {
		t.Fatalf("valid guarded differentiation failed: %v", err)
	}
	simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{out}, nil, nil)
	want, err := Canonicalize(Mul(MustRational(2, 1), x))
	if err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t, s, simp.OutputRef); !got.Equal(want) {
		t.Fatalf("d/dx(x^2) is not 2x: %v", got)
	}

	nested := mustStoreExpr(t, s, Add(Pow(x, MustRational(2, 1)), Pow(y, MustRational(2, 1))))
	out2, _, _, err := DifferentiateCoordinate(s, nested, chart, 0, mapping, declFn, 1, nil)
	if err != nil {
		t.Fatalf("nested guarded differentiation failed: %v", err)
	}
	simp2 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{out2}, nil, nil)
	if got := mustResolve(t, s, simp2.OutputRef); !got.Equal(want) {
		t.Fatalf("d/dx(x^2+y^2) is not 2x: %v", got)
	}

	// Record carries a replayable derivation; Check and Replay both pass.
	if err := CheckCoordinateDifferentiation(s, recRef); err != nil {
		t.Fatalf("fresh record rejected: %v", err)
	}
	if err := ReplayCoordinateDifferentiation(s, recRef); err != nil {
		t.Fatalf("fresh record does not replay: %v", err)
	}
	if err := Replay(s, d, nil); err != nil {
		t.Fatalf("returned derivation does not replay: %v", err)
	}
}

// TestDifferentiateCoordinateVacuous: a validly declared coordinate absent
// from the source differentiates to zero through the guarded API.
func TestDifferentiateCoordinateVacuous(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	x := syms[0]

	src := mustStoreExpr(t, s, Pow(x, MustRational(2, 1)))
	out, recRef, _, err := DifferentiateCoordinate(s, src, chart, 1, mapping, declFn, 1, nil)
	if err != nil {
		t.Fatalf("vacuous guarded differentiation failed: %v", err)
	}
	simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{out}, nil, nil)
	if got := mustResolve(t, s, simp.OutputRef); !got.Equal(MustRational(0, 1)) {
		t.Fatalf("off-coordinate derivative is not 0: %v", got)
	}
	constSrc := mustStoreExpr(t, s, MustRational(1, 1))
	out2, _, _, err := DifferentiateCoordinate(s, constSrc, chart, 1, mapping, declFn, 1, nil)
	if err != nil {
		t.Fatalf("constant vacuous differentiation failed: %v", err)
	}
	simp2 := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{out2}, nil, nil)
	if got := mustResolve(t, s, simp2.OutputRef); !got.Equal(MustRational(0, 1)) {
		t.Fatalf("constant derivative is not 0: %v", got)
	}
	if err := CheckCoordinateDifferentiation(s, recRef); err != nil {
		t.Fatalf("vacuous record rejected: %v", err)
	}
	if err := ReplayCoordinateDifferentiation(s, recRef); err != nil {
		t.Fatalf("vacuous record does not replay: %v", err)
	}
}

// TestDifferentiateCoordinateWrongDisplay: same-PID foreign displays fail
// through the guarded API at root and nested positions.
func TestDifferentiateCoordinateWrongDisplay(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, _ := covChart(t, s, "x", "y")
	p0 := DeriveParameterID(0)
	bad := NewBoundSymbol("q", p0)

	// identityReason accepts either intended identity boundary: the
	// shared operand-disjointness gate (firing first in the guarded path)
	// or the rule-level wrong-variable rejection.
	identityReason := func(err error) bool {
		if err == nil || !errors.Is(err, ErrBadInput) {
			return false
		}
		return wrongVarReason(err) || strings.Contains(err.Error(), "reserved for coordinate")
	}
	root := mustStoreExpr(t, s, bad)
	if _, _, _, err := DifferentiateCoordinate(s, root, chart, 0, mapping, declFn, 1, nil); !identityReason(err) {
		t.Fatalf("wrong-display root accepted (or wrong reason): %v", err)
	}
	nested := mustStoreExpr(t, s, Add(Mul(Pow(bad, MustRational(2, 1)), MustRational(3, 1)), MustRational(1, 1)))
	if _, _, _, err := DifferentiateCoordinate(s, nested, chart, 0, mapping, declFn, 1, nil); !identityReason(err) {
		t.Fatalf("wrong-display nested accepted (or wrong reason): %v", err)
	}
}

// TestDifferentiateCoordinateBadContext covers mapping and metadata negatives.
func TestDifferentiateCoordinateBadContext(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	src := mustStoreExpr(t, s, syms[0])

	// Swapped mapping: structurally valid, semantically wrong coordinate.
	swapped := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: declFn, ParameterID: DeriveParameterID(1)},
		{DeclaringArtifactRef: declFn, ParameterID: DeriveParameterID(0)},
	}}
	if err := CheckCoordParamMap(s, chart, swapped, declFn); err != nil {
		t.Fatalf("swapped mapping should pass structural gates: %v", err)
	}
	if _, _, _, err := DifferentiateCoordinate(s, src, chart, 0, swapped, declFn, 1, nil); err == nil {
		t.Fatal("swapped mapping accepted by guarded differentiation")
	}

	// Nameless declaring function: ErrMissingIdentity. The mapping must
	// reference the legacy function itself so structural gates pass and
	// the metadata leg is reached.
	// Two-parameter nameless function (legacy layout, no ParamNames).
	legacyBody := mustStoreExpr(t, s, NewBoundSymbol("x", DeriveParameterID(0)))
	legacyFn := Function{
		ParameterIDs: []ParameterID{DeriveParameterID(0), DeriveParameterID(1)},
		BodyRef:      legacyBody,
	}
	legacyPayload, err := legacyFn.Encode()
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := s.Put(FamilyFunction, legacyPayload)
	if err != nil {
		t.Fatal(err)
	}
	legacyMap := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: legacy, ParameterID: DeriveParameterID(0)},
		{DeclaringArtifactRef: legacy, ParameterID: DeriveParameterID(1)},
	}}
	_ = mapping
	if _, _, _, err := DifferentiateCoordinate(s, src, chart, 0, legacyMap, legacy, 1, nil); !errors.Is(err, ErrMissingIdentity) {
		t.Fatalf("nameless function accepted (or wrong reason): %v", err)
	}

	// Cross-function mapping entry.
	other, _ := mustStoreFunction(t, s, []string{"v"}, NewBoundSymbol("v", DeriveParameterID(0)))
	cross := CoordParamMap{Entries: []BoundParameterRef{
		{DeclaringArtifactRef: declFn, ParameterID: DeriveParameterID(0)},
		{DeclaringArtifactRef: other, ParameterID: DeriveParameterID(0)},
	}}
	if _, _, _, err := DifferentiateCoordinate(s, src, chart, 0, cross, declFn, 1, nil); err == nil {
		t.Fatal("cross-function mapping accepted")
	}

	// Ghost chart.
	ghost := ComputeTestRef(t, "physmath", "coordinate_system/1")
	if _, _, _, err := DifferentiateCoordinate(s, src, ghost, 0, mapping, declFn, 1, nil); err == nil {
		t.Fatal("unresolvable chart accepted")
	}
}

// TestDifferentiateCoordinateSharedAndDisjoint: the source may be the chart
// coordinate itself (d/dx(x) = 1) or carry independent unmapped parameters.
func TestDifferentiateCoordinateSharedAndDisjoint(t *testing.T) {
	s := NewMemStore()
	chart, mapping, declFn, syms := covChart(t, s, "x", "y")
	_, cpayload, err := s.Lookup(chart)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		t.Fatal(err)
	}
	out, _, _, err := DifferentiateCoordinate(s, cs.CoordinateRefs[0], chart, 0, mapping, declFn, 1, nil)
	if err != nil {
		t.Fatalf("shared-identity differentiation failed: %v", err)
	}
	if got := mustResolve(t, s, out); !got.Equal(MustRational(1, 1)) {
		t.Fatalf("d/dx(x) is not 1: %v", got)
	}
	indep := mustStoreExpr(t, s, Mul(NewBoundSymbol("a", DeriveParameterID(2)), syms[0]))
	out2, _, _, err := DifferentiateCoordinate(s, indep, chart, 0, mapping, declFn, 1, nil)
	if err != nil {
		t.Fatalf("disjoint-parameter differentiation failed: %v", err)
	}
	simp := applyRule(t, s, RuleExprSimplify, []artifact.ArtifactRef{out2}, nil, nil)
	want, err := Canonicalize(NewBoundSymbol("a", DeriveParameterID(2)))
	if err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t, s, simp.OutputRef); !got.Equal(want) {
		t.Fatalf("d/dx(a*x) is not a: %v", got)
	}
}

// tamperRecord decodes a record, applies mutate, and re-mints it as a new
// artifact, simulating corrupted provenance evidence.
func tamperRecord(t *testing.T, s Store, ref artifact.ArtifactRef, mutate func(*CoordinateDifferentiation)) artifact.ArtifactRef {
	t.Helper()
	rec := coordRecord(t, s, ref)
	mutate(&rec)
	raw, err := rec.Encode()
	if err != nil {
		t.Fatalf("tampered record does not encode: %v", err)
	}
	bad, err := s.Put(FamilyCoordinateDifferentiation, raw)
	if err != nil {
		t.Fatal(err)
	}
	return bad
}

// TestCoordDiffProvenance: substituted results, corrupted selections, and
// altered variables fail Check/Replay at the intended boundary.
func TestCoordDiffProvenance(t *testing.T) {
	mk := func(t *testing.T) (*MemStore, artifact.ArtifactRef) {
		s := NewMemStore()
		chart, mapping, declFn, syms := covChart(t, s, "x", "y")
		src := mustStoreExpr(t, s, Pow(syms[0], MustRational(2, 1)))
		_, recRef, _, err := DifferentiateCoordinate(s, src, chart, 0, mapping, declFn, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		return s, recRef
	}

	t.Run("substituted-result", func(t *testing.T) {
		s, recRef := mk(t)
		rec := coordRecord(t, s, recRef)
		other := mustStoreExpr(t, s, MustRational(99, 1))
		bad := tamperRecord(t, s, recRef, func(c *CoordinateDifferentiation) { c.ResultRef = other })
		_ = rec
		if err := CheckCoordinateDifferentiation(s, bad); err == nil {
			t.Fatal("substituted result accepted by Check")
		} else if !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("substituted result wrong boundary: %v", err)
		}
		// Artifact-level replay validates the complete record, so the
		// substitution fails here too — even though the inner derivation
		// remains independently replayable via raw Replay.
		if err := ReplayCoordinateDifferentiation(s, bad); err == nil {
			t.Fatal("substituted result accepted by artifact replay")
		} else if !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("substituted result replay wrong boundary: %v", err)
		}
		// Derivation-level output substitution fails both gates.
		_, dpayload, err := s.Lookup(rec.DerivationRef)
		if err != nil {
			t.Fatal(err)
		}
		dd, err := DecodeDerivation(dpayload)
		if err != nil {
			t.Fatal(err)
		}
		dd.Steps[0].OutputRef = other
		draw, err := dd.Encode()
		if err != nil {
			t.Fatal(err)
		}
		altDeriv, err := s.Put(FamilyDerivation, draw)
		if err != nil {
			t.Fatal(err)
		}
		badDeriv := tamperRecord(t, s, recRef, func(c *CoordinateDifferentiation) {
			c.ResultRef = other
			c.DerivationRef = altDeriv
		})
		// Artifact replay rejects at the record-derivation agreement
		// gate ("does not match") before raw re-execution is reached.
		if err := ReplayCoordinateDifferentiation(s, badDeriv); err == nil {
			t.Fatal("substituted derivation output accepted by Replay")
		} else if !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("substituted derivation output replay wrong boundary: %v", err)
		}
		if err := CheckCoordinateDifferentiation(s, badDeriv); err == nil {
			t.Fatal("substituted derivation output accepted by Check")
		}
	})

	t.Run("corrupted-mapping", func(t *testing.T) {
		s, recRef := mk(t)
		rec := coordRecord(t, s, recRef)
		swapped := CoordParamMap{Entries: []BoundParameterRef{
			rec.Mapping.Entries[1], rec.Mapping.Entries[0],
		}}
		bad := tamperRecord(t, s, recRef, func(c *CoordinateDifferentiation) { c.Mapping = swapped })
		if err := CheckCoordinateDifferentiation(s, bad); err == nil {
			t.Fatal("corrupted mapping accepted by Check")
		} else if !strings.Contains(err.Error(), "PID mismatch") && !strings.Contains(err.Error(), "parameters mismatch") {
			t.Fatalf("corrupted mapping wrong boundary: %v", err)
		}
	})

	t.Run("altered-variable-replay", func(t *testing.T) {
		// Inner derivation retargeted to the other coordinate while the
		// recorded output still claims d/dx(x^2): replay must fail output
		// equality rather than silently differentiate the wrong variable.
		s := NewMemStore()
		chart, mapping, declFn, syms := covChart(t, s, "x", "y")
		src := mustStoreExpr(t, s, Pow(syms[0], MustRational(2, 1)))
		_, recRef, _, err := DifferentiateCoordinate(s, src, chart, 0, mapping, declFn, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		rec := coordRecord(t, s, recRef)
		_, dpayload, err := s.Lookup(rec.DerivationRef)
		if err != nil {
			t.Fatal(err)
		}
		d, err := DecodeDerivation(dpayload)
		if err != nil {
			t.Fatal(err)
		}
		altParams := append(BoundParameterRef{declFn, DeriveParameterID(1)}.Encode(), artifact.EncodeUInt32(1)...)
		d.Steps[0].Parameters = altParams
		draw, err := d.Encode()
		if err != nil {
			t.Fatal(err)
		}
		altDeriv, err := s.Put(FamilyDerivation, draw)
		if err != nil {
			t.Fatal(err)
		}
		bad := tamperRecord(t, s, recRef, func(c *CoordinateDifferentiation) { c.DerivationRef = altDeriv })
		// Artifact replay rejects at the recorded-selection gate before
		// any output comparison is reached.
		if err := ReplayCoordinateDifferentiation(s, bad); err == nil {
			t.Fatal("altered-variable derivation replayed clean")
		} else if !strings.Contains(err.Error(), "parameters mismatch") {
			t.Fatalf("altered variable replay wrong boundary: %v", err)
		}
		if err := CheckCoordinateDifferentiation(s, bad); err == nil {
			t.Fatal("altered-variable derivation accepted by Check")
		}
	})

	t.Run("unresolvable-declaring", func(t *testing.T) {
		s, recRef := mk(t)
		ghost := ComputeTestRef(t, "physmath", "function/1")
		bad := tamperRecord(t, s, recRef, func(c *CoordinateDifferentiation) { c.DeclaringFnRef = ghost })
		if err := CheckCoordinateDifferentiation(s, bad); err == nil {
			t.Fatal("unresolvable declaring function accepted")
		}
	})

	t.Run("wrong-family-target", func(t *testing.T) {
		s, _ := mk(t)
		exprRef := mustStoreExpr(t, s, MustRational(1, 1))
		if err := CheckCoordinateDifferentiation(s, exprRef); err == nil {
			t.Fatal("expression target accepted as coord-diff record")
		}
		if err := ReplayCoordinateDifferentiation(s, exprRef); err == nil {
			t.Fatal("expression target replayed as coord-diff record")
		}
	})
}

// TestGuardedMigrationBoundaries proves the migrated geometry callers route
// through the guarded path: swapped mappings fail at the binding boundary
// (which CheckCoordParamMap alone passes), and every covariant derivation
// premise set carries verifiable differentiation records.
func TestGuardedMigrationBoundaries(t *testing.T) {
	t.Run("covariant-swapped-mapping", func(t *testing.T) {
		s := NewMemStore()
		chart, _, declFn, syms := covChart(t, s, "x", "y")
		swapped := CoordParamMap{Entries: []BoundParameterRef{
			{DeclaringArtifactRef: declFn, ParameterID: DeriveParameterID(1)},
			{DeclaringArtifactRef: declFn, ParameterID: DeriveParameterID(0)},
		}}
		f := covTensor(t, s, 0, 2, nil, []Expr{Add(Pow(syms[0], MustRational(2, 1)), Pow(syms[1], MustRational(2, 1)))})
		field, err := s.Put(FamilyTensorField, TensorField{TensorRef: f, CoordinateSystemRef: chart}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		conn := zeroConn(t, s, chart, 2)
		_, _, err = EvaluateCovariantDerivative(s, field, conn, swapped, declFn, nil)
		if err == nil {
			t.Fatal("covariant evaluation accepted swapped mapping")
		} else if !strings.Contains(err.Error(), "PID mismatch") && !strings.Contains(err.Error(), "does not match chart coordinate") {
			t.Fatalf("covariant swapped mapping wrong boundary: %v", err)
		}
	})

	t.Run("covariant-records-verifiable", func(t *testing.T) {
		s := NewMemStore()
		chart, mapping, declFn, syms := covChart(t, s, "x", "y")
		f := covTensor(t, s, 0, 2, nil, []Expr{Add(Pow(syms[0], MustRational(2, 1)), Pow(syms[1], MustRational(2, 1)))})
		field, err := s.Put(FamilyTensorField, TensorField{TensorRef: f, CoordinateSystemRef: chart}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		conn := zeroConn(t, s, chart, 2)
		result, d, err := EvaluateCovariantDerivative(s, field, conn, mapping, declFn, nil)
		if err != nil {
			t.Fatalf("guarded covariant evaluation failed: %v", err)
		}
		found := 0
		for _, r := range d.PremiseRefs {
			fam, _, err := s.Lookup(r)
			if err != nil {
				t.Fatal(err)
			}
			if fam == FamilyCoordinateDifferentiation {
				found++
				if err := CheckCoordinateDifferentiation(s, r); err != nil {
					t.Fatalf("embedded differentiation record rejected: %v", err)
				}
			}
		}
		if found != 2 {
			t.Fatalf("covariant derivation carries %d differentiation records, want 2", found)
		}
		_ = result
	})

	t.Run("levicivita-swapped-mapping", func(t *testing.T) {
		s := NewMemStore()
		chart, metric, fn, mapping, _, _, cond := polarSetup(t, s)
		_ = mapping
		swapped := CoordParamMap{Entries: []BoundParameterRef{
			{DeclaringArtifactRef: fn, ParameterID: DeriveParameterID(1)},
			{DeclaringArtifactRef: fn, ParameterID: DeriveParameterID(0)},
		}}
		if _, _, err := LeviCivita(s, metric, chart, swapped, fn, []artifact.ArtifactRef{cond}); err == nil {
			t.Fatal("levi-civita accepted swapped mapping")
		}
	})

	t.Run("levicivita-records-verifiable", func(t *testing.T) {
		s := NewMemStore()
		chart, metric, fn, mapping, _, _, cond := polarSetup(t, s)
		_, d, err := LeviCivita(s, metric, chart, mapping, fn, []artifact.ArtifactRef{cond})
		if err != nil {
			t.Fatalf("guarded levi-civita failed: %v", err)
		}
		found := 0
		for _, r := range d.PremiseRefs {
			fam, _, err := s.Lookup(r)
			if err != nil {
				t.Fatal(err)
			}
			if fam == FamilyCoordinateDifferentiation {
				found++
				if err := CheckCoordinateDifferentiation(s, r); err != nil {
					t.Fatalf("embedded levi-civita record rejected: %v", err)
				}
			}
		}
		// dim 2 with components (1, 0, 0, r^2): the two off-diagonal
		// zeros share one content-addressed record, so 2 x 3 = 6.
		if found != 6 {
			t.Fatalf("levi-civita derivation carries %d differentiation records, want 6", found)
		}
	})
}

// TestCoordDiffArtifactReplay is the R1 acceptance table: every tampered
// coordinate_differentiation/1 record must fail artifact-level replay even
// when its inner derivation remains independently replayable, while a valid
// record — including vacuous differentiation — must pass. Both
// ReplayCoordinateDifferentiation and CheckCoordinateDifferentiation are
// asserted per case with the intended identity/provenance reason.
func TestCoordDiffArtifactReplay(t *testing.T) {
	// mk builds a store with a valid x/y chart record for d/dx(x^2).
	mk := func(t *testing.T) (*MemStore, artifact.ArtifactRef, CoordParamMap, artifact.ArtifactRef, artifact.ArtifactRef) {
		s := NewMemStore()
		chart, mapping, declFn, syms := covChart(t, s, "x", "y")
		src := mustStoreExpr(t, s, Pow(syms[0], MustRational(2, 1)))
		_, recRef, _, err := DifferentiateCoordinate(s, src, chart, 0, mapping, declFn, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		return s, chart, mapping, declFn, recRef
	}

	cases := []struct {
		name   string
		mutate func(t *testing.T, s *MemStore, rec CoordinateDifferentiation) CoordinateDifferentiation
		reason string
	}{
		{
			name: "substituted-result",
			mutate: func(t *testing.T, s *MemStore, rec CoordinateDifferentiation) CoordinateDifferentiation {
				rec.ResultRef = mustStoreExpr(t, s, MustRational(99, 1))
				return rec
			},
			reason: "does not match",
		},
		{
			name: "substituted-source",
			mutate: func(t *testing.T, s *MemStore, rec CoordinateDifferentiation) CoordinateDifferentiation {
				rec.SourceRef = mustStoreExpr(t, s, MustRational(7, 1))
				return rec
			},
			reason: "step linkage broken",
		},
		{
			name: "substituted-chart",
			mutate: func(t *testing.T, s *MemStore, rec CoordinateDifferentiation) CoordinateDifferentiation {
				foreign, _, _, _ := covChart(t, s, "u", "v")
				rec.ChartRef = foreign
				return rec
			},
			reason: "does not match chart coordinate",
		},
		{
			name: "altered-index",
			mutate: func(t *testing.T, s *MemStore, rec CoordinateDifferentiation) CoordinateDifferentiation {
				rec.CoordIndex = 1
				return rec
			},
			reason: "parameters mismatch",
		},
		{
			name: "altered-order",
			mutate: func(t *testing.T, s *MemStore, rec CoordinateDifferentiation) CoordinateDifferentiation {
				rec.Order = 2
				return rec
			},
			reason: "parameters mismatch",
		},
		{
			name: "altered-conditions",
			mutate: func(t *testing.T, s *MemStore, rec CoordinateDifferentiation) CoordinateDifferentiation {
				rec.ConditionRefs = []artifact.ArtifactRef{
					detCond(t, s, mustStoreExpr(t, s, MustRational(3, 1))),
				}
				return rec
			},
			reason: "condition sets differ",
		},
		{
			name: "substituted-derivation",
			mutate: func(t *testing.T, s *MemStore, rec CoordinateDifferentiation) CoordinateDifferentiation {
				_, _, _, syms := covChart(t, s, "x", "y")
				_ = syms
				chart, mapping, declFn, syms2 := covChart(t, s, "x", "y")
				other := mustStoreExpr(t, s, Pow(syms2[1], MustRational(2, 1)))
				_, otherRec, _, err := DifferentiateCoordinate(s, other, chart, 1, mapping, declFn, 1, nil)
				if err != nil {
					t.Fatal(err)
				}
				rec.DerivationRef = coordRecord(t, s, otherRec).DerivationRef
				return rec
			},
			reason: "does not match",
		},
		{
			name: "constant-source-wrong-variable",
			mutate: func(t *testing.T, s *MemStore, rec CoordinateDifferentiation) CoordinateDifferentiation {
				// d/dx(5) = 0 re-minted claiming coordinate 1: the output
				// coincides (0 == 0) but the selection does not.
				five := mustStoreExpr(t, s, MustRational(5, 1))
				_, constRec, _, err := DifferentiateCoordinate(s, five, rec.ChartRef, 0, rec.Mapping, rec.DeclaringFnRef, 1, nil)
				if err != nil {
					t.Fatal(err)
				}
				constRecDec := coordRecord(t, s, constRec)
				constRecDec.CoordIndex = 1
				raw, err := constRecDec.Encode()
				if err != nil {
					t.Fatal(err)
				}
				swapped, err := s.Put(FamilyCoordinateDifferentiation, raw)
				if err != nil {
					t.Fatal(err)
				}
				_ = rec
				return coordRecord(t, s, swapped)
			},
			reason: "parameters mismatch",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _, _, recRef := mk(t)
			rec := coordRecord(t, s, recRef)
			mut := tc.mutate(t, s, rec)
			raw, err := mut.Encode()
			if err != nil {
				t.Fatalf("tampered record does not encode: %v", err)
			}
			bad, err := s.Put(FamilyCoordinateDifferentiation, raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := ReplayCoordinateDifferentiation(s, bad); err == nil {
				t.Fatalf("%s accepted by artifact replay", tc.name)
			} else if !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("%s replay wrong boundary (want %q): %v", tc.name, tc.reason, err)
			}
			if err := CheckCoordinateDifferentiation(s, bad); err == nil {
				t.Fatalf("%s accepted by Check", tc.name)
			} else if !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("%s check wrong boundary (want %q): %v", tc.name, tc.reason, err)
			}
		})
	}

	t.Run("valid-record-passes", func(t *testing.T) {
		s, _, _, _, recRef := mk(t)
		if err := ReplayCoordinateDifferentiation(s, recRef); err != nil {
			t.Fatalf("valid record rejected by artifact replay: %v", err)
		}
		if err := CheckCoordinateDifferentiation(s, recRef); err != nil {
			t.Fatalf("valid record rejected by Check: %v", err)
		}
	})

	t.Run("vacuous-record-passes", func(t *testing.T) {
		s := NewMemStore()
		chart, mapping, declFn, syms := covChart(t, s, "x", "y")
		src := mustStoreExpr(t, s, Pow(syms[0], MustRational(2, 1)))
		_, recRef, _, err := DifferentiateCoordinate(s, src, chart, 1, mapping, declFn, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := ReplayCoordinateDifferentiation(s, recRef); err != nil {
			t.Fatalf("vacuous record rejected by artifact replay: %v", err)
		}
		if err := CheckCoordinateDifferentiation(s, recRef); err != nil {
			t.Fatalf("vacuous record rejected by Check: %v", err)
		}
	})
}
