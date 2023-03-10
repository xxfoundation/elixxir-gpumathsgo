////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Package cryptops wraps various cryptographic operations around a generic interface.
// Operations include but are not limited to: key generation, ElGamal, multiplication, etc.
package cryptops

import (
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/xx_network/crypto/csprng"
)

const ShareKeyBytesLen = 256 / 8

// GeneratePrototype is the function type for generating phase and sharing keys.
// phase keys are those used to encrypt/decrypt/permute during realtime, and
// share keys are used to share the phase keys under encryption.
type GeneratePrototype func(g *cyclic.Group, phaseKey,
	shareKey *cyclic.Int, rng csprng.Source) error

// Generate implements the Generate Prototype. The share key is
// generated per guidelines here:
//   https://www.keylength.com/en/4/
// Our prime group size is 4096 (> 3072), so these (ephemeral, valid
// only for this round) 256 bit secret keys for this operation should
// provide security at least as good as a 128 bit symmetric
// algorithm. See RFC 3766 (pg 19) and/or ECRYPT CSA "Algorithms, Key
// Size and Protocols Report (2018)" (pg 48) for more details.
var Generate GeneratePrototype = func(g *cyclic.Group, phaseKey,
	shareKey *cyclic.Int, rng csprng.Source) error {
	p := g.GetPBytes()
	var shareKeyBytes, phaseKeyBytes []byte
	var err error

	shareKeyBytes, err = csprng.GenerateInGroup(p, ShareKeyBytesLen, rng)
	if err != nil {
		return err
	}
	phaseKeyBytes, err = csprng.GenerateInGroup(p, len(p), rng)
	if err != nil {
		return err
	}

	g.SetBytes(shareKey, shareKeyBytes)
	g.SetBytes(phaseKey, phaseKeyBytes)
	return nil
}

// GetName returns the name of the Generate cryptop, "Generate"
func (GeneratePrototype) GetName() string {
	return "Generate"
}

// GetInputSize returns the input size (the number of parallel computations
// it does at once)
func (GeneratePrototype) GetInputSize() uint32 {
	return uint32(1)
}
