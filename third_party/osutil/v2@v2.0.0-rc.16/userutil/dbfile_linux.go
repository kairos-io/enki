// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package userutil

// Since tests will be done in temporary files, there is to use variables to
// change the values at testing.
var (
	fileUser    = "/etc/passwd"
	fileGroup   = "/etc/group"
	fileShadow  = "/etc/shadow"
	fileGShadow = "/etc/gshadow"
)
