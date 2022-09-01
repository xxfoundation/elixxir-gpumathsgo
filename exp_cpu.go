////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

//+build !linux !gpu

package gpumaths

import (
	"errors"
	"gitlab.com/elixxir/crypto/cyclic"
)

// ExpChunk is stubbed unless GPU is present.
var ExpChunk ExpChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	x, y, z *cyclic.IntBuffer) (*cyclic.IntBuffer, error) {
	return z, errors.New(NoGpuErrStr)
}
