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

// StripChunk is stubbed unless GPU is present.
var StripChunk StripChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	publicCypherKey *cyclic.Int,
	precomputation []*cyclic.Int, cypher *cyclic.IntBuffer) error {
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
