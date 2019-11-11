package main

import (
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
