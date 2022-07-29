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

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/inchworks/webparts/multiforms"
	"inchworks.com/picinch/internal/models"
)

// displayTagged returns data for slideshows with user-specific tags.
func (s *GalleryState) displayTagged(topicId int64, rootId int64, tagId int64, forUserId int64, byUserId int64, role int, nMax int) (status int, dt *DataTagged) {

	defer s.updatesNone()()

	// validate that user has permission for this tag
	if role < models.UserAdmin && !s.app.tagger.HasPermission(rootId, byUserId) {
		status = http.StatusUnauthorized
		return
	}

	// ## should validate that tag is a child of the root

	parentName, tagName := s.app.tagger.Names(tagId)

	// get slideshows, tagged for user
	var slideshows []*models.SlideshowTagRef
	if forUserId == 0 {
		slideshows = s.app.SlideshowStore.ForTagSystem(tagId, nMax)

	} else {
		slideshows = s.app.SlideshowStore.ForTagUser(tagId, forUserId, nMax)
	}

	// ## no support for topic-specific

	var dShows []*DataPublished

	for _, sh := range slideshows {
		dShows = append(dShows, &DataPublished{
			Id:          sh.Id,
			Title:       sh.Title,
			Image:       sh.Image,
			DisplayName: sh.Caption,
			NTagRef:     sh.TagRefId,
		})
	}

	dt = &DataTagged{
		NRoot:      rootId,
		NUser:      forUserId,
		Parent:     parentName,
		Tag:        tagName,
		Slideshows: dShows,
	}
	return
}

// displayUserTags returns a tree of all tags assigned to the user, with reference counts.
func (s *GalleryState) displayUserTags(userId int64, role int) *DataTags {

	defer s.updatesNone()()

	if role >= models.UserAdmin {

		// root tags for system
		ts := s.app.tagger.TagStore.ForSystem()
		dts := s.app.dataTags(ts, 0, 0, 0)

		// root tags for all users
		tus := s.app.tagger.TagStore.AllRoot()

		for _, tu := range tus {
			// prefix tag name with user ID
			tu.Tag.Name = tu.UsersName + " : " + tu.Tag.Name

			// process the root tags one by one
			tsUser := []*models.Tag{&tu.Tag}
			dtsUser := s.app.dataTags(tsUser, 0, 0, tu.UserId)
			dts = append(dts, dtsUser...)
		}
		return &DataTags{
			Tags: dts,
		}

	} else if role >= models.UserFriend {
		// root tags for this user
		ts := s.app.tagger.TagStore.ForUser(userId)
		for _, t := range ts {
			t.Name = "Own : " + t.Name
		}
		dts := s.app.dataTags(ts, 0, 0, userId)

		for _, t := range ts {
			// root tags for team members (sharing the root tag)
			tus := s.app.tagger.TagStore.ForTeam(t.Id)
			for _, tu := range tus {

				if tu.UserId != userId {
					// prefix tag name with user
					tu.Tag.Name = tu.UsersName + " : " + tu.Tag.Name

					// process the root tags one by one
					tsUser := []*models.Tag{&tu.Tag}
					dtsUser := s.app.dataTags(tsUser, 0, 0, tu.UserId)
					dts = append(dts, dtsUser...)
				}
			}
		}
		return &DataTags{
			Tags: dts,
		}

	} else {
		// root tags for a normal user
		tags := s.app.tagger.TagStore.ForUser(userId)
		return &DataTags{
			Tags: s.app.dataTags(tags, 0, 0, userId),
		}
	}

}

// forEditSlideshowTags returns a form, showing and editing relevant tags.
func (s *GalleryState) forEditSlideshowTags(slideshowId int64, rootId int64, forUserId int64, byUserId int64, role int, tok string) (status int, f *multiforms.Form, title string, usersTags []*userTags) {

	// serialisation
	defer s.updatesNone()()

	// validate that user has permission for this tag
	if role < models.UserAdmin {
		if !s.app.tagger.HasPermission(rootId, byUserId) {
			status = http.StatusUnauthorized
			return
		}
		// edit as self, not as team member
		forUserId = byUserId
	}

	// slideshow title
	show := s.app.SlideshowStore.GetIf(slideshowId)
	if show == nil {
		status = http.StatusNotFound
		return
	}
	title = show.Title

	// all users holding the same tags as this user
	users := s.app.userStore.ForTag(rootId)
	for _, u := range users {
		if u.Id != forUserId {

			// include only users with referenced tags
			cts := s.app.tagger.ChildSlideshowTags(slideshowId, rootId, u.Id, false)
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
	ets := s.app.tagger.ChildSlideshowTags(slideshowId, rootId, forUserId, true)
	if len(ets) > 0 {
		et := &userTags{
			id:   forUserId,
			name: "Me",
			tags: ets,
		}
		usersTags = append(usersTags, et)
	}

	// current data
	var d = make(url.Values)
	f = multiforms.New(d, tok)
	f.Set("nShow", strconv.FormatInt(slideshowId, 36))
	f.Set("nRoot", strconv.FormatInt(rootId, 36))
	f.Set("nUser", strconv.FormatInt(forUserId, 36))
	return
}

// onEditSlideshowTags processes a form of tag changes, and returns an HHTP status (0 for a valid request).
func (s *GalleryState) onEditSlideshowTags(slideshowId int64, rootId int64, forUserId int64, byUserId int64, role int, f *multiforms.Form) int {

	// serialisation
	defer s.updatesGallery()()

	// validate that user has permission for this tag
	if role < models.UserAdmin {
		if !s.app.tagger.HasPermission(rootId, byUserId) || forUserId != byUserId {
			return s.rollback(http.StatusBadRequest, nil)
		}
	}

	// tags to be edited, as specified by the selected tag, same as form request
	tags := s.app.tagger.ChildSlideshowTags(slideshowId, rootId, forUserId, true)
	if s.app.editTags(f, forUserId, slideshowId, tags) {
		return 0
	} else {
		return s.rollback(http.StatusInternalServerError, nil)
	}
}

// forSelectSlideshow returns a form to select a slideshow by its ID.
func (s *GalleryState) forSelectSlideshow(tok string) (f *multiforms.Form) {

	// serialisation
	defer s.updatesNone()()

	// current data
	var d = make(url.Values)
	f = multiforms.New(d, tok)
	f.Set("nShow", "")

	return
}

// onSelectSlideshow checks if the specified slideshow exists.
func (s *GalleryState) onSelectSlideshow(slideshowId int64) bool {

	// serialisation
	defer s.updatesNone()()

	return s.app.SlideshowStore.GetIf(slideshowId) != nil
}

// INTERNAL FUNCTIONS

// dataTags returns all referenced tags, with child tags
func (app *Application) dataTags(tags []*models.Tag, level int, rootId int64, userId int64) []*DataTag {

	var dTags []*DataTag

	for _, t := range tags {

		// note the root tags (needed for selection of tags to be edited)
		if level == 0 {
			rootId = t.Id
		}

		// references
		n := app.tagger.TagRefStore.CountItems(t.Id, userId)
		children := app.dataTags(app.tagger.TagStore.ForParent(t.Id), level+1, rootId, userId)

		// skip unreferenced tags
		if n+len(children) > 0 {

			var sCount, sDisable string
			if n > 0 {
				sCount = strconv.Itoa(n)
			} else {
				sDisable = "disabled"
			}

			dTags = append(dTags, &DataTag{
				NRoot:   rootId,
				NTag:    t.Id,
				ForUser: userId,
				Name:    t.Name,
				Count:   sCount,
				Disable: sDisable,
				Indent:  "offset-" + strconv.Itoa(level*2),
			})
			dTags = append(dTags, children...)
		}
	}
	return dTags
}
