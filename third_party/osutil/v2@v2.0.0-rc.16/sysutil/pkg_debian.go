// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Distro: Debian

package sysutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/tredoe/osutil/v2"
	"github.com/tredoe/osutil/v2/config/shconf"
	"github.com/tredoe/osutil/v2/executil"
	"github.com/tredoe/osutil/v2/fileutil"
)

// 'apt' is for the terminal and gives beautiful output.
// 'apt-get' and 'apt-cache' are for scripts and give stable, parsable output.

const (
	fileDeb = "apt-get"
	pathDeb = "/usr/bin/apt-get"

	pathGpg = "/usr/bin/gpg"
)

// ManagerDeb is the interface to handle the package manager of Linux systems based at Debian.
type ManagerDeb struct {
	pathExec string
	cmd      *executil.Command
}

// NewManagerDeb returns the Deb package manager.
func NewManagerDeb() ManagerDeb {
	return ManagerDeb{
		pathExec: pathDeb,
		cmd: cmd.Command("", "").
			AddEnv([]string{"DEBIAN_FRONTEND=noninteractive"}).
			BadExitCodes([]int{100}),
	}
}

func (m ManagerDeb) setExecPath(p string) { m.pathExec = p }

func (m ManagerDeb) SetStdout(out io.Writer) { m.cmd.Stdout(out) }

func (m ManagerDeb) Cmd() *executil.Command { return m.cmd }

func (m ManagerDeb) ExecPath() string { return m.pathExec }

func (m ManagerDeb) PackageType() string { return Deb.String() }

func (m ManagerDeb) Install(name ...string) error {
	osutil.Log.Print(taskInstall)
	args := append([]string{pathDeb, "install", "-y"}, name...)

	_, err := m.cmd.Command(sudo, args...).Run()
	return err
}

func (m ManagerDeb) Remove(name ...string) error {
	osutil.Log.Print(taskRemove)
	args := append([]string{pathDeb, "remove", "-y"}, name...)

	_, err := m.cmd.Command(sudo, args...).Run()
	return err
}

func (m ManagerDeb) Purge(name ...string) error {
	osutil.Log.Print(taskPurge)
	args := append([]string{pathDeb, "purge", "-y"}, name...)

	_, err := m.cmd.Command(sudo, args...).Run()
	return err
}

func (m ManagerDeb) Update() error {
	osutil.Log.Print(taskUpdate)
	_, err := m.cmd.Command(sudo, pathDeb, "update", "-qq").Run()
	return err
}

func (m ManagerDeb) Upgrade() error {
	osutil.Log.Print(taskUpgrade)
	_, err := m.cmd.Command(sudo, pathDeb, "upgrade", "-y").Run()
	return err
}

func (m ManagerDeb) Clean() error {
	osutil.Log.Print(taskClean)
	_, err := m.cmd.Command(sudo, pathDeb, "autoremove", "-y").Run()
	if err != nil {
		return err
	}

	_, err = m.cmd.Command(sudo, pathDeb, "clean").Run()
	return err
}

// https://www.linuxuprising.com/2021/01/apt-key-is-deprecated-how-to-add.html

func (m ManagerDeb) ImportKey(alias, keyUrl string) error {
	osutil.Log.Print(taskImportKey)
	if file := path.Base(keyUrl); !strings.Contains(file, ".") {
		return ErrKeyUrl
	}

	var keyFile bytes.Buffer

	err := fileutil.Dload(keyUrl, &keyFile)
	if err != nil {
		return err
	}

	stdout, stderr, err := m.cmd.Command(
		pathGpg, "--dearmor", keyFile.String(),
	).OutputCombined()
	if err = executil.CheckStderr(stderr, err); err != nil {
		return err
	}

	fi, err := os.Create(m.keyring(alias))
	if err != nil {
		return err
	}
	defer func() {
		if err2 := fi.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()
	if _, err = fi.Write(stdout); err != nil {
		return err
	}

	return fi.Sync()
}

func (m ManagerDeb) ImportKeyFromServer(alias, keyServer, key string) error {
	osutil.Log.Print(taskImportKeyFromServer)
	if keyServer == "" {
		keyServer = "hkp://keyserver.ubuntu.com:80"
	}

	stderr, err := m.cmd.Command(
		pathGpg,
		"--no-default-keyring",
		"--keyring", m.keyring(alias),
		"--keyserver", keyServer,
		"--recv-keys", key,
	).OutputStderr()

	err = executil.CheckStderr(stderr, err)
	return err
}

func (m ManagerDeb) RemoveKey(alias string) error {
	osutil.Log.Print(taskRemoveKey)
	return os.Remove(m.keyring(alias))
}

func (m ManagerDeb) AddRepo(alias string, url ...string) (err error) {
	osutil.Log.Print(taskAddRepo)
	distroName, err := distroCodeName()
	if err != nil {
		return err
	}

	err = fileutil.CreateFromString(
		m.repository(alias),
		fmt.Sprintf("deb [signed-by=%s] %s %s main",
			path.Dir(url[0]), distroName, m.keyring(alias),
		),
	)
	if err != nil {
		return err
	}

	return m.Update()
}

func (m ManagerDeb) RemoveRepo(alias string) error {
	osutil.Log.Print(taskRemoveRepo)
	err := os.Remove(m.keyring(alias))
	if err != nil {
		return err
	}
	if err = os.Remove(m.repository(alias)); err != nil {
		return err
	}

	return m.Update()
}

// == Utility
//

// distroCodeName returns the version like code name.
func distroCodeName() (string, error) {
	_, err := os.Stat("/etc/os-release")
	if os.IsNotExist(err) {
		return "", fmt.Errorf("%s", DistroUnknown)
	}

	cfg, err := shconf.ParseFile("/etc/os-release")
	if err != nil {
		return "", err
	}

	return cfg.Get("VERSION_CODENAME")
}

func (m ManagerDeb) keyring(alias string) string {
	return "/usr/share/keyrings/" + alias + "-archive-keyring.gpg"
}

func (m ManagerDeb) repository(alias string) string {
	return "/etc/apt/sources.list.d/" + alias + ".list"
}
