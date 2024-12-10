// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package executil

import "os"

const path_ = "/sbin:/bin:/usr/sbin:/usr/bin"

// To be used at functions related to 'RunAsBash'.
var (
	env  []string
	home string // to expand symbol "~"
)

// Gets some environment variables.
func init() {
	// Note: if the next line is commented out, see line 75 at file 'bash.go'.
	//env = os.Environ()
	home = os.Getenv("HOME")

	/*if path := os.Getenv("PATH"); path == "" {
		if err = os.Setenv("PATH", path_); err != nil {
			log.Print(err)
		}
	}*/

	env = []string{"PATH=" + path_} // from file boot
}
