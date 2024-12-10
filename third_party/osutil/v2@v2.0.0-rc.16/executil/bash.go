// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package executil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/tredoe/osutil/v2"
)

// RunAsBashWithMatch executes external commands with access to shell features such as
// filename wildcards, shell pipes, environment variables, and expansion of the
// shortcut character "~" to home directory.
// It also logs the command.
//
// This function avoids to have execute commands through a shell since an
// unsanitized input from an untrusted source makes a program vulnerable to
// shell injection, a serious security flaw which can result in arbitrary
// command execution.
//
// The most of commands return a text in output or an error if any.
// `match` is used in commands like *grep*, *find*, or *cmp* to indicate if the
// search is matched.
func RunAsBashWithMatch(command string) (output []byte, match bool, err error) {
	var (
		cmds           []*exec.Cmd
		outPipes       []io.ReadCloser
		stdout, stderr bytes.Buffer
	)

	commands := strings.Split(command, "|")
	lastIdxCmd := len(commands) - 1

	// Check lonely pipes.
	for _, cmd := range commands {
		if strings.TrimSpace(cmd) == "" {
			err = execAsBashError{
				cmd:     command,
				debug:   "",
				errType: "ERR",
				err:     errNoCmdInPipe,
			}
			return
		}
	}

	for i, cmd := range commands {
		cmdEnv := env  // environment variables for each command
		indexArgs := 1 // position where the arguments start
		fields := strings.Fields(cmd)
		lastIdxFields := len(fields) - 1

		// == Get environment variables in the first arguments, if any.
		for j, fCmd := range fields {
			if fCmd[len(fCmd)-1] == '=' || // VAR= foo
				(j < lastIdxFields && fields[j+1][0] == '=') { // VAR =foo
				err = execAsBashError{
					cmd:     command,
					debug:   "",
					errType: "ERR",
					err:     errEnvVar,
				}
				return
			}

			// Note: if the environ already has the environment variable, then it is not going
			// to be used by the system. Must be replaced by the next variable
			if strings.ContainsRune(fields[0], '=') {
				cmdEnv = append([]string{fields[0]}, env...) // Insert the environment variable
				fields = fields[1:]                          // and it is removed from arguments
			} else {
				break
			}
		}
		// ==

		cmdPath, e := exec.LookPath(fields[0])
		if e != nil {
			err = execAsBashError{
				cmd:     command,
				debug:   "",
				errType: "ERR",
				err:     e,
			}
			return
		}

		// == Get the path of the next command, if any
		for j, fCmd := range fields {
			cmdBase := path.Base(fCmd)

			if cmdBase != "sudo" && cmdBase != "xargs" {
				break
			}
			// It should have an extra command.
			if j+1 == len(fields) {
				err = execAsBashError{
					cmd:     command,
					debug:   "",
					errType: "ERR",
					err:     extraCmdError(cmdBase),
				}
				return
			}

			nextCmdPath, e := exec.LookPath(fields[j+1])
			if e != nil {
				err = execAsBashError{
					cmd:     command,
					debug:   "",
					errType: "ERR",
					err:     e,
				}
				return
			}

			if fields[j+1] != nextCmdPath {
				fields[j+1] = nextCmdPath
				indexArgs = j + 2
			}
		}

		// == Expansion of arguments
		expand := make(map[int][]string, len(fields))

		for j := indexArgs; j < len(fields); j++ {
			// Skip flags
			if fields[j][0] == '-' {
				continue
			}

			// Shortcut character "~"
			if fields[j] == "~" || strings.HasPrefix(fields[j], "~/") {
				fields[j] = strings.Replace(fields[j], "~", home, 1)
			}

			// File name wildcards
			names, e := filepath.Glob(fields[j])
			if e != nil {
				err = execAsBashError{
					cmd:     command,
					debug:   "",
					errType: "ERR",
					err:     e,
				}
				return
			}
			if names != nil {
				expand[j] = names
			}
		}

		// Substitute the names generated for the pattern starting from last field.
		if len(expand) != 0 {
			for j := len(fields) - indexArgs; j >= indexArgs; j-- {
				if v, ok := expand[j]; ok {
					fields = append(fields[:j], append(v, fields[j+1:]...)...)
				}
			}
		}

		// == Handle arguments with quotes
		hasQuote := false
		needUpdate := false
		tmpFields := []string{}

		for j := indexArgs; j < len(fields); j++ {
			v := fields[j]
			lastChar := v[len(v)-1]

			if !hasQuote && (v[0] == '\'' || v[0] == '"') {
				if !needUpdate {
					needUpdate = true
				}

				v = v[1:] // skip quote

				if lastChar == '\'' || lastChar == '"' {
					v = v[:len(v)-1] // remove quote
				} else {
					hasQuote = true
				}

				tmpFields = append(tmpFields, v)
				continue
			}

			if hasQuote {
				if lastChar == '\'' || lastChar == '"' {
					v = v[:len(v)-1] // remove quote
					hasQuote = false
				}
				tmpFields[len(tmpFields)-1] += " " + v
				continue
			}

			tmpFields = append(tmpFields, v)
		}

		if needUpdate {
			fields = append(fields[:indexArgs], tmpFields...)
		}

		// == Create command
		c := &exec.Cmd{
			Path: cmdPath,
			Args: append([]string{fields[0]}, fields[1:]...),
			Env:  cmdEnv,
		}

		// == Connect pipes
		outPipe, e := c.StdoutPipe()
		if e != nil {
			err = execAsBashError{
				cmd:     command,
				debug:   "",
				errType: "ERR",
				err:     e,
			}
			return
		}

		if i == 0 {
			c.Stdin = os.Stdin
		} else {
			c.Stdin = outPipes[i-1] // anterior output
		}

		// == Buffers
		c.Stderr = &stderr

		// Only save the last output
		if i == lastIdxCmd {
			c.Stdout = &stdout
		}

		// == Start command
		if e := c.Start(); e != nil {
			err = execAsBashError{
				cmd:     command,
				debug:   fmt.Sprintf("- Command: %s\n- Args: %s", c.Path, c.Args),
				errType: "Start",
				err:     fmt.Errorf("%s", c.Stderr),
			}
			return
		}

		//
		cmds = append(cmds, c)
		outPipes = append(outPipes, outPipe)
	}

	for _, c := range cmds {
		if e := c.Wait(); e != nil {
			_, isExitError := e.(*exec.ExitError)

			// Error type due I/O problems.
			if !isExitError {
				err = execAsBashError{
					cmd:     command,
					debug:   fmt.Sprintf("- Command: %s\n- Args: %s", c.Path, c.Args),
					errType: "Wait",
					err:     fmt.Errorf("%s", c.Stderr),
				}
				return
			}

			if c.Stderr != nil {
				if stderr := fmt.Sprintf("%s", c.Stderr); stderr != "" {
					stderr = strings.TrimRight(stderr, "\n")
					err = execAsBashError{
						cmd:     command,
						debug:   fmt.Sprintf("- Command: %s\n- Args: %s", c.Path, c.Args),
						errType: "Stderr",
						err:     fmt.Errorf("%s", stderr),
					}
					return
				}
			}
		} else {
			match = true
		}
	}

	osutil.LogShell.Print(command)
	return stdout.Bytes(), match, nil
}

// RunAsBash executes external commands just like RunAsBashWithMatch, but does not return
// the boolean `match`.
func RunAsBash(command string) (output []byte, err error) {
	output, _, err = RunAsBashWithMatch(command)
	return
}

// RunAsBashf is like RunAsBash, but formats its arguments according to the format.
// Analogous to Printf().
func RunAsBashf(format string, args ...interface{}) ([]byte, error) {
	return RunAsBash(fmt.Sprintf(format, args...))
}

// RunAsBashWithMatchf is like RunAsBashWithMatch, but formats its arguments according to
// the format. Analogous to Printf().
func RunAsBashWithMatchf(format string, args ...interface{}) ([]byte, bool, error) {
	return RunAsBashWithMatch(fmt.Sprintf(format, args...))
}

// == Errors
//

// DebugRunAsBash shows debug messages at functions related to 'RunAsBash()'.
var DebugAsBash bool

type execAsBashError struct {
	cmd     string
	debug   string
	errType string
	err     error
}

func (e execAsBashError) Error() string {
	if DebugAsBash {
		if e.debug != "" {
			e.debug = "\n## DEBUG\n" + e.debug + "\n"
		}
		return fmt.Sprintf("Command line: `%s`\n%s\n## %s\n%s", e.cmd, e.debug, e.errType, e.err)
	}
	return fmt.Sprintf("\n%s", e.err)
}
