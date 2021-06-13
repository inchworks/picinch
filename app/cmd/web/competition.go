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

	"github.com/inchworks/webparts/multiforms"
	"github.com/inchworks/webparts/users"
)

type userTags struct {
	id   int64
	name string
	tags []*slideshowTag
}

// addUserTagsAll updates tag definitions for all users.
func (s *GalleryState) addUserTagsAll() bool {

	// serialisation
	defer s.updatesGallery()()

	// all users with root tags
	us := s.app.userStore.Taggers()

	for _, u := range us {
		if !s.app.addUserTags(u.Id) {
			return false
		}
	}
	return true
}

// displayClasses returns a template and data for competition classes.
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

// forEditSlideshowTags returns a form, with tags for just the current user.
// ## Not useful?
func (s *GalleryState) forEditSlideshowTags1(userId int64, tagRefId int64, tok string) (f *multiforms.Form, title string, tags []*slideshowTag) {

	// serialisation
	defer s.updatesNone()()

	// selected tag
	tagShow := s.app.tagStore.ForReference(tagRefId)
	if tagShow == nil {
		return
	}
	title = tagShow.Name

	// tags to be edited, as specified by the selected tag
	tags = s.app.formTags0(userId, tagShow.SlideshowId, &tagShow.Tag)

	// current data
	var d = make(url.Values)
	f = multiforms.New(d, tok)
	f.Set("nTagRef", strconv.FormatInt(tagRefId, 36))
	return
}

// forEditSlideshowTags returns a form, showing and editing relevant tags.
func (s *GalleryState) forEditSlideshowTags(slideshowId int64, userTagId int64, userId int64, tok string) (f *multiforms.Form, title string, usersTags []*userTags) {

	// serialisation
	defer s.updatesNone()()

	// validate that holder tag is for this user
	ut := s.app.tagStore.GetIf(userTagId)
	if ut == nil || ut.User != userId {
		return
	}

	// all users holding the same tags as this user
	users := s.app.userStore.ForTag(ut.Name)
	for _, u := range users {
		if u.Id != userId {

			// include only users with referenced tags
			cts := s.app.childSlideshowTags(u.TagId, 0, slideshowId)
			if len(cts) > 0 {
				ut := &userTags{
					id:   u.Id,
					name: u.Name,
					tags: cts,
				}
				usersTags = append(usersTags, ut)
			}
		}
	}

	// this user's tags, to edit
	ets := s.app.childSlideshowTags(userTagId, userId, slideshowId)
	if len(ets) > 0 {
		et := &userTags{
			id:   userId,
			name: "Edit",
			tags: ets,
		}
		usersTags = append(usersTags, et)
	}

	// current data
	var d = make(url.Values)
	f = multiforms.New(d, tok)
	f.Set("nShow", strconv.FormatInt(slideshowId, 36))
	f.Set("nUserTag", strconv.FormatInt(userTagId, 36))
	return
}

// onEditSlideshowTags processes a form of tag changes, and returns true for a valid request.
func (s *GalleryState) onEditSlideshowTags(slideshowId int64, userTagId int64, userId int64, f *multiforms.Form) bool {

	// serialisation
	defer s.updatesGallery()()

	// #### validation

	// tags to be edited, as specified by the selected tag, same as form request
	tags := s.app.childSlideshowTags(userTagId, userId, slideshowId)
	return s.app.editTags(f, userId, slideshowId, tags)
}

// editTags recursively processes tag changes.
func (app *Application) editTags(f *multiforms.Form, userId int64, slideshowId int64, tags []*slideshowTag) bool {
	for _, tag := range tags {

		// name for form input and element ID
		// (We just use it to identify the field, and don't trust it as a database ID)
		nm := strconv.FormatInt(tag.id, 36)

		// One of the rubbish parts of HTML. For a checkbox, the name is the tag and any value indicates set.
		// For a radio button, the name is the parent tag and the value is the set tag.
		src := false
		switch tag.format {
		case "C":
			if f.Get(nm) != "" {
				src = true // any value indicates set, "on" is default value
			}

		case "R":
			radio := strconv.FormatInt(tag.parent, 36)
			if f.Get(radio) == nm {
				src = true
			}

		default:
			src = tag.set // not editable
		}

		if src && !tag.set {

			// set tag reference, and do any corresponding actions
			if !app.setTagRef(tag.parent, tag.name, userId, slideshowId, 0, "") {
				return false
			}

		} else if !src && tag.set {

			// drop tag reference, and do any corresponding actions
			if !app.dropTagRef(tag.parent, tag.name, userId, slideshowId) {
				return false
			}
		}

		if !app.editTags(f, userId, slideshowId, tag.children) {
			return false
		}
	}
	return true
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

// onEnterComp processes a competition entry and returns true for valid client request.
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
		Visible: models.SlideshowClub, // ## Private would be better, but needs something else for judges to view.
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

// ForUserTags returns data for user-specific tags.
func (s *GalleryState) userTags(userId int64) *DataTags {

	defer s.updatesNone()()

	return &DataTags{
		Tags: s.dataUserTags(userId),
	}
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
