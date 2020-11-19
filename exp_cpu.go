///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

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

// exp is stubbed unless GPU is present.
func Exp(input ExpInput, stream Stream) chan ExpResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan ExpResult, 1)
	resultChan <- ExpResult{
		Err: errors.New("gpumaths stubbed build doesn't support CUDA stream pool"),
	}
	return resultChan
}
