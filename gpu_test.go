///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// gpu_test.go merely has helper functions used in all the other gpu tests.

//+build linux,gpu

package gpumaths

import (
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/gpumathsgo/cryptops"
	"gitlab.com/xx_network/crypto/csprng"
	"gitlab.com/xx_network/crypto/large"
	"math/rand"
)

// setupGroup is a helper for generating a cyclic group for testing
func initTestGroup() *cyclic.Group {
	// NOTE: These should reflect what we'd be using in server deployments,
	// which is typically going to be MODP4096 or MODP2048 from the RFC.
	// Using MODP4096 for now.
	primeString := "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1" +
		"29024E088A67CC74020BBEA63B139B22514A08798E3404DD" +
		"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245" +
		"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED" +
		"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D" +
		"C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F" +
		"83655D23DCA3AD961C62F356208552BB9ED529077096966D" +
		"670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B" +
		"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9" +
		"DE2BCBF6955817183995497CEA956AE515D2261898FA0510" +
		"15728E5A8AAAC42DAD33170D04507A33A85521ABDF1CBA64" +
		"ECFB850458DBEF0A8AEA71575D060C7DB3970F85A6E1E4C7" +
		"ABF5AE8CDB0933D71E8C94E04A25619DCEE3D2261AD2EE6B" +
		"F12FFA06D98A0864D87602733EC86A64521F2B18177B200C" +
		"BBE117577A615D6C770988C0BAD946E208E24FA074E5AB31" +
		"43DB5BFCE0FD108E4B82D120A92108011A723C12A787E6D7" +
		"88719A10BDBA5B2699C327186AF4E23C1A946834B6150BDA" +
		"2583E9CA2AD44CE8DBBBC2DB04DE8EF92E8EFC141FBECAA6" +
		"287C59474E6BC05D99B2964FA090C3A2233BA186515BE7ED" +
		"1F612970CEE2D7AFB81BDD762170481CD0069127D5B05AA9" +
		"93B4EA988D8FDDC186FFB7DC90A6C08F4DF435C934063199" +
		"FFFFFFFFFFFFFFFF"
	generator := large.NewInt(2)
	grp := cyclic.NewGroup(large.NewIntFromString(primeString, 16),
		generator)
	return grp
}

type rngT struct {
	rand.Rand
}

func newRng(seed int64) *rngT {
	return &rngT{Rand: *rand.New(rand.NewSource(seed))}
}

// Make math's rand work with our csprng interface
func (r *rngT) SetSeed(seed []byte) error {
	numBytes := int64(len(seed))
	if numBytes > 8 {
		numBytes = 8
	}
	seedVal := int64(0)
	for i := int64(0); i < numBytes; i++ {
		seedVal ^= int64(seed[i]) << 8 * i
	}
	r.Seed(seedVal)
	return nil
}

// initKeys initializes a key buffer using the given seed so it can be
// reproduce-able
func initKeys(grp *cyclic.Group, batchSize uint32,
	phaseKeys, shareKeys *cyclic.IntBuffer, seed int64) {
	rng := newRng(seed)
	for i := uint32(0); i < batchSize; i++ {
		err := cryptops.Generate(grp, phaseKeys.Get(i),
			shareKeys.Get(i), rng)
		if err != nil {
			panic(err.Error())
		}
	}
}

// intSize is length of random ints in bytes
func initRandomIntBuffer(grp *cyclic.Group, batchSize uint32,
	seed int64, intSize int) *cyclic.IntBuffer {
	rng := newRng(seed)
	buffer := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	// default to prime length
	if intSize == 0 {
		intSize = len(grp.GetPBytes())
	}
	for i := uint32(0); i < batchSize; i++ {
		b, err := csprng.GenerateInGroup(grp.GetPBytes(), intSize, rng)
		if err != nil {
			panic(err.Error())
		}
		grp.SetBytes(buffer.Get(i), b)
	}
	return buffer
}
