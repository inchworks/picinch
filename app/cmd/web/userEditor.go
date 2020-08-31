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
	"time"

	"inchworks.com/gallery/pkg/models"
)

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

// Get slideshows in sequence order

func (s *GalleryState) Slideshows() []*models.Slideshow {

	// Serialisation
	defer s.updatesNone()()

	return s.app.SlideshowStore.All()
}

// Get slideshow title

func (s *GalleryState) SlideshowTitle(showId int64) string {

	// serialisation
	defer s.updatesNone()()

	r, _ := s.app.SlideshowStore.Get(showId)

	return r.Title
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