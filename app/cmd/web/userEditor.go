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

// Processing for user data.
//
// These functions may modify application state.

import (
	"net/url"
	"time"

	"inchworks.com/picinch/pkg/form"
	"inchworks.com/picinch/pkg/models"
)

// Get data to edit users

func (s *GalleryState) ForEditUsers() (f *form.UsersForm) {

	// serialisation
	defer s.updatesNone()()

	// users
	users := s.app.UserStore.ByName()

	// form
	var d = make(url.Values)
	f = form.NewUsers(d)

	// add template and users to form
	f.AddTemplate()
	for i, u := range users {
		f.Add(i, u)
	}

	return
}

// Processing when users are modified.
//
// Returns true if no client errors.

func (s *GalleryState) OnEditUsers(usSrc []*form.UserFormData) bool {

	// serialisation
	defer s.updatesGallery()()

	// compare modified users against current users, and update
	usDest := s.app.UserStore.ByName()

	nSrc := len(usSrc)
	nDest := len(usDest)

	// skip template
	iSrc := 1
	var iDest int

	for iSrc < nSrc || iDest < nDest {

		if iSrc == nSrc {
			// no more source users - delete from destination
			s.onRemoveUser(usDest[iDest])
			iDest++

		} else if iDest == nDest {
			// no more destination users - add new user
			u := models.User{
					Name: usSrc[iSrc].DisplayName,
					Username: usSrc[iSrc].Username,
					Status: usSrc[iSrc].Status,
					Password: []byte(""),
			}
			s.app.UserStore.Update(&u)
			iSrc++

		} else {
			ix := usSrc[iSrc].ChildIndex
			if ix > iDest {
				// source user removed - delete from destination
				s.onRemoveUser(usDest[iDest])
				iDest++

			} else if ix == iDest {
				// check if user's details changed
				uSrc := usSrc[iSrc]
				uDest := usDest[iDest]
				if uSrc.DisplayName != uDest.Name ||
					uSrc.Username != uDest.Username ||
					uSrc.Status != uDest.Status {
					uDest.Name = uSrc.DisplayName
					uDest.Username = uSrc.Username
					uDest.Status = uSrc.Status
					if err := s.app.UserStore.Update(uDest); err != nil {
						return false  // unexpected database error
					}
				}
				iSrc++
				iDest++

			} else {
				// out of sequence team index
				return false
			}
		}
	}

	return true
}

// Signup user
//
// Assumes serialisation started earlier

func (s *GalleryState) OnUserSignup(user *models.User, name string, password string) error {

	// serialisation
	// #### should serialise from call to CanSignup
	defer s.updatesGallery()()

	// set details for active user
	user.Name = name
	user.SetPassword(password) // encrypted password
	user.Status = models.UserActive
	user.Created = time.Now()

	return s.app.UserStore.Update(user)
}

// Get user's display name

func (s *GalleryState) UserDisplayName(userId int64) string {

	// serialisation
	defer s.updatesNone()()

	r, _ := s.app.UserStore.Get(userId)

	return r.Name
}

// Get users

func (s *GalleryState) Users() []*models.User {

	// Serialisation
	defer s.updatesNone()()

	return s.app.UserStore.All()
}

// Processing when a user is removed

func (s *GalleryState) onRemoveUser(user *models.User) {

	// all slideshow IDs for user
	shows := s.app.SlideshowStore.ForUser(user.Id, models.SlideshowTopic)
	showIds := make([]int64, 0, 10)
	topics := make(map[int64]bool)
	for _, show := range shows {
		showIds = append(showIds, show.Id)
		if show.Topic != 0 { topics[show.Topic] = true }
	}

	// slideshows and slides will be removed by cascade delete
	s.app.UserStore.DeleteId(user.Id)

	// remove user's images
	s.app.chShowIds <- showIds

	// change topic images as needed 
	for topicId, _ := range topics {
		s.app.chTopicId <- topicId
	}
}