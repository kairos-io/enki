// Copyright 2019 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysutil

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/tredoe/osutil/v2/executil"
)

func TestExecWinshell(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.SkipNow()
	}
	for _, v := range executil.ListWinShell {
		out, err := executil.RunWinshell(v, false, `dir C:\`)
		fmt.Println(string(out))
		if err != nil {
			t.Error("error:", err)
		}
	}
}

func TestDetectSystem(t *testing.T) {
	sys, _, err := SystemFromGOOS()
	if err != nil {
		t.Fatal(err)
	}

	ver, err := DetectSystemVer(sys)
	if err != nil {
		t.Error(err)
	}
	t.Logf("System version: %q", ver)
}
