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
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
)

// copyFile copies a file to the specified directory.
func CopyFile(toDir, name, from string) error {
	var src *os.File
	var dst *os.File
	var err error

	if src, err = os.Open(from); err != nil {
		return err
	}
	defer src.Close()

	if name == "" {
		name = filepath.Base(from)
	}

	if dst, err = os.Create(filepath.Join(toDir, name)); err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return nil
}

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
