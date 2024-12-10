// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package edi

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
)

// A ModeFind value is a set of flags (or 0) to control behavior at find into a file.
type ModeFind uint

// Modes used at find into a file.
const (
	_              ModeFind = iota
	ModTrimSpace            // Removes all leading and trailing white spaces.
	ModSkipComment          // Skip skip lines that start with the comment string.
)

var errComment = errors.New("no comment string")

// Finder represents the file where find a string.
type Finder struct {
	filename string
	comment  []byte
	mode     ModeFind
	Begin    int64 // Line begin position where the string was found (if any).
	End      int64 // Line end position where the string was found (if any).
}

// NewFinder prepares the Finder.
func NewFinder(filename, comment string, mode ModeFind) (*Finder, error) {
	if mode&ModSkipComment != 0 && comment == "" {
		return nil, errComment
	}

	fn := &Finder{
		filename: filename,
		mode:     mode,
	}
	if comment != "" {
		fn.comment = []byte(comment)
	}

	return fn, nil
}

// Filename returns the file name.
func (fn *Finder) Filename() string {
	return fn.filename
}

// Contains reports whether the file contains 'b'.
func (fn *Finder) Contains(b []byte) (found bool, err error) {
	f, err := os.Open(fn.filename)
	if err != nil {
		return false, err
	}
	defer func() {
		if err2 := f.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	buf := bufio.NewReader(f)

	if fn.mode&ModSkipComment != 0 && fn.mode&ModTrimSpace != 0 {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}
			_line := bytes.TrimSpace(line)
			if len(_line) == 0 || bytes.HasPrefix(_line, fn.comment) {
				fn.Begin += int64(len(line))
				continue
			}

			if idx := bytes.Index(_line, b); idx != -1 {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	} else if fn.mode&ModSkipComment != 0 {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}
			if bytes.HasPrefix(line, fn.comment) {
				fn.Begin += int64(len(line))
				continue
			}

			if idx := bytes.Index(line, b); idx != -1 {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	} else if fn.mode&ModTrimSpace != 0 {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}

			if idx := bytes.Index(bytes.TrimSpace(line), b); idx != -1 {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	} else {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}

			if idx := bytes.Index(line, b); idx != -1 {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	}

	return false, nil
}

// HasPrefix reports whether the file has a line that begins with 'b'.
func (fn *Finder) HasPrefix(b []byte) (found bool, err error) {
	f, err := os.Open(fn.filename)
	if err != nil {
		return false, err
	}
	defer func() {
		if err2 := f.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	buf := bufio.NewReader(f)

	if fn.mode&ModSkipComment != 0 && fn.mode&ModTrimSpace != 0 {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}
			_line := bytes.TrimSpace(line)
			if len(_line) == 0 || bytes.HasPrefix(_line, fn.comment) {
				fn.Begin += int64(len(line))
				continue
			}

			if bytes.HasPrefix(_line, b) {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	} else if fn.mode&ModSkipComment != 0 {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}
			if bytes.HasPrefix(line, fn.comment) {
				fn.Begin += int64(len(line))
				continue
			}

			if bytes.HasPrefix(line, b) {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	} else if fn.mode&ModTrimSpace != 0 {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}

			if bytes.HasPrefix(bytes.TrimSpace(line), b) {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	} else {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}

			if bytes.HasPrefix(line, b) {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	}

	return false, nil
}

// HasSuffix reports whether the file has a line that ends with 'b'.
func (fn *Finder) HasSuffix(b []byte) (found bool, err error) {
	f, err := os.Open(fn.filename)
	if err != nil {
		return false, err
	}
	defer func() {
		if err2 := f.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	buf := bufio.NewReader(f)

	if fn.mode&ModSkipComment != 0 && fn.mode&ModTrimSpace != 0 {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}
			_line := bytes.TrimSpace(line)
			if len(_line) == 0 || bytes.HasPrefix(_line, fn.comment) {
				fn.Begin += int64(len(line))
				continue
			}

			if bytes.HasSuffix(_line, b) {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	} else if fn.mode&ModSkipComment != 0 {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}
			if bytes.HasPrefix(line, fn.comment) {
				fn.Begin += int64(len(line))
				continue
			}

			if bytes.HasSuffix(line, b) {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	} else if fn.mode&ModTrimSpace != 0 {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}

			if bytes.HasSuffix(bytes.TrimSpace(line), b) {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	} else {
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			}

			if bytes.HasSuffix(line, b) {
				fn.End = fn.Begin + int64(len(line))
				return true, nil
			}
			fn.Begin += int64(len(line))
		}
	}

	return false, nil
}
