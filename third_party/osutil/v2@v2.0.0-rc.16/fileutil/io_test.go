// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fileutil

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

func TestCreate(t *testing.T) {
	if err := Create(fileTemp, []byte(`
  Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor 
incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis 
nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. 
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu 
fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in 
culpa qui officia deserunt mollit anim id est laborum.
`)); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("wc", "-l", fileTemp)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}

	out = bytes.TrimSpace(out)
	if out[0] != '7' {
		t.Fatalf("got %q lines, want 7", out[0])
	}
}

const FILENAME = "doc.go"

func TestTempFile(t *testing.T) {
	filename, err := CopytoTemp(FILENAME)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filename); err != nil {
		t.Error(err)
	}
}
