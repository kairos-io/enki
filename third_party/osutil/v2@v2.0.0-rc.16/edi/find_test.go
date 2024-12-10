// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package edi

import (
	"testing"
)

func TestFind(t *testing.T) {
	find, err := NewFinder(tmpFilename, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	word := []byte("magna")

	ok, err := find.Contains(word)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("Contains: could not find %s", word)
	}

	word = []byte("foo")
	find, _ = NewFinder(tmpFilename, "", 0)

	if ok, err = find.Contains(word); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Errorf("Contains: must not find %s", word)
	}

	// * * *

	start := []byte("Lorem")
	find, _ = NewFinder(tmpFilename, "", 0)

	if ok, err = find.HasPrefix(start); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Errorf("HasPrefix: must not find %s", start)
	}

	find, _ = NewFinder(tmpFilename, "", ModTrimSpace)

	if ok, err = find.HasPrefix(start); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Errorf("HasPrefix: could not find %s", start)
	}

	// * * *

	end := []byte("quis")
	find, _ = NewFinder(tmpFilename, "", 0)

	if ok, err = find.HasSuffix(end); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Errorf("HasSuffix: must not find %s", end)
	}

	find, _ = NewFinder(tmpFilename, "", ModTrimSpace)

	if ok, err = find.HasSuffix(end); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Errorf("HasSuffix: could not find %s", end)
	}
}
