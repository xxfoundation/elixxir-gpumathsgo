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

// StripChunk is stubbed unless GPU is present.
var StripChunk StripChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	precomputationOut *cyclic.IntBuffer, publicCypherKey *cyclic.Int,
	precomputationIn []*cyclic.Int, cypher *cyclic.IntBuffer) error {
	return errors.New(NoGpuErrStr)
}

// Strip is an empty stub that returns an error when called.
func Strip(input StripInput, stream Stream) chan StripResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan StripResult, 1)
	resultChan <- StripResult{
		Err: errors.New(NoGpuErrStr),
	}
	return resultChan
}
