// Copyright 2021 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// System: Windows

// + Chocolatey
//
// https://community.chocolatey.org/packages
//
// + Winget
//
// https://github.com/microsoft/winget-cli
//
// Packages:
// https://github.com/microsoft/winget-pkgs/tree/master/manifests

package sysutil

import (
	"io"

	"github.com/tredoe/osutil/v2"
	"github.com/tredoe/osutil/v2/executil"
)

const (
	fileChoco = "choco"
	pathChoco = "choco"

	fileWinget = "winget"
	pathWinget = "winget"
)

// ManagerChoco is the interface to handle the package manager of Windows systems using Chocolatey.
type ManagerChoco struct {
	pathExec string
	cmd      *executil.Command
}

// NewManagerChoco returns the Chocolatey package manager.
func NewManagerChoco() ManagerChoco {
	return ManagerChoco{
		pathExec: pathChoco,
		// https://docs.chocolatey.org/en-us/choco/commands/install#exit-codes
		cmd: cmdWin.Command("", "").
			OkExitCodes([]int{0, 1641, 3010}),
	}
}

func (m ManagerChoco) setExecPath(p string) { m.pathExec = p }

func (m ManagerChoco) SetStdout(out io.Writer) { m.cmd.Stdout(out) }

func (m ManagerChoco) Cmd() *executil.Command { return m.cmd }

func (m ManagerChoco) ExecPath() string { return m.pathExec }

func (m ManagerChoco) PackageType() string { return Choco.String() }

func (m ManagerChoco) Install(name ...string) error {
	osutil.Log.Print(taskInstall)
	args := append([]string{"install"}, name...)
	args = append(args, "-y")

	_, err := m.cmd.Command(pathChoco, args...).Run()
	return err
}

func (m ManagerChoco) Remove(name ...string) error {
	osutil.Log.Print(taskRemove)
	args := append([]string{"uninstall"}, name...)
	args = append(args, "-y")

	_, err := m.cmd.Command(pathChoco, args...).Run()
	return err
}

func (m ManagerChoco) Purge(name ...string) error {
	osutil.Log.Print(taskPurge)
	return m.Remove(name...)
}

func (m ManagerChoco) Update() error {
	return ErrManagCmd
}

func (m ManagerChoco) Upgrade() error {
	osutil.Log.Print(taskUpgrade)
	_, err := m.cmd.Command(pathChoco, "upgrade", "all", "-y").Run()
	return err
}

func (m ManagerChoco) Clean() error {
	return ErrManagCmd
}

func (m ManagerChoco) ImportKey(alias, keyUrl string) error {
	return ErrManagCmd
}

func (m ManagerChoco) ImportKeyFromServer(alias, keyServer, key string) error {
	return ErrManagCmd
}

func (m ManagerChoco) RemoveKey(alias string) error {
	return ErrManagCmd
}

func (m ManagerChoco) AddRepo(alias string, url ...string) (err error) {
	return ErrManagCmd
}

func (m ManagerChoco) RemoveRepo(alias string) error {
	return ErrManagCmd
}

// * * *

// ManagerWinget is the interface to handle the package manager of Windows systems using winget.
type ManagerWinget struct {
	pathExec string
	cmd      *executil.Command
}

// NewManagerWinget returns the winget package manager.
func NewManagerWinget() ManagerWinget {
	return ManagerWinget{
		pathExec: pathChoco,
		cmd:      cmdWin.Command("", ""),
	}
}

func (m ManagerWinget) setExecPath(p string) { m.pathExec = p }

func (m ManagerWinget) SetStdout(out io.Writer) { m.cmd.Stdout(out) }

func (m ManagerWinget) Cmd() *executil.Command { return m.cmd }

func (m ManagerWinget) ExecPath() string { return m.pathExec }

func (m ManagerWinget) PackageType() string { return Choco.String() }

func (m ManagerWinget) Install(name ...string) error {
	osutil.Log.Print(taskInstall)
	var err error

	for _, v := range name {
		_, err = m.cmd.Command(pathChoco, "install", v).Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m ManagerWinget) Remove(name ...string) error {
	return ErrManagCmd
	/*args := append([]string{"uninstall"}, name...)
	args = append(args, "-y")

	_, err := m.cmd.Command(pathChoco, args...).Run()
	return err*/
}

func (m ManagerWinget) Purge(name ...string) error {
	osutil.Log.Print(taskPurge)
	return m.Remove(name...)
}

func (m ManagerWinget) Update() error {
	return ErrManagCmd
}

func (m ManagerWinget) Upgrade() error {
	return ErrManagCmd
}

func (m ManagerWinget) Clean() error {
	return ErrManagCmd
}

func (m ManagerWinget) ImportKey(alias, keyUrl string) error {
	return ErrManagCmd
}

func (m ManagerWinget) ImportKeyFromServer(alias, keyServer, key string) error {
	return ErrManagCmd
}

func (m ManagerWinget) RemoveKey(alias string) error {
	return ErrManagCmd
}

func (m ManagerWinget) AddRepo(alias string, url ...string) (err error) {
	return ErrManagCmd
}

func (m ManagerWinget) RemoveRepo(alias string) error {
	return ErrManagCmd
}
