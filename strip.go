///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package gpumaths

import "gitlab.com/elixxir/crypto/cyclic"

// strip.go contains the input, results, and other types for running the strip
// operation against the GPU. The actual GPU call is in strip_gpu.go and is
// marked to require `-tags cuda` in your build.

// StripInputSlot needs the Cypher Payload and the Precomputation Payload
// to compute the payload's Encryption (Decryption) Key for removing the
// network encryption from the payload.
type StripInputSlot struct {
	Precomputation []byte
	Cypher         []byte
}

// StripInput uses each individual slot and includes the Prime, and the
// computed PublicCypherKey (Z)
type StripInput struct {
	Slots           []StripInputSlot
	PublicCypherKey []byte
	Prime           []byte
}

// StripResultSlot returns the Payload Precomputation for each slot
// e.g., the result of Cypher*RootCoPrime(Precomputation) payload
type StripResultSlot struct {
	Precomputation []byte
}

// StripResult returns results for each slot or an error
type StripResult struct {
	Slots []StripResultSlot
	Err   error
}

// Prototype Definition

// StripChunkPrototype defines the function type for running the Strip
// kernel in the GPU.
type StripChunkPrototype func(p *StreamPool, g *cyclic.Group,
	precomputationOut *cyclic.IntBuffer, publicCypherKey *cyclic.Int,
	precomputation []*cyclic.Int, cypher *cyclic.IntBuffer) error

// GetInputSize is how big chunk sizes should be to run the reveal operation
func (StripChunkPrototype) GetInputSize() uint32 {
	return 128
}

// GetName return the name of the StripChunk operation
func (StripChunkPrototype) GetName() string {
	return "StripChunk"
}
