// Copyright 2019 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fileutil

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Untar uncompresses a 'tar.gz' or 'tar' file.
func Untar(filename, dirDst string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	var tr *tar.Reader

	// TODO: maybe used gzip at '.tar' files
	if strings.HasSuffix(filename, ".tar.gz") {
		uncompressedStream, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		tr = tar.NewReader(uncompressedStream)
	} else if strings.HasSuffix(filename, ".tar") {
		tr = tar.NewReader(file)
	} else {
		return errNotTar
	}

	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		// If the header is nil, just skip it (not sure how this happens)
		if header == nil {
			continue
		}

		// The target location where the dir/file should be created.
		target := filepath.Join(dirDst, header.Name)

		// Check the file type
		switch header.Typeflag {
		default: // TODO: remove?
			return fmt.Errorf(
				"Untar: unknown type: %s in %s",
				string(header.Typeflag),
				header.Name,
			)

		// If it's a dir and it doesn't exist, create it.
		case tar.TypeDir:
			if _, err = os.Stat(target); err != nil {
				if err = os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
		// If it's a file, create it.
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// Copy over contents
			if _, err = io.Copy(f, tr); err != nil {
				return err
			}

			if err2 := f.Close(); err2 != nil && err == nil {
				err = err2
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// == Errors
//

var errNotTar = errors.New("not a tar file")
