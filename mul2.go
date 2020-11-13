///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package gpumaths

import "gitlab.com/elixxir/crypto/cyclic"

// mul2.go contains the input, results, and other types for running the mul2
// operation against the GPU. The actual GPU call is in mul2_gpu.go and is
// marked to require `-tags cuda` in your build.

// Mul2ChunkPrototype defines the function type for running the mul2
// kernel in the GPU.
type Mul2ChunkPrototype func(p *StreamPool, g *cyclic.Group,
	x, y, result *cyclic.IntBuffer) error

// Mul2Slice is similar, but it takes a slice of cyclic ints as input and result instead of an int buffer
type Mul2SlicePrototype func(p *StreamPool, g *cyclic.Group,
	x *cyclic.IntBuffer, y, result []*cyclic.Int) error

// GetInputSize is how big chunk sizes should be to run the mul2 operation
func (Mul2ChunkPrototype) GetInputSize() uint32 {
	return 256
}

// GetName return the name of the Mul2Chunk operation
func (Mul2ChunkPrototype) GetName() string {
	return "Mul2Chunk"
}

// GetInputSize is how big chunk sizes should be to run the mul2 operation
func (Mul2SlicePrototype) GetInputSize() uint32 {
	return 256
}

// GetName return the name of the Mul2Slice operation
func (Mul2SlicePrototype) GetName() string {
	return "Mul2Slice"
}
