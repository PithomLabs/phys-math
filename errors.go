package physmath

import "errors"

// Phase-1 rule execution errors. The specification does not require particular
// Go error types or strings; these sentinels give callers stable matching
// without changing rule semantics (plan9.3 permitted detail).
var (
	// ErrUnknownRuleID: unknown (ProducerNamespace, RuleID) pair (§§17.11, 24.5).
	ErrUnknownRuleID = errors.New("physmath: unknown rule pair")
	// ErrArityMismatch: wrong input arity for the selected rule (§17.2).
	ErrArityMismatch = errors.New("physmath: input arity mismatch")
	// ErrBadParameters: malformed rule parameter bytes (§17.2).
	ErrBadParameters = errors.New("physmath: invalid rule parameters")
	// ErrBadInput: input fails the rule's root/family/applicability predicate.
	ErrBadInput = errors.New("physmath: invalid rule input")
	// ErrMissingCondition: required explicit condition absent (§§17.4, 17.10).
	ErrMissingCondition = errors.New("physmath: missing required condition")
	// ErrOperatorNotExecutable: reserved operator invoked for execution
	// (A4 covariant_derivative reservation; K1 s1/s4-class steps).
	ErrOperatorNotExecutable = errors.New("physmath: reserved operator is not executable")
	// ErrSignatureUnverifiable: diagonal entry not rationally decidable;
	// uncertainty preserved, never asserted (R3 signature validation).
	ErrSignatureUnverifiable = errors.New("physmath: signature not decidable")
	// ErrMissingIdentity: a trusted entry point requires parameter-identity
	// metadata (Function display names) that the artifact does not carry.
	// Decoding and structural validation succeed without it; only trusted
	// differentiation refuses to proceed.
	ErrMissingIdentity = errors.New("physmath: missing parameter identity metadata")
)
