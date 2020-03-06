//+build !linux !cuda

package gpumaths

import (
	"errors"
	"gitlab.com/elixxir/crypto/cyclic"
)

// Using this function doesn't allow you to do other things while waiting on the kernel to finish
// Perform exponentiation for two operands and place the result in z (which is also returned)
var ExpChunk ExpChunkPrototype = func(p *StreamPool, g *cyclic.Group, x, y, z *cyclic.IntBuffer) (*cyclic.IntBuffer, error) {
	return z, errors.New("gpumaths stubbed build doesn't support CUDA stream pool")
}

// Precondition: All int buffers must have the same length
// Perform the ElGamal operation on two int buffers
var ElGamalChunk ElGamalChunkPrototype = func(p *StreamPool, g *cyclic.Group, key, privateKey *cyclic.IntBuffer, publicCypherKey *cyclic.Int, ecrKey, cypher *cyclic.IntBuffer) error {
	return errors.New("gpumaths stubbed build doesn't support CUDA stream pool")
}

