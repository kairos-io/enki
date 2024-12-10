// Copyright 2010 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package userutil

import (
	"io"
	"os"
	"sort"
	"strconv"
)

// nextUID returns the next free user id to use, according to whether it is a
// system's user.
func nextUID(isSystem bool) (db *dbfile, uid int, err error) {
	loadConfig()

	db, err = openDBFile(fileUser, os.O_RDWR)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			db.close()
		}
	}()

	// Seek to file half size.

	info, err := db.file.Stat()
	if err != nil {
		return db, 0, err
	}
	if _, err = db.file.Seek(info.Size()/2, io.SeekStart); err != nil {
		return db, 0, err
	}
	// To starting to read from a new line
	if _, _, err = db.rd.ReadLine(); err != nil {
		return db, 0, err
	}

	var minId, maxId int
	var listId []int

	if isSystem {
		minId, maxId = config.login.SYS_UID_MIN, config.login.SYS_UID_MAX
	} else {
		minId, maxId = config.login.UID_MIN, config.login.UID_MAX
	}

	for {
		line, _, err := db.rd.ReadLine()
		if err == io.EOF {
			break
		}

		u, err := parseUser(string(line))
		if err != nil {
			return db, 0, err
		}
		if u.UID >= minId && u.UID <= maxId {
			listId = append(listId, u.UID)
		}
	}
	sort.Ints(listId)
	//fmt.Println(listId)

	switch len(listId) {
	case 0:
		uid = minId
	case 1:
		// Sum 1 to the last value
		uid = listId[0]
		uid++
	default:
		// May have ids unused
		nextId := listId[0]
		found := false

		for _, v := range listId {
			if v != nextId {
				uid = nextId
				found = true
				break
			}
			nextId++
		}
		if !found {
			uid = listId[len(listId)-1]
			uid++
		}
	}

	if uid == maxId {
		return db, 0, &IdRangeError{maxId, isSystem, true}
	}
	return
}

// nextGUID returns the next free group id to use, according to whether it is a
// system's group.
func nextGUID(isSystem bool) (db *dbfile, gid int, err error) {
	loadConfig()

	db, err = openDBFile(fileGroup, os.O_RDWR)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			db.close()
		}
	}()

	// Seek to file half size.

	info, err := db.file.Stat()
	if err != nil {
		return db, 0, err
	}
	if _, err = db.file.Seek(info.Size()/2, io.SeekStart); err != nil {
		return db, 0, err
	}
	// To starting to read from a new line
	if _, _, err = db.rd.ReadLine(); err != nil {
		return db, 0, err
	}

	var minId, maxId int
	var listId []int

	if isSystem {
		minId, maxId = config.login.SYS_GID_MIN, config.login.SYS_GID_MAX
	} else {
		minId, maxId = config.login.GID_MIN, config.login.GID_MAX
	}

	for {
		line, _, err := db.rd.ReadLine()
		if err == io.EOF {
			break
		}

		gr, err := parseGroup(string(line))
		if err != nil {
			return db, 0, err
		}
		if gr.GID >= minId && gr.GID <= maxId {
			listId = append(listId, gr.GID)
		}
	}
	sort.Ints(listId)
	//fmt.Println(listId)

	switch len(listId) {
	case 0:
		gid = minId
	case 1:
		// Sum 1 to the last value
		gid = listId[0]
		gid++
	default:
		// May have ids unused
		nextId := listId[0]
		found := false

		for _, v := range listId {
			if v != nextId {
				gid = nextId
				found = true
				break
			}
			nextId++
		}
		if !found {
			gid = listId[len(listId)-1]
			gid++
		}
	}

	if gid == maxId {
		return db, 0, &IdRangeError{maxId, isSystem, false}
	}
	return
}

// NextSystemUID returns the next free system user id to use.
func NextSystemUID() (int, error) {
	db, uid, err := nextUID(true)
	if err != nil {
		return uid, err
	}

	err = db.close()
	return uid, err
}

// NextSystemGID returns the next free system group id to use.
func NextSystemGID() (int, error) {
	db, gid, err := nextGUID(true)
	if err != nil {
		return gid, err
	}

	err = db.close()
	return gid, err
}

// NextUID returns the next free user id to use.
func NextUID() (int, error) {
	db, uid, err := nextUID(false)
	if err != nil {
		return uid, err
	}

	err = db.close()
	return uid, err
}

// NextGID returns the next free group id to use.
func NextGID() (int, error) {
	db, gid, err := nextGUID(false)
	if err != nil {
		return gid, err
	}

	err = db.close()
	return gid, err
}

// * * *

// IdRangeError records an error during the search for a free id to use.
type IdRangeError struct {
	LastId   int
	IsSystem bool
	IsUser   bool
}

func (e *IdRangeError) Error() string {
	str := ""
	if e.IsSystem {
		str = "system "
	}
	if e.IsUser {
		str += "user: "
	} else {
		str += "group: "
	}
	str += strconv.Itoa(e.LastId)

	return "reached maximum identifier in " + str
}
