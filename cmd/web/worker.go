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

	"github.com/inchworks/webparts/etx"
	"github.com/inchworks/webparts/uploader"

	"inchworks.com/picinch/internal/models"
)

// data for for request to validate competition entry
type validationData struct {
	Name  string
	Entry string
	Link  string
}

// Implement RM interface for webparts.etx.

// Operation types (keep unchanges)
const (
	OpComp = 0
	OpShowV1 = 1
	OpTopic = 2
	OpShow = 3
)

// We need an arbitary status code for rollback(). This one is ideal!
const statusTeapot = 418

func (s *GalleryState) Name() string {
	return "gallery"
}

func (s *GalleryState) ForOperation(opType int) etx.Op {
	switch opType {
	case OpComp, OpShow, OpShowV1:
		return &OpUpdateShow{}
	case OpTopic:
		return &OpUpdateTopic{}
	default:
		var unknown struct{}
		return &unknown
	}
}

// Do operation requested via TM.
func (s *GalleryState) Operation(id etx.OpId, opType int, op etx.Op) {

	// send the request to the worker
	switch req := op.(type) {
	case *OpUpdateShow:
		req.op = id
		switch opType {
		case OpComp:
			s.app.chComp <- *req
		case OpShow:
			s.app.chShow <- *req
		case OpShowV1:
			s.app.chShowV1 <- *req
		}

	case *OpUpdateTopic:
		req.op = id
		s.app.chTopic <- *req

	default:
		s.app.errorLog.Print("Unknown TX operation")
	}
}

// worker does all background processing for PicInch.
func (s *GalleryState) worker(
	chComp <-chan OpUpdateShow,
	chShow <-chan OpUpdateShow,
	chShowV1 <-chan OpUpdateShow,
	chShows <-chan []OpUpdateShow,
	chTopic <-chan OpUpdateTopic,
	chBind <- chan reqBindShow,
	chRefresh <-chan time.Time,
	chPurge <-chan time.Time,
	done <-chan bool) {

	for {
		// returns to client sooner?
		runtime.Gosched()

		select {
		case req := <-chComp:

			// a competition entry has been added
			s.onCompEntry(req.ShowId, req.op, req.Revised)

		case req := <-chShow:

			// a slideshow has been updated or removed
			s.onUpdateShow(req.ShowId, req.TopicId, req.op, req.Revised)

		case req := <-chShowV1:

			// a slideshow has been updated or removed
			s.onUpdateShowV1(req.ShowId, req.TopicId, req.op, req.Revised)

		case reqs := <-chShows:

			// a set of slideshows have been updated (e.g. user removed)
			for _, req := range reqs {
				s.onUpdateShow(req.ShowId, req.TopicId, req.op, req.Revised)
				runtime.Gosched()
			}

		case req := <-chTopic:

			// a topic slideshow has been updated or removed
			s.onUpdateTopic(req.TopicId, req.op, req.Revised)

		case req := <-chBind:

			// media files have been processed, ready to be displayed
			s.onBindShow(req.showId, req.revised, req.op)

			// update topic, to drop any old thumbnails and choose new ones
			if req.topicId != 0 {
				s.onUpdateTopic(req.topicId, 0, req.revised)
			}

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

// onBindShow sets uploaded images for a new and updated slideshow.
func (s *GalleryState) onBindShow(showId int64, revised bool, opId etx.OpId) {

	defer s.updatesGallery()()

	// setup
	bind := s.app.uploader.StartBind(etx.Transaction(opId))

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

	// terminate this operation
	if err := s.app.tm.End(opId); err != nil {
		s.app.log(err)
	}
}

// onCompEntry processes a competition entry.
func (s *GalleryState) onCompEntry(showId int64, opId etx.OpId, revised bool) int {

	app := s.app

	// slideshow for competition
	s.onUpdateShow(showId, 0, opId, revised)

	// email configured?
	if s.app.emailer == nil {
		return 0
	}

	// entry and user
	defer s.updatesGallery()()

	show, err := app.SlideshowStore.Get(showId)
	if err != nil {
		return s.rollback(statusTeapot, err)
	} else if show == nil {
		return s.rollback(statusTeapot, errors.New("Unknown slideshow for validation"))
	}

	user, err := app.userStore.Get(show.User.Int64)
	if err != nil {
		return s.rollback(statusTeapot, err)
	} else if user == nil {
		return s.rollback(statusTeapot, errors.New("Unknown user for validation"))
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
		return s.rollback(statusTeapot, err)
	}
	return 0
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

		// remove competition slideshow
		// (slides removed by on delete cascade)
		if err := s.app.SlideshowStore.DeleteId(show.Id); err != nil {
			s.app.log(err)
		}

		// remove media files after slideshow has been deleted
		// #### revise to match V2 uploader
		// ## log error
		s.app.tm.AddNext(tx, s, OpShow, &OpUpdateShow{
			ShowId:  show.Id,
			TopicId: show.Topic,
			Revised: false})

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

// Change topic(s) thumbnails

func (s *GalleryState) onRefresh() int {

	defer s.updatesGallery()()

	topics := s.app.SlideshowStore.AllTopics()

	for _, topic := range topics {
		if err := s.updateTopic(topic, false); err != nil {
			return s.rollback(statusTeapot, err)
		}
	}

	return 0
}

// onUpdateShow processes an updated or deleted slideshow.
func (s *GalleryState) onUpdateShow(showId int64, topicId int64, opId etx.OpId, revised bool) {

	// setup
	claim := s.app.uploader.StartClaim(etx.Transaction(opId))

	// showId will be 0 if the user submitted slides with no images for a topic
	if showId == 0 {
		// delete any uploaded files that were never added to a slide
		claim.End(nil)
		return
	}

	// identify which uploaded files are referenced
	s.claimFiles(showId, claim, true)

	// remove unclaimed files and continue when all uploads have been processed
	claim.End(func(err error) {
		if err == nil {
			s.app.chBind <- reqBindShow{showId: showId, topicId: topicId, revised: revised, op: opId}
		} else {
			s.app.log(err)
		}
	})
}

// onUpdateShowV1 processes a V1 operation for an updated or deleted slideshow.
func (s *GalleryState) onUpdateShowV1(showId int64, topicId int64, opId etx.OpId, revised bool) {

	// setup
	claim := s.app.uploader.StartClaimV1(etx.Transaction(opId))

	// identify which uploaded files are referenced
	s.claimFiles(showId, claim, false)

	// update topic, before we remove any thumbnails it might use
	if topicId != 0 {
		s.onUpdateTopic(topicId, 0, revised)
	}

	// update highlighted images
	if err := s.updateHighlights(showId); err != nil {
		s.app.log(err)
	}

	// remove unused versions
	claim.EndV1()

	// terminate the extended transaction
	defer s.updatesGallery()()
	if err := s.app.tm.End(opId); err != nil {
		s.app.log(err)
	}
}

// onUpdateTopic processes a topic with an updated or deleted slideshow.
// A TxId is specified if the extended transaction should be ended as part of the database transaction.
func (s *GalleryState) onUpdateTopic(topicId int64, opId etx.OpId, revised bool) {
	defer s.updatesGallery()()

	if opId != 0 {
		// this is the end of the aynchronous operation
		if err := s.app.tm.End(opId); err != nil {
			s.app.log(err)
		}
	}

	topic := s.app.SlideshowStore.GetIf(topicId)
	if topic == nil {
		return
	}

	if err := s.updateTopic(topic, revised); err != nil {
		s.app.log(err)
	}
}

// updateHighlights changes the cached images when a highlight slideshow is changed.
func (s *GalleryState) updateHighlights(id int64) error {

	defer s.updatesNone()()

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

	// check each slide for an updated media file
	slides := s.app.SlideStore.ForSlideshow(showId, 1000)
	for _, slide := range slides {

		if slide.Image != "" {

			// ### better to check if file is for this transaction?
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
	if err := s.app.SlideshowStore.Update(show); err != nil {
		s.app.log(err)
	}

}

// updateSlides claims media files for a slideshow. It also sets the slideshow revision time.
// process is set false only for V1 operations, in which case the uploaded media will have been processed already.
func (s *GalleryState) claimFiles(showId int64, claim *uploader.Claim, process bool) {

	// serialise display state while slides are changing
	defer s.updatesNone()()

	show := s.app.SlideshowStore.GetIf(showId)
	if show == nil {
		// No slides to be updated. A following call to Bind.End will delete any uploaded images.
		return
	}

	// check each slide for an updated media file
	slides := s.app.SlideStore.ForSlideshow(showId, 1000)
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

// updateTopic changes the topic thumbnail, and if required updates the revision date
func (s *GalleryState) updateTopic(t *models.Slideshow, revised bool) error {

	update := false

	// exclude hidden topics and fixed image competition categories
	if t.Visible >= models.SlideshowClub && t.Format != "F" {
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
