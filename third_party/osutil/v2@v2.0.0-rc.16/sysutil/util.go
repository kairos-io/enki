// Copyright 2019 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// LookDirExec looks up the directory of an executable.
func LookDirExec(sys System, filename string) (string, error) {
	homeExec, err := exec.LookPath(filename)
	if err != nil {
		if homeExec, err = LookPath(sys, filename); err != nil {
			if errors.Is(err, ErrNotFound) {
				return "", fmt.Errorf("home directory for %s not found", filename)
			}
			return "", err
		}
	} else {
		if !filepath.IsAbs(homeExec) {
			if homeExec, err = filepath.Abs(homeExec); err != nil {
				return "", err
			}
		}
	}

	return filepath.Dir(homeExec), nil
}

// LookPath searches for an executable named file in the system directories given one or several
// executables.
func LookPath(sys System, filename ...string) (string, error) {
	dirInit := "/"
	skipDirs := make([]string, 0)

	switch sys {
	case Windows:
		dirInit = `\`
		skipDirs = []string{
			`\$Recycle.Bin`, `\ProgramData`, `\Users`, `\Windows`,
		}
	case Linux:
		skipDirs = []string{
			"/home", "/bin", "/sbin", "/snap",

			"/boot", "/etc", "/root", "/tmp", "/var", "/lost+found",
			"/dev", "/proc", "/run", "/sys",
			"/cdrom", "/media", "/mnt", "/srv",
			"/lib", "/lib32", "/lib64", "/libx32",

			"/usr/lib", "/usr/lib32", "/usr/lib64", "/usr/libx32", "/usr/libexec",
			"/usr/games", "/usr/include", "/usr/share", "/usr/src",

			// Look at:
			// "/opt", "/usr/bin", "/usr/sbin", "/usr/local",
		}
	case MacOS:
		skipDirs = []string{
			"/Applications",
			"/System/", "/Users", "/Volumes",
			"/cores", "/private",
			"/bin", "/dev", "/etc", "/home", "/opt", "/sbin", "/tmp", "/usr", "/var",
		}
	default:
		panic("unimplemented for system " + sys.String())
	}
	pathFound := ""

	err := filepath.Walk(dirInit, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			//fmt.Printf("- %q: %s\n", path, err)
			return filepath.SkipDir
		}

		if path == dirInit {
			return nil
		}
		name := info.Name()

		if info.IsDir() {
			for _, v := range skipDirs {
				if path == v {
					//fmt.Printf("+ Skipping dir: %+v\n", info.Name())
					return filepath.SkipDir
				}
			}
			return nil
		}

		for _, v := range filename {
			if v == name {
				//fmt.Println(path)
				pathFound = path
				return errFound
			}
		}

		return nil
	})
	if err == nil {
		return "", ErrNotFound
	}
	//fmt.Println("PATH:", pathFound)

	if sys == Windows { // Get the volume name
		pathWin, err := filepath.Abs(pathFound)
		if err != nil {
			return "", err
		}
		return pathWin, nil
	}
	return pathFound, nil
}

// == Errors
//

var (
	errFound = errors.New("file found")
	// ErrNotFound indicates when a search does not find a file at 'LookPath()'.
	ErrNotFound = errors.New("file not found")
)
