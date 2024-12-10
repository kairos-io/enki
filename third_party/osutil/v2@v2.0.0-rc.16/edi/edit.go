// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package edi

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"regexp"

	"github.com/tredoe/osutil/v2"
)

// A ModeEdit value is a set of flags (or 0) to control behavior at edit a file.
type ModeEdit uint

// Modes used at edit a file.
const (
	_         ModeEdit = iota
	ModBackup          // Do backup before of edit.
)

// ConfEditer represents the editer configuration.
type ConfEditer struct {
	Comment []byte
	Mode    ModeEdit
}

// Editer represents the file to edit.
type Editer struct {
	file *os.File
	buf  *bufio.ReadWriter
	conf *ConfEditer
}

// NewEdit prepares a file to edit.
// You must use 'Close()' to close the file.
func NewEdit(filename string, conf *ConfEditer) (*Editer, error) {
	if conf != nil && conf.Mode&ModBackup != 0 {
		if err := Backup(filename); err != nil {
			return nil, err
		}
	}

	file, err := os.OpenFile(filename, os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	return &Editer{
		file: file,
		buf:  bufio.NewReadWriter(bufio.NewReader(file), bufio.NewWriter(file)),
		conf: conf,
	}, nil
}

// Close closes the file.
func (ed *Editer) Close() error {
	if err := ed.file.Close(); err != nil {
		return err
	}

	osutil.Log.Printf("File %q edited", ed.file.Name())
	return nil
}

// Append writes len(b) bytes at the end of the File. It returns an error, if any.
func (ed *Editer) Append(b []byte) error {
	_, err := ed.file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	_, err = ed.file.Write(b)
	return err
}

// AppendString is like Append, but writes the contents of string s rather than an array of bytes.
func (ed *Editer) AppendString(s string) error {
	return ed.Append([]byte(s))
}

// Delete removes the text given at position 'begin:end'.
func (ed *Editer) Delete(begin, end int64) error {
	stat, err := ed.file.Stat()
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)

	data := make([]byte, stat.Size()-end)
	if _, err = ed.file.Seek(end, io.SeekStart); err != nil && err != io.EOF {
		return err
	}
	if _, err = ed.file.Read(data); err != nil && err != io.EOF {
		return err
	}
	buf.Write(data)
	data = data[:]

	if _, err = ed.file.Seek(begin, io.SeekStart); err != nil {
		return err
	}
	if err = ed.file.Truncate(stat.Size() - (end - begin)); err != nil {
		return err
	}

	_, err = ed.file.Write(buf.Bytes())
	return err
}

// Comment inserts the comment character in lines that mach any regular expression in reLine.
func (ed *Editer) Comment(reLine []string) error {
	if ed.conf == nil || len(ed.conf.Comment) == 0 {
		return errComment
	}

	allReSearch := make([]*regexp.Regexp, len(reLine))

	for i, v := range reLine {
		re, err := regexp.Compile(v)
		if err != nil {
			return err
		}

		allReSearch[i] = re
	}

	_, err := ed.file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	char := append(ed.conf.Comment, ' ')
	isNew := false
	buf := new(bytes.Buffer)

	// Check every line.
	for {
		line, err := ed.buf.ReadBytes('\n')
		if err == io.EOF {
			break
		}

		for _, v := range allReSearch {
			if v.Match(line) {
				line = append(char, line...)

				if !isNew {
					isNew = true
				}
				break
			}
		}

		if _, err = buf.Write(line); err != nil {
			return err
		}
	}

	if isNew {
		return ed.rewrite(buf.Bytes())
	}
	return nil
}

// CommentOut removes the comment character of lines that mach any regular expression in reLine.
func (ed *Editer) CommentOut(reLine []string) error {
	if ed.conf == nil || len(ed.conf.Comment) == 0 {
		return errComment
	}
	allSearch := make([]ReplacerAtLine, len(reLine))

	for i, v := range reLine {
		allSearch[i] = ReplacerAtLine{
			v, "[[:space:]]*" + string(ed.conf.Comment) + "[[:space:]]*", "",
		}
	}

	return ed.ReplaceAtLineN(allSearch, 1)
}

/*// Insert writes len(b) bytes at the start of the File. It returns an error, if any.
func (ed *Editer) Insert(b []byte) error {
	return ed.rewrite(b)
}

// InsertString is like Insert, but writes the contents of string s rather than an array of bytes.
func (ed *Editer) InsertString(s string) error {
	return ed.rewrite([]byte(s))
}*/

func (ed *Editer) rewrite(b []byte) error {
	_, err := ed.file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	if err = ed.file.Truncate(int64(len(b))); err != nil {
		return err
	}

	_, err = ed.file.Write(b)
	return err
	//return ed.file.Sync()
}
