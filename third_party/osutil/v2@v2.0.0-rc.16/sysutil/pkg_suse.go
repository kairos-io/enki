// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Distro: SUSE

package sysutil

import (
	"io"

	"github.com/tredoe/osutil/v2"
	"github.com/tredoe/osutil/v2/executil"
)

const fileZypp = "zypper"

var pathZypp = "/usr/bin/zypper"

// ManagerZypp is the interface to handle the package manager of Linux systems based at SUSE.
type ManagerZypp struct {
	pathExec string
	cmd      *executil.Command
}

// NewManagerZypp returns the Zypp package manager.
func NewManagerZypp() ManagerZypp {
	return ManagerZypp{
		pathExec: pathZypp,
		cmd: cmd.Command("", "").
			// https://www.unix.com/man-page/suse/8/zypper/
			BadExitCodes([]int{1, 2, 3, 4, 5, 104}),
	}
}

func (m ManagerZypp) setExecPath(p string) { m.pathExec = p }

func (m ManagerZypp) SetStdout(out io.Writer) { m.cmd.Stdout(out) }

func (m ManagerZypp) Cmd() *executil.Command { return m.cmd }

func (m ManagerZypp) ExecPath() string { return m.pathExec }

func (m ManagerZypp) PackageType() string { return Zypp.String() }

func (m ManagerZypp) Install(name ...string) error {
	osutil.Log.Print(taskInstall)
	args := append(
		[]string{
			pathZypp,
			"--non-interactive",
			"install", "--auto-agree-with-licenses", "-y",
		}, name...)

	_, err := m.cmd.Command(sudo, args...).Run()
	return err
}

func (m ManagerZypp) Remove(name ...string) error {
	osutil.Log.Print(taskRemove)
	args := append([]string{pathZypp, "remove", "-y"}, name...)

	_, err := m.cmd.Command(sudo, args...).Run()
	return err
}

func (m ManagerZypp) Purge(name ...string) error {
	osutil.Log.Print(taskPurge)
	return m.Remove(name...)
}

func (m ManagerZypp) Update() error {
	osutil.Log.Print(taskUpdate)
	_, err := m.cmd.Command(sudo, pathZypp, "refresh").Run()
	return err
}

func (m ManagerZypp) Upgrade() error {
	osutil.Log.Print(taskUpgrade)
	_, err := m.cmd.Command(
		sudo, pathZypp, "up", "--auto-agree-with-licenses", "-y",
	).Run()
	return err
}

func (m ManagerZypp) Clean() error {
	osutil.Log.Print(taskClean)
	_, err := m.cmd.Command(sudo, pathZypp, "clean").Run()
	return err
}

// https://opensuse-guide.org/repositories.php

func (m ManagerZypp) ImportKey(alias, keyUrl string) error {
	return ErrManagCmd
}

func (m ManagerZypp) ImportKeyFromServer(alias, keyServer, key string) error {
	return ErrManagCmd
}

func (m ManagerZypp) RemoveKey(alias string) error {
	return ErrManagCmd
}

func (m ManagerZypp) AddRepo(alias string, url ...string) error {
	osutil.Log.Print(taskAddRepo)
	_, err := m.cmd.Command(sudo, pathZypp, "addrepo", "-f", url[0], alias).Run()
	if err != nil {
		return err
	}

	return m.Update()
}

func (m ManagerZypp) RemoveRepo(r string) error {
	osutil.Log.Print(taskRemoveRepo)
	if _, err := m.cmd.Command(sudo, pathZypp, "removerepo", r).Run(); err != nil {
		return err
	}

	return m.Update()
}
