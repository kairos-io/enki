// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysutil

import (
	"fmt"
	"os"
	"testing"

	"github.com/tredoe/osutil/v2"
)

func TestPackager(t *testing.T) {
	osutil.LogShell.SetOutput(os.Stdout)
	osutil.LogShell.SetFlags(0)
	osutil.LogShell.SetPrefix("  >> ")

	sys, dis, err := SystemFromGOOS()
	if err != nil {
		t.Fatal(err)
	}

	mng, err := DetectPkgManag(sys)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Package type detected: %s", mng.PackageType())

	if mng, err = NewPkgManagFromSystem(sys, dis); err != nil {
		t.Fatal(err)
	}
	t.Logf("Package type to use: %s", mng.PackageType())

	if err = mng.Install("foo"); err == nil {
		t.Errorf("\n%v", err)
	}

	if !testing.Verbose() {
		return
	}
	testUpdate(mng, t)
	testInstall(mng, t)
}

func testUpdate(mng PkgManager, t *testing.T) {
	var err error

	fmt.Printf("\n+ Update\n")
	if err = mng.Update(); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("\n+ Upgrade\n")
	if err = mng.Upgrade(); err != nil {
		t.Fatal(err)
	}
}

func testInstall(mng PkgManager, t *testing.T) {
	var err error
	pkg := "nano" // vim

	fmt.Printf("\n+ Remove\n")
	if err = mng.Remove(pkg); err != nil {
		t.Errorf("\n%s", err)
	}
	fmt.Printf("\n+ Install\n")
	if err = mng.Install(pkg); err != nil {
		t.Errorf("\n%s", err)
	}
	fmt.Printf("\n+ Purge\n")
	if err = mng.Purge(pkg); err != nil {
		t.Errorf("\n%s", err)
	}
	fmt.Printf("\n+ Install\n")
	if err = mng.Install(pkg); err != nil {
		t.Errorf("\n%s", err)
	}

	fmt.Printf("\n+ Clean\n")
	if err = mng.Clean(); err != nil {
		t.Errorf("\n%s", err)
	}
}
