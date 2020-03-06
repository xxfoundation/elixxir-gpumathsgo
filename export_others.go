//+build !linux !cuda

package gpumaths

import "C"
import "errors"

// Stub exported symbols for systems that can't build with gpu

func ElGamal(input ElGamalInput, stream Stream) chan ElGamalResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan ElGamalResult, 1)
	resultChan<-ElGamalResult{
		Err:   errors.New("gpumaths stubbed build doesn't support CUDA stream pool"),
	}
	return resultChan
}

// Precondition: Modulus must be odd
func Exp(input ExpInput, stream Stream) chan ExpResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan ExpResult, 1)
	resultChan<-ExpResult{
		Err:   errors.New("gpumaths stubbed build doesn't support CUDA stream pool"),
	}
	return resultChan
}
