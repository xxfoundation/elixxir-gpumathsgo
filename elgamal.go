///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package gpumaths

import "git.xx.network/elixxir/crypto/cyclic"

// elgamal.go contains the input, results, and other types for running the
// elgamal operation against the GPU. The actual GPU call is in elgamal_gpu.go
// and is marked to require `-tags cuda` in your build.
// ElGamalChunkPrototyp is the type necessary to implement cryptop interface
type ElGamalChunkPrototype func(p *StreamPool, g *cyclic.Group,
	key, privateKey *cyclic.IntBuffer, publicCypherKey *cyclic.Int,
	ecrKey, cypher *cyclic.IntBuffer) error

// GetInputSize returns the chunk size for the op
func (ElGamalChunkPrototype) GetInputSize() uint32 {
	return 64
}

// GetName returns the name of the op (ElGamalChunk)
func (ElGamalChunkPrototype) GetName() string {
	return "ElGamalChunk"
}
