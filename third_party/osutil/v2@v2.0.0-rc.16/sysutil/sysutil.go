// Copyright 2021 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

/*
Package sysutil defines operating systems and detects the Linux distribution.
Also, it handles basic operations in the management of packages in
FreeBSD, Linux and macOs operating systems.

The output of the commands run by the package managers are not printed. To set the standard output,
to use the method 'SetStdout()'.

NOTE: Package management systems untested:

 + Packman (Arch)
 + ebuild  (Gentoo)
 + RPM     (CentOS)
*/
package sysutil

import (
	"os"
)

// MustDisableColor returns true to indicate that the color should be disabled.
// It is useful to know when disable the color into a command line interface (cli).
//
// It checks the next environment variables:
//
//  + The NO_COLOR environment variable is set.
//  + The TERM environment variable has the value 'dumb'.
func MustDisableColor() bool {
	_, found := os.LookupEnv("NO_COLOR")
	if found {
		return true
	}

	if v := os.Getenv("TERM"); v == "dumb" {
		return true
	}

	return false
}
