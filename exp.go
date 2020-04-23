////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package gpumaths

import "gitlab.com/elixxir/crypto/cyclic"

// exp.go contains the input, results, and other types for running the
// exp operation against the GPU. The actual GPU call is in exp_gpu.go
// and is marked to require `-tags cuda` in your build.

// ExpInputSlot is the input structure for each slot in the batch
type ExpInputSlot struct {
	Base     []byte
	Exponent []byte
}

// ExpInput is the input structure for the op
type ExpInput struct {
	Slots   []ExpInputSlot
	Modulus []byte
}

// ExpResult is the output structure for the op
type ExpResult struct {
	Results [][]byte
	Err     error
}

// ExpChunkPrototype Implement cryptop interface for ExpChunk
type ExpChunkPrototype func(p *StreamPool, g *cyclic.Group,
	x, y, z *cyclic.IntBuffer) (*cyclic.IntBuffer, error)

// GetName returns name of op (ExpChunk)
func (ExpChunkPrototype) GetName() string {
	return "ExpChunk"
}

// GetInputSize is the size of each chunk for this op
func (ExpChunkPrototype) GetInputSize() uint32 {
	return 256
}
