// Copyright 2021 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package service handles the services at systems Linux, FreeBSD, macOS and Windows.
//
// The information messages are written through 'logShell' configured at 'SetupLogger()'.
package service

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tredoe/osutil/v2"
	"github.com/tredoe/osutil/v2/executil"
	"github.com/tredoe/osutil/v2/sysutil"
	"github.com/tredoe/osutil/v2/userutil"
)

// timeKillServ is the time used to wait before of kill a service.
const timeKillServ = 90 * time.Second

var (
	excmd    = executil.NewCommand("", "").TimeKill(timeKillServ).Env([]string{"LANG=C"})
	excmdWin = executil.NewCommand("", "").TimeKill(timeKillServ)
)

// ColumnWin represents the column name at Windows where to find the service name.
type ColumnWin uint8

const (
	ColWinName        = iota // Column 'Name', by default.
	ColWinDisplayname        // Column 'Displayname'
)

func (c ColumnWin) String() string {
	switch c {
	case ColWinName:
		return "Name"
	case ColWinDisplayname:
		return "DisplayName"

	default:
		panic("unimplemented")
	}
}

// Service represents a service.
type Service struct {
	name string
	path string
	sys  sysutil.System
	dis  sysutil.Distro

	// Custom commands to both start and stop.
	start *executil.Command
	stop  *executil.Command
}

// Name returns the service name.
func (s *Service) Name() string { return s.name }

// * * *

// NewService creates a new service with the given name.
func NewService(
	sys sysutil.System, dis sysutil.Distro, name string,
) (*Service, error) {
	if name == "" {
		return nil, ErrNoService
	}
	err := userutil.CheckSudo(sys)
	if err != nil {
		return nil, err
	}

	return &Service{
		name: name,
		sys:  sys,
		dis:  dis,
	}, nil
}

// NewCustomService creates a new service with the custom commands.
func NewCustomService(
	sys sysutil.System,
	cmdStart string, argsStart []string,
	cmdStop string, argsStop []string,
) *Service {
	s := new(Service)

	if cmdStart != "" {
		s.start = executil.NewCommand(cmdStart, argsStart...).TimeKill(timeKillServ)

		if s.sys != sysutil.Windows {
			s.start.Env([]string{"LANG=C"})
		}
	}
	if cmdStop != "" {
		s.stop = executil.NewCommand(cmdStop, argsStop...).TimeKill(timeKillServ)

		if s.sys != sysutil.Windows {
			s.stop.Env([]string{"LANG=C"})
		}
	}

	return s
}

// LookupService returns the service that matchs a pattern using the syntax of Match.
// If 'exclude' is set, it discards the names that contains it.
// When a service name is not found, returns error 'ServNotFoundError'.
func LookupService(
	sys sysutil.System,
	dis sysutil.Distro,
	pattern, exclude string,
	column ColumnWin,
) (*Service, error) {
	err := userutil.CheckSudo(sys)
	if err != nil {
		return nil, err
	}

	switch sys {
	case sysutil.Linux:
		return lookupServiceLinux(dis, pattern, exclude)
	case sysutil.MacOS:
		return lookupServiceMacos(pattern, exclude)
	case sysutil.Windows:
		return lookupServiceWindows(pattern, exclude, column)
	case sysutil.FreeBSD:
		return lookupServiceFreebsd(pattern, exclude)
	}

	panic("unreachable")
}

func lookupServiceFreebsd(pattern, exclude string) (*Service, error) {
	dirs := []string{"/usr/local/etc/rc.d/"}

	for _, dir := range dirs {
		files, err := filepath.Glob(dir + pattern)
		if err != nil {
			return nil, err
		}
		if files == nil {
			continue
		}

		for i := len(files) - 1; i >= 0; i-- {
			pathService := files[i]

			if exclude != "" && strings.Contains(pathService, exclude) {
				continue
			}
			serviceName := filepath.Base(pathService)

			return &Service{
				name: serviceName,
				path: pathService,
				sys:  sysutil.FreeBSD,
			}, nil
		}
	}

	return nil, ServNotFoundError{pattern: pattern, dirs: dirs}
}

func lookupServiceLinux(dis sysutil.Distro, pattern, exclude string) (*Service, error) {
	var dirs []string

	switch dis {
	case sysutil.Debian, sysutil.Ubuntu:
		dirs = []string{
			"/lib/systemd/system/",
		}
	case sysutil.CentOS, sysutil.Fedora:
		dirs = []string{
			"/lib/systemd/system/",
			"/etc/init.d/",
		}
	case sysutil.OpenSUSE:
		dirs = []string{
			"/usr/lib/systemd/system/",
			"/etc/init.d/",
		}

	default:
		dirs = []string{
			"/lib/systemd/system/",
			"/usr/lib/systemd/system/",
			"/etc/init.d/",
		}
	}

	for _, dir := range dirs {
		files, err := filepath.Glob(dir + pattern)
		if err != nil {
			return nil, err
		}
		if files == nil {
			continue
		}

		for i := len(files) - 1; i >= 0; i-- {
			pathService := files[i]

			if exclude != "" && strings.Contains(pathService, exclude) {
				continue
			}
			if strings.Contains(pathService, "@") {
				continue
			}

			//fmt.Println("SERVICE:", pathService) // DEBUG
			serviceName := filepath.Base(pathService)

			// The file could be finished with an extension like '.service' or '.target',
			// and with several dots like 'firebird3.0.service'
			if strings.Contains(serviceName, ".") {
				idLastDot := strings.LastIndex(serviceName, ".")
				part1 := serviceName[:idLastDot]
				part2 := serviceName[idLastDot+1:] // discard dot

				if len(part2) > 2 {
					serviceName = part1
				}
			}

			return &Service{
				name: serviceName,
				path: pathService,
				sys:  sysutil.Linux,
				dis:  dis,
			}, nil
		}
	}

	return nil, ServNotFoundError{pattern: pattern, dirs: dirs}
}

// Handle services installed through HomeBrew:
//
// + brew services list
// + ls -l ~/Library/LaunchAgents/

func lookupServiceMacos(pattern, exclude string) (*Service, error) {
	dirs := []string{ // Installed by
		fmt.Sprintf("/Library/LaunchDaemons/*.%s*", pattern),   // binary installer
		fmt.Sprintf("/usr/local/Cellar/%s/*/*.plist", pattern), // HomeBrew
	}

	for iDir, dir := range dirs {
		files, err := filepath.Glob(dir)
		if err != nil {
			return nil, err
		}
		if files == nil {
			continue
		}

		for i := 0; i < len(files); i++ {
			pathService := files[i]

			serviceName := ""
			switch iDir {
			case 0:
				serviceName = strings.SplitAfter(pathService, "/Library/LaunchDaemons/")[1]
				//if split := strings.SplitN(serviceName, ".plist", 2); len(split) != 1 {
				//	serviceName = split[0]
				//}
				serviceName = strings.SplitN(serviceName, ".plist", 2)[0]
			case 1:
				serviceName = strings.SplitAfter(pathService, "/usr/local/Cellar/")[1]
				serviceName = strings.SplitN(serviceName, "/", 2)[0]
			}

			if exclude != "" && strings.Contains(serviceName, exclude) {
				continue
			}

			return &Service{
				name: serviceName,
				path: pathService,
				sys:  sysutil.MacOS,
			}, nil
		}
	}

	return nil, ServNotFoundError{pattern: pattern, dirs: dirs}
}

func lookupServiceWindows(pattern, exclude string, column ColumnWin) (*Service, error) {
	var out bytes.Buffer
	cmd := exec.Command(
		"powershell.exe",
		fmt.Sprintf("Get-Service -%s %q | Select-Object Name", column, pattern),
	)
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	rd := bytes.NewReader(out.Bytes())
	sc := bufio.NewScanner(rd)
	line := ""

	for sc.Scan() {
		line = strings.TrimSpace(sc.Text())

		if line == "" {
			continue
		}
		if line[0] == '-' || strings.HasPrefix(line, "Name") {
			line = ""
			continue
		}

		break
	}
	if err = sc.Err(); err != nil {
		return nil, err
	}
	if line == "" {
		return nil, ServNotFoundError{pattern: pattern}
	}

	return &Service{
		name: line,
		sys:  sysutil.Windows,
	}, nil
}

// * * *

// Start starts the service.
func (srv Service) Start() error {
	osutil.Log.Print("Starting service ...")

	if srv.start != nil {
		stderr, err := srv.start.OutputStderr()
		return executil.CheckStderr(stderr, err)
	}

	switch srv.sys {
	case sysutil.Linux:
		stderr, err := excmd.Command(
			"sudo", "systemctl", "start", srv.name,
		).OutputStderr()

		if err = executil.CheckStderr(stderr, err); err != nil {
			return err
		}

	case sysutil.FreeBSD:
		stderr, err := excmd.Command(
			"sudo", "service", srv.name, "start",
		).OutputStderr()

		if err = executil.CheckStderr(stderr, err); err != nil {
			return err
		}

	case sysutil.MacOS:
		stderr, err := excmd.Command(
			"sudo", "launchctl", "load", "-F", srv.name,
		).OutputStderr()

		if err != nil {
			if !bytes.Contains(stderr, []byte("service already loaded")) {
				return fmt.Errorf("%s", stderr)
			}
			//logs.Debug.Printf("%s\n%s", stderr, err)
		}
		if err = executil.CheckStderr(stderr, err); err != nil {
			return err
		}

	case sysutil.Windows:
		stderr, err := excmdWin.Command(
			"net", "start", srv.name,
		).OutputStderr()

		if err != nil {
			if !bytes.Contains(stderr, []byte("already been started")) {
				return fmt.Errorf("%s", stderr)
			}
			//logs.Debug.Printf("%s\n%s", stderr, err)
		}
		if err = executil.CheckStderr(stderr, err); err != nil {
			return err
		}

	default:
		panic("unimplemented: " + srv.sys.String())
	}

	return nil
}

// Stop stops the service.
func (srv Service) Stop() error {
	osutil.Log.Print("Stopping service ...")

	if srv.stop != nil {
		stderr, err := srv.stop.OutputStderr()
		return executil.CheckStderr(stderr, err)
	}

	switch srv.sys {
	case sysutil.Linux:
		stdout, stderr, err := excmd.Command(
			"systemctl", "is-active", srv.name,
		).OutputCombined()

		if err = executil.CheckStderr(stderr, err); err != nil {
			return err
		}

		if bytes.HasPrefix(stdout, []byte("active")) {
			stderr, err := excmd.Command(
				"sudo", "systemctl", "stop", srv.name,
			).OutputStderr()

			if err = executil.CheckStderr(stderr, err); err != nil {
				return err
			}
		}

	case sysutil.FreeBSD:
		stderr, err := excmd.Command(
			"sudo", "service", srv.name, "stop",
		).OutputStderr()

		if err = executil.CheckStderr(stderr, err); err != nil {
			return err
		}

	case sysutil.MacOS:
		stderr, err := excmd.Command(
			"sudo", "launchctl", "unload", "-F", srv.name,
		).OutputStderr()

		if stderr != nil {
			if bytes.Contains(stderr, []byte("Operation now in progress")) {
				return nil
			}
			if !bytes.Contains(stderr, []byte("Could not find specified service")) {
				return fmt.Errorf("%s", stderr)
			}
			//logs.Debug.Printf("%s\n%s", stderr, err)
		}
		if err != nil {
			return err
		}

	case sysutil.Windows:
		stderr, err := excmdWin.Command(
			"net", "stop", srv.name,
		).OutputStderr()

		if stderr != nil {
			if !bytes.Contains(stderr, []byte("is not started")) {
				return fmt.Errorf("%s", stderr)
			}
			//logs.Debug.Printf("%s\n%s", stderr, err)
		}
		if err != nil {
			return err
		}

	default:
		panic("unimplemented: " + srv.sys.String())
	}

	return nil
}

// Restart stops and starts the service.
func (srv Service) Restart() error {
	switch srv.sys {
	case sysutil.Linux:
		osutil.Log.Print("Re-starting service ...")

		stderr, err := excmd.Command(
			"sudo", "systemctl", "restart", srv.name,
		).OutputStderr()

		if err = executil.CheckStderr(stderr, err); err != nil {
			return err
		}

		// Wait to restart the service.
		time.Sleep(1 * time.Second)

		return nil

	case sysutil.FreeBSD:
		stderr, err := excmd.Command(
			"sudo", "service", srv.name, "restart",
		).OutputStderr()

		if err = executil.CheckStderr(stderr, err); err != nil {
			return err
		}

		return nil

	default:
		if err := srv.Stop(); err != nil {
			return err
		}
		return srv.Start()
	}
}

// Enable enables the service.
func (srv Service) Enable() error {
	osutil.Log.Print("Enabling service ...")

	cmd := "sudo"
	var args []string

	switch srv.sys {
	case sysutil.Linux:
		args = []string{"systemctl", "enable", srv.name}

		switch srv.dis {
		case sysutil.CentOS:
			ver := 7

			if ver < 7 {
				args = []string{"chkconfig", srv.name, "on"}
			}
		}

		stderr, err := excmd.Command(
			cmd, args...,
		).OutputStderr()

		return executil.CheckStderr(stderr, err)

	case sysutil.FreeBSD:
		//"sysrc sshd_enable='YES'"

	case sysutil.MacOS:
		args = []string{"launchctl", "enable", srv.name}

		stderr, err := excmd.Command(
			cmd, args...,
		).OutputStderr()

		return executil.CheckStderr(stderr, err)

	case sysutil.Windows:
		cmd = "sc"
		args = []string{"sc", "config", srv.name, "start= demand"}

		stderr, err := excmdWin.Command(
			cmd, args...,
		).OutputStderr()

		return executil.CheckStderr(stderr, err)

	default:
		panic("unimplemented: " + srv.sys.String())
	}

	return nil
}

// Disable disables the service.
func (srv Service) Disable() error {
	osutil.Log.Print("Disabling service ...")

	switch srv.sys {
	case sysutil.Linux:
		cmd := "sudo"
		args := []string{"systemctl", "disable", srv.name}

		switch srv.dis {
		case sysutil.CentOS:
			ver := 7

			if ver < 7 {
				args = []string{"chkconfig", srv.name, "off"}
			}
		}

		stderr, err := excmd.Command(
			cmd, args...,
		).OutputStderr()

		return executil.CheckStderr(stderr, err)

	case sysutil.FreeBSD:
		//"sysrc sshd_enable='YES'"

	case sysutil.MacOS:
		stderr, err := excmd.Command(
			"sudo", "launchctl", "disable", srv.name,
		).OutputStderr()

		return executil.CheckStderr(stderr, err)

	case sysutil.Windows:
		stderr, err := excmdWin.Command(
			"sc", "config", srv.name, "start= disabled",
		).OutputStderr()

		return executil.CheckStderr(stderr, err)

	default:
		panic("unimplemented: " + srv.sys.String())
	}

	return nil
}

// == Errors
//
// ErrNoService represents an error
var ErrNoService = errors.New("no service name")

// ServNotFoundError indicates whether a service is not found.
type ServNotFoundError struct {
	pattern string
	dirs    []string
}

func (e ServNotFoundError) Error() string {
	return fmt.Sprintf(
		"failed at searching the service by pattern %q at directories %v",
		e.pattern, e.dirs,
	)
}
