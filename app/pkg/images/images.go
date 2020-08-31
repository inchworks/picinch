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

package images

// Note that we use revision numbers for two reasons.
// (1) A different name forces browsers to fetch the updated image after an image has been changed.
// (2) It allows us to upload an image without overwriting the current one, and then forget it the update form is not submitted.

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
)

type Imager struct {

	// parameters
	ImagePath string
	MaxW      int
	MaxH      int
	ThumbW    int
	ThumbH    int

	// state
	showId      int64
	versions    map[string]fileVersion
	delVersions []fileVersion
}

type fileVersion struct {
	fileName string
	revision int
	replace  bool
	keep     bool
}

type ReqSave struct {
	ShowId   int64
	Name     string
	Fullsize bytes.Buffer
	Img      image.Image
}

// Make stored file name from user's name for image
//
// Newly uploaded file has no revision, so it doesn't overwrite previous copy yet.

func FileFromName(showId int64, name string, rev int) string {
	if name != "" {
		if rev != 0 {
			return fmt.Sprintf("P-%s$%s-%s",
				strconv.FormatInt(showId, 36),
				strconv.FormatInt(int64(rev), 36),
				name)
		} else {
			return fmt.Sprintf("P-%s-%s", strconv.FormatInt(showId, 36), name)
		}
	} else {
		return ""
	}
}

// Get show ID, image name and revison from file name

func NameFromFile(fileName string) (int64, string, int) {
	if len(fileName) > 0 {
		// ss[0] is "P"
		sf := strings.SplitN(fileName, "-", 3)
		ss := strings.Split(sf[1], "$")
		showId, _ := strconv.ParseInt(ss[0], 36, 64)

		var rev int64
		if len(ss) > 1 {
			rev, _ = strconv.ParseInt(ss[1], 36, 0)
		}
		return showId, sf[2], int(rev)

	} else {
		return 0, "", 0
	}
}

// Load updated versions

func (im *Imager) ReadVersions(showId int64) error {

	// reset state
	im.showId = showId
	im.delVersions = nil

	s := strconv.FormatInt(showId, 36)

	// find new files, and existing ones
	// (newVersions could be just a slice)
	newVersions := im.globVersions(filepath.Join(im.ImagePath, "P-"+s+"-*"))
	im.versions = im.globVersions(filepath.Join(im.ImagePath, "P-"+s+"$*"))

	// generate new revision nunbers
	// Note that fileNames for new files don't have revision numbers yet, we may need to delete some files.
	for name, nv := range newVersions {
		nv.replace = true

		cv := im.versions[name]
		if cv.revision != 0 {

			// current version is to be replaced and deleted
			nv.revision = cv.revision + 1
			im.delVersions = append(im.delVersions, cv)

		} else {

			// this is a new name
			nv.revision = 1
		}
		im.versions[name] = nv
	}

	return nil
}

// ## Then update slides in database, renaming images to current revision as needed

// Delete unused versions

func (im *Imager) RemoveVersions() error {

	// add unreferenced files to the deletion list
	for _, cv := range im.versions {

		if !cv.keep {
			im.delVersions = append(im.delVersions, cv)
		}
	}

	// delete unreferenced and old versions
	for _, cv := range im.delVersions {
		if err := os.Remove(filepath.Join(im.ImagePath, cv.fileName)); err != nil { return err }
		if err := os.Remove(filepath.Join(im.ImagePath, Thumbnail(cv.fileName))); err != nil { return err }
	}

	return nil
}

// Save image

func Save(fh *multipart.FileHeader, showId int64, chImage chan<- ReqSave) (err error, byClient bool) {

	// get image from request header
	file, err := fh.Open()
	if err != nil {
		return err, false
	}
	defer file.Close()

	// duplicate file in buffer, since we can only read it from the header once
	var buffered bytes.Buffer
	tee := io.TeeReader(file, &buffered)

	// decode image
	img, err := imaging.Decode(tee, imaging.AutoOrientation(true))
	if err != nil {
		return err, true // this is a bad image from client
	}

	// resizing is slow, so do the remaining processing in background worker
	chImage <- ReqSave{
		ShowId:   showId,
		Name:     fh.Filename,
		Fullsize: buffered,
		Img:      img,
	}

	return nil, true
}

// Image processing, called from background worker

func (im *Imager) SaveResized(req ReqSave) error {

	// convert non-displayable file types to JPG
	name, convert := changeType(req.Name)

	// path for saved files
	filename := FileFromName(req.ShowId, name, 0)
	savePath := filepath.Join(im.ImagePath, filename)
	thumbPath := filepath.Join(im.ImagePath, Thumbnail(filename))

	// check if uploaded image small enough to save
	size := req.Img.Bounds().Size()
	if size.X <= im.MaxW && size.Y <= im.MaxH && !convert {

		// save uploaded file unchanged
		saved, err := os.OpenFile(savePath, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			return err // could be a bad name?
		}
		defer saved.Close()
		_, err = io.Copy(saved, &req.Fullsize)

	} else {

		// ## set compression option
		// ## could sharpen, but how much?
		// ## give someone else a chance - not sure if it helps
		resized := imaging.Fit(req.Img, im.MaxW, im.MaxH, imaging.Lanczos)
		runtime.Gosched()

		if err := imaging.Save(resized, savePath); err != nil {
			return err // ## could be a bad name?
		}
	}

	// save thumbnail
	thumbnail := imaging.Fit(req.Img, im.ThumbW, im.ThumbH, imaging.Lanczos)
	if err := imaging.Save(thumbnail, thumbPath); err != nil {
		return err
	}
	return nil
}

// Prefixed name from filename

func Thumbnail(filename string) string { return "S" + filename[1:] }

// Check if image file has changed, called from background worker

func (im *Imager) Updated(fileName string) (bool, string, error) {

	// is there an image?
	if fileName == "" {
		return false, "", nil
	}

	// name and revision
	_, name, rev := NameFromFile(fileName)

	// convert non-displayable file types, to match converted image
	name, _ = changeType(name)

	cv := im.versions[name]
	if cv.revision == 0 {
		// we might have no versioned file if the user has just changed the slideshow a second time
		// never mind, we'll fix it on the next call
		return false, "", nil
	}

	var err error
	var updated bool
	if rev != cv.revision {

		// first slide to use the new image?
		if cv.replace {

			// the newly uploaded image is being used on a slide
			cv.fileName, err = im.saveVersion(im.showId, name, cv.revision)
			if err != nil {
				return false, "", err
			}
			cv.replace = false
		}
		updated = true
	}

	// keep this file
	cv.keep = true
	im.versions[name] = cv

	return updated, cv.fileName, nil
}

// Valid type?

func ValidType(name string) bool {

	_, err := imaging.FormatFromFilename(name)
	return err == nil
}

// Change file extension to a displayable type

func changeType(name string) (nm string, changed bool) {

	// convert other file types to JPG
	fmt, err := imaging.FormatFromFilename(name)
	if err != nil { return name, false}  // unikely error, never mind

	switch fmt {
	case imaging.JPEG:
		fallthrough

	case imaging.PNG:
		nm = name
		changed = false

	default:
		// change filename to JPG
		nm = strings.TrimSuffix(name, filepath.Ext(name)) + ".jpg"
		changed = true
	}
	return
}

// Find versions of new or existing files

func (im *Imager) globVersions(pattern string) map[string]fileVersion {

	versions := make(map[string]fileVersion)

	newFiles, _ := filepath.Glob(pattern)
	for _, newFile := range newFiles {

		fileName := filepath.Base(newFile)
		_, name, rev := NameFromFile(fileName)
		versions[name] = fileVersion{
			fileName: fileName,
			revision: rev,
		}
	}

	return versions
}

// Save new file with revision number.

func (im *Imager) saveVersion(showId int64, name string, rev int) (string, error) {

	// the file should already be saved without a revision nuumber
	uploaded := FileFromName(showId, name, 0)
	revised := FileFromName(showId, name, rev)

	// main image ..
	uploadedPath := filepath.Join(im.ImagePath, uploaded)
	revisedPath := filepath.Join(im.ImagePath, revised)
	if err := os.Rename(uploadedPath, revisedPath); err != nil { return revised, err }

	// .. and thumbnail
	uploadedPath = filepath.Join(im.ImagePath, Thumbnail(uploaded))
	revisedPath = filepath.Join(im.ImagePath, Thumbnail(revised))
	err := os.Rename(uploadedPath, revisedPath)

	// rename with a revision number
	return revised, err
}
