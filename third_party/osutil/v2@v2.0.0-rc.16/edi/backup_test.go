// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package edi

import (
	"path/filepath"
	"testing"
)

func TestBackupSuffix(t *testing.T) {
	okFilenames := []string{"foo+1~", "foo+2~", "foo+5~", "foo+8~", "foo+9~"}
	badFilenames := []string{"foo+0~", "foo+10~", "foo+11~", "foo+22~"}

	for _, v := range okFilenames {
		ok, err := filepath.Match("foo"+suffixBackup, v)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("expected to not match %q", v)
		}
	}

	for _, v := range badFilenames {
		ok, err := filepath.Match("foo"+suffixBackup, v)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Errorf("expected to not match %q", v)
		}
	}
}
