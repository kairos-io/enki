// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package edi

import (
	"bytes"
	"io"
	"regexp"
)

// Replacer represents the text to be replaced.
type Replacer struct {
	Search, Replace string
}

// ReplacerAtLine represents the text to be replaced into a line.
type ReplacerAtLine struct {
	Line, Search, Replace string
}

// Replace replaces all regular expressions mathed in r.
func (ed *Editer) Replace(r []Replacer) error {
	return ed.genReplace(r, -1)
}

// ReplaceN replaces regular expressions mathed in r. The count determines the number to match:
//   n > 0: at most n matches
//   n == 0: the result is none
//   n < 0: all matches
func (ed *Editer) ReplaceN(r []Replacer, n int) error {
	return ed.genReplace(r, n)
}

// ReplaceAtLine replaces all regular expressions mathed in r, if the line is matched at the first.
func (ed *Editer) ReplaceAtLine(r []ReplacerAtLine) error {
	return ed.genReplaceAtLine(r, -1)
}

// ReplaceAtLineN replaces regular expressions mathed in r, if the line is matched at the first.
// The count determines the number to match:
//   n > 0: at most n matches
//   n == 0: the result is none
//   n < 0: all matches
func (ed *Editer) ReplaceAtLineN(r []ReplacerAtLine, n int) error {
	return ed.genReplaceAtLine(r, n)
}

// Generic Replace: replaces a number of regular expressions matched in r.
func (ed *Editer) genReplace(r []Replacer, n int) error {
	if n == 0 {
		return nil
	}
	if _, err := ed.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	content, err := io.ReadAll(ed.buf)
	if err != nil {
		return err
	}

	isNew := false

	for _, v := range r {
		reSearch, err := regexp.Compile(v.Search)
		if err != nil {
			return err
		}

		i := n
		repl := []byte(v.Replace)

		content = reSearch.ReplaceAllFunc(content, func(s []byte) []byte {
			if !isNew {
				isNew = true
			}

			if i != 0 {
				i--
				return repl
			}
			return s
		})
	}

	if isNew {
		return ed.rewrite(content)
	}
	return nil
}

// Generic ReplaceAtLine: replaces a number of regular expressions matched in r,
// if the line is matched at the first.
func (ed *Editer) genReplaceAtLine(r []ReplacerAtLine, n int) error {
	if n == 0 {
		return nil
	}
	_, err := ed.file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	// == Cache the regular expressions
	allReLine := make([]*regexp.Regexp, len(r))
	allReSearch := make([]*regexp.Regexp, len(r))
	allRepl := make([][]byte, len(r))

	for i, v := range r {
		reLine, err := regexp.Compile(v.Line)
		if err != nil {
			return err
		}
		allReLine[i] = reLine

		reSearch, err := regexp.Compile(v.Search)
		if err != nil {
			return err
		}
		allReSearch[i] = reSearch

		allRepl[i] = []byte(v.Replace)
	}

	buf := new(bytes.Buffer)
	isNew := false

	// Replace every line, if it maches
	for {
		line, err := ed.buf.ReadBytes('\n')
		if err == io.EOF {
			break
		}

		for i := range r {
			if allReLine[i].Match(line) {
				j := n

				line = allReSearch[i].ReplaceAllFunc(line, func(s []byte) []byte {
					if !isNew {
						isNew = true
					}

					if j != 0 {
						j--
						return allRepl[i]
					}
					return s
				})
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
