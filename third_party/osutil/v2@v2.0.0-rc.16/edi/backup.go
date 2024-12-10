// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package edi

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

const suffixBackup = "+[1-9]~" // Suffix pattern added to backup's file name.

// Backup creates a backup of the named file.
//
// The schema used for the new name is: {name}\+[1-9]~
//   name: The original file name.
//   + : Character used to separate the file name from rest.
//   number: A number from 1 to 9, using rotation.
//   ~ : To indicate that it is a backup, just like it is used in Unix systems.
func Backup(filename string) error {
	// Check if it is empty
	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() == 0 {
		return nil
	}

	files, err := filepath.Glob(filename + suffixBackup)
	if err != nil {
		return err
	}

	// Number rotation
	numBackup := byte(1)

	if len(files) != 0 {
		lastFile := files[len(files)-1]
		numBackup = lastFile[len(lastFile)-2] + 1 // next number

		if numBackup > '9' {
			numBackup = '1'
		}
	} else {
		numBackup = '1'
	}

	// == Copy

	fileBackup := fmt.Sprintf("%s+%s~", filename, string(numBackup))

	// Open original file
	fsrc, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fsrc.Close()

	// Create new file
	fdst, err := os.Create(fileBackup)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := fdst.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	if _, err := io.Copy(fdst, fsrc); err != nil {
		return err
	}
	log.Printf("Backup created from %q at %q", filename, fileBackup)

	// Flushes memory to disk
	return fdst.Sync()
}
