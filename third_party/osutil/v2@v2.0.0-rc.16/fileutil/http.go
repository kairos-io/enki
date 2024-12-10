// Copyright 2019 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fileutil

import (
	"io"
	"net/http"
	"net/url"
)

// Dload downloads a file.
func Dload(urlFile string, dst io.Writer) error {
	_, err := url.Parse(urlFile)
	if err != nil {
		return err
	}

	/*fileURL, err := url.Parse(urlFile)
	if err != nil {
		return err
	}*/
	/*path := fileURL.Path
	segments := strings.Split(path, "/")
	filename = segments[len(segments)-1]*/
	/*filename := filepath.Base(fileURL.Path)

	// Create blank file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()*/

	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	// Put content on file
	resp, err := client.Get(urlFile)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(dst, resp.Body)
	return err
}
