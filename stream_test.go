///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

package gpumaths

import "testing"

func TestMaxSlots(t *testing.T) {
	env := gpumaths4096{}
	// Elgamal does about twice the math, so the max number of slots should be about half of powm odd
	// The difference comes from the number of constants needed
	offOfHalf := (float32(env.maxSlots(88888, kernelPowmOdd)) / float32(env.maxSlots(88888, kernelElgamal))) - 2
	t.Log(offOfHalf)
	if offOfHalf > 0.1 {
		t.Errorf("The same memory should be able to hold about 2x powm odd slots as elgamal slots, but the actual mem size capacity ratio was %v off from that", offOfHalf/2)
	}
}
