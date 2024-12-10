// Copyright 2010 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package userutil

import (
	"fmt"
	"os"
	"reflect"
)

var isRoot bool

func init() {
	// Root's user ID is 0.
	if os.Getuid() == 0 {
		isRoot = true
	}
}

// checkRoot checks if the user is root.
func checkRoot() {
	if !isRoot {
		panic("you have to be Root")
	}
}

// exist checks if the file exists.
func exist(file string) (bool, error) {
	if _, err := os.Stat(file); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// printStruct prints a struct to debug.
func printStruct(t interface{}) {
	valueof := reflect.ValueOf(t).Elem()
	typeof := valueof.Type()

	for i := 0; i < valueof.NumField(); i++ {
		f := valueof.Field(i)
		fmt.Printf(" %s (%s) = %v\n",
			typeof.Field(i).Name, f.Type(), f.Interface(),
		)
	}
}

// _SEC_PER_DAY is the number of secons that a day has.
const _SEC_PER_DAY = 24 * 60 * 60

// secToDay converts from secons to days.
func secToDay(sec int64) int { return int(sec / _SEC_PER_DAY) }
