// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package executil

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tredoe/osutil/v2"
)

// Command represents a command to execute.
type Command struct {
	cmd  string
	args []string
	env  []string

	stdout     io.Writer
	stderr     io.Writer
	saveStdout bool
	saveStderr bool
	bufStdout  []byte
	bufStderr  []byte

	exitCode     int
	exitError    *exec.ExitError
	badExitCodes []int
	okExitCodes  []int

	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewCommand sets the basic arguments to execute a command.
func NewCommand(cmd string, args ...string) *Command {
	return &Command{
		cmd:  cmd,
		args: args,
		//exitCode: -1,
	}
}

// Command sets the arguments to run other command.
func (c *Command) Command(cmd string, args ...string) *Command {
	return &Command{
		cmd:  cmd,
		args: args,
		env:  c.env,

		stdout: c.stdout,
		stderr: c.stderr,

		//exitCode:     -1,
		badExitCodes: c.badExitCodes,

		ctx:        c.ctx,
		cancelFunc: c.cancelFunc,
	}
}

// TimeKill sets the time of waiting to finish the command before of kill it.
func (c *Command) TimeKill(tm time.Duration) *Command {
	c.ctx, c.cancelFunc = context.WithTimeout(context.Background(), tm)
	return c
}

// BadExitCodes sets the exit codes with errors for the command.
func (c *Command) BadExitCodes(codes []int) *Command {
	c.badExitCodes = codes
	return c
}

// OkExitCodes sets the exit codes without errors for the command.
func (c *Command) OkExitCodes(codes []int) *Command {
	c.okExitCodes = codes
	return c
}

// Env sets the environment variables.
func (c *Command) Env(e []string) *Command {
	c.env = e
	return c
}

// AddEnv adds environment variables.
func (c *Command) AddEnv(e []string) *Command {
	c.env = append(c.env, e...)
	return c
}

// Stdout sets the standard out.
func (c *Command) Stdout(out io.Writer) *Command {
	c.stdout = out
	return c
}

// Stderr sets the standard error.
func (c *Command) Stderr(err io.Writer) *Command {
	c.stderr = err
	return c
}

// * * *

// ExitCode returns the exit status code which is returned after of call to Run().
func (c *Command) ExitCode() int { return c.exitCode }

// OutputStdout runs the command and returns the standard output.
func (c *Command) OutputStdout() (stdout []byte, err error) {
	c.saveStdout = true
	_, err = c.Run()

	return c.bufStdout, err
}

// OutputStderr runs the command and returns the standard error.
func (c *Command) OutputStderr() (stderr []byte, err error) {
	c.saveStderr = true
	_, err = c.Run()

	return c.bufStderr, err
}

// OutputCombined runs the command and returns both standard output and error.
func (c *Command) OutputCombined() (stdout, stderr []byte, err error) {
	c.saveStdout = true
	c.saveStderr = true
	_, err = c.Run()

	return c.bufStdout, c.bufStderr, err
}

// StdoutTofile runs the command and saves the standard output into a file.
// The full name is formed with the value of 'filename' plus "_stdout.log".
func (c *Command) StdoutTofile(dir, filename string) error {
	c.saveStdout = true
	_, errRun := c.Run()

	if !errors.As(errRun, &c.exitError) {
		return errRun
	}

	if c.bufStdout != nil {
		//fmt.Println(string(c.bufStdout))
		err := os.WriteFile(filepath.Join(dir, filename+"_stdout.log"), c.bufStdout, 0600)
		if err != nil {
			return err
		}
	}

	return errRun
}

// StderrTofile runs the command and saves the standard error into a file.
// The full name is formed with the value of 'filename' plus "_stderr.log".
// fnCheckStderr is a function to check the standard error.
func (c *Command) StderrTofile(dir, filename string, fnCheckStderr func([]byte) error) error {
	c.saveStderr = true
	_, errRun := c.Run()

	if !errors.As(errRun, &c.exitError) {
		return errRun
	}

	if c.bufStderr != nil {
		var err error

		if fnCheckStderr != nil {
			if err = fnCheckStderr(c.bufStderr); err != nil {
				return err
			}
		}
		err = os.WriteFile(filepath.Join(dir, filename+"_stderr.log"), c.bufStderr, 0600)
		if err != nil {
			return err
		}
	}

	return errRun
}

// StdCombinedTofile runs the command and saves both standard output and error into files.
// The full names are formed with the values of 'filename' plus "_stdout.log"
// and 'filename' plus "_stderr.log".
func (c *Command) StdCombinedTofile(
	dir, filename string, fnCheckStderr func([]byte) error,
) error {
	c.saveStdout = true
	c.saveStderr = true
	_, errRun := c.Run()

	if !errors.As(errRun, &c.exitError) {
		return errRun
	}
	var err error

	if c.bufStderr != nil {
		if fnCheckStderr != nil {
			if err = fnCheckStderr(c.bufStderr); err != nil {
				return err
			}
		}
		err = os.WriteFile(filepath.Join(dir, filename+"_stderr.log"), c.bufStderr, 0600)
		if err != nil {
			return err
		}
	}
	if c.bufStdout != nil {
		//fmt.Println(string(c.bufStdout))
		err = os.WriteFile(filepath.Join(dir, filename+"_stdout.log"), c.bufStdout, 0600)
		if err != nil {
			return err
		}
	}

	return errRun
}

// Run executes the command.
// Logs the command and the exit code.
func (c *Command) Run() (exitCode int, err error) {
	var cmd *exec.Cmd

	if c.ctx == nil {
		cmd = exec.Command(c.cmd, c.args...)
	} else {
		cmd = exec.CommandContext(c.ctx, c.cmd, c.args...)
		defer c.cancelFunc()
	}
	osutil.LogShell.Printf("%s", strings.Join(cmd.Args, " "))

	if len(c.env) != 0 {
		cmd.Env = c.env
	}

	var outPipe, errPipe io.ReadCloser

	if c.saveStdout {
		if outPipe, err = cmd.StdoutPipe(); err != nil {
			return -1, err
		}
	} else if c.stdout != nil {
		cmd.Stdout = c.stdout
	}

	if c.saveStderr {
		if errPipe, err = cmd.StderrPipe(); err != nil {
			return -1, err
		}
	} else if c.stderr != nil {
		cmd.Stderr = c.stderr
	}

	if err = cmd.Start(); err != nil {
		return -1, err
	}

	// Using 'bytes.Buffer' to get stdout and stderr gives error:
	// https://github.com/golang/go/issues/23019
	//
	// var bufStdOut, bufStderr bytes.Buffer
	//c.bufStdout = &bufStdOut
	//c.bufStderr = &bufStderr

	if c.saveStdout {
		// Std out
		go func() {
			var bufOut bytes.Buffer
			buf := bufio.NewReader(outPipe)
			for {
				line, err2 := buf.ReadBytes('\n')
				if len(line) > 0 {
					bufOut.Write(line)
				}
				if err2 != nil {
					c.bufStdout = bufOut.Bytes()
					if err2 != io.EOF && !errors.Is(err2, os.ErrClosed) && err == nil {
						err = err2
					}

					return
				}
			}
		}()
	}
	if c.saveStderr {
		// Std error
		go func() {
			var bufStderr bytes.Buffer
			buf := bufio.NewReader(errPipe)
			for {
				line, err2 := buf.ReadBytes('\n')
				if len(line) > 0 {
					bufStderr.Write(line)
				}
				if err2 != nil {
					c.bufStderr = bufStderr.Bytes()
					if err2 != io.EOF && !errors.Is(err2, os.ErrClosed) && err == nil {
						err = err2
					}

					return
				}
			}
		}()
	}

	err = cmd.Wait()

	if errors.As(err, &c.exitError) {
		exitCode = err.(*exec.ExitError).ExitCode()
		c.exitCode = exitCode
		osutil.LogShell.Printf("Exit code: %d", exitCode)

		if c.ctx != nil && exitCode == -1 {
			return -1, ErrProcKilled
		}

		if len(c.badExitCodes) != 0 {
			for _, v := range c.badExitCodes {
				if v == exitCode {
					return exitCode, err
				}
			}
		} else if len(c.okExitCodes) != 0 {
			for _, v := range c.okExitCodes {
				if v == exitCode {
					return exitCode, nil
				}
			}
		}

		return exitCode, err
	}
	return -1, err
}
