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
// marked to require `-tags gpu` in your build.

// RevealChunkPrototype defines the function type for running the reveal
// kernel in the GPU.
type RevealChunkPrototype func(p *StreamPool, g *cyclic.Group,
	publicCypherKey *cyclic.Int, cypher *cyclic.IntBuffer, result *cyclic.IntBuffer) error

// GetInputSize is how big chunk sizes should be to run the reveal operation
func (RevealChunkPrototype) GetInputSize() uint32 {
	return 64
}

// GetName return the name of the RevealChunk operation
func (RevealChunkPrototype) GetName() string {
	return "RevealChunk"
}
