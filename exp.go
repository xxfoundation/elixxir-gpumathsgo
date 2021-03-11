///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package gpumaths

import "gitlab.com/elixxir/crypto/cyclic"

// exp.go contains the input, results, and other types for running the
// exp operation against the GPU. The actual GPU call is in exp_gpu.go
// and is marked to require `-tags cuda` in your build.

// ExpChunkPrototype Implement cryptop interface for ExpChunk
type ExpChunkPrototype func(p *StreamPool, g *cyclic.Group,
	x, y, z *cyclic.IntBuffer) (*cyclic.IntBuffer, error)

// GetName returns name of op (ExpChunk)
func (ExpChunkPrototype) GetName() string {
	return "ExpChunk"
}

// GetInputSize is the size of each chunk for this op
func (ExpChunkPrototype) GetInputSize() uint32 {
	return 64
}
