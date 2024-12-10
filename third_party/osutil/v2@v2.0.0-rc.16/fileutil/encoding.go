// Copyright 2019 Joan Meg - All Rights Reserved.

package fileutil

import (
	"encoding/gob"
	"os"
)

// ReadGob gets data from a Go binary value.
func ReadGob(filePath string, x interface{}) (err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	err = gob.NewDecoder(file).Decode(x)

	if err2 := file.Close(); err2 != nil && err == nil {
		err = err2
	}
	return
}

// WriteGob exports data to a Go binary value.
func WriteGob(filePath string, x interface{}) (err error) {
	file, err := os.OpenFile(
		filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_SYNC, 0660,
	)
	if err != nil {
		return err
	}

	err = gob.NewEncoder(file).Encode(x)

	if err2 := file.Close(); err2 != nil && err == nil {
		err = err2
	}
	return
}
