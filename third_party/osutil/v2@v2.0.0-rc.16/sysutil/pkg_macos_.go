// Copyright 2021 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// System: macOS
// Note: running Homebrew as root is extremely dangerous and no longer supported.

package sysutil

import (
	"io"

	"github.com/tredoe/osutil/v2"
	"github.com/tredoe/osutil/v2/executil"
)

const (
	fileBrew = "brew"
	pathBrew = "/usr/local/bin/brew"
)

// ManagerBrew is the interface to handle the macOS package manager.
type ManagerBrew struct {
	pathExec string
	cmd      *executil.Command
}

// NewManagerBrew returns the Homebrew package manager.
func NewManagerBrew() ManagerBrew {
	return ManagerBrew{
		pathExec: pathBrew,
		cmd: cmd.Command("", "").
			BadExitCodes([]int{1}),
	}
}

func (m ManagerBrew) setExecPath(p string) { m.pathExec = p }

func (m ManagerBrew) SetStdout(out io.Writer) { m.cmd.Stdout(out) }

func (m ManagerBrew) Cmd() *executil.Command { return m.cmd }

func (m ManagerBrew) ExecPath() string { return m.pathExec }

func (m ManagerBrew) PackageType() string { return Brew.String() }

func (m ManagerBrew) Install(name ...string) error {
	osutil.Log.Print(taskInstall)
	args := append([]string{"install"}, name...)

	_, err := m.cmd.Command(pathBrew, args...).Run()
	return err
}

func (m ManagerBrew) Remove(name ...string) error {
	osutil.Log.Print(taskRemove)
	args := append([]string{"uninstall"}, name...)

	_, err := m.cmd.Command(pathBrew, args...).Run()
	return err
}

func (m ManagerBrew) Purge(name ...string) error {
	osutil.Log.Print(taskPurge)
	return m.Remove(name...)
}

func (m ManagerBrew) Update() error {
	osutil.Log.Print(taskUpdate)
	_, err := m.cmd.Command(pathBrew, "update").Run()
	return err
}

func (m ManagerBrew) Upgrade() error {
	osutil.Log.Print(taskUpgrade)
	_, err := m.cmd.Command(pathBrew, "upgrade").Run()
	return err
}

//var msgWarning = []byte("Warning:")

func (m ManagerBrew) Clean() error {
	osutil.Log.Print(taskClean)
	_, err := m.cmd.Command(pathBrew, "autoremove").Run()
	if err != nil {
		return err
	}

	// TODO: check exit code
	//return executil.RunToStdButErr(msgWarning, nil, pathBrew, "cleanup")
	_, err = m.cmd.Command(pathBrew, "cleanup").Run()
	return err
}

func (m ManagerBrew) ImportKey(alias, keyUrl string) error {
	return ErrManagCmd
}

func (m ManagerBrew) ImportKeyFromServer(alias, keyServer, key string) error {
	return ErrManagCmd
}

func (m ManagerBrew) RemoveKey(alias string) error {
	return ErrManagCmd
}

func (m ManagerBrew) AddRepo(alias string, url ...string) error {
	osutil.Log.Print(taskAddRepo)
	_, err := m.cmd.Command(pathBrew, "tap", url[0]).Run()
	return err
}

func (m ManagerBrew) RemoveRepo(r string) error {
	osutil.Log.Print(taskRemoveRepo)
	_, err := m.cmd.Command(pathBrew, "untap", r).Run()
	return err
}
