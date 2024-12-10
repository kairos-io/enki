// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package executil

import (
	"bytes"
	"errors"
	"fmt"
)

var (
	errEnvVar      = errors.New("the format of the variable has to be VAR=value")
	errNoCmdInPipe = errors.New("no command around of pipe")
)

// ErrProcKilled reports an error by a process killed.
var ErrProcKilled = errors.New("the process hasn't exited or was terminated by a signal")

// errFromStderr returns the standard error like a Go error.
func errFromStderr(e []byte) error {
	return fmt.Errorf("[stderr]\n%s", e)
}

// CheckStderr returns an error whether 'stderr' is not empty or there is any error.
func CheckStderr(stderr []byte, err error) error {
	if stderr != nil {
		return errFromStderr(stderr)
	}
	if err != nil {
		return err
	}

	return nil
}

// CheckStderrSkipWarn returns an error whether 'stderr' is not empty
// and it is not a message which starts with 'warning', or there is any error.
func CheckStderrSkipWarn(stderr, warning []byte, err error) error {
	if stderr != nil {
		if !bytes.HasPrefix(stderr, warning) {
			return errFromStderr(stderr)
		}
	}
	if err != nil {
		return err
	}

	return nil
}

// * * *

// extraCmdError reports an error due to the lack of an extra command.
type extraCmdError string

func (e extraCmdError) Error() string {
	return "command not added to " + string(e)
}
