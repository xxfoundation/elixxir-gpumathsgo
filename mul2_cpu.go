////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

//+build !linux !cuda

package gpumaths

import (
"errors"
"gitlab.com/elixxir/crypto/cyclic"
)

// Mul2Chunk is stubbed unless GPU is present.
var Mul2Chunk Mul2ChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	result *cyclic.IntBuffer, x *cyclic.IntBuffer, y *cyclic.IntBuffer) error {
	return errors.New(NoGpuErrStr)
}

// Mul2 is an empty stub that returns an error when called.
func Mul2(input Mul2Input, stream Stream) chan Mul2Result {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan Mul2Result, 1)
	resultChan <- Mul2Result{
		Err: errors.New(NoGpuErrStr),
	}
	return resultChan
}
