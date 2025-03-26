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
	"net/http"

	"strings"
	"sync"
	"time"

	"inchworks.com/picinch/internal/cache"
	"inchworks.com/picinch/internal/models"
)

// ## Doesn't need to be an object? No state between functions.

type GalleryState struct {
	app        *Application
	muGallery  sync.RWMutex
	rollbackTx bool

	// cached state
	gallery     *models.Gallery
	highlights  []string // highlighted images
	publicPages *cache.PageCache
	usersHidden int

	// for browser caching
	muCache         sync.RWMutex
	lastModified    time.Time      // (truncated to one second precision for HTTP Last-Modified header)
	lastModifiedS   string         // formatted for HTTP headers
	publicSlideshow map[int64]bool // true for public cache, false for private
}

// Initialisation
func (s *GalleryState) Init(a *Application, usersHidden int) {
	s.app = a
	s.usersHidden = usersHidden
}

// Begin implements the DB interface for uploader.
func (s *GalleryState) Begin() func() {

	return s.updatesGallery()
}

// Cache highlight image names

func (s *GalleryState) cacheHighlights() error {

	// highlight slides, most recent first
	slides := s.app.SlideStore.RecentForTopic(s.app.SlideshowStore.HighlightsId, s.usersHidden, s.app.cfg.MaxHighlights, s.app.cfg.MaxHighlightsParent)

	// cache the image names
	var images []string
	for _, slide := range slides {
		images = append(images, slide.Image)
	}
	s.highlights = images

	return nil
}

// cachePages builds the cache of information pages.
// It returns a list of menu confict warnings.
func (s *GalleryState) cachePages() []string {

	s.updatesNone()()

	var warn []string

	// reset cache
	cache := cache.NewPages()
	s.publicPages = cache

	// add any menu template files (always public)
	for file := range s.app.templateCache {
		if strings.HasPrefix(file, "menu-") {
			cache.AddFile(file, "menu-", ".page.tmpl")
		}
	}

	// add public diaries and information pages
	for _, pg := range s.app.PageStore.AllVisible(models.SlideshowPublic) {
		ss := s.app.SlideStore.ForSlideshowOrdered(pg.Id, false, 100)  // ## configure max
		w := cache.AddPage(pg, ss)
		if len(w) > 0 {
			warn = append(warn, w...)
		}
	}

	// ## add private information slideshows to separate cache, if supported

	// build
	cache.BuildMenus()

	return warn
}

// homeId returns the slideshow ID for the home page.
func (s *GalleryState) homeId() int64 {

	s.updatesNone()()

	return 	s.publicPages.Infos["/"].Id
}

// isHome returns true if the slideshow ID is for home page information.
func (s *GalleryState) isHome(id int64) bool {

	s.updatesNone()()

	return s.publicPages.Paths[id] == "/"
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

// setLastModified saves the last gallery update time for browser caching.
func (s *GalleryState) setLastModified() {

	// serialised
	s.muCache.Lock()

	// truncated to remove sub-second resolution so that it matches HTTP If-Modified-Since
	s.lastModified = time.Now().Truncate(time.Second)
	s.lastModifiedS = s.lastModified.UTC().Format(http.TimeFormat)

	// reset slideshow information
	s.publicSlideshow = make(map[int64]bool, 20)

	s.muCache.Unlock()
}

// Setup cached context

func (s *GalleryState) setupCache(g *models.Gallery) (warn []string, err error) {

	// cache gallery record for dynamic parameters
	s.gallery = g

	// assume everything has changed on server restart
	s.setLastModified()

	// information pages
	warn = s.cachePages()

	// cached highlight images
	err = s.cacheHighlights()

	return
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
			q.setLastModified()
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
