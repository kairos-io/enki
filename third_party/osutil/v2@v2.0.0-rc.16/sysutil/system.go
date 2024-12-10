// Copyright Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysutil

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"runtime"

	"github.com/tredoe/osutil/v2/executil"
)

var errSystem = errors.New("unsopported operating system")

// ListSystem is the list of allowed operating systems.
var ListSystem = [...]System{FreeBSD, Linux, MacOS, Windows}

// System represents an operating system.
type System uint8

// The operating systems.
const (
	SystemUndefined System = iota
	Linux
	FreeBSD
	MacOS
	Windows
)

func (s System) String() string {
	switch s {
	case Linux:
		return "Linux"
	case FreeBSD:
		return "FreeBSD"
	case MacOS:
		return "macOS"
	case Windows:
		return "Windows"
	default:
		panic("unreachable")
	}
}

// SystemFromGOOS returns the system from 'GOOS', and the distribution at Linux systems.
func SystemFromGOOS() (sys System, dist Distro, err error) {
	switch runtime.GOOS {
	case "linux":
		sys = Linux

		if dist, err = DetectDistro(); err != nil {
			return 0, 0, err
		}
	case "freebsd":
		sys = FreeBSD
	case "darwin":
		sys = MacOS
	case "windows":
		sys = Windows

	default:
		return 0, 0, errSystem
	}

	return
}

// * * *

var (
	signColon = []byte{':'}
	osVersion = []byte("os version")
)

// DetectSystemVer returns the operating system version.
func DetectSystemVer(sys System) (string, error) {
	switch sys {
	case Linux:
		ver, _, err := DetectDistroVer()

		return ver, err
	case MacOS:
		// /usr/bin/sw_vers
		stdout, stderr, err := executil.NewCommand("sw_vers", "-productVersion").
			OutputCombined()

		if err = executil.CheckStderr(stderr, err); err != nil {
			return "", err
		}
		return string(bytes.TrimSpace(stdout)), nil
	case FreeBSD:
		// The freebsd-version command appeared in FreeBSD 10.0 and above.
		stdout, err := executil.NewCommand("freebsd-version", "-k").
			OutputStdout()
		if err != nil {
			var stderr []byte

			stdout, stderr, err = executil.NewCommand("uname", "-r").
				OutputCombined()

			if err = executil.CheckStderr(stderr, err); err != nil {
				return "", err
			}
		}

		return string(bytes.TrimSpace(stdout)), nil
	case Windows:
		stdout, stderr, err := executil.NewCommand("systeminfo").
			OutputCombined()
		if err = executil.CheckStderr(stderr, err); err != nil {
			return "", err
		}

		// The output format is:
		// KEY: VALUE
		scanner := bufio.NewScanner(bytes.NewReader(stdout))

		for scanner.Scan() {
			ln := scanner.Bytes()
			lnSplit := bytes.Split(ln, signColon)
			key := bytes.ToLower(lnSplit[0])

			if bytes.Equal(key, osVersion) {
				return string(bytes.TrimSpace(lnSplit[1])), nil
			}
		}
		if err = scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("key %q not found at:\n\n%q", osVersion, stdout)

	default:
		panic("unreachable")
	}
}
