////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// +build linux,gpu

package gpumaths

// gpu_gpu exports ResetDevice when gpumaths is built with GPU support
func ResetDevice() error {
	return resetDevice()
}
