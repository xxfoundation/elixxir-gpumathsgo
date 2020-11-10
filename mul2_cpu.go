///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//+build !linux !gpu

package gpumaths

import (
	"errors"
	"gitlab.com/elixxir/crypto/cyclic"
)

// Mul2Chunk is stubbed unless GPU is present.
var Mul2Chunk Mul2ChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	result *cyclic.IntBuffer, x *cyclic.IntBuffer, y *cyclic.IntBuffer) error {
	return errors.New(NoGpuErrStr)
}

// Mul2 is an empty stub that returns an error when called.
//func Mul2(g *cyclic.Group, x, y, result *cyclic.IntBuffer, env gpumathsEnv, stream Stream) chan error {
// Return the result later, when the GPU job finishes
//resultChan := make(chan error, 1)
//resultChan <- errors.New(NoGpuErrStr)
//return resultChan
//}
