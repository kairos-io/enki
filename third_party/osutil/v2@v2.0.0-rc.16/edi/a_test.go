// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package edi

import (
	"io/ioutil"
	"testing"
)

var (
	tmpFilename string
	fileBackup  string
)

var (
	content = []byte(`
  Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor 
incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis 
nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. 
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu 
fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in 
culpa qui officia deserunt mollit anim id est laborum.
`)
)

func Test_a(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "edi-")
	if err != nil {
		t.Fatal(err)
	}
	tmpFilename = tmpFile.Name()
	fileBackup = tmpFilename + "+1~"

	if _, err = tmpFile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err = tmpFile.Close(); err != nil {
		t.Fatal(err)
	}
}
