// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysutil

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tredoe/osutil/v2/executil"
)

// sudo is the path by default at Linux systems.
const sudo = "/usr/bin/sudo"

const (
	taskInstall             = "Installing ..."
	taskRemove              = "Removing ..."
	taskPurge               = "Purging ..."
	taskUpdate              = "Updating repositories ..."
	taskUpgrade             = "Upgrading packages ..."
	taskClean               = "Cleaning ..."
	taskImportKey           = "Importing key ..."
	taskImportKeyFromServer = "Importing key from server ..."
	taskRemoveKey           = "Removing key ..."
	taskAddRepo             = "Adding repository ..."
	taskRemoveRepo          = "Removing repository ..."
)

var (
	cmd = executil.NewCommand("", "").
		Env(append([]string{"LANG=C"}, os.Environ()...))

	cmdWin = executil.NewCommand("", "").
		Env(os.Environ())
)

// PkgManager is the common interface to handle different package systems.
type PkgManager interface {
	// setExecPath sets the executable path.
	setExecPath(p string)

	// SetStdout sets the standard out for the commands of the package manager.
	SetStdout(out io.Writer)

	// Cmd returns the command configured for the package manager.
	Cmd() *executil.Command

	// ExecPath returns the executable path.
	ExecPath() string

	// PackageType returns the package type.
	PackageType() string

	// Install installs packages.
	Install(name ...string) error

	// Remove removes packages.
	Remove(name ...string) error

	// Purge removes packages and its configuration files (if the package system does it).
	Purge(name ...string) error

	// Update resynchronizes the package index files from their sources.
	Update() error

	// Upgrade upgrades all the packages on the system.
	Upgrade() error

	// Clean erases both packages downloaded and orphaned dependencies.
	Clean() error

	// ImportKey downloads the OpenPGP key and add it to the system.
	ImportKey(alias, keyUrl string) error

	// ImportKeyFromServer imports OpenPGP key directly from a keyserver.
	// Whether 'keyServer' is empty, then it uses a value by default.
	ImportKeyFromServer(alias, keyServer, key string) error

	// RemoveKey removes the OpenPGP key.
	RemoveKey(alias string) error

	// AddRepo adds a repository.
	AddRepo(alias string, url ...string) error

	// RemoveRepo removes a repository.
	RemoveRepo(string) error
}

// PackageType represents a package management system.
type PackageType int8

const (
	// Linux
	Deb PackageType = iota + 1
	Dnf
	Ebuild
	Pacman
	Rpm
	Yum
	Zypp

	// BSD
	Brew
	Pkg

	// Windows
	Choco
	Winget
)

func (pkg PackageType) String() string {
	switch pkg {
	// Linux
	case Deb:
		return "Deb"
	case Dnf:
		return "DNF"
	case Ebuild:
		return "Ebuild"
	case Pacman:
		return "Pacman"
	case Rpm:
		return "RPM"
	case Yum:
		return "YUM"
	case Zypp:
		return "ZYpp"

	// BSD
	case Brew:
		return "brew"
	case Pkg:
		return "pkg"

	case Choco:
		return "Chocolatey"
	case Winget:
		return "winget"
	}
	panic("unreachable")
}

// NewPkgTypeFromStr returns a package management system from the string.
func NewPkgTypeFromStr(s string) (PackageType, error) {
	switch strings.ToLower(s) {
	case fileDeb:
		return Deb, nil
	case fileDnf:
		return Dnf, nil
	case fileEbuild:
		return Ebuild, nil
	case filePacman:
		return Pacman, nil
	case fileRpm:
		return Rpm, nil
	case fileYum:
		return Yum, nil
	case fileZypp:
		return Zypp, nil

	case fileBrew:
		return Brew, nil
	case filePkg:
		return Pkg, nil

	case fileChoco:
		return Choco, nil
	case fileWinget:
		return Winget, nil

	default:
		return -1, pkgTypeError(s)
	}
}

// * * *

// NewPkgManagFromType returns the interface to handle the package manager.
func NewPkgManagFromType(pkg PackageType) PkgManager {
	switch pkg {
	// Linux
	case Deb:
		return NewManagerDeb()
	case Dnf:
		return NewManagerDnf()
	case Ebuild:
		return NewManagerEbuild()
	case Pacman:
		return NewManagerPacman()
	case Rpm:
		return NewManagerRpm()
	case Yum:
		return NewManagerYum()
	case Zypp:
		return NewManagerZypp()

	// BSD
	case Brew:
		return NewManagerBrew()
	case Pkg:
		return NewManagerPkg()

	// Windows
	case Choco:
		return NewManagerChoco()
	case Winget:
		return NewManagerWinget()
	}
	panic("unreachable")
}

// * * *

// NewPkgManagFromSystem returns the package manager used by a system.
func NewPkgManagFromSystem(sys System, dis Distro) (PkgManager, error) {
	switch sys {
	case Linux:
		return NewPkgManagFromDistro(dis)
		/*pkg, err := NewPkgManagFromDistro(dis)
		if err != nil {
			return ManagerVoid{}, err
		}

		if len(pkg) == 1 {
			return pkg[0], nil
		}
		for _, v := range pkg {
			_, err := exec.LookPath(v.ExecPath())
			if err == nil {
				return v, nil
			}
		}
		return ManagerVoid{}, pkgManagNotfoundError{dis}*/

	case MacOS:
		return NewManagerBrew(), nil
	case FreeBSD:
		return NewManagerPkg(), nil
	case Windows:
		// TODO: in the future, to use winget
		return NewManagerChoco(), nil

	default:
		panic("unimplemented")
	}
}

// NewPkgManagFromDistro returns the package manager used by a Linux distro.
func NewPkgManagFromDistro(dis Distro) (PkgManager, error) {
	switch dis {
	case Debian, Ubuntu:
		return NewManagerDeb(), nil

	case OpenSUSE:
		return NewManagerZypp(), nil

	case Arch, Manjaro:
		return NewManagerPacman(), nil

	// DNF is the default package manager of Fedora 22, CentOS8, and RHEL8.
	case CentOS, Fedora:
		verStr, err := DetectSystemVer(Linux)
		if err != nil {
			return ManagerVoid{}, err
		}
		ver, err := strconv.Atoi(verStr)
		if err != nil {
			return ManagerVoid{}, err
		}

		useDnf := true
		switch dis {
		case CentOS:
			if ver < 8 {
				useDnf = false
			}
		case Fedora:
			if ver < 22 {
				useDnf = false
			}
		}

		if useDnf {
			return NewManagerDnf(), nil
		}
		return NewManagerYum(), nil

	default:
		panic("unimplemented")
	}
}

// * * *

// execPkgLinux is a list of package managers executables for Linux.
var execPkgLinux = []string{
	fileDeb,
	fileDnf,
	fileYum,
	fileZypp,
	filePacman,
	fileEbuild,
	fileRpm,
}

// execPkgFreebsd is a list of package managers executables for FreeBSD.
var execPkgFreebsd = []string{
	fileBrew,
}

// execPkgMacos is a list of package managers executables for MacOS.
var execPkgMacos = []string{
	filePkg,
}

// execPkgWindows is a list of package managers executables for Windows.
var execPkgWindows = []string{
	fileChoco,
	fileWinget,
}

// DetectPkgManag tries to get the package manager used in the system, looking for
// executables at directories in $PATH.
func DetectPkgManag(sys System) (PkgManager, error) {
	var execPkg []string
	switch sys {
	case Linux:
		execPkg = execPkgLinux
	case FreeBSD:
		execPkg = execPkgFreebsd
	case MacOS:
		execPkg = execPkgMacos
	case Windows:
		execPkg = execPkgWindows

	default:
		panic("unimplemented: " + sys.String())
	}

	for _, p := range execPkg {
		pathExec, err := exec.LookPath(p)
		if err == nil {
			pkg, err := NewPkgTypeFromStr(p)
			if err != nil {
				return ManagerVoid{}, err
			}
			mng := NewPkgManagFromType(pkg)

			if mng.ExecPath() != pathExec {
				mng.setExecPath(pathExec)
			}

			return mng, nil
		}
	}

	return ManagerVoid{}, fmt.Errorf("package manager not found in $PATH")
}
