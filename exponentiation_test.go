package main

import (
	"gitlab.com/elixxir/crypto/cryptops"
	"gitlab.com/elixxir/crypto/cyclic"
	"testing"
)

// CUDA powm result should match golang powm result for all slots
func TestPowm4096(t *testing.T) {
	numSlots := 128
	// Do computations with CUDA first
	const xBitLen = 4096
	const xByteLen = xBitLen / 8
	const yBitLen = 256
	const yByteLen = yBitLen / 8
	g := makeTestGroup4096()
	inputMemGenerator := benchmarkInputMemGenerator(g, xByteLen, yByteLen, numSlots, xByteLen)
	pMem := g.GetP().CGBNMem(bnSizeBits)

	inputMem := <-inputMemGenerator

	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	results, err := powm4096(pMem, inputMem, numSlots)
	if err != nil {
		t.Fatal(err)
	}
	err = resetDevice()
	if err != nil {
		t.Fatal(err)
	}

	// Compare to results from the Golang library
	// z = x**y mod p
	for inputStart, resultStart := 0, 0; inputStart < len(inputMem); inputStart, resultStart = inputStart+2*xByteLen, resultStart+xByteLen {
		x := g.NewIntFromCGBN(inputMem[inputStart : inputStart+xByteLen])
		y := g.NewIntFromCGBN(inputMem[inputStart+xByteLen : inputStart+2*xByteLen])
		goResult := g.NewInt(2)
		g.Exp(x, y, goResult)
		cgbnResult := g.NewIntFromCGBN(results[resultStart:resultStart+xByteLen])
		if goResult.Cmp(cgbnResult) != 0 {
			t.Errorf("Go results (%+v) didn't match CUDA results (%+v) in slot %v", goResult.Text(16), cgbnResult.Text(16), resultStart / xByteLen)
		}
	}
}

func TestElgamal4096(t *testing.T) {
	const numSlots = 12
	// Do computations with CUDA first
	g := makeTestGroup4096()

	// Build some random inputs for elgamal kernel
	var (
		publicCypherKey *cyclic.Int
		key []*cyclic.Int
		privateKey []*cyclic.Int
		ecrKeys []*cyclic.Int
		cypher []*cyclic.Int
	)
	publicCypherKey = g.Random(g.NewInt(1))
	constantMem := stageElgamalConstants(g, publicCypherKey)
	for i := 0; i < numSlots; i++ {
		privateKey = append(privateKey, g.Random(g.NewInt(1)))
		key = append(key, g.Random(g.NewInt(1)))
		ecrKeys = append(ecrKeys, g.NewInt(1))
		cypher = append(cypher, g.NewInt(1))
	}
	inputMem, err := stageElgamalInputs(privateKey, key, ecrKeys, cypher)
	if err != nil {
		t.Fatal(err)
	}

	// Run elgamal kernel
	streams, err := createStreams(1, numSlots, kernelElgamal)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := destroyStreams(streams)
		if err != nil {
			panic(err)
		}
	}()
	stream := streams[0]
	err = upload(constantMem, inputMem, numSlots, stream, kernelElgamal)
	if err != nil {
		t.Fatal(err)
	}
	err = run(stream, kernelElgamal)
	if err != nil {
		t.Fatal(err)
	}
	err = download(stream)
	if err != nil {
		t.Fatal(err)
	}
	results, err := getResults(stream, numSlots, kernelElgamal)
	if err != nil {
		t.Fatal(err)
	}

	// Turn results into cyclic ints
	var (
		ecrKeysResults []*cyclic.Int
		cypherResults []*cyclic.Int
	)
	for len(results) > 0 {
		ecrKeysResults = append(ecrKeysResults, g.NewIntFromCGBN(results[:bnSizeBytes]))
		cypherResults = append(cypherResults, g.NewIntFromCGBN(results[bnSizeBytes:2*bnSizeBytes]))
		results = results[2*bnSizeBytes:]
	}

	// Compare with results from elixxir/crypto implementation
	for i := 0; i < len(ecrKeys); i++ {
		cpuEcrKeys := g.NewInt(1)
		cpuCypher := g.NewInt(1)
		cryptops.ElGamal(g, key[i], privateKey[i], publicCypherKey, cpuEcrKeys, cpuCypher)
		if cpuEcrKeys.Cmp(ecrKeysResults[i]) != 0 {
			t.Errorf("ecrkeys didn't match cpu result at index %v", i)
		}
		if cpuCypher.Cmp(cypherResults[i]) != 0 {
			t.Errorf("cypher didn't match cpu result at index %v", i)
		}
	}
}
