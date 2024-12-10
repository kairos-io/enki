// Copyright 2019 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fileutil

import (
	"path/filepath"
)

// PathAbsDir returns the absolute path of a directory.
func PathAbsDir(dir string) (string, error) {
	out := dir

	if !filepath.IsAbs(dir) {
		dirAbs, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		out = dirAbs
	}

	return out, nil
}

// PathRelative returns the relative path of a file.
func PathRelative(dir, file string) string {
	relFile, _ := filepath.Rel(dir, file)
	if relFile == "" {
		relFile = file
	}

	return relFile
}
