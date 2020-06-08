////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package gpumaths

import "gitlab.com/elixxir/crypto/cyclic"

// mul2.go contains the input, results, and other types for running the mul2
// operation against the GPU. The actual GPU call is in mul2_gpu.go and is
// marked to require `-tags cuda` in your build.

// Mul2 needs two numbers to multiply together, modulo a prime.
type Mul2InputSlot struct {
	X []byte
	Y []byte
}

// Mul2Input uses each individual slot and includes the Prime
type Mul2Input struct {
	Slots []Mul2InputSlot
	Prime []byte
}

// Mul2ResultSlot returns the multiplication result
type Mul2ResultSlot struct {
	Result []byte
}

// Mul2Result returns results for each slot or an error
type Mul2Result struct {
	Slots []Mul2ResultSlot
	Err   error
}

// Prototype Definition

// Mul2ChunkPrototype defines the function type for running the Mul2
// kernel in the GPU.
type Mul2ChunkPrototype func(p *StreamPool, g *cyclic.Group,
	result *cyclic.IntBuffer, x *cyclic.IntBuffer, y *cyclic.IntBuffer) error

// Multiply into a slice to provide compatibility with permutations
type Mul2SlicePrototype func(p *StreamPool, g *cyclic.Group,
	result []*cyclic.Int, x *cyclic.IntBuffer, y []*cyclic.Int) error

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

// GetName return the name of the Mul2Chunk operation
func (Mul2SlicePrototype) GetName() string {
	return "Mul2Slice"
}
