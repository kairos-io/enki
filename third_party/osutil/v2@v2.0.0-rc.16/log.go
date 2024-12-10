// Copyright 2021 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package osutil

import (
	"io"
	"log"
)

// Global loggers. By default, they do not write.
var (
	// Log is used by the packages 'edi', 'fileutil', 'sysutil' and 'sysutil/service' and 'userutil'.
	Log = log.New(io.Discard, "", -1)
	// LogShell is used by the package 'executil' for the commands run from the shell.
	LogShell = log.New(io.Discard, "", -1)
)
