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

// RevealChunk is stubbed unless GPU is present.
var RevealChunk RevealChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	publicCypherKey *cyclic.Int, cypher *cyclic.IntBuffer, result *cyclic.IntBuffer) error {
	return errors.New(NoGpuErrStr)
}
