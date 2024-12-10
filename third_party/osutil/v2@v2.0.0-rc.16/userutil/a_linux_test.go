// Copyright 2013 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package userutil

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tredoe/osutil/v2/fileutil"
	"github.com/tredoe/osutil/v2/sysutil"
)

const (
	USER     = "u_foo"
	USER2    = "u_foo2"
	SYS_USER = "usys_bar"

	GROUP     = "g_foo"
	SYS_GROUP = "gsys_bar"
)

var MEMBERS = []string{USER, SYS_USER}

// Stores the ids at creating the groups.
var GID, SYS_GID int

// == Copy the system files before of be edited.

var removeFiles []string

func init() {
	err := MustBeSuperUser(sysutil.SystemUndefined)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if fileUser, err = fileutil.CopytoTemp(fileUser); err != nil {
		goto _error
	}
	removeFiles = append(removeFiles, fileUser)

	if fileGroup, err = fileutil.CopytoTemp(fileGroup); err != nil {
		goto _error
	}
	removeFiles = append(removeFiles, fileGroup)

	if fileShadow, err = fileutil.CopytoTemp(fileShadow); err != nil {
		goto _error
	}
	removeFiles = append(removeFiles, fileShadow)

	if useGshadow {
		if fileGShadow, err = fileutil.CopytoTemp(fileGShadow); err != nil {
			goto _error
		}
		removeFiles = append(removeFiles, fileGShadow)
	}

	return

_error:
	removeTempFiles()
	log.Fatalf("%s", err)
}

func removeTempFiles() {
	for _, fname := range removeFiles {
		if dir := filepath.Dir(fname); dir == "/etc" {
			fmt.Printf("WARNING! Can not remove file %q", fname)
			continue
		}

		for _, f := range []string{fname, fname + "+1~"} {
			if err := os.Remove(f); err != nil {
				log.Printf("%s", err)
			}
		}
	}
}
