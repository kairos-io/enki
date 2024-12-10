// Copyright 2019 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fileutil

import (
	"fmt"
	"os"
)

// CheckFile checks if the path exists and if it is a file.
func CheckFile(p string) error {
	info, err := os.Stat(p)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("expect a regular file at \"%s\"", p)
	}
	return nil
}
