package physmath

import (
	"fmt"

	"github.com/PithomLabs/phys-artifact"
)

// A4: reserved operator vocabulary. "differentiate" is the executable
// Phase-1 operator (via DIFFERENTIATE-001 with a declaring function).
// "covariant_derivative" is representable but NEVER executable in this
// slice: a deliberate reservation for tensor-calculus requirements (K1
// s1/s4-class steps) that must stay visible yet unexecuted. Any future
// execution needs a separate bounded proposal and approval.
const OperatorCovariantDerivative = "covariant_derivative"

// ValidOperatorID reports membership in the reserved operator vocabulary:
// exactly {differentiate, covariant_derivative}. Anything else fails closed.
func ValidOperatorID(id string) bool {
	return id == OperatorReservedID || id == OperatorCovariantDerivative
}

// Executable reports whether an OperatorApplication may execute. Only
// differentiate executes (through DIFFERENTIATE-001); covariant_derivative
// is representational only. Callers MUST consult this before treating an
// operator artifact as an executed check.
func (o OperatorApplication) Executable() bool {
	return o.OperatorID == OperatorReservedID
}

// ExecuteOperator executes a validated OperatorApplication. The declaring
// function reference supplies the differentiation target context required
// by DIFFERENTIATE-001. The reserved covariant_derivative operator ALWAYS
// fails with ErrOperatorNotExecutable — even with perfect parameters — so
// reservation can never decay into silent success.
func ExecuteOperator(store Store, op OperatorApplication, declaringFnRef artifact.ArtifactRef) (TransformResult, error) {
	if err := op.Validate(); err != nil {
		return TransformResult{}, err
	}
	if !op.Executable() {
		return TransformResult{}, fmt.Errorf("physmath: operator %q is reserved, not executable: %w",
			op.OperatorID, ErrOperatorNotExecutable)
	}
	if len(op.Args) != 1 {
		return TransformResult{}, fmt.Errorf("%w: differentiate operator needs exactly 1 arg",
			ErrArityMismatch)
	}
	return Apply(store, Invocation{
		ProducerNamespace: NamespacePhysMath,
		RuleID:            RuleDifferentiate,
		InputRefs:         []artifact.ArtifactRef{op.Args[0], declaringFnRef},
		ConditionRefs:     nil,
		Parameters:        op.Parameters,
	})
}
