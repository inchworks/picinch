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

// Worker for ETX and all other background processing

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"time"

	"github.com/inchworks/webparts/v2/etx"
	"github.com/inchworks/webparts/v2/uploader"
	"github.com/inchworks/webparts/v2/users"

	"inchworks.com/picinch/internal/models"
)

// data for for request to validate competition entry
type validationData struct {
	Name  string
	Entry string
	Link  string
}

// Implement RM interface for webparts.etx.

// Operation types (keep unchanged)
const (
	OpComp    = 0
	OpShowV1  = 1
	OpTopic   = 2
	OpShow    = 3
	OpDropShow = 4
	OpDropUser = 5
	OpRelease = 6
)

// We need an arbitary status code for rollback(). This one is ideal!
const statusTeapot = 418

// Name, ForOperation and Operation implement the RM interface for webparts.etx.

func (s *GalleryState) Name() string {
	return "gallery"
}

func (s *GalleryState) ForOperation(opType int) etx.Op {
	switch opType {
	case OpComp:
		return &OpValidate{}
	case OpShow, OpShowV1:
		return &OpUpdateShow{}
	case OpRelease:
		return &OpReleaseShow{}
	case OpTopic:
		return &OpUpdateTopic{}
	case OpDropShow, OpDropUser:
		return &OpDrop{}
	default:
		var unknown struct{}
		return &unknown
	}
}

// Do operation requested via TM.
func (s *GalleryState) Operation(tx etx.TxId, opType int, op etx.Op) {

	switch req := op.(type) {

	case *OpDrop:
		switch opType {
		case OpDropShow:
			// final deletion of slideshow, or reduction in visibility
			s.onDropShow(tx, req.Id, req.Access)

		case OpDropUser:
			// final deletion of user
			s.onDropUser(tx, req.Id, req.Access)
		}

	case *OpReleaseShow:
		// final release of show from topic
		s.onReleaseShow(tx, req.ShowId, req.TopicId)

	case *OpUpdateShow:
		switch opType {
		case OpShow:
			// a slideshow has been updated or removed
			s.onUpdateShow(tx, req.ShowId, req.TopicId, req.Revised)

		case OpShowV1:
			// a slideshow has been updated or removed
			s.onUpdateShowV1(tx, req.ShowId, req.TopicId, req.Revised)
		}

	case *OpUpdateTopic:
		// change topic thumbnails asynchronously
		req.tx = tx
		s.app.chTopic <- *req
	
	case *OpValidate:
		// a competition entry has been added
		s.onCompEntry(req.ShowId, tx)

	default:
		s.app.errorLog.Print("Unknown TX operation")
	}
}

// worker does timer processing for PicInch.
func (s *GalleryState) worker(
	chTopic <-chan OpUpdateTopic,
	chRefresh <-chan time.Time,
	chPurge <-chan time.Time,
	done <-chan bool) {

	// refresh topic thumbnails on restart
	s.onRefresh()

	for {
		// returns to client sooner?
		runtime.Gosched()

		select {

		case op := <-chTopic:
			// a topic slideshow has been updated or removed
			s.onUpdateTopic(op.TopicId, op.tx, op.Revised)
			s.app.tm.Do(op.tx)

		case <-chRefresh:

			// refresh topic thumbnails
			s.onRefresh()

		case t := <-chPurge:

			// purge unvalidated competition entries
			tx := s.onPurge(t)
			s.app.tm.Do(tx)

		case <-done:
			// ## do something to finish other pending requests
			return
		}
	}
}

// onAbandon terminates an update that does not need more processing.
func (s *GalleryState) onAbandon(tx etx.TxId) {

	defer s.updatesGallery()()

	// terminate this operation
	if err := s.app.tm.End(tx); err != nil {
		s.app.log(err)
	}
}

// onBindShow sets uploaded media for a new or updated slideshow.
func (s *GalleryState) onBindShow(showId int64, topicId int64, revised bool, tx etx.TxId) {

	defer s.updatesGallery()()

	// setup
	bind := s.app.uploader.StartBind(tx)

	// set versioned images, and update slideshow
	s.bindFiles(showId, revised, bind)

	// update highlighted images
	if err := s.updateHighlights(showId); err != nil {
		s.app.log(err)
	}

	// all bound
	if err := bind.End(); err != nil {
		s.app.log(err)
	}

	// update topic, to drop any old thumbnails and choose new ones
	if topicId != 0 {
		topic := s.app.SlideshowStore.GetIf(topicId)
		if topic != nil {
			if err := s.updateTopic(topic, revised); err != nil {
				s.app.log(err)
			}
		}
	}

	// terminate this operation
	if err := s.app.tm.End(tx); err != nil {
		s.app.log(err)
	}
}

// onCompEntry processes a competition entry.
func (s *GalleryState) onCompEntry(showId int64, tx etx.TxId) {

	app := s.app

	// entry and user
	defer s.updatesGallery()()

	show, err := app.SlideshowStore.Get(showId)
	if err != nil {
		s.app.log(err)
		return
	} else if show == nil {
		s.app.log(errors.New("Unknown slideshow for validation"))
	}

	user, err := app.userStore.Get(show.User.Int64)
	if err != nil {
		s.app.log(err)
		return
	} else if user == nil {
		s.app.log(errors.New("Unknown user for validation"))
		return
	}

	// validation link
	var link string
	if len(app.cfg.Domains) > 0 {
		link = fmt.Sprintf("https://%s/validate/%s", app.cfg.Domains[0], strconv.FormatInt(show.Shared, 36))
	} else {
		link = fmt.Sprintf("http://localhost:8000/validate/%s", strconv.FormatInt(show.Shared, 36)) // for testing
	}

	// send validation request
	// (page.tmpl name is confusing. Actually it means top-level template file.)
	err = s.app.emailer.Send(user.Username, "validation-request.page.tmpl", &validationData{
		Name:  user.Name,
		Entry: show.Title,
		Link:  link,
	})
	if err != nil {
		// ## Should we do more than rollback the processing if email has failed?
		s.app.log(err)
	}

	// terminate this operation
	if err := s.app.tm.End(tx); err != nil {
		s.app.log(err)
	}
}

// onDropShow reduces access a slideshow, or deletes it with its images, deferred to support caching.
func (s *GalleryState) onDropShow(tx etx.TxId, id int64, access int) {

	defer s.updatesGallery()()

	// terminate the extended transaction
	if err := s.app.tm.End(tx); err != nil {
		s.app.log(err)
		return
	}
	
	st := s.app.SlideshowStore
	show, err := st.Get(id)
	if err != nil {
		s.app.log(err)
		return
	}

	// nothing to do if visibility has been increased in the meantime
	if show.Visible > access {
		return
	}

	if access == models.SlideshowRemoved {
		if show.User.Valid {
			// delete images
			s.app.deleteImages(id)
		} else {
				// release topic slideshows
				s.app.deleteTopic(show)
			}
		err = st.DeleteId(id) // delete slideshow or topic
	} else {
		// reduce access
		show.Access = access
		err = st.Update(show)
	}

	if err != nil {
		s.app.log(err)
	}
}

// onDropUser reduces visibility of a user's slideshows, or deletes the user, their slideshows and all their images, deferred to support caching.
func (s *GalleryState) onDropUser(tx etx.TxId, id int64, status int) {

	defer s.updatesGallery()()

	// terminate the extended transaction
	if err := s.app.tm.End(tx); err != nil {
		s.app.log(err)
		return
	}

	var err error
	if status == users.UserRemoved {
		// delete all slideshow images
		shows := s.app.SlideshowStore.ForUser(id, models.SlideshowRemoved)
		for _, show := range shows {
			s.app.deleteImages(show.Id)
		}

		// slideshows and slides will be removed by cascade delete in caller
		err = s.app.userStore.DeleteId(id)
	}
	// ## cannot implement deferred reduction in status because we don't have a separate visibility field

	if err != nil {
		s.app.log(err)
	}
}

// onPurge removes old unvalidated competition entries.
// It returns the ID of a following transaction to remove media files.
func (s *GalleryState) onPurge(t time.Time) etx.TxId {

	var nDelShows, nDelUsers int

	defer s.updatesGallery()()
	tx := s.app.tm.Begin()

	// counts of remaining entries
	entries := make(map[int64]int, 4)

	// entries before cut-off time
	before := t.Add(-s.app.cfg.MaxUnvalidatedAge)
	shows := s.app.SlideshowStore.ForTagOld(0, "validate", before)

	for _, show := range shows {
		uId := show.User.Int64

		// no of entries for this competitor
		if _, seen := entries[uId]; !seen {
			entries[uId] = s.app.SlideshowStore.CountForUser(uId) - 1
		} else {
			entries[uId] = entries[uId] - 1
		}

		// delete all media files
		s.app.deleteImages(show.Id)

		// remove competition slideshow
		// (slides removed by on delete cascade)
		if err := s.app.SlideshowStore.DeleteId(show.Id); err != nil {
			s.app.log(err)
		}

		nDelShows++
	}

	// remove competitors with no other entries
	for uId, n := range entries {
		if n == 0 {

			user, err := s.app.userStore.Get(uId)
			if err == nil && user == nil {
				err = fmt.Errorf("Lost competitor %d during purge.", uId)
			}
			if err == nil && user.Status == models.UserUnknown {
				err = s.app.userStore.DeleteId(uId)
			}
			if err == nil {
				nDelUsers++
			} else {
				s.app.log(err)
			}
		}
	}

	if nDelShows+nDelUsers > 0 {
		s.app.infoLog.Printf("Removed %d unverified competition entries, and %d entrants", nDelShows, nDelUsers)
	}
	return tx
}

// onRefresh changes topic thumbnails
func (s *GalleryState) onRefresh() int {

	// process topics as separate transactions, for better responsiveness
	topics := s.topicsToRefresh()

	for _, t := range topics {
		if status := s.refreshTopic(t); status != 0 {
			return status
		}
		runtime.Gosched() // ## not sure if this helps
	}

	return 0
}

// onReleaseShow removes the owning topic from a slideshow
func (s *GalleryState) onReleaseShow(tx etx.TxId, showId int64, topicId int64) {
	defer s.updatesGallery()()

	// terminate the extended transaction
	if err := s.app.tm.End(tx); err != nil {
		s.app.log(err)
		return
	}

	st := s.app.SlideshowStore

	show, err := st.Get(showId)
	if err != nil {
		s.app.log(err)
		return
	}

	// check that show hasn't already been assigned to another topic
	if show.Topic == topicId {
		show.Topic = 0
		if err = st.Update(show); err != nil {
			s.app.log(err)
		}
	}
}

// onUpdateShow processes an updated or deleted slideshow.
func (s *GalleryState) onUpdateShow(tx etx.TxId, showId int64, topicId int64, revised bool) {

	// setup
	claim := s.app.uploader.StartClaim(tx)

	// showId will be 0 if the user submitted slides with no images for a topic
	if showId == 0 {
		// delete any uploaded files that were never added to a slide
		claim.End(func(err error) {
			s.onAbandon(tx)
			s.app.tm.Do(tx) // next operation of transaction
		})
		return
	}

	// identify which uploaded files are referenced
	defer s.updatesNone()()
	s.claimFiles(showId, claim, true)

	// remove unclaimed files and continue when all uploads have been processed
	claim.End(func(err error) {
		if err == nil {
			s.onBindShow(showId, topicId, revised, tx)
		} else {
			s.app.log(err)
		}
		s.app.tm.Do(tx) // next operation of transaction
	})
}

// onUpdateShowV1 processes a V1 operation for an updated or deleted slideshow.
func (s *GalleryState) onUpdateShowV1(tx etx.TxId, showId int64, topicId int64, revised bool) {

	// setup
	claim := s.app.uploader.StartClaimV1(tx)

	// identify which uploaded files are referenced
	defer s.updatesGallery()()
	s.claimFiles(showId, claim, false)

	// update topic, before we remove any thumbnails it might use
	if topicId != 0 {
		topic := s.app.SlideshowStore.GetIf(topicId)
		if topic != nil {
			s.updateTopic(topic, revised)
		}
	}

	// update highlighted images
	if err := s.updateHighlights(showId); err != nil {
		s.app.log(err)
	}

	// remove unused versions
	claim.EndV1()

	// terminate the extended transaction
	if err := s.app.tm.End(tx); err != nil {
		s.app.log(err)
	}
}

// onUpdateTopic processes a topic with an updated or deleted slideshow.
func (s *GalleryState) onUpdateTopic(topicId int64, tx etx.TxId, revised bool) {
	defer s.updatesGallery()()

	if err := s.app.tm.End(tx); err != nil {
		s.app.log(err)
		return
	}

	topic := s.app.SlideshowStore.GetIf(topicId)
	if topic == nil {
		return
	}

	if err := s.updateTopic(topic, revised); err != nil {
		s.app.log(err)
	}
}

// refreshTopic changes a topic thumbnail.
func (s *GalleryState) refreshTopic(t *models.Slideshow) int {
	defer s.updatesGallery()()

	if err := s.updateTopic(t, false); err != nil {
		return s.rollback(statusTeapot, err)
	}
	return 0
}

func (s *GalleryState) topicsToRefresh() []*models.Slideshow {
	defer s.updatesNone()()
	return s.app.SlideshowStore.AllTopics()
}


// updateHighlights changes the cached images when a highlight slideshow is changed.
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

// bindFiles updates slides to show uploaded media after processing. It also sets the slideshow revision time.
func (s *GalleryState) bindFiles(showId int64, revised bool, bind *uploader.Bind) {

	// check if this is an update or deletion
	show := s.app.SlideshowStore.GetIf(showId)
	if show == nil {
		// No slides to be updated, because another update deleted the slideshow while we were processing.
		// A following call to Bind.End will delete all uploads for this transaction.
		return
	}

	thumbnail := ""
	nImages := 0

	// is this a contribution to highlights?
	var recent bool
	if show.Topic != 0 {
		t, _ := s.app.SlideshowStore.Get(show.Topic)
		if t != nil {
			fmt, _ := t.ParseFormat(0)
			recent = fmt == "H"
		}
	}

	// check each slide for an updated media file, with effectively no max
	slides := s.app.SlideStore.ForSlideshowOrdered(showId, recent, 10000)
	for _, slide := range slides {

		if slide.Image != "" {

			var newImage string
			var err error
			if newImage, err = bind.File(slide.Image); err != nil {
				// ## We have lost the file, but have no way to warn the user :-(
				// We must remove the reference so that all viewers don't get a missing file error.
				// log the error, but process the remaining slides
				slide.Image = ""
				slide.Format = slide.Format &^ models.SlideImage
				s.app.SlideStore.Update(slide)
				s.app.log(err)

			} else if newImage != "" {
				// updated media
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

	// update slideshow thumbnail, if changed
	if thumbnail != show.Image {
		show.Image = thumbnail
	}

	if revised {
		// update slideshow revision date
		show.Revised = time.Now()

		// topic contribution published on first media file
		if show.Topic != 0 && show.Created.IsZero() {
			show.Created = show.Revised
		}
	}

	if err := s.app.SlideshowStore.Update(show); err != nil {
		s.app.log(err)
	}
}

// claimFiles claims media files for a slideshow.
// process is set false only for V1 operations, in which case the uploaded media will have been processed already.
func (s *GalleryState) claimFiles(showId int64, claim *uploader.Claim, process bool) {

	show := s.app.SlideshowStore.GetIf(showId)
	if show == nil {
		// No slides to be updated. A following call to Bind.End will delete any uploaded images.
		return
	}

	// check each slide for an updated media file
	slides := s.app.SlideStore.ForSlideshow(showId)
	for _, slide := range slides {

		if slide.Image != "" {
			if process {
				claim.File(slide.Image)
			} else {
				claim.FileV1(slide.Image)
			}
		}
	}
}

// deleteImages performs immediate deletion of all images for a slideshow.
func (app *Application) deleteImages(showId int64) {
	for _, slide := range app.SlideStore.ForSlideshow(showId) {
		if slide.Image != "" {
			if err := app.uploader.DeleteNow(slide.Image); err != nil {
				app.log(err)
			}
		}
	}
}

// deleteTopic releases the contributing slideshows from the topic.
func (app *Application) deleteTopic(t *models.Slideshow) {

	// give the users back their own slideshows
	store := app.SlideshowStore
	slideshows := store.ForTopic(t.Id)
	for _, s := range slideshows {
		s.Topic = 0
		s.Title = t.Title // with current topic title
		s.Access = models.SlideshowPrivate
		s.Visible = models.SlideshowPrivate
		store.Update(s)
	}
}

// updateTopic changes the topic thumbnail, and if required updates the revision date
func (s *GalleryState) updateTopic(t *models.Slideshow, revised bool) error {

	st := s.app.SlideStore
	update := false

	// exclude hidden topics and fixed image competition categories
	if t.Visible >= models.SlideshowClub && t.Format != "F" {
		var images []int64

		fmt, max := t.ParseFormat(s.app.cfg.MaxSlidesTopic)
		if fmt =="H" {
			images = st.ImagesForHighlights(t.Id, max, s.app.cfg.MaxHighlightsTopic)
		} else {
			images = st.ImagesForTopic(t.Id, max)
		}
		
		nImages := len(images)
		img := ""

		if nImages > 0 {
			// select random image for topic thumbnail
			i := int64(rand.Float32() * float32(nImages))
			s, err := st.Get(images[i])
			if err != nil {
				return err
			}
			img = s.Image
		}
		if t.Image != img {
			t.Image = img
			update = true
		}
	}
	
	if revised {
		// change revision date
		t.Revised = time.Now() 
		fmt, _ := t.ParseFormat(0)
		if fmt == "H" {
			// highlights topics are moved up display order when contributions revised
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
