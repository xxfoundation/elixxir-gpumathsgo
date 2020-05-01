////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

package gpumaths

import (
	"gitlab.com/elixxir/crypto/cryptops"
	"testing"
)

func TestExpChunk(t *testing.T) {
	const numSlots = 32
	// Do computations with CUDA first
	g := makeTestGroup4096()
	base := g.NewIntBuffer(numSlots, g.NewInt(2))
	exp := g.NewIntBuffer(numSlots, g.NewInt(2))
	result := g.NewIntBuffer(numSlots, g.NewInt(2))


	for i := 0; i < numSlots; i++ {
		g.Random(base.Get(uint32(i)))
		g.Random(exp.Get(uint32(i)))
	}

	// Ensure correct behavior if the stream doesn't have enough memory to process the whole chunk
	streamPool, err := NewStreamPool(1, streamSizeContaining(numSlots, kernelPowmOdd) / 3 - 800)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ExpChunk(streamPool, g, base, exp, result)
	if err != nil {
		t.Fatal(err.Error())
	}

	// Compare to results from the Golang library
	// z = x**y mod p
	for i := 0; i < numSlots; i++ {
		x := base.Get(uint32(i))
		y := exp.Get(uint32(i))
		goResult := g.NewInt(2)
		g.Exp(x, y, goResult)
		cgbnResult := result.Get(uint32(i))
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

func TestElGamalChunk(t *testing.T) {
	const numSlots = 20
	// Do computations with CUDA first
	g := makeTestGroup4096()
	key := g.NewIntBuffer(numSlots, g.NewInt(2))
	privateKey := g.NewIntBuffer(numSlots, g.NewInt(2))
	publicCypherKey := g.Random(g.NewInt(2))
	ecrKey := g.NewIntBuffer(numSlots, g.NewInt(2))
	cypher := g.NewIntBuffer(numSlots, g.NewInt(2))


	for i := 0; i < numSlots; i++ {
		g.Random(key.Get(uint32(i)))
		g.Random(privateKey.Get(uint32(i)))
		g.Random(ecrKey.Get(uint32(i)))
		g.Random(cypher.Get(uint32(i)))
	}
	goEcrKey := ecrKey.DeepCopy()
	goCypher := cypher.DeepCopy()

	// Ensure correct behavior if the stream doesn't have enough memory to process the whole chunk
	streamPool, err := NewStreamPool(1, streamSizeContaining(numSlots, kernelElgamal) / 3 - 800)
	if err != nil {
		t.Fatal(err)
	}
	err = ElGamalChunk(streamPool, g, key, privateKey, publicCypherKey, ecrKey, cypher)
	if err != nil {
		t.Fatal(err.Error())
	}

	// Compare to results from the Golang library
	// z = x**y mod p
	for i := 0; i < numSlots; i++ {
		cryptops.ElGamal(g, key.Get(uint32(i)), privateKey.Get(uint32(i)), publicCypherKey, goEcrKey.Get(uint32(i)), goCypher.Get(uint32(i)))
		cgbnEcrKey := ecrKey.Get(uint32(i))
		cgbnCypher := cypher.Get(uint32(i))
		goEcrKeyResult := goEcrKey.Get(uint32(i))
		goCypherResult := goCypher.Get(uint32(i))
		if cgbnEcrKey.Cmp(goEcrKeyResult) != 0 {
			t.Errorf("Go EcrKey (%+v) didn't match CUDA results (%+v) in slot %v", goEcrKeyResult.Text(16), cgbnEcrKey.Text(16), i)
		}
		if cgbnCypher.Cmp(goCypherResult) != 0 {
			t.Errorf("Go cypher (%+v) didn't match CUDA results (%+v) in slot %v", goCypherResult.Text(16), cgbnCypher.Text(16), i)
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
