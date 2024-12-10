// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package shconf

import (
	"bufio"
	"fmt"
	"os"
	"testing"
)

var testdata = []struct {
	k string
	v string
}{
	{"BOOL", "true"},
	{"INT", "-2"},
	{"UINT", "5"},
	{"FLOAT", "3.3"},
	{"STRING", "small"},
}

type conf struct {
	BOOL   bool
	INT    int
	UINT   uint
	FLOAT  float64
	STRING string
}

func TestParseFile(t *testing.T) {
	testParseFile('=', t)
	//testParseFile(':', t)
}

func testParseFile(separator_ rune, t *testing.T) {
	// Create temporary file
	fname, err := createTempFile(separator_)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err = os.Remove(fname); err != nil {
			t.Error(err)
		}
	}()

	// == Parser
	conf_ok := &conf{}
	conf_bad := conf{}

	cfg, err := ParseFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	for k, _ := range cfg.data {
		switch k {
		case "BOOL":
			_, err = cfg.Getbool(k)
		case "INT":
			_, err = cfg.Getint(k)
		case "UINT":
			_, err = cfg.Getuint(k)
		case "FLOAT":
			_, err = cfg.Getfloat(k)
		case "STRING":
			_, err = cfg.Get(k)
		}
		if err != nil {
			t.Errorf("parser: %q got wrong value", k)
		}
	}
	if _, err = cfg.Get("no_key"); err != ErrKey {
		t.Error("expected to get ErrKey")
	}

	if err = cfg.Unmarshal(conf_ok); err != nil {
		t.Error(err)
	}
	if err = cfg.Unmarshal(conf_bad); err != ErrStructPtr {
		t.Error("expected to get ErrStructPtr")
	}

	if separator[0] != cfg.separator[0] {
		t.Errorf("separator: expected %q, got %q", separator, cfg.separator)
	}

	// == Editing
	if err = cfg.Set("STRING", "big"); err != nil {
		t.Fatal(err)
	}
	if cfg.data["STRING"] != "big" {
		t.Errorf("edit: value %q could not be set in key %q", "big", "STRING")
	}

	if err = cfg.Set("Not", ""); err == nil {
		t.Errorf("edit: key %q should not exist", "Not")
	}
	// ==

	if testing.Verbose() {
		cfg.Print()
	}
}

// * * *

// createTempFile creates a temporary file.
func createTempFile(separator rune) (fname string, err error) {
	file, err := os.CreateTemp("", "test")
	if err != nil {
		return "", err
	}

	buf := bufio.NewWriter(file)
	buf.WriteString("# main comment\n\n")
	buf.WriteString(fmt.Sprintf("%s%c%s\n", testdata[0].k, separator, testdata[0].v))
	buf.WriteString(fmt.Sprintf("%s%c%s\n\n", testdata[1].k, separator, testdata[1].v))
	buf.WriteString(fmt.Sprintf("%s%c%s\n\n", testdata[2].k, separator, testdata[2].v))
	buf.WriteString("# Another comment\n")
	buf.WriteString(fmt.Sprintf("%s%c%s\n", testdata[3].k, separator, testdata[3].v))
	buf.WriteString(fmt.Sprintf("%s%c%s\n", testdata[4].k, separator, testdata[4].v))
	buf.Flush()
	file.Close()

	return file.Name(), nil
}
