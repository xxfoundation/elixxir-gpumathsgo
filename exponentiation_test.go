///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

package gpumaths

import (
	"gitlab.com/elixxir/crypto/cryptops"
	"gitlab.com/xx_network/crypto/large"
	"testing"
)

// CUDA powm result should match golang powm result for all slots
func TestPowm4096(t *testing.T) {
	env := &gpumathsEnv4096
	const numSlots = 128
	// Do computations with CUDA first
	g := makeTestGroup4096()

	Base := initRandomIntBuffer(g, numSlots, 42, 0)
	Exponent := initRandomIntBuffer(g, numSlots, 42, 256/8)
	Result := g.NewIntBuffer(numSlots, g.NewInt(1))

	streamPool, err := NewStreamPool(1, env.streamSizeContaining(numSlots, kernelPowmOdd))
	if err != nil {
		t.Fatal(err)
	}
	stream := streamPool.TakeStream()
	errors := exp(g, Base, Exponent, Result, env, stream)
	err = <-errors
	if err != nil {
		t.Fatal(err)
	}
	streamPool.ReturnStream(stream)

	// Compare to results from the Golang library
	// z = x**y mod p
	for i := uint32(0); i < numSlots; i++ {
		x := Base.Get(i)
		y := Exponent.Get(i)
		goResult := g.NewInt(2)
		g.Exp(x, y, goResult)
		cgbnResult := Result.Get(i)
		if goResult.Cmp(cgbnResult) != 0 {
			t.Errorf("Go results (%+v) didn't match CUDA results (%+v) in slot %v", goResult.Text(16), cgbnResult.Text(16), i)
		}
	}
	err = streamPool.Destroy()
	if err != nil {
		t.Fatal(err)
	}
	// flush system profiling data, just in case
	err = resetDevice()
	if err != nil {
		t.Fatal(err)
	}
}

func TestElgamal4096(t *testing.T) {
	const numSlots = 12
	// Do computations with CUDA first
	g := makeTestGroup4096()
	env := &gpumathsEnv4096

	// Build some random inputs for elgamal kernel
	PublicCypherKey := g.Random(g.NewInt(1))
	PrivateKey := initRandomIntBuffer(g, numSlots, 42, 0)
	Key := initRandomIntBuffer(g, numSlots, 42, 0)
	EcrKey := g.NewIntBuffer(numSlots, g.NewInt(1))
	Cypher := g.NewIntBuffer(numSlots, g.NewInt(1))

	// Run elgamal kernel
	streamPool, err := NewStreamPool(1, env.streamSizeContaining(numSlots, kernelElgamal))
	if err != nil {
		t.Fatal(err)
	}
	stream := streamPool.TakeStream()
	// I think I want to actually pass a stream to ElGamal...
	// Is that too explicit/weird?
	resultChan := elGamal(g, Key, PrivateKey, PublicCypherKey, EcrKey, Cypher, env, stream)
	err = <-resultChan
	if err != nil {
		t.Error(err)
	}

	// Compare with results from elixxir/crypto implementation
	for i := uint32(0); i < numSlots; i++ {
		cpuEcrKeys := g.NewInt(1)
		cpuCypher := g.NewInt(1)
		cryptops.ElGamal(g, Key.Get(i), PrivateKey.Get(i), PublicCypherKey, cpuEcrKeys, cpuCypher)
		if cpuEcrKeys.Cmp(EcrKey.Get(i)) != 0 {
			t.Errorf("ecrkeys didn't match cpu result at index %v", i)
		}
		if cpuCypher.Cmp(Cypher.Get(i)) != 0 {
			t.Errorf("cypher didn't match cpu result at index %v", i)
		}
	}
	streamPool.ReturnStream(stream)
	err = streamPool.Destroy()
	if err != nil {
		t.Fatal(err)
	}
	// flush system profiling data, just in case
	err = resetDevice()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPutInt(t *testing.T) {
	a := large.Bits{1, 2, 3, 4, 5}
	b := make(large.Bits, len(a))
	putBits(b, a, len(a))
	t.Logf("%+v\n", b)

	// PutInt also needs to work correctly if the src int is short (consequence of Bytes() function being shorter if the int is shorter)
	//  In this case it should overwrite the rest with zeroes
	//  So, a straight-up reverse and copy is a no-go
	// Two loops > branching
	c := large.Bits{1, 2, 3}
	d := make(large.Bits, len(a)+5)
	// Convert to cgbn
	putBits(d, c, len(d))
	t.Logf("%+v\n", d)
	// Convert to normal bytes
	e := make(large.Bits, len(d))
	putBits(e, d, len(d))
	t.Logf("%+v\n", e)
	if large.NewIntFromBits(c).Cmp(large.NewIntFromBits(e)) != 0 {
		t.Error("short int not preserved by importing/exporting")
	}
}
