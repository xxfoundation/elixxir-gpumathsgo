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
	numSlots := 128
	// Do computations with CUDA first
	g := makeTestGroup4096()

	input := ExpInput{
		Slots:   make([]ExpInputSlot, numSlots),
		Modulus: g.GetPBytes(),
	}

	for i := 0; i < numSlots; i++ {
		input.Slots[i] = ExpInputSlot{
			Base: g.Random(g.NewInt(1)).Bytes(),
			// Only use 256 bits of the exponent
			Exponent: g.Random(g.NewInt(1)).Bytes()[480:],
		}
	}

	streamPool, err := NewStreamPool(1, env.streamSizeContaining(numSlots, kernelPowmOdd))
	if err != nil {
		t.Fatal(err)
	}
	stream := streamPool.TakeStream()
	resultChan := Exp(input, env, stream)
	result := <-resultChan
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	streamPool.ReturnStream(stream)

	// Compare to results from the Golang library
	// z = x**y mod p
	for i := 0; i < numSlots; i++ {
		x := g.NewIntFromBytes(input.Slots[i].Base)
		y := g.NewIntFromBytes(input.Slots[i].Exponent)
		goResult := g.NewInt(2)
		g.Exp(x, y, goResult)
		cgbnResult := g.NewIntFromBytes(result.Results[i])
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
	input := ElGamalInput{
		Slots:           make([]ElGamalInputSlot, numSlots),
		PublicCypherKey: g.Random(g.NewInt(1)).Bytes(),
		Prime:           g.GetP().Bytes(),
		G:               g.GetG().Bytes(),
	}
	for i := 0; i < numSlots; i++ {
		input.Slots[i] = ElGamalInputSlot{
			PrivateKey: g.Random(g.NewInt(1)).Bytes(),
			Key:        g.Random(g.NewInt(1)).Bytes(),
			EcrKey:     g.NewInt(1).Bytes(),
			Cypher:     g.NewInt(1).Bytes(),
		}
	}

	// Run elgamal kernel
	streamPool, err := NewStreamPool(1, env.streamSizeContaining(numSlots, kernelElgamal))
	if err != nil {
		t.Fatal(err)
	}
	stream := streamPool.TakeStream()
	// I think I want to actually pass a stream to ElGamal...
	// Is that too explicit/weird?
	resultChan := ElGamal(input, env, stream)
	result := <-resultChan
	if result.Err != nil {
		t.Error(result.Err)
	}

	// Compare with results from elixxir/crypto implementation
	for i := 0; i < numSlots; i++ {
		cpuEcrKeys := g.NewInt(1)
		cpuCypher := g.NewInt(1)
		cryptops.ElGamal(g, g.NewIntFromBytes(input.Slots[i].Key), g.NewIntFromBytes(input.Slots[i].PrivateKey),
			g.NewIntFromBytes(input.PublicCypherKey), cpuEcrKeys, cpuCypher)
		if cpuEcrKeys.Cmp(g.NewIntFromBytes(result.Slots[i].EcrKey)) != 0 {
			t.Errorf("ecrkeys didn't match cpu result at index %v", i)
		}
		if cpuCypher.Cmp(g.NewIntFromBytes(result.Slots[i].Cypher)) != 0 {
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
	a := []byte{1, 2, 3, 4, 5}
	b := make([]byte, len(a))
	putInt(b, a, len(a))
	t.Logf("%+v\n", b)

	// PutInt also needs to work correctly if the src int is short (consequence of Bytes() function being shorter if the int is shorter)
	//  In this case it should overwrite the rest with zeroes
	//  So, a straight-up reverse and copy is a no-go
	// Two loops > branching
	c := []byte{1, 2, 3}
	d := make([]byte, len(a)+5)
	// Convert to cgbn
	putInt(d, c, len(d))
	t.Logf("%+v\n", d)
	// Convert to normal bytes
	e := make([]byte, len(d))
	putInt(e, d, len(d))
	t.Logf("%+v\n", e)
	if large.NewIntFromBytes(c).Cmp(large.NewIntFromBytes(e)) != 0 {
		t.Error("short int not preserved by importing/exporting")
	}
}
