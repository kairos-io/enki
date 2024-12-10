// Copyright 2013 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package env implements the setting of persistent environment variables.
//
// The environment variables must be named with only English capital letters and
// underscore signs (_).
package env

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/tredoe/osutil/v2/config/shconf"
	"github.com/tredoe/osutil/v2/edi"
	"github.com/tredoe/osutil/v2/userutil"
)

var isRoot bool

func init() {
	if os.Getuid() == 0 {
		isRoot = true
	}
}

// == Errors

var ErrNoRoot = errors.New("you have to be Root")

// NoShellError represents an account without shell.
type NoShellError string

func (e NoShellError) Error() string {
	return "shell not found for user: " + string(e)
}

// NoHomeError represents an account without home directory.
type NoHomeError string

func (e NoHomeError) Error() string {
	return "home directory not found for user: " + string(e)
}

var errKey = errors.New("environment variables must use only English capital" +
	" letters and underscore signs")

// ===

// Families of shell commands.
var (
	shFamily  = []string{"bash", "ksh", "sh"}
	cshFamily = []string{"tcsh", "csh"}
)

// shellType represents a type of shell.
type shellType int

const (
	shellSh shellType = 1 + iota
	shellCsh
)

// == Settings for each Shell family
//

var systemFile = []string{
	shellSh: "/etc/environment",
	//shellCsh: "",
}

var userFile = []string{
	shellSh: ".pam_environment",
	//shellCsh: "",
}

// To set environment variables that affect your whole session, KDE will execute
// any script it finds in '$HOME/.kde/env' whose filename ends in '.sh', and it
// will maintain all the environment variables set by them.
var kdeFile = ".kde/env"

// settings represents the files of configuration for an user, according to the shell.
type settings struct {
	global string // System wide
	user   string
	kde    string

	useKDE bool
}

// Settings files of the caller.
var _settings settings

// Sets the settings files of the caller.
func init() {
	var err error

	_settings, err = settingsForUID(os.Getuid())
	if err != nil {
		panic(err)
	}
}

// settingsForUID returns the settings files of the given user id.
func settingsForUID(id int) (settings, error) {
	u, err := userutil.LookupUID(id)
	if err != nil {
		return settings{}, err
	}
	if u.Shell == "" {
		return settings{}, NoShellError(u.Name)
	}
	if u.Dir == "" {
		return settings{}, NoHomeError(u.Name)
	}

	shell := path.Base(u.Shell)

	_settings := settings{
		global: systemFile[shellSh],
		user:   path.Join(u.Dir, userFile[shellSh]),
		kde:    path.Join(u.Dir, kdeFile),
	}
	info, err := os.Stat(_settings.kde)
	if err == nil && info.IsDir() {
		_settings.useKDE = true
	}

	for _, v := range shFamily {
		if v == shell {
			return _settings, nil
		}
	}
	/*for _, v := range cshFamily {
		if v == shell {
			return _settings, nil
		}
	}*/

	return settings{}, fmt.Errorf("shell unsopported: %s", shell)
}

// == Set variables
//

// _Set sets the value named by the key in the given filename.
func _Set(filename, key, value string) error {
	// Check if the key is already used.
	conf, err := shconf.ParseFile(filename)
	if err != nil {
		if err != os.ErrNotExist {
			return err
		}
		println("ErrNotExist") //TODO: remove
	} else {
		if _, err = conf.Get(key); err != shconf.ErrKey {
			panic("OPS")
		}
	}

	return edi.Append(filename, edi.ModBackup, []byte(key+string(conf.Separator())+value))
}

// _MSet sets multiple values named by the keys in the given filename.
func _MSet(filename string, keys, values []string) error {
	if len(keys) != len(values) {
		return fmt.Errorf("number of keys is different to number of values")
	}

	// Check if the key is already used.
	conf, err := shconf.ParseFile(filename)
	if err != nil {
		if err != os.ErrNotExist {
			return err
		}
		println("ErrNotExist") //TODO: remove
	}

	var buf bytes.Buffer
	for i, key := range keys {
		if _, err = conf.Get(key); err != nil {
			continue // TODO: log key already set.
		}

		buf.WriteString(key)
		buf.Write(conf.Separator())
		buf.WriteString(values[i])
		buf.WriteByte('\n')
	}

	return edi.Append(filename, edi.ModBackup, buf.Bytes())
}

// == Set session-wide variables

// Set sets the value of the environment variable named by the key that
// affects the current user.
// It returns an error, if any.
func Set(key, value string) error {
	err := _Set(_settings.user, key, value)
	if err != nil {
		return err
	}

	if _settings.useKDE {
		return _Set(_settings.kde, key, value)
	}
	return nil
}

// MSet sets multiple values of the environment variables named by the keys
// that affects the current user.
// It returns an error, if any.
func MSet(keys, values []string) error {
	err := _MSet(_settings.user, keys, values)
	if err != nil {
		return err
	}

	if _settings.useKDE {
		return _MSet(_settings.kde, keys, values)
	}
	return nil
}

// SetForUid sets the value of the environment variable named by the key that
// affects a particular user.
// It returns an error, if any.
func SetForUid(id int, key, value string) error {
	_settings, err := settingsForUID(id)
	if err != nil {
		return err
	}

	if err = _Set(_settings.user, key, value); err != nil {
		return err
	}
	if _settings.useKDE {
		return _Set(_settings.kde, key, value)
	}
	return nil
}

// MSetForUid sets multiple values of the environment variables named by the
// keys that affects a particular user.
// It returns an error, if any.
func MSetForUid(id int, keys, values []string) error {
	_settings, err := settingsForUID(id)
	if err != nil {
		return err
	}

	if err = _MSet(_settings.user, keys, values); err != nil {
		return err
	}
	if _settings.useKDE {
		return _MSet(_settings.kde, keys, values)
	}
	return nil
}

// == Set system-wide variables

// Setsys sets the value of the environment variable named by the key that
// affects the system as a whole. You must be Root.
// It returns an error, if any.
func Setsys(key, value string) error {
	if !isRoot {
		return ErrNoRoot
	}
	return _Set(_settings.global, key, value)
}

// MSetsys sets multiple values of the environment variables named by the keys
// that affects the system as a whole. You must be Root.
// It returns an error, if any.
func MSetsys(keys, values []string) error {
	if !isRoot {
		return ErrNoRoot
	}
	return _MSet(_settings.global, keys, values)
}

// SetsysForUid sets the value of the environment variable named by the key that
// affects the system as a whole. You must be Root.
// It returns an error, if any.
func SetsysForUid(id int, key, value string) error {
	if !isRoot {
		return ErrNoRoot
	}

	_settings, err := settingsForUID(id)
	if err != nil {
		return err
	}
	return _Set(_settings.global, key, value)
}

// MSetsysForUid sets multiple values of the environment variables named by the
// keys that affects the system as a whole. You must be Root.
// It returns an error, if any.
func MSetsysForUid(id int, keys, values []string) error {
	if !isRoot {
		return ErrNoRoot
	}

	_settings, err := settingsForUID(id)
	if err != nil {
		return err
	}
	return _MSet(_settings.global, keys, values)
}

// == Unset variables
//

// _Unset unsets the key in the given filename.
/*func _Unset(filename, key string) error {

}*/

// == Utility
//

// It is a common practice to name all environment variables with only English
// capital letters and underscore (_) signs.

// checkKey reports whether the key uses English capital letters and underscore
// signs.
func checkKey(s string) {
	for _, char := range s {
		if (char < 'A' || char > 'Z') && char != '_' {
			panic(errKey)
		}
	}
}
