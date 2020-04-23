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

// ElGamalChunk is stubbed unless GPU is present.
var ElGamalChunk ElGamalChunkPrototype = func(p *StreamPool, g *cyclic.Group, key, privateKey *cyclic.IntBuffer, publicCypherKey *cyclic.Int, ecrKey, cypher *cyclic.IntBuffer) error {
	return errors.New(NoGpuErrStr)
}

// ElGamal is stubbed unless GPU is present.
func ElGamal(input ElGamalInput, stream Stream) chan ElGamalResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan ElGamalResult, 1)
	resultChan <- ElGamalResult{
		Err: errors.New("gpumaths stubbed build doesn't support CUDA stream pool"),
	}
	return resultChan
}
