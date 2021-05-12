// Copyright © Rob Burke inchworks.com, 2021.

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

// Processing for a competition.
//
// These functions may modify application state.

import (
	"database/sql"
	"net/url"
	"strconv"
	"time"

	"inchworks.com/picinch/pkg/form"
	"inchworks.com/picinch/pkg/images"
	"inchworks.com/picinch/pkg/models"

	"github.com/inchworks/webparts/users"
)

// DisplayComp returns a template and data for competition classes.
func (s *GalleryState) displayClasses(member bool) (string, *dataCompetition) {

	defer s.updatesNone()()

	a := s.app

	// ## restrict to published categories
	dShows := s.dataShowsPublished(
		a.SlideshowStore.AllEditableTopics(), a.cfg.MaxSlideshowsPublic, a.cfg.MaxSlideshowsTotal)

	// template and its data
	return "classes.page.tmpl", &dataCompetition{
		Categories: dShows,
	}
}

// ForEditGallery returns a competition entry form.
func (s *GalleryState) forEnterComp(categoryId int64, tok string) (*form.PublicCompForm, string, error) {

	// serialisation
	defer s.updatesNone()()

	// get the category topic
	show, err := s.app.SlideshowStore.Get(categoryId)
	if err != nil {
		return nil, "", err
	}

	// initial data
	var d = make(url.Values)
	f := form.NewPublicComp(d, 1, tok)
	f.Set("category", strconv.FormatInt(show.Id, 36))

	// generate request timestamp for uploaded images (we don't have a user ID yet)
	f.Set("timestamp", strconv.FormatInt(time.Now().UnixNano(), 36))

	return f, show.Title, nil
}

// OnEnterComp processes a competition entry and returns true for valid client request.
// ## Temporary - returns show Id for auto-validation.
func (s *GalleryState) onEnterComp(categoryId int64, timestamp string, name string, email string, location string, title string, caption string, image string, nAgreed int) int64 {

	// serialisation
	defer s.updatesGallery()()

	// create user for entry
	u, err := s.app.userStore.GetNamed(email)
	if err != nil && !s.app.userStore.IsNoRecord(err) {
		return 0
	}
	if u == nil {
		u = &users.User{
			Username: email,
			Name:     name,
			Password: []byte(""),
			Created:  time.Now(),
		}
		if err = s.app.userStore.Update(u); err != nil {
			return 0
		}
	}

	// generate validation code for public entry
	vc, err := secureCode(8)
	if err != nil {
		s.app.errorLog.Print(err)
		return 0
	}

	// create slideshow for entry
	show := &models.Slideshow{
		User:    sql.NullInt64{Int64: u.Id, Valid: true},
		Shared:  vc,
		Topic:   categoryId,
		Revised: time.Now(),
		Title:   title,
		Caption: location,
		Format:  "E",
	}
	if err = s.app.SlideshowStore.Update(show); err != nil {
		return 0
	}

	// create slide for image
	// ## a future version will allow multiple slides
	slide := &models.Slide{
		Slideshow: show.Id,
		Revised:   time.Now(),
		Caption:   caption,
		Image:     images.FileFromName(timestamp, image, 0),
	}
	if err = s.app.SlideStore.Update(slide); err != nil {
		return 0
	}

	// tag entry as unvalidated
	s.app.setTagRef(0, "new", 0, show.Id, 0, "")

	// tag agreements (This is a legal requirement, so we make the logic as explicit as possible.)
	if nAgreed > 0 {
		s.app.setTagRef(0, "agreements", 0, show.Id, 0, strconv.Itoa(nAgreed))
	}

	// request worker to generate image version, and remove unused images
	s.app.chShow <- reqUpdateShow{showId: show.Id, timestamp: timestamp, revised: false}

	return vc
}

// Validate tags an entry as validated and returns a template and data to confirm a validated entry.
func (s *GalleryState) validate(code int64) (string, *dataValidated) {

	defer s.updatesGallery()()

	// check if code is valid
	show := s.app.SlideshowStore.GetIfShared(code)
	if show == nil {
		return "", nil
	}

	// remove new tag, which triggers addition of successor tags
	if !s.app.dropTagRef(0, "new", 0, show.Id) {
		// ## warn the user
		return "", nil
	}

	// get confirmation details
	u, err := s.app.userStore.Get(show.User.Int64)
	if err != nil {
		return "", nil
	}
	t, err := s.app.SlideshowStore.Get(show.Topic)
	if err != nil {
		return "", nil
	}

	// validated
	return "validated.page.tmpl", &dataValidated{

		Name:     u.Name,
		Category: t.Title,
		Title:    show.Title,
	}
}
