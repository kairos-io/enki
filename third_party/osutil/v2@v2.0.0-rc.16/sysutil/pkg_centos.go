// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// TODO: to use
//
// dnf config-manager -> install: dnf-plugins-core
// yum-config-manager -> install: yum-utils

package sysutil

import (
	"io"
	"os"

	"github.com/tredoe/osutil/v2"
	"github.com/tredoe/osutil/v2/executil"
)

const (
	fileDnf = "dnf" // Preferable to YUM
	pathDnf = "/usr/bin/dnf"

	fileYum    = "yum"
	pathYum    = "/usr/bin/yum"
	pathYumCfg = "/usr/bin/yum-config-manager"

	// RPM is used to install/uninstall local packages.
	fileRpm = "rpm"
	pathRpm = "/usr/bin/rpm"
)

// ManagerDnf is the interface to handle the package manager DNG of Linux systems
// based at Red Hat.
type ManagerDnf struct {
	pathExec string
	cmd      *executil.Command

	rpm ManagerRpm
}

// NewManagerDnf returns the DNF package manager.
func NewManagerDnf() ManagerDnf {
	return ManagerDnf{
		pathExec: pathDnf,
		cmd: cmd.Command("", "").
			// https://dnf.readthedocs.io/en/latest/command_ref.html
			BadExitCodes([]int{1, 3, 200}),
		rpm: NewManagerRpm(),
	}
}

func (m ManagerDnf) setExecPath(p string) { m.pathExec = p }

func (m ManagerDnf) SetStdout(out io.Writer) { m.cmd.Stdout(out) }

func (m ManagerDnf) Cmd() *executil.Command { return m.cmd }

func (m ManagerDnf) ExecPath() string { return m.pathExec }

func (m ManagerDnf) PackageType() string { return Dnf.String() }

func (m ManagerDnf) Install(name ...string) error {
	args := append([]string{pathDnf, "install", "-y"}, name...)

	_, err := m.cmd.Command(sudo, args...).Run()
	return err
}

func (m ManagerDnf) Remove(name ...string) error {
	args := append([]string{pathDnf, "remove", "-y"}, name...)

	_, err := m.cmd.Command(sudo, args...).Run()
	return err
}

func (m ManagerDnf) Purge(name ...string) error {
	return m.Remove(name...)
}

func (m ManagerDnf) Update() error {
	// check-update does not update else it checks the updating.
	return nil

	// check-update returns exit value of 100 if there are packages available for an update.
	// Also returns a list of the packages to be updated in list format.
	// Returns 0 if no packages are available for update.
	// Returns 1 if an error occurred.
	/*err := m.cmd.Command(sudo, pathDnf, "check-update")
	if err != nil {
		// Check the exit code
	}
	return err*/
}

func (m ManagerDnf) Upgrade() error {
	_, err := m.cmd.Command(sudo, pathDnf, "update", "-y").Run()
	return err
}

func (m ManagerDnf) Clean() error {
	_, err := m.cmd.Command(sudo, pathDnf, "autoremove", "-y").Run()
	if err != nil {
		return err
	}
	_, err = m.cmd.Command(sudo, pathDnf, "clean", "all").Run()
	return err
}

func (m ManagerDnf) ImportKey(alias, keyUrl string) error {
	return m.rpm.ImportKey("", keyUrl)
}

func (m ManagerDnf) ImportKeyFromServer(alias, keyServer, key string) error {
	return ErrManagCmd
}

func (m ManagerDnf) RemoveKey(alias string) error {
	return ErrManagCmd
}

// https://docs.fedoraproject.org/en-US/quick-docs/adding-or-removing-software-repositories-in-fedora/

func (m ManagerDnf) AddRepo(alias string, url ...string) error {
	/*pathRepo := m.repository(alias)

	err := fileutil.CreateFromString(pathRepo, url[0]+"\n")
	if err != nil {
		return err
	}*/

	stderr, err := m.cmd.Command(
		pathDnf, "config-manager", "--add-repo", url[0],
	).OutputStderr()

	return executil.CheckStderr(stderr, err)
}

func (m ManagerDnf) RemoveRepo(alias string) error {
	return os.Remove(m.repository(alias))
}

// * * *

// ManagerYum is the interface to handle the package manager YUM of Linux systems
// based at Red Hat.
type ManagerYum struct {
	pathExec string
	cmd      *executil.Command

	rpm ManagerRpm
}

// NewManagerYum returns the YUM package manager.
func NewManagerYum() ManagerYum {
	return ManagerYum{
		pathExec: pathYum,
		cmd: cmd.Command("", "").
			BadExitCodes([]int{1, 2, 3, 16}),
		rpm: NewManagerRpm(),
	}
}

func (m ManagerYum) setExecPath(p string) { m.pathExec = p }

func (m ManagerYum) SetStdout(out io.Writer) { m.cmd.Stdout(out) }

func (m ManagerYum) Cmd() *executil.Command { return m.cmd }

func (m ManagerYum) ExecPath() string { return m.pathExec }

func (m ManagerYum) PackageType() string { return Yum.String() }

func (m ManagerYum) Install(name ...string) error {
	osutil.Log.Print(taskInstall)
	args := append([]string{pathYum, "install", "-y"}, name...)

	_, err := m.cmd.Command(sudo, args...).Run()
	return err
}

func (m ManagerYum) Remove(name ...string) error {
	osutil.Log.Print(taskRemove)
	args := append([]string{pathYum, "remove", "-y"}, name...)

	_, err := m.cmd.Command(sudo, args...).Run()
	return err
}

func (m ManagerYum) Purge(name ...string) error {
	osutil.Log.Print(taskPurge)
	return m.Remove(name...)
}

func (m ManagerYum) Update() error {
	// check-update does not update else it checks the updating.
	return ErrManagCmd
}

func (m ManagerYum) Upgrade() error {
	osutil.Log.Print(taskUpgrade)
	_, err := m.cmd.Command(sudo, pathYum, "update", "-y").Run()
	return err
}

func (m ManagerYum) Clean() error {
	osutil.Log.Print(taskClean)
	_, err := m.cmd.Command(sudo, pathYum, "clean", "packages").Run()
	return err
}

func (m ManagerYum) ImportKey(alias, keyUrl string) error {
	osutil.Log.Print(taskImportKey)
	return m.rpm.ImportKey("", keyUrl)
}

func (m ManagerYum) ImportKeyFromServer(alias, keyServer, key string) error {
	return ErrManagCmd
}

func (m ManagerYum) RemoveKey(alias string) error {
	return ErrManagCmd
}

// https://docs.fedoraproject.org/en-US/Fedora/16/html/System_Administrators_Guide/sec-Managing_Yum_Repositories.html

func (m ManagerYum) AddRepo(alias string, url ...string) error {
	osutil.Log.Print(taskAddRepo)
	stderr, err := m.cmd.Command(
		pathYumCfg, "--add-repo", url[0],
	).OutputStderr()

	return executil.CheckStderr(stderr, err)
}

func (m ManagerYum) RemoveRepo(alias string) error {
	osutil.Log.Print(taskRemoveRepo)
	return os.Remove(m.repository(alias))
}

// * * *

// ManagerRpm is the interface to handle the package manager RPM of Linux systems
// based at Red Hat.
type ManagerRpm struct {
	pathExec string
	cmd      *executil.Command
}

// NewManagerRpm returns the RPM package manager.
func NewManagerRpm() ManagerRpm {
	return ManagerRpm{
		pathExec: pathRpm,
		cmd:      cmd.Command("", ""),
		//BadExitCodes([]int{}),
	}
}

func (m ManagerRpm) setExecPath(p string) { m.pathExec = p }

func (m ManagerRpm) SetStdout(out io.Writer) { m.cmd.Stdout(out) }

func (m ManagerRpm) Cmd() *executil.Command { return m.cmd }

func (m ManagerRpm) ExecPath() string { return m.pathExec }

func (m ManagerRpm) PackageType() string { return Rpm.String() }

func (m ManagerRpm) Install(name ...string) error {
	osutil.Log.Print(taskInstall)
	args := append([]string{"-i"}, name...)

	_, err := m.cmd.Command(pathRpm, args...).Run()
	return err
}

func (m ManagerRpm) Remove(name ...string) error {
	osutil.Log.Print(taskRemove)
	args := append([]string{"-e"}, name...)

	_, err := m.cmd.Command(pathRpm, args...).Run()
	return err
}

func (m ManagerRpm) Purge(name ...string) error {
	osutil.Log.Print(taskPurge)
	return m.Remove(name...)
}

func (m ManagerRpm) Update() error {
	return ErrManagCmd
}

func (m ManagerRpm) Upgrade() error {
	return ErrManagCmd
}

func (m ManagerRpm) Clean() error {
	return ErrManagCmd
}

func (m ManagerRpm) ImportKey(alias, keyUrl string) error {
	osutil.Log.Print(taskImportKey)
	stderr, err := m.cmd.Command(pathRpm, "--import", keyUrl).OutputStderr()

	err = executil.CheckStderr(stderr, err)
	return err
}

func (m ManagerRpm) ImportKeyFromServer(alias, keyServer, key string) error {
	return ErrManagCmd
}

func (m ManagerRpm) RemoveKey(alias string) error {
	return ErrManagCmd
}

func (m ManagerRpm) AddRepo(alias string, url ...string) error {
	return ErrManagCmd
}

func (m ManagerRpm) RemoveRepo(r string) error {
	return ErrManagCmd
}

// == Utility
//

func (m ManagerDnf) repository(alias string) string {
	return "/etc/yum.repos.d/" + alias + ".repo"
}

func (m ManagerYum) repository(alias string) string {
	return "/etc/yum.repos.d/" + alias + ".repo"
}
