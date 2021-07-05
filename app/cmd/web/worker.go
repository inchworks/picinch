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

// Worker goroutine for all background processing

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"time"

	"inchworks.com/picinch/pkg/images"
	"inchworks.com/picinch/pkg/models"
)

// data for for request to validate competition entry
type validationData struct {
	Name  string
	Entry string
	Link  string
}

// worker does all background processing for PicInch.
func (s *GalleryState) worker(
	chComp <-chan reqUpdateShow,
	chImage <-chan images.ReqSave,
	chShow <-chan reqUpdateShow,
	chShows <-chan []reqUpdateShow,
	chTopic <-chan reqUpdateTopic,
	chRefresh <-chan time.Time,
	done <-chan bool) {

	for {
		// returns to client sooner?
		runtime.Gosched()

		select {
		case req := <-chComp:

			// a competition entry has been added
			if err := s.onCompEntry(req.showId, req.timestamp, req.revised); err != nil {
				s.app.errorLog.Print(err.Error())
			}

		case req := <-chImage:

			// resize and save image, with thumbnail
			if err := s.app.imager.SaveRequested(req); err != nil {
				s.app.errorLog.Print(err.Error())
			}

		case req := <-chShow:

			// a slideshow has been updated or removed
			if err := s.onUpdateShow(req.showId, req.timestamp, req.revised); err != nil {
				s.app.errorLog.Print(err.Error())
			}

		case reqs := <-chShows:

			// a set of slideshows have been updated (e.g. user removed)
			for _, req := range reqs {
				if err := s.onUpdateShow(req.showId, req.timestamp, req.revised); err != nil {
					s.app.errorLog.Print(err.Error())
				}
				runtime.Gosched()
			}

		case req := <-chTopic:

			// a topic slideshow has been updated or removed
			if err := s.onUpdateTopic(req.topicId, req.revised); err != nil {
				s.app.errorLog.Print(err.Error())
			}

		case <-chRefresh:

			// refresh topic thumbnails
			if err := s.onRefresh(); err != nil {
				s.app.errorLog.Print(err.Error())
			}

		case <-done:
			// ## do something to finish other pending requests
			return
		}
	}
}

// onCompEntry processes a competition entry.
func (s *GalleryState) onCompEntry(showId int64, timestamp string, revised bool) error {

	app := s.app

	// slideshow for competition
	if err := s.onUpdateShow(showId, timestamp, revised); err != nil {
		return err
	}

	// entry and user
	defer s.updatesNone()
	show, err := app.SlideshowStore.Get(showId)
	if err != nil {
		return err
	}
	user, err := app.userStore.Get(show.User.Int64)
	if err != nil {
		return err
	} else if user == nil {
		return errors.New("Unknown user for validation")
	}

	// validation link
	var link string
	if len(app.cfg.Domains) > 0 {
		link = fmt.Sprintf("https://%s/validate/%s", app.cfg.Domains[0], strconv.FormatInt(show.Shared, 36))
	} else {
		link = fmt.Sprintf("http://localhost:8000/validate/%s", strconv.FormatInt(show.Shared, 36)) // for testing
	}

	// send validation request
	// ## page.tmpl name is confusing. Actually it means top-level template file.
	return s.app.emailer.Send(user.Username, "validation-request.page.tmpl", &validationData{
		Name:  user.Name,
		Entry: show.Title,
		Link:  link,
	})
}

// Change topic(s) thumbnails

func (s *GalleryState) onRefresh() error {

	defer s.updatesGallery()()

	topics := s.app.SlideshowStore.AllTopics()

	for _, topic := range topics {
		if err := s.updateTopic(topic, false); err != nil {
			return err
		}
	}

	return nil
}

// onUpdateShow processes an updated or deleted slideshow.
func (s *GalleryState) onUpdateShow(showId int64, timestamp string, revised bool) error {

	// images
	im := s.app.imager

	// setup
	if err := im.ReadVersions(showId, timestamp); err != nil {
		return err
	}

	// set versioned images, and update slideshow
	if err := s.updateSlides(showId, revised); err != nil {
		return err
	}

	// remove unused versions
	if err := im.RemoveVersions(); err != nil {
		return err
	}

	// update highlighted images
	return s.updateHighlights(showId)
}

// A slideshow for a topic has been updated or deleted

func (s *GalleryState) onUpdateTopic(id int64, revised bool) error {

	defer s.updatesGallery()()

	topic := s.app.SlideshowStore.GetIf(id)
	if topic == nil {
		return nil
	}

	return s.updateTopic(topic, revised)
}

// Update highlighted images when a highlight slideshow is changed

func (s *GalleryState) updateHighlights(id int64) error {

	show := s.app.SlideshowStore.GetIf(id)
	if show == nil {
		return nil
	}

	// is this for the highlights topic?
	if show.Topic == s.app.SlideshowStore.HighlightsId {
		return s.cacheHighlights()
	}

	return nil
}

// updateSlides changes image versions for a slideshow. It also sets the slideshow revision time.
func (s *GalleryState) updateSlides(showId int64, revised bool) error {

	// serialise display state while slides are changing
	defer s.updatesGallery()()

	// check if this is an update or deletion
	show := s.app.SlideshowStore.GetIf(showId)
	if show == nil {
		// No slides to be updated. A following call to imager.RemoveVersions will delete all images.
		return nil
	}

	thumbnail := ""
	nImages := 0

	// check each slide for an updated image
	slides := s.app.SlideStore.ForSlideshow(showId, 1000)
	for _, slide := range slides {

		if slide.Image != "" {

			var newImage string
			var err error
			if newImage, err = s.app.imager.Updated(slide.Image); err != nil {
				// ## We have lost the image, but have no way to warn the user :-(
				// We must remove the reference so that all viewers don't get a missing file error.
				// log the error, but process the remaining slides
				slide.Image = ""
				s.app.SlideStore.Update(slide)
				s.app.errorLog.Print(err.Error()) 

			} else if newImage != "" {
				// updated image
				slide.Image = newImage
				s.app.SlideStore.Update(slide)
			}

			// use first image in show as its thumbnail
			if thumbnail == "" {
				thumbnail = slide.Image
			}
			nImages++
		}
	}

	// remove empty show for topic
	// ## beware race with user re-opening show to add back an image
	if nImages == 0 && show.Visible == models.SlideshowTopic {
		s.app.SlideshowStore.DeleteId(showId)

	} else {

		// update slideshow thumbnail, if changed
		if thumbnail != show.Image {
			show.Image = thumbnail
		}

		// ## temporary fix for missing creation time
		if show.Topic == 1 && show.Created == (time.Time{}) {
			show.Created = time.Now()
			show.Revised = time.Now()
		}

		// update slideshow revision date
		if revised {
			show.Revised = time.Now()
			if show.Topic == s.app.SlideshowStore.HighlightsId {
				// highlights show moved up display order when images added
				show.Created = show.Revised
			}
		}
		s.app.SlideshowStore.Update(show)
	}

	return nil
}

// updateTopic changes the topic thumbnail, and if required updates the revision date
func (s *GalleryState) updateTopic(t *models.Slideshow, revised bool) error {

	update := false

	if t.Visible >= models.SlideshowClub {
		images := s.app.SlideStore.ImagesForTopic(t.Id)
		nImages := len(images)

		// (beware empty topic)
		if nImages > 0 {
			// select random image for topic thumbnail
			i := int(rand.Float32() * float32(nImages))
			t.Image = images[i]
			update = true
		}
	}
	if revised {
		// change revision date
		t.Revised = time.Now()
		if t.Id == s.app.SlideshowStore.HighlightsId {
			// highlights topic moved up display order when images added
			t.Created = t.Revised
		}
		update = true
	}

	if update {
		if err := s.app.SlideshowStore.Update(t); err != nil {
			return err
		}
	}
	return nil
}
