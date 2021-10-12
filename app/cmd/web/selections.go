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
	"net/url"

	"github.com/inchworks/webparts/multiforms"
)

// displayTagged returns data for slideshows with user-specific tags.
func (s *GalleryState) displayTagged(topicId int64, rootId int64, tagId int64, userId int64, nMax int) *DataTagged {

	defer s.updatesNone()()

	// validate that user has permission for this tag
	if !s.app.tagger.HasPermission(rootId, userId) {
		return nil
	}

	// ## should validate that tag is a child of the root

	parentName, tagName := s.app.tagger.Names(tagId)

	// get slideshows, tagged for user
	slideshows := s.app.SlideshowStore.ForTagUser(tagId, userId, nMax)

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

	return &DataTagged{
		NRoot:      rootId,
		Parent:     parentName,
		Tag:        tagName,
		Slideshows: dShows,
	}
}

// displayUserTags returns a tree of all tags assigned to the user, with reference counts.
func (s *GalleryState) displayUserTags(userId int64) *DataTags {

	defer s.updatesNone()()

	return &DataTags{
		Tags: s.app.dataTags(s.app.tagger.TagStore.ForUser(userId), 0, 0, userId),
	}
}

// ForSelectSlideshow returns a form to select a slideshow by its ID.
func (s *GalleryState) forSelectSlideshow(tok string) (f *multiforms.Form) {

	// serialisation
	defer s.updatesNone()()

	// current data
	var d = make(url.Values)
	f = multiforms.New(d, tok)
	f.Set("nShow", "")

	return
}

// OnSelectSlideshow checks if the specified slideshow exists.
func (s *GalleryState) onSelectSlideshow(slideshowId int64) bool {

	// serialisation
	defer s.updatesNone()()

	return s.app.SlideshowStore.GetIf(slideshowId) != nil
}
