////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

//+build !linux !gpu

package gpumaths

import (
	"errors"
	"gitlab.com/elixxir/crypto/cyclic"
)

// ExpChunk is stubbed unless GPU is present.
var ExpChunk ExpChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	x, y, z *cyclic.IntBuffer) (*cyclic.IntBuffer, error) {
	return z, errors.New(NoGpuErrStr)
}

// Exp is stubbed unless GPU is present.
func Exp(input ExpInput, stream Stream) chan ExpResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan ExpResult, 1)
	resultChan <- ExpResult{
		Err: errors.New("gpumaths stubbed build doesn't support CUDA stream pool"),
	}
	return resultChan
}
