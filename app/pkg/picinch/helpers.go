// Copyright © Rob Burke inchworks.com, 2020.

// This file is part of PicInch.
//
// PicInch is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// PicInch is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with PicInch.  If not, see <https://www.gnu.org/licenses/>.

package picinch

import (
	"crypto/rand"
	"net/http"
	"math/big"
	"os"
	"strings"
)

// fremPage returns the referring page
// ## Not used - it is more complex than this. Must recognise own pages and handle "/userId" etc.
func FromPage(path string) string {

	// remove trailing forward slash.
	if strings.HasSuffix(path, "/") {
		nLastChar := len(path) - 1
		path = path[:nLastChar]
	}
	// get final element
	els := strings.Split(path, "/")
	final := els[len(els)-1]

	if final == "" {
		return "/"
	}
	return final
}

// SecureCode returns an access code for a shared slideshow, shared topic, or a validation email.
// n pecifies the number of characters to show the code in base-36.
func SecureCode(nChars int) (int64, error) {
	n := int64(nChars)

	// generate exact number of characters, just for neatness
	// (using big because crypto needs it, not because the numbers get large
	min := new(big.Int).Exp(big.NewInt(36), big.NewInt(n-1), nil) 
	max := new(big.Int).Exp(big.NewInt(36), big.NewInt(n), nil)
	max.Sub(max, min)

	// OK, cryptographically secure generation is overkill for this use.
	code, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}
	return code.Add(code, min).Int64(), nil
}

// ServeFile returns a file as an HTTP response.
// Implementation is needed because http.ServeFile does not support file systems.
// This version is a simplified copy of http.serveFile, omitting:
// - the check for a path with ".."
// - handling of index.html
// - redirection to canonical path
// - directory listing.
func ServeFile(w http.ResponseWriter, r *http.Request, fs http.FileSystem, name string) {

	f, err := fs.Open(name)
	if err != nil {
		msg, code := toHTTPError(err)
		http.Error(w, msg, code)
		return
	}
	defer f.Close()

	d, err := f.Stat()
	if err != nil {
		msg, code := toHTTPError(err)
		http.Error(w, msg, code)
		return
	}

	// serveContent will check modification time
	http.ServeContent(w, r, d.Name(), d.ModTime(), f)
}

// toHTTPError converts OS errors to HTTP errors.
// This implementation is identical to http.toHTTPError.
func toHTTPError(err error) (msg string, httpStatus int) {
	if os.IsNotExist(err) {
		return "404 page not found", http.StatusNotFound
	}
	if os.IsPermission(err) {
		return "403 Forbidden", http.StatusForbidden
	}
	// Default:
	return "500 Internal Server Error", http.StatusInternalServerError
}
