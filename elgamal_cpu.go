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

// ElGamalChunk is stubbed unless GPU is present.
var ElGamalChunk ElGamalChunkPrototype = func(p *StreamPool, g *cyclic.Group, key, privateKey *cyclic.IntBuffer, publicCypherKey *cyclic.Int, ecrKey, cypher *cyclic.IntBuffer) error {
	return errors.New(NoGpuErrStr)
}
