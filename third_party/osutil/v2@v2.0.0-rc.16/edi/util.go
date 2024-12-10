// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package edi

// Append writes len(b) bytes at the end of the named file.
// It returns an error, if any.
func Append(filename string, mode ModeEdit, b []byte) error {
	ed, err := NewEdit(filename, &ConfEditer{Mode: mode})
	if err != nil {
		return err
	}

	err = ed.Append(b)
	err2 := ed.Close()
	if err != nil {
		return err
	}
	return err2
}

// AppendString is like Append, but writes the contents of string s rather than an array of bytes.
func AppendString(filename string, mode ModeEdit, s string) error {
	return Append(filename, mode, []byte(s))
}

// Delete removes the text given at position 'begin:end'.
func Delete(filename string, begin, end int64) error {
	ed, err := NewEdit(filename, &ConfEditer{Mode: ModBackup})
	if err != nil {
		return err
	}

	err = ed.Delete(begin, end)
	err2 := ed.Close()
	if err != nil {
		return err
	}
	return err2
}

// Comment inserts the comment character in lines that mach the regular expression in reLine,
// in the named file.
func Comment(filename string, conf *ConfEditer, reLine string) error {
	return CommentM(filename, conf, []string{reLine})
}

// CommentM inserts the comment character in lines that mach any regular expression in reLine,
// in the named file.
func CommentM(filename string, conf *ConfEditer, reLine []string) error {
	ed, err := NewEdit(filename, conf)
	if err != nil {
		return err
	}

	err = ed.Comment(reLine)
	err2 := ed.Close()
	if err != nil {
		return err
	}
	return err2
}

// CommentOut removes the comment character of lines that mach the regular expression in reLine,
// in the named file.
func CommentOut(filename string, conf *ConfEditer, reLine string) error {
	return CommentOutM(filename, conf, []string{reLine})
}

// CommentOutM removes the comment character of lines that mach any regular expression in reLine,
// in the named file.
func CommentOutM(filename string, conf *ConfEditer, reLine []string) error {
	ed, err := NewEdit(filename, conf)
	if err != nil {
		return err
	}

	err = ed.CommentOut(reLine)
	err2 := ed.Close()
	if err != nil {
		return err
	}
	return err2
}

/*// Insert writes len(b) bytes at the start of the named file. It returns an error, if any.
func Insert(filename string, conf *ConfEditer, b []byte) error {
	ed, err := NewEdit(filename, conf)
	if err != nil {
		return err
	}

	err = ed.Insert(b)
	err2 := ed.Close()
	if err != nil {
		return err
	}
	return err2
}

// InsertString is like Insert, but writes the contents of string s rather than an array of bytes.
func InsertString(filename string, conf *ConfEditer, s string) error {
	return Insert(filename, conf, []byte(s))
}*/

// Replace replaces all regular expressions mathed in r for the named file.
func Replace(filename string, conf *ConfEditer, r []Replacer) error {
	ed, err := NewEdit(filename, conf)
	if err != nil {
		return err
	}

	err = ed.genReplace(r, -1)
	err2 := ed.Close()
	if err != nil {
		return err
	}
	return err2
}

// ReplaceN replaces a number of regular expressions mathed in r for the named file.
func ReplaceN(filename string, conf *ConfEditer, r []Replacer, n int) error {
	ed, err := NewEdit(filename, conf)
	if err != nil {
		return err
	}

	err = ed.genReplace(r, n)
	err2 := ed.Close()
	if err != nil {
		return err
	}
	return err2
}

// ReplaceAtLine replaces all regular expressions mathed in r for the named file,
// if the line is matched at the first.
func ReplaceAtLine(filename string, conf *ConfEditer, r []ReplacerAtLine) error {
	ed, err := NewEdit(filename, conf)
	if err != nil {
		return err
	}

	err = ed.genReplaceAtLine(r, -1)
	err2 := ed.Close()
	if err != nil {
		return err
	}
	return err2
}

// ReplaceAtLineN replaces a number of regular expressions mathed in r for the named file,
// if the line is matched at the first.
func ReplaceAtLineN(filename string, conf *ConfEditer, r []ReplacerAtLine, n int) error {
	ed, err := NewEdit(filename, conf)
	if err != nil {
		return err
	}

	err = ed.genReplaceAtLine(r, n)
	err2 := ed.Close()
	if err != nil {
		return err
	}
	return err2
}
