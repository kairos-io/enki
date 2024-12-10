// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Distro: Arch

package sysutil

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tredoe/osutil/v2"
	"github.com/tredoe/osutil/v2/edi"
	"github.com/tredoe/osutil/v2/executil"
)

const (
	filePacman = "pacman"
	pathPacman = "/usr/bin/pacman"
)

// ManagerPacman is the interface to handle the package manager of Linux systems based at Arch.
type ManagerPacman struct {
	pathExec string
	cmd      *executil.Command
}

// NewManagerPacman returns the Pacman package manager.
func NewManagerPacman() ManagerPacman {
	return ManagerPacman{
		pathExec: pathPacman,
		cmd: cmd.Command("", "").
			// https://wiki.archlinux.org/title/Talk:Pacman#Exit_codes
			BadExitCodes([]int{1}),
	}
}

func (m ManagerPacman) setExecPath(p string) { m.pathExec = p }

func (m ManagerPacman) SetStdout(out io.Writer) { m.cmd.Stdout(out) }

func (m ManagerPacman) Cmd() *executil.Command { return m.cmd }

func (m ManagerPacman) ExecPath() string { return m.pathExec }

func (m ManagerPacman) PackageType() string { return Pacman.String() }

func (m ManagerPacman) Install(name ...string) error {
	osutil.Log.Print(taskInstall)
	args := append([]string{"-S", "--needed", "--noprogressbar"}, name...)

	_, err := m.cmd.Command(pathPacman, args...).Run()
	return err
}

func (m ManagerPacman) Remove(name ...string) error {
	osutil.Log.Print(taskRemove)
	args := append([]string{"-Rs"}, name...)

	_, err := m.cmd.Command(pathPacman, args...).Run()
	return err
}

func (m ManagerPacman) Purge(name ...string) error {
	osutil.Log.Print(taskPurge)
	args := append([]string{"-Rsn"}, name...)

	_, err := m.cmd.Command(pathPacman, args...).Run()
	return err
}

func (m ManagerPacman) Update() error {
	osutil.Log.Print(taskUpdate)
	_, err := m.cmd.Command(pathPacman, "-Syu", "--needed", "--noprogressbar").Run()
	return err
}

func (m ManagerPacman) Upgrade() error {
	osutil.Log.Print(taskUpgrade)
	_, err := m.cmd.Command(pathPacman, "-Syu").Run()
	return err
}

func (m ManagerPacman) Clean() error {
	osutil.Log.Print(taskClean)
	_, err := m.cmd.Command("/usr/bin/paccache", "-r").Run()
	return err
}

// https://arcolinux.com/how-to-add-the-third-party-repo-to-pacman-conf/

func (m ManagerPacman) ImportKey(alias, keyUrl string) error {
	return ErrManagCmd
}

func (m ManagerPacman) ImportKeyFromServer(alias, keyServer, key string) error {
	return ErrManagCmd
}

func (m ManagerPacman) RemoveKey(alias string) error {
	return ErrManagCmd
}

func (m ManagerPacman) AddRepo(alias string, url ...string) error {
	osutil.Log.Print(taskAddRepo)
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "[%s]\n", alias)
	for _, v := range url {
		fmt.Fprintf(&buf, "Server = %s\n", v)
	}

	ed, err := edi.NewEdit("/etc/pacman.conf", nil)
	if err != nil {
		return err
	}

	if err = ed.Append(buf.Bytes()); err != nil {
		return err
	}

	return m.Update()
}

func (m ManagerPacman) RemoveRepo(r string) error {
	osutil.Log.Print(taskRemoveRepo)
	// TODO
	panic("unimplemented")
}
