////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

//+build !linux !gpu

package gpumaths

import "errors"

// Stub for ResetDevice when gpu support isn't built
func ResetDevice() error {
	return errors.New("Built without GPU support, so cannot reset device")
}
