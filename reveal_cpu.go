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

// RevealChunk is stubbed unless GPU is present.
var RevealChunk RevealChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	publicCypherKey *cyclic.Int, cypher *cyclic.IntBuffer) error {
	return errors.New(NoGpuErrStr)
}

// Reveal is an empty stub that returns an error when called.
func Reveal(input RevealInput, stream Stream) chan RevealResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan RevealResult, 1)
	resultChan <- RevealResult{
		Err: errors.New(NoGpuErrStr),
	}
	return resultChan
}
