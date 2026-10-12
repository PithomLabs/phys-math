package physmath

import (
	"bytes"
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// LeviCivita constructs the metric-derived connection
//
//	Γ^ρ_μν = ½ g^ρσ (∂_μ g_σν + ∂_ν g_σμ − ∂_σ g_μν)
//
// from an explicitly represented symmetric metric through the existing
// executable path: general inverse (W2) for g^ρσ, DIFFERENTIATE-001
// partials of the metric components through the coordinate mapping, and
// Mul/Add assembly with CANON+SIMPLIFY replay per coefficient. It returns
// the affine_connection/1 reference (with SourceMetricRef set) plus one
// flat replayable derivation covering the inverse, partials, and assembly.
//
// Boundaries: the metric must be symmetric_bilinear/1 (general tensor/1
// metrics are rejected as out of scope); the chart dimension must equal
// the metric dimension; ClaimSymmetric is set true only when input
// symmetry (canonical component equality) AND pairwise output equality
// both verify — otherwise false with no error, since an unverified claim
// must never be asserted. Torsion-freeness beyond lower-index symmetry
// and general metric compatibility are not claimed here.
func LeviCivita(store Store, metricRef, chartRef artifact.ArtifactRef, mapping CoordParamMap, declaringFnRef artifact.ArtifactRef, conds []artifact.ArtifactRef) (artifact.ArtifactRef, Derivation, error) {
	fail := func(err error) (artifact.ArtifactRef, Derivation, error) {
		return artifact.ArtifactRef{}, Derivation{}, err
	}
	mfam, mpayload, err := store.Lookup(metricRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: levi-civita metric unresolvable: %w", err))
	}
	if mfam != FamilySymmetricBilinear {
		return fail(fmt.Errorf("physmath: levi-civita metric is not symmetric_bilinear/1: %w", ErrBadInput))
	}
	metric, err := DecodeSymmetricBilinear(mpayload)
	if err != nil {
		return fail(err)
	}
	if err := CheckCoordParamMap(store, chartRef, mapping, declaringFnRef); err != nil {
		return fail(err)
	}
	cfam, cpayload, err := store.Lookup(chartRef)
	if err != nil {
		return fail(fmt.Errorf("physmath: levi-civita chart unresolvable: %w", err))
	}
	if cfam != FamilyCoordinateSystem {
		return fail(fmt.Errorf("physmath: levi-civita chart is not coordinate_system/1: %w", ErrBadInput))
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		return fail(err)
	}
	if int(chart.Dimension) != int(metric.Dimension) {
		return fail(fmt.Errorf("physmath: chart dimension %d != metric dimension %d: %w",
			chart.Dimension, metric.Dimension, ErrBadInput))
	}
	d := int(metric.Dimension)
	if d > MatrixDimBound {
		return fail(fmt.Errorf("physmath: levi-civita dimension %d exceeds bound %d: %w",
			d, MatrixDimBound, ErrBadInput))
	}
	if len(mapping.Entries) != d {
		return fail(fmt.Errorf("physmath: mapping entries %d != dimension %d: %w",
			len(mapping.Entries), d, ErrBadInput))
	}
	// Input symmetry by canonical component equality (no inversion or
	// signature claim, mirroring CheckSymmetricBilinear).
	symmetricInput := true
	gcomp := make([]Expr, len(metric.ComponentRefs))
	for i, r := range metric.ComponentRefs {
		e, err := resolveExpression(store, r)
		if err != nil {
			return fail(fmt.Errorf("physmath: metric component %d unresolvable: %w", i, err))
		}
		gcomp[i] = e
	}
	for i := 0; i < d && symmetricInput; i++ {
		for j := i + 1; j < d; j++ {
			ab, err := gcomp[i*d+j].Encode()
			if err != nil {
				return fail(err)
			}
			bb, err := gcomp[j*d+i].Encode()
			if err != nil {
				return fail(err)
			}
			if !bytes.Equal(ab, bb) {
				symmetricInput = false
			}
		}
	}
	// Parameter-identity scoping over the metric components, before the
	// first mint below (the inverse path mints). Metric symbols sharing a
	// mapped PID under a different display would differentiate as chart
	// coordinates.
	if err := CheckDisjointIdentity(store, chartRef, mapping, declaringFnRef, metric.ComponentRefs); err != nil {
		return fail(err)
	}
	// General inverse through the shared matrix path (never a diagonal
	// shortcut, even for diagonal inputs).
	detRef, invComps, invDeriv, err := InverseMatrix(store, metricRef, conds)
	if err != nil {
		return fail(err)
	}
	_ = detRef
	ginv := make([]Expr, len(invComps))
	for i, r := range invComps {
		e, err := resolveExpression(store, r)
		if err != nil {
			return fail(fmt.Errorf("physmath: inverse component %d unresolvable: %w", i, err))
		}
		ginv[i] = e
	}
	partial := func(comp artifact.ArtifactRef, coord uint32) (artifact.ArtifactRef, Expr, error) {
		bpr := mapping.Entries[coord]
		params := append(bpr.Encode(), artifact.EncodeUInt32(1)...)
		res, err := Apply(store, Invocation{
			ProducerNamespace: NamespacePhysMath, RuleID: RuleDifferentiate,
			InputRefs:     []artifact.ArtifactRef{comp, declaringFnRef},
			ConditionRefs: conds,
			Parameters:    params,
		})
		if err != nil {
			return artifact.ArtifactRef{}, Expr{}, fmt.Errorf("physmath: metric partial failed: %w", err)
		}
		e, err := resolveExpression(store, res.OutputRef)
		if err != nil {
			return artifact.ArtifactRef{}, Expr{}, err
		}
		return res.OutputRef, e, nil
	}
	// Partials of every component w.r.t. every coordinate, each replayable
	// on its own through DIFFERENTIATE-001.
	dg := make([][]artifact.ArtifactRef, d)
	dgExpr := make([][]Expr, d)
	var partSteps []DerivationStep
	var partPremises []artifact.ArtifactRef
	var partResults []artifact.ArtifactRef
	for c := 0; c < d; c++ {
		dg[c] = make([]artifact.ArtifactRef, d*d)
		dgExpr[c] = make([]Expr, d*d)
		for k := 0; k < d*d; k++ {
			out, e, err := partial(metric.ComponentRefs[k], uint32(c))
			if err != nil {
				return fail(err)
			}
			_ = out
			// Record a replayable one-step derivation per partial.
			bpr := mapping.Entries[c]
			params := append(bpr.Encode(), artifact.EncodeUInt32(1)...)
			dg[c][k] = out
			dgExpr[c][k] = e
			partPremises = append(partPremises, metric.ComponentRefs[k])
			partResults = append(partResults, out)
			partSteps = append(partSteps, DerivationStep{
				Ordinal:           0,
				ProducerNamespace: NamespacePhysMath, RuleID: RuleDifferentiate,
				InputRefs:     []artifact.ArtifactRef{metric.ComponentRefs[k], declaringFnRef},
				OutputRef:     out,
				ConditionRefs: normalizeConditionRefs(conds),
				Parameters:    params,
			})
		}
	}
	// Assembly: Γ^ρ_μν = Σ_σ ½·g^ρσ·(∂_μ g_σν + ∂_ν g_σμ − ∂_σ g_μν).
	half := MustRational(1, 2)
	coeffRefs := make([]artifact.ArtifactRef, 0, d*d*d)
	var asmPremises []artifact.ArtifactRef
	var asmResults []artifact.ArtifactRef
	var asmSteps []DerivationStep
	for rho := 0; rho < d; rho++ {
		for mu := 0; mu < d; mu++ {
			for nu := 0; nu < d; nu++ {
				terms := make([]Expr, 0, d)
				for sigma := 0; sigma < d; sigma++ {
					a := cloneExpr(dgExpr[uint32(mu)][sigma*d+nu])
					b := cloneExpr(dgExpr[uint32(nu)][sigma*d+mu])
					cc := cloneExpr(dgExpr[uint32(sigma)][mu*d+nu])
					terms = append(terms, Mul(half, cloneExpr(ginv[rho*d+sigma]),
						Add(a, b, Neg(cc))))
				}
				premise, err := PutExpression(store, Add(terms...))
				if err != nil {
					return fail(err)
				}
				canon, err := Apply(store, Invocation{
					ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
					InputRefs: []artifact.ArtifactRef{premise},
				})
				if err != nil {
					return fail(fmt.Errorf("physmath: connection coefficient canon failed: %w", err))
				}
				res, err := Apply(store, Invocation{
					ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
					InputRefs: []artifact.ArtifactRef{canon.OutputRef},
				})
				if err != nil {
					return fail(fmt.Errorf("physmath: connection coefficient simplify failed: %w", err))
				}
				asmPremises = append(asmPremises, premise)
				asmResults = append(asmResults, res.OutputRef)
				coeffRefs = append(coeffRefs, res.OutputRef)
				asmSteps = append(asmSteps,
					DerivationStep{
						Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprCanon,
						InputRefs: []artifact.ArtifactRef{premise}, OutputRef: canon.OutputRef,
					},
					DerivationStep{
						Ordinal: 0, ProducerNamespace: NamespacePhysMath, RuleID: RuleExprSimplify,
						InputRefs: []artifact.ArtifactRef{canon.OutputRef}, OutputRef: res.OutputRef,
					},
				)
			}
		}
	}
	// Symmetry verification by canonical output equality over lower-index
	// swaps; the claim is set only when every pair matches.
	symmetricOutput := true
	for rho := 0; rho < d && symmetricOutput; rho++ {
		for mu := 0; mu < d && symmetricOutput; mu++ {
			for nu := mu + 1; nu < d; nu++ {
				a := coeffRefs[(rho*d+mu)*d+nu]
				b := coeffRefs[(rho*d+nu)*d+mu]
				if a != b {
					_, pa, err := store.Lookup(a)
					if err != nil {
						return fail(err)
					}
					_, pb, err := store.Lookup(b)
					if err != nil {
						return fail(err)
					}
					if !bytes.Equal(pa, pb) {
						symmetricOutput = false
					}
				}
			}
		}
	}
	conn := AffineConnection{CoordRef: chartRef, Dimension: uint16(d),
		CoefficientRefs: coeffRefs, ClaimSymmetric: symmetricInput && symmetricOutput,
		SourceMetricRef: &metricRef}
	if err := conn.Validate(); err != nil {
		return fail(err)
	}
	connRef, err := store.Put(FamilyAffineConnection, conn.Encode())
	if err != nil {
		return fail(err)
	}
	// One flat derivation: inverse evidence, partials, assembly — ordinals
	// reassigned contiguously in execution order.
	var steps []DerivationStep
	ord := uint32(0)
	emit := func(s DerivationStep) {
		s.Ordinal = ord
		ord++
		steps = append(steps, s)
	}
	for _, s := range invDeriv.Steps {
		emit(s)
	}
	for _, s := range partSteps {
		emit(s)
	}
	for _, s := range asmSteps {
		emit(s)
	}
	// The declaring function is a genuine premise: every partial step
	// consumes it as its second input for DIFFERENTIATE-001 arity.
	premises := uniqueRefs(concatRefs(
		[]artifact.ArtifactRef{declaringFnRef},
		invDeriv.PremiseRefs, partPremises, asmPremises))
	results := uniqueRefs(concatRefs(
		invDeriv.ResultRefs, partResults, asmResults))
	dd := Derivation{PremiseRefs: premises, Steps: steps, ResultRefs: results}
	if err := Replay(store, dd, nil); err != nil {
		return fail(fmt.Errorf("physmath: levi-civita derivation does not replay: %w", err))
	}
	return connRef, dd, nil
}

// ConnectionSources optionally binds a connection to its source metric for
// full recomputation checks. Nil means shape/family/symmetry-claim checks
// only (arbitrary connections).
type ConnectionSources struct {
	MetricRef      artifact.ArtifactRef
	Mapping        CoordParamMap
	DeclaringFnRef artifact.ArtifactRef
	Conds          []artifact.ArtifactRef
}

// concatRefs flattens ref lists in order.
func concatRefs(lists ...[]artifact.ArtifactRef) []artifact.ArtifactRef {
	var out []artifact.ArtifactRef
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

// CheckAffineConnection is the store-aware gate for connection artifacts:
// family/shape resolution, chart-dimension agreement, and symmetry-claim
// verification by canonical output equality. With non-nil sources it
// additionally recomputes every coefficient through LeviCivita and
// requires full reference equality — the connection equivalent of the
// volume-density premise check.
func CheckAffineConnection(store Store, connRef artifact.ArtifactRef, src *ConnectionSources) error {
	fam, payload, err := store.Lookup(connRef)
	if err != nil {
		return fmt.Errorf("physmath: connection unresolvable: %w", err)
	}
	if fam != FamilyAffineConnection {
		return fmt.Errorf("physmath: target is not affine_connection/1: %w", ErrBadInput)
	}
	conn, err := DecodeAffineConnection(payload)
	if err != nil {
		return err
	}
	for _, tc := range conn.TargetChecks() {
		tfam, _, err := store.Lookup(tc.Ref)
		if err != nil {
			return fmt.Errorf("physmath: %s unresolvable: %w", tc.Field, err)
		}
		ok := false
		for _, a := range tc.Allowed {
			if tfam == a {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("physmath: %s targets %s/%s/%s, not in closed target set",
				tc.Field, tfam.Namespace, tfam.ArtifactType, tfam.SchemaVersion)
		}
	}
	cfam, cpayload, err := store.Lookup(conn.CoordRef)
	if err != nil {
		return fmt.Errorf("physmath: connection chart unresolvable: %w", err)
	}
	if cfam != FamilyCoordinateSystem {
		return fmt.Errorf("physmath: connection chart is not coordinate_system/1: %w", ErrBadInput)
	}
	chart, err := DecodeCoordinateSystem(cpayload)
	if err != nil {
		return err
	}
	if int(chart.Dimension) != int(conn.Dimension) {
		return fmt.Errorf("physmath: chart dimension %d != connection dimension %d: %w",
			chart.Dimension, conn.Dimension, ErrBadInput)
	}
	if conn.ClaimSymmetric {
		d := int(conn.Dimension)
		for rho := 0; rho < d; rho++ {
			for mu := 0; mu < d; mu++ {
				for nu := mu + 1; nu < d; nu++ {
					a := conn.CoefficientRefs[(rho*d+mu)*d+nu]
					b := conn.CoefficientRefs[(rho*d+nu)*d+mu]
					if a == b {
						continue
					}
					_, pa, err := store.Lookup(a)
					if err != nil {
						return fmt.Errorf("physmath: symmetry check unresolvable: %w", err)
					}
					_, pb, err := store.Lookup(b)
					if err != nil {
						return fmt.Errorf("physmath: symmetry check unresolvable: %w", err)
					}
					if !bytes.Equal(pa, pb) {
						return fmt.Errorf("physmath: claimed symmetry violated at (%d,%d,%d)", rho, mu, nu)
					}
				}
			}
		}
	}
	if src == nil {
		return nil
	}
	recomp, _, err := LeviCivita(store, src.MetricRef, conn.CoordRef, src.Mapping, src.DeclaringFnRef, src.Conds)
	if err != nil {
		return fmt.Errorf("physmath: connection recomputation failed: %w", err)
	}
	if recomp != connRef {
		return fmt.Errorf("physmath: connection does not match recomputed coefficients")
	}
	return nil
}
