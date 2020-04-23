////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package gpumaths

import "gitlab.com/elixxir/crypto/cyclic"

// elgamal.go contains the input, results, and other types for running the
// elgamal operation against the GPU. The actual GPU call is in elgamal_gpu.go
// and is marked to require `-tags cuda` in your build.

// ElGamalInputSlot is the input required from each slot in the batch
type ElGamalInputSlot struct {
	PrivateKey []byte
	Key        []byte
	EcrKey     []byte
	Cypher     []byte
}

// ElGamalInput is the input structure for the ElGamal op, it requires a
// group generator (G), the Prime, and the PublicCypherKey (Z)
type ElGamalInput struct {
	Slots           []ElGamalInputSlot
	PublicCypherKey []byte
	Prime           []byte
	G               []byte
}

// ElGamalResultSlot is the output from the op for each slot in the batch
type ElGamalResultSlot struct {
	EcrKey []byte
	Cypher []byte
}

// ElGamalResult is the output structure for the op
type ElGamalResult struct {
	Slots []ElGamalResultSlot
	Err   error
}

// ElGamalChunkPrototyp is the type necessary to implement cryptop interface
type ElGamalChunkPrototype func(p *StreamPool, g *cyclic.Group,
	key, privateKey *cyclic.IntBuffer, publicCypherKey *cyclic.Int,
	ecrKey, cypher *cyclic.IntBuffer) error

// GetInputSize returns the chunk size for the op
func (ElGamalChunkPrototype) GetInputSize() uint32 {
	return 128
}

// GetName returns the name of the op (ElGamalChunk)
func (ElGamalChunkPrototype) GetName() string {
	return "ElGamalChunk"
}
