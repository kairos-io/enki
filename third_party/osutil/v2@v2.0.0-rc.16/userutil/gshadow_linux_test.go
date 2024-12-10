// Copyright 2010 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package userutil

import (
	"bufio"
	"io"
	"os"
	"testing"
)

func TestGShadowParser(t *testing.T) {
	if !useGshadow {
		t.Skip()
	}

	f, err := os.Open(fileGShadow)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	buf := bufio.NewReader(f)

	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Error(err)
			continue
		}

		if _, err = parseGShadow(string(line)); err != nil {
			t.Error(err)
		}
	}
}

func TestGShadowFull(t *testing.T) {
	if !useGshadow {
		t.Skip()
	}

	entry, err := LookupGShadow("root")
	if err != nil || entry == nil {
		t.Error(err)
	}

	entries, err := LookupInGShadow(GS_PASSWD, "!", -1)
	if err != nil || entries == nil {
		t.Error(err)
	}

	entries, err = LookupInGShadow(GS_ALL, "", -1)
	if err != nil || len(entries) == 0 {
		t.Error(err)
	}
}

func TestGShadowCount(t *testing.T) {
	if !useGshadow {
		t.Skip()
	}

	count := 5
	entries, err := LookupInGShadow(GS_ALL, "", count)
	if err != nil || len(entries) != count {
		t.Error(err)
	}
}

func TestGShadowError(t *testing.T) {
	if !useGshadow {
		t.Skip()
	}

	_, err := LookupGShadow("!!!???")
	if _, ok := err.(NoFoundError); !ok {
		t.Error("expected to report NoFoundError")
	}

	if _, err = LookupInGShadow(GS_MEMBER, "", 0); err != errSearch {
		t.Error("expected to report errSearch")
	}

	gs := &GShadow{}
	if err = gs.Add(nil); err != RequiredError("Name") {
		t.Error("expected to report RequiredError")
	}
}

func TestGShadow_Add(t *testing.T) {
	if !useGshadow {
		t.Skip()
	}

	shadow, err := NewGShadow(GROUP, MEMBERS...)
	if err != nil {
		t.Fatal(err)
	}
	if err = shadow.Add(nil); err != nil {
		t.Fatal(err)
	}

	if err = shadow.Add(nil); err == nil {
		t.Fatal("a shadowed group existent can not be added again")
	} else {
		if !IsExist(err) {
			t.Error("shadowed group: expected to report ErrExist")
		}
	}

	s, err := LookupGShadow(GROUP)
	if err != nil {
		t.Fatal(err)
	}

	if s.Name != GROUP {
		t.Errorf("shadowed group: expected to get name %q", GROUP)
	}
}

var (
	groupKey1 = []byte("abc")
	groupKey2 = []byte("def")
)

func TestGShadowCrypt(t *testing.T) {
	if !useGshadow {
		t.Skip()
	}

	gs, err := LookupGShadow(GROUP)
	if err != nil {
		t.Fatal(err)
	}
	gs.Passwd(groupKey1)
	if err = config.crypter.Verify(gs.password, groupKey1); err != nil {
		t.Fatalf("expected to get the same hashed password for %q", groupKey1)
	}

	if err = ChGPasswd(GROUP, groupKey2); err != nil {
		t.Fatalf("expected to change password: %s", err)
	}
	gs, _ = LookupGShadow(GROUP)
	if err = config.crypter.Verify(gs.password, groupKey2); err != nil {
		t.Fatalf("ChGPasswd: expected to get the same hashed password for %q", groupKey2)
	}
}
