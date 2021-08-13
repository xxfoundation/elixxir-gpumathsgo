///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//+build !linux !gpu

package gpumaths

import (
	"github.com/pkg/errors"
	"git.xx.network/elixxir/crypto/cyclic"
)

// Mul3Chunk is stubbed unless GPU is present.
var Mul3Chunk Mul3ChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	x *cyclic.IntBuffer, y *cyclic.IntBuffer, z *cyclic.IntBuffer, result *cyclic.IntBuffer) error {
	return errors.New(NoGpuErrStr)
}
