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

package main

// Processing related to gallery state

import (
	"path"
	"strconv"
	"sync"

	"inchworks.com/picinch/pkg/models"
)

// ## Doesn't need to be an object? No state between functions.

type GalleryState struct {
	app       *Application
	muGallery sync.RWMutex
	rollbackTx  bool

	// cached state
	gallery    *models.Gallery
	highlights []string // highlighted images
}

// Initialisation
func (s *GalleryState) Init(a *Application) {
	s.app = a
}

// Begin implements the DB interface for uploader.
func (s *GalleryState) Begin() func() {

	return s.updatesGallery()
}

// Cache highlight image names

func (s *GalleryState) cacheHighlights() error {

	// highlight slides, most recent first
	slides := s.app.SlideStore.RecentForTopic(s.app.SlideshowStore.HighlightsId, s.app.cfg.MaxHighlights, s.app.cfg.MaxHighlightsParent)

	// cache the image names
	var images []string
	for _, slide := range slides {
		images = append(images, slide.Image)
	}
	s.highlights = images

	return nil
}

// Construct response URL

func respPath(route string, display string, nRound int, index int) string {

	// URL
	path := path.Join("/", route, display, strconv.Itoa(nRound))

	// add slide index
	if index > 0 {
		path = path + "#slide-" + strconv.Itoa(index)
	}

	return path
}

// rollback must be called on all error returns from any function that calls updatesGallery.
// It returns an HTTP status that indicates whether the error is thought to be a fault on the client or server side. 
func (s *GalleryState) rollback(httpStatus int, err error) int {

	s.rollbackTx = true
	if err != nil {
		s.app.log(err)
	}

	return httpStatus
}

// Commit changes and start new transaction

func (s *GalleryState) save() {

	s.app.tx.Commit()
	s.app.tx = s.app.db.MustBegin()
}

// Setup cached context

func (s *GalleryState) setupCache(g *models.Gallery) error {

	// cache gallery record for dynamic parameters
	s.gallery = g

	// cached highlight images
	return s.cacheHighlights()
}

// Take mutex and start transaction for update to gallery and, possibly, displays
//
// Returns an anonymous function to be deferred. Call as: "defer updatesGallery() ()".

func (q *GalleryState) updatesGallery() func() {

	// acquire exclusive locks
	q.muGallery.Lock()

	// start transaction
	q.app.tx = q.app.db.MustBegin()
	q.rollbackTx = false

	return func() {

		// end transaction
		if q.rollbackTx {
			q.app.tx.Rollback()
		} else {
			q.app.tx.Commit()
		}

		q.app.tx = nil

		// release locks
		q.muGallery.Unlock()
	}
}

// Take mutexes for non-updating request

func (q *GalleryState) updatesNone() func() {

	// acquire shared locks
	q.muGallery.RLock()
	q.rollbackTx = false

	return func() {

		// release lock
		q.muGallery.RUnlock()
	}
}
