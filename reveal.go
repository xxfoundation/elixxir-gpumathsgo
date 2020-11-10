///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package gpumaths

import "gitlab.com/elixxir/crypto/cyclic"

// reveal.go contains the input, results, and other types for running the reveal
// operation against the GPU. The actual GPU call is in reveal_gpu.go and is
// marked to require `-tags cuda` in your build.

// RevealInputSlot only needs the Cypher Payload for computation of rootCoprime
type RevealInputSlot struct {
	Cypher []byte
}

// RevealInput uses each individual slot and includes the Prime, and the
// computed PublicCypherKey (Z)
type RevealInput struct {
	Slots           []RevealInputSlot
	PublicCypherKey []byte
	Prime           []byte
}

// RevealResultSlot returns the computed Cypher payload after rootCoprime
type RevealResultSlot struct {
	Cypher []byte
}

// RevealResult returns results for each slot or an error
type RevealResult struct {
	Slots []RevealResultSlot
	Err   error
}

// Prototype Definition

// RevealChunkPrototype defines the function type for running the Reveal
// kernel in the GPU.
type RevealChunkPrototype func(p *StreamPool, g *cyclic.Group,
	publicCypherKey *cyclic.Int, cypher *cyclic.IntBuffer) error

// GetInputSize is how big chunk sizes should be to run the reveal operation
func (RevealChunkPrototype) GetInputSize() uint32 {
	return 64
}

// GetName return the name of the RevealChunk operation
func (RevealChunkPrototype) GetName() string {
	return "RevealChunk"
}
