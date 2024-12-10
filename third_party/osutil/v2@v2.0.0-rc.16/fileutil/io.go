// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fileutil

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/tredoe/osutil/v2"
)

// CopyFile copies a file from 'src' to 'dst'. If 'src' and 'dst' files exist, and are
// the same, then return success. Otherwise, copy the file contents from 'src' to 'dst'.
//
// The file will be created if it does not already exist. If the destination file exists,
// all it's contents will be replaced by the contents of the source file.
func CopyFile(src, dst string) error {
	sInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !sInfo.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories, symlinks, devices, etc.)
		return fmt.Errorf("non-regular source file %s (%q)", sInfo.Name(), sInfo.Mode().String())
	}

	dInfo, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if !(dInfo.Mode().IsRegular()) {
			return fmt.Errorf("non-regular destination file %s (%q)", dInfo.Name(), dInfo.Mode().String())
		}
		if os.SameFile(sInfo, dInfo) {
			return nil
		}
	}

	// Open original file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create new file
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, sInfo.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() {
		if err2 := dstFile.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	// Copy the bytes to destination from source
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	// Commit the file contents. Flushes memory to disk.
	if err = dstFile.Sync(); err != nil {
		return err
	}

	osutil.Log.Printf("File %q copied at %q", src, dst)
	return nil
}

// Create creates a new file with b bytes.
// If the file already exists, it is truncated. If the file does not exist,
// it is created with mode 0666 (before umask).
func Create(filename string, b []byte) (err error) {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}

	_, err = file.Write(b)
	err2 := file.Close()
	if err2 != nil && err == nil {
		err = err2
	}
	if err != nil {
		return err
	}

	osutil.Log.Printf("File %q created", filename)
	return nil
}

// CreateFromString creates a new file with the string 's'.
func CreateFromString(filename string, s string) error {
	return Create(filename, []byte(s))
}

// Overwrite truncates the named file to zero and writes len(b) bytes.
// It is created with mode 0666 (before umask).
func Overwrite(filename string, b []byte) (err error) {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}

	_, err = file.Write(b)
	err2 := file.Close()
	if err2 != nil && err == nil {
		err = err2
	}
	if err != nil {
		return err
	}

	osutil.Log.Printf("File %q overwritted", filename)
	return nil
}

// CopytoTemp copies a file from the filename to the default directory with
// temporary files (see os.TempDir).
// Returns the temporary file name.
func CopytoTemp(filename string) (tmpFile string, err error) {
	fsrc, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer fsrc.Close()

	fdst, err := ioutil.TempFile("", filepath.Base(fsrc.Name())+"_")
	if err != nil {
		return "", err
	}
	defer func() {
		if err2 := fdst.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	if _, err = io.Copy(fdst, fsrc); err != nil {
		return "", err
	}
	if err = fdst.Sync(); err != nil {
		return "", err
	}

	osutil.Log.Printf("File %q copied at %q", filename, fdst.Name())
	return fdst.Name(), nil
}

// WritetoTemp writes bytes to a temporary file and returns its name.
func WritetoTemp(b []byte, name string) (filename string, err error) {
	tmpfile, err := os.CreateTemp("", name+"_")
	if err != nil {
		return "", err
	}
	filename = tmpfile.Name()

	defer func() {
		if err2 := tmpfile.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	if _, err = tmpfile.Write(b); err != nil {
		return "", err
	}
	if err = tmpfile.Sync(); err != nil {
		return "", err
	}

	osutil.Log.Printf("Created file \"%s\"\n```\n%s```", filename, b)
	return
}
