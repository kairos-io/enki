// Copyright 2021 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package userutil

import (
	"errors"
	"os"
)

var useGshadow bool = true

// Some distribution (like OpenSuse) could have not the gshadow file.
func init() {
	_, err := os.Stat(fileGShadow)
	if err != nil && os.IsNotExist(err) {
		useGshadow = false
	}
}

// == Errors
//

var ErrGshadow error = errors.New("file gshadow unsopported")
