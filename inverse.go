package cryptops

import "gitlab.com/elixxir/crypto/cyclic"

// An alias for a unary inverse operation which sets and returns an out variable
type InversePrototype func(g *cyclic.Group, x, out *cyclic.Int) *cyclic.Int

// Inverts a number x in the group and stores it in out.
// It also returns out.
var Inverse InversePrototype = func(g *cyclic.Group, x, out *cyclic.Int) *cyclic.Int {
	g.Inverse(x, out)
	return out
}

// Returns the function name for debugging.
func (InversePrototype) GetName() string {
	return "Inverse"
}

// Returns the input size; used in safety checks.
func (InversePrototype) GetInputSize() uint32 {
	return 1
}
