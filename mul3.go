////////////////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                                       //
//                                                                                        //
// Use of this source code is governed by a license that can be found in the LICENSE file //
////////////////////////////////////////////////////////////////////////////////////////////

package gpumaths

import "gitlab.com/elixxir/crypto/cyclic"

type Mul3ChunkPrototype func(p *StreamPool, g *cyclic.Group,
	x, y, z, result *cyclic.IntBuffer) error

// GetInputSize is how big chunk sizes should be to run the mul3 operation
func (Mul3ChunkPrototype) GetInputSize() uint32 {
	return 256
}

func (Mul3ChunkPrototype) GetName() string {
	return "Mul3Chunk"
}
