// Copyright 2019 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package userutil

import (
	"bytes"
	"errors"
	"os/exec"
	"os/user"

	"github.com/tredoe/osutil/v2/sysutil"
)

var ErrNoSuperUser = errors.New("you MUST have superuser privileges")

var (
	newLineB = []byte{'\n'}
	emptyB   = []byte{}
)

// CheckSudo executes command 'sudo' to check that the user has permission.
func CheckSudo(sys sysutil.System) (err error) {
	if sys == sysutil.SystemUndefined {
		if sys, _, err = sysutil.SystemFromGOOS(); err != nil {
			return err
		}
	}

	switch sys {
	case sysutil.Linux, sysutil.FreeBSD, sysutil.MacOS:
		cmd := exec.Command("sudo", "true")

		return cmd.Run()
	case sysutil.Windows:
		return MustBeSuperUser(sysutil.Windows)
	}
	panic("unimplemented: " + sys.String())
}

// MustBeSuperUser checks if the current user is in the superusers group.
func MustBeSuperUser(sys sysutil.System) (err error) {
	if sys == sysutil.SystemUndefined {
		if sys, _, err = sysutil.SystemFromGOOS(); err != nil {
			return err
		}
	}

	usr, err := user.Current()
	if err != nil {
		return err
	}
	groups, err := usr.GroupIds()
	if err != nil {
		return err
	}

	findGroup := ""
	switch sys {
	case sysutil.Linux, sysutil.FreeBSD:
		findGroup = "root"
	case sysutil.MacOS:
		findGroup = "admin"
	case sysutil.Windows:
		findGroup = "Administrators"
	}

	for _, v := range groups {
		grp, err := user.LookupGroupId(v)
		if err != nil {
			return err
		}
		if grp.Name == findGroup {
			return nil
		}
	}
	return ErrNoSuperUser
}

// RealUser returns the original user at Unix systems.
func RealUser(sys sysutil.System) (string, error) {
	switch sys {
	default:
		panic("unimplemented: " + sys.String())

	case sysutil.Linux:
		username, err := exec.Command("logname").Output()
		if err != nil {
			return "", err
		}
		username = bytes.Replace(username, newLineB, emptyB, 1) // Remove the new line.

		return string(username), nil
	}
}
