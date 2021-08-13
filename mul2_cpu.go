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
	"git.xx.network/elixxir/crypto/cyclic"
)

// Mul2Chunk is stubbed unless GPU is present.
var Mul2Chunk Mul2ChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	result *cyclic.IntBuffer, x *cyclic.IntBuffer, y *cyclic.IntBuffer) error {
	return errors.New(NoGpuErrStr)
}

var Mul2Slice Mul2SlicePrototype = func(p *StreamPool, g *cyclic.Group,
	x *cyclic.IntBuffer, y, result []*cyclic.Int) error {
	return errors.New(NoGpuErrStr)
}
