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

// Processing for gallery setup and editing.
//
// These functions may modify application state.

import (
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"inchworks.com/picinch/internal/form"
	"inchworks.com/picinch/internal/models"
	"inchworks.com/picinch/internal/picinch"

	"github.com/inchworks/webparts/v2/etx"
	"github.com/inchworks/webparts/v2/multiforms"
	"github.com/inchworks/webparts/v2/uploader"
	"github.com/inchworks/webparts/v2/users"
	"inchworks.com/picinch/internal/tags"
)

type userTags struct {
	id   int64
	name string
	tags []*tags.ItemTag
}

// ForAssignShows returns a form with data to assign slideshows to topics.
func (s *GalleryState) ForAssignShows(tok string) (f *form.AssignShowsForm) {

	// serialisation
	defer s.updatesNone()()

	// get slideshows
	slideshows := s.app.SlideshowStore.AllForUsers()

	// form
	var d = make(url.Values)
	f = form.NewAssignShows(d, tok)

	// add slideshows to form
	for i, sh := range slideshows {
		// warn if slideshow is already being assigned
		// ## disable update somehow
		var updating bool
		var topic int64
		if sh.Topic != 0 && sh.Visible != models.SlideshowTopic {
			topic = 0
			updating = true // removing from topic
		} else if sh.Access != sh.Visible {
			topic = sh.Topic
			updating = true // adding to topic
		} else {
			topic = sh.Topic
		}

		f.Add(i, sh.Id, topic, sh.Shared != 0, sh.Title, s.app.userStore.Name(sh.User.Int64), updating)
	}

	return
}

// OnAssignShows processes updates when slideshows are assigned to topics.
// It returns an extended transaction ID if there are no client errors.
func (s *GalleryState) OnAssignShows(rsSrc []*form.AssignShowFormData) (int, etx.TxId) {

	// ## I don't like using database IDs in a form, because it exposes us to a user that manipulates the form.
	// ## In this case the user has to be authorised as a curator, and (I think) they can only make changes
	// ## that the form allows anyway. Still, I'd like an alternative :-(.

	// serialisation
	defer s.updatesGallery()()

	// start extended transaction
	tx := s.app.tm.Begin()

	nConflicts := 0
	nSrc := len(rsSrc)

	// skip template
	i := 1

	for i < nSrc {

		// get current slideshow
		rSrc := rsSrc[i]
		rDest := s.app.SlideshowStore.GetIf(rSrc.NShow)
		if rDest == nil {
			nConflicts++ // just deleted by user

		} else {
			// ignore in-progress removal from topic
			if rSrc.NTopic == 0 && rDest.Visible != models.SlideshowTopic {
				rSrc.NTopic = rDest.Topic
			}

			// check if details changed
			if rSrc.NTopic != rDest.Topic || rSrc.Title != rDest.Title {

				if rSrc.NTopic != 0 && s.app.SlideshowStore.GetIf(rSrc.NTopic) == nil {
					nConflicts++ // another curator deleted the topic!

				} else {

					// title change is mostly useful when returning a slideshow to the user
					rDest.Title = rSrc.Title

					if rSrc.NTopic == 0 && rDest.Topic != 0 {
						// give slideshow back to user
						rDest.Access = models.SlideshowPrivate
						rDest.Visible = models.SlideshowPrivate

						// final removal from topic is deferred
						if err := s.app.tm.AddTimed(tx, s, OpRelease, &OpReleaseShow{
							ShowId:  rDest.Id,
							TopicId: rDest.Topic,
						}, s.app.cfg.DropDelay); err != nil {
							return s.rollback(http.StatusInternalServerError, err), 0
						}
					} else {
						// add or move show to topic, or perhaps just change title
						var visible int
						rDest.Topic = rSrc.NTopic
						if rDest.Topic != 0 {
							visible = models.SlideshowTopic
						} else {
							visible = rDest.Visible
						}

						// change slideshow visibility and access
						if err := s.setVisible(tx, rDest, visible); err != nil {
							return s.rollback(http.StatusInternalServerError, err), 0
						}
					}

					s.app.SlideshowStore.Update(rDest)
				}
			}
		}
		i++
	}

	if nConflicts > 0 {
		return http.StatusConflict, tx
	} else {
		return 0, tx
	}
}

// Get data to edit gallery

func (s *GalleryState) ForEditGallery(tok string) (f *multiforms.Form) {

	// serialisation
	defer s.updatesNone()()

	// current data
	var d = make(url.Values)
	f = multiforms.New(d, tok)
	f.Set("organiser", s.gallery.Organiser)
	f.Set("title", s.gallery.Title)
	f.Set("nMaxSlides", strconv.Itoa(s.gallery.NMaxSlides))
	f.Set("nShowcased", strconv.Itoa(s.gallery.NShowcased))

	return
}

// OnEditGallery processes the modification of a gallery. It returns an HTTP status or 0.
func (s *GalleryState) OnEditGallery(organiser string, title string,  nMaxSlides int, nShowcased int) int {

	// serialisation
	defer s.updatesGallery()()

	// save changes via cache (conversions already checked)
	s.gallery.Organiser = organiser
	s.gallery.Title = title
	s.gallery.NMaxSlides = nMaxSlides
	s.gallery.NShowcased = nShowcased
	if err := s.app.GalleryStore.Update(s.gallery); err != nil {
		return s.rollback(http.StatusBadRequest, err)
	}

	return 0
}

// ForEditSlideshow returns the data to edit a page or slideshow.
func (s *GalleryState) ForEditSlideshow(showId int64, tok string) (status int, f *form.SlidesForm, show *models.Slideshow) {

	// serialisation
	defer s.updatesGallery()()

	// title and slides
	show = s.app.SlideshowStore.GetIf(showId)
	if show == nil {
		status = s.rollback(http.StatusNotFound, nil)
		return
	}
	slides := s.app.SlideStore.ForSlideshowOrdered(show.Id, false, 100)

	// start multi-step transaction for uploaded files
	ts, err := s.app.uploader.Begin()
	if err != nil {
		status = s.rollback(http.StatusInternalServerError, err)
		return
	}

	// form
	var d = make(url.Values)
	f = form.NewSlides(d, len(slides), tok)
	f.Set("nShow", strconv.FormatInt(showId, 36))
	f.Set("nTopic", strconv.FormatInt(show.Topic, 36))
	f.Set("nUser", strconv.FormatInt(show.User.Int64, 36))
	f.Set("timestamp", ts)

	// template for new slide form
	f.AddTemplate(len(slides))

	// add slides to form
	for i, sl := range slides {
		image := uploader.NameFromFile(sl.Image)
		f.Add(i, sl.ShowOrder, sl.Title, image, sl.Caption, sl.ManualFormat())
	}

	return
}

// OnEditSlideshow processes the modification of a slideshow. It returns 0 and the user ID on success, or an HTTP status code.
// topicId and userId are needed only for a new slideshow for a topic. Otherwise we prefer to trust the database.
func (s *GalleryState) OnEditSlideshow(showId int64, topicId int64, tx etx.TxId, userId int64, qsSrc []*form.SlideFormData, page bool) (int, int64) {

	// serialisation
	defer s.updatesGallery()()

	// commit uploads, unless request has been running so long that we have discarded them
	if err := s.app.uploader.Commit(tx); err != nil {
		return s.rollback(http.StatusRequestTimeout, err), 0
	}

	now := time.Now()
	nSrc := len(qsSrc)
	revised := false
	nMedia := 0
	var show *models.Slideshow

	if showId != 0 {
		// slideshow already exists
		show = s.app.SlideshowStore.GetIf(showId)
		if show == nil {
			return s.rollback(http.StatusBadRequest, nil), 0
		}
		topicId = show.Topic
		userId = show.User.Int64

	} else if nSrc > 0 {
		// no slideshow specified - these must be slides for a topic
		topic := s.app.SlideshowStore.GetIf(topicId)
		if topic == nil {
			return s.rollback(http.StatusBadRequest, nil), 0
		}

		// It might already exist, if the user is attempting an edit on two devices at the same time,
		// and we allow only one. (Yes, it has happened!)
		show = s.app.SlideshowStore.ForTopicUserIf(topicId, userId)
		if show == nil {

			// create a new slideshow from the topic details
			show = &models.Slideshow{
				GalleryOrder: 5, // default
				Access:       models.SlideshowTopic,
				Visible:      models.SlideshowTopic,
				User:         sql.NullInt64{Int64: userId, Valid: true},
				Topic:        topicId,
				Created:      time.Time{},
				Revised:      now,
				Title:        topic.Title,
			}
		}
		s.app.SlideshowStore.Update(show)
		showId = show.Id
	}

	// compare modified slides against current slides, and update
	qsDest := s.app.SlideStore.ForSlideshowOrdered(showId, false, 100)

	updated := false
	nDest := len(qsDest)

	iSrc := 1 // skip template slide
	var iDest int

	for iSrc < nSrc || iDest < nDest {

		if iSrc == nSrc {
			// no more source slides - delete from destination
			// ## errors ignored - better to aggregate and report them
			s.app.uploader.Delete(tx, qsDest[iDest].Image)
			s.app.SlideStore.DeleteId(qsDest[iDest].Id)
			updated = true
			iDest++

		} else if iDest == nDest {
			// no more destination slides - add new one
			mediaName := uploader.CleanName(qsSrc[iSrc].MediaName)
			qd := models.Slide{
				Slideshow: showId,
				Format:    s.app.slideFormat(qsSrc[iSrc], page),
				ShowOrder: qsSrc[iSrc].ShowOrder,
				Created:   now,
				Revised:   now,
				Title:     s.sanitize(qsSrc[iSrc].Title, ""),
				Caption:   s.sanitizeUnless(page, qsSrc[iSrc].Caption, ""),
				Image:     uploader.FileFromName(tx, qsSrc[iSrc].Version, mediaName),
			}
			if mediaName != "" {
				revised = true // update revision when new media file added
			}

			s.app.SlideStore.Update(&qd)
			updated = true
			iSrc++

		} else {
			ix := qsSrc[iSrc].ChildIndex
			if ix < iDest {
				// out of sequence slide index
				return s.rollback(http.StatusBadRequest, nil), 0
			}

			// count existing media slides
			qDest := qsDest[iDest]
			if qDest.Image != "" {
				nMedia++
			}

			if ix > iDest {
				// source slide removed - delete from destination
				s.app.uploader.Delete(tx, qDest.Image)
				s.app.SlideStore.DeleteId(qDest.Id)
				updated = true
				iDest++

			} else if ix == iDest {
				// check if details changed
				if qsSrc[iSrc].ShowOrder != qDest.ShowOrder ||
					qsSrc[iSrc].Format != qDest.ManualFormat() ||
					qsSrc[iSrc].Title != qDest.Title ||
					qsSrc[iSrc].Caption != qDest.Caption ||
					qsSrc[iSrc].Version != 0 {

					qDest.Format = s.app.slideFormat(qsSrc[iSrc], page)
					qDest.ShowOrder = qsSrc[iSrc].ShowOrder
					qDest.Revised = now
					qDest.Title = s.sanitize(qsSrc[iSrc].Title, qDest.Title)
					qDest.Caption = s.sanitizeUnless(page, qsSrc[iSrc].Caption, qDest.Caption)

					if qsSrc[iSrc].Version != 0 {
						// replace media file
						s.app.uploader.Delete(tx, qsDest[iDest].Image)
						mediaName := uploader.CleanName(qsSrc[iSrc].MediaName)
						qDest.Image = uploader.FileFromName(tx, qsSrc[iSrc].Version, mediaName)
					}

					s.app.SlideStore.Update(qDest)
					updated = true
				}
				iSrc++
				iDest++
			}
		}
	}

	// re-sequence slides, removing missing or duplicate orders
	// If two slides have the same order, the later update comes first
	var slides []*models.Slide
	if updated {

		nImages := 0
		slides = s.app.SlideStore.ForSlideshowOrderedTx(showId, 100)

		for ix, sl := range slides {
			nOrder := ix + 1
			if sl.ShowOrder != nOrder {

				// update sequence
				sl.ShowOrder = nOrder
				s.app.SlideStore.Update(sl)
			}
			if sl.Image != "" {
				nImages++ // count slides with images
			}
		}

		if topicId != 0 && nImages == 0 {
			// remove empty show for topic
			// ### beware race with user re-opening show to add back an image
			if err := s.removeSlideshow(tx, show, true); err != nil {
				return s.rollback(http.StatusInternalServerError, err), 0
			}
			showId = 0
		}
	}

	// Note that if showId is still 0 at this point, the user submitted a slideshow with no images for a topic.
	// We still do OpUpdateShow to remove any uploads added to a slide and then removed.

	// request worker to generate media versions, and remove unused images
	if err := s.app.tm.AddNext(tx, s, OpShow,
		&OpUpdateShow{
			ShowId:  showId,
			TopicId: topicId,
			Revised: revised,
		}); err != nil {
		return s.rollback(http.StatusInternalServerError, err), 0
	}

	// request to change topic thumbnail (after processing new images)
	if topicId != 0 {
		if err := s.app.tm.AddNext(tx, s, OpShow,
			&OpUpdateTopic{
				TopicId: topicId,
				Revised: revised,
			}); err != nil {
			return s.rollback(http.StatusInternalServerError, err), 0
		}
	}

	// update cached page
	if page && updated {
		s.publicPages.SetSections(showId, slides)
	}

	return 0, userId
}

// Get data to edit slideshows for a user

func (s *GalleryState) ForEditSlideshows(userId int64, tok string) (f *form.SlideshowsForm, user *users.User) {

	// serialisation
	defer s.updatesNone()()

	// get user
	user = s.app.getUserIf(userId)
	if user == nil {
		return
	}

	// get slideshows
	slideshows := s.app.SlideshowStore.ForUser(userId, models.SlideshowPrivate)

	// form
	var d = make(url.Values)
	f = form.NewSlideshows(d, tok)

	// add template and slideshows to form
	f.AddTemplate()
	for i, sh := range slideshows {
		f.Add(i, sh.Id, sh.Topic, sh.Visible, sh.Shared != 0, sh.Title, "")
	}

	return
}

// OnEditSlideshows processes updates when slideshows are modified.
// It returns an extended transaction ID if there are no client errors.
func (s *GalleryState) OnEditSlideshows(userId int64, rsSrc []*form.SlideshowFormData) (int, etx.TxId) {

	// serialisation
	defer s.updatesGallery()()

	// start extended transaction
	tx := s.app.tm.Begin()

	now := time.Now()

	// compare modified slideshows against current ones, and update
	rsDest := s.app.SlideshowStore.ForUser(userId, models.SlideshowPrivate)

	nSrc := len(rsSrc)
	nDest := len(rsDest)

	// skip template
	iSrc := 1
	var iDest int

	for iSrc < nSrc || iDest < nDest {

		if iSrc == nSrc {
			// no more source slideshows - delete from destination
			if err := s.removeSlideshow(tx, rsDest[iDest], true); err != nil {
				return s.rollback(http.StatusBadRequest, err), 0
			}
			iDest++

		} else if iDest == nDest {

			// set creation date just once, when published
			var created time.Time
			visible := rsSrc[iSrc].Visible
			if visible > models.SlideshowPrivate {
				created = now
			}

			// no more destination slideshows - add new one
			r := models.Slideshow{
				GalleryOrder: 5, // default order
				Access:       visible,
				Visible:      visible,
				User:         sql.NullInt64{Int64: userId, Valid: true},
				Created:      created,
				Revised:      now,
				Title:        s.sanitize(rsSrc[iSrc].Title, ""), // ## not essential
			}
			s.app.SlideshowStore.Update(&r)
			iSrc++

		} else {
			ix := rsSrc[iSrc].ChildIndex
			if ix > iDest {
				// source slideshow removed - delete from destination
				if err := s.removeSlideshow(tx, rsDest[iDest], true); err != nil {
					return s.rollback(http.StatusBadRequest, err), 0
				}
				iDest++

			} else if ix == iDest {
				// check if details changed
				rSrc := rsSrc[iSrc]
				rDest := rsDest[iDest]

				if rSrc.Visible != rDest.Visible ||
					rSrc.Title != rDest.Title {

					if err := s.setVisible(tx, rDest, rSrc.Visible); err != nil {
						return s.rollback(http.StatusBadRequest, err), 0
					}
					rDest.Title = s.sanitize(rSrc.Title, rDest.Title)

					// set creation date just once, when published
					if rSrc.Visible > models.SlideshowPrivate && rDest.Created.IsZero() {
						rDest.Created = now
						rDest.Revised = now
					}

					s.app.SlideshowStore.Update(rDest)
				}
				iSrc++
				iDest++

			} else {
				// out of sequence slideshow index
				return s.rollback(http.StatusBadRequest, nil), 0
			}
		}
	}

	return 0, tx
}

// Get data to edit a user's contribution to a topic

func (s *GalleryState) ForEditTopic(topicId int64, userId int64, tok string) (status int, f *form.SlidesForm, title string) {

	var slides []*models.Slide

	// serialisation
	defer s.updatesGallery()()

	// user's show for topic
	var showId int64
	show := s.app.SlideshowStore.ForTopicUserIf(topicId, userId)
	if show == nil {
		topic := s.app.SlideshowStore.GetIf(topicId)
		if topic == nil {
			status = s.rollback(http.StatusBadRequest, nil)
			return
		}
		title = topic.Title

	} else {
		// user's existing contribution to topic
		showId = show.Id
		title = show.Title
		slides = s.app.SlideStore.ForSlideshowOrdered(showId, false, 100)
	}

	// start multi-step transaction for uploaded files
	ts, err := s.app.uploader.Begin()
	if err != nil {
		status = s.rollback(http.StatusInternalServerError, err)
		return
	}

	// form
	var d = make(url.Values)
	f = form.NewSlides(d, len(slides), tok)
	f.Set("nShow", strconv.FormatInt(showId, 36))
	f.Set("nTopic", strconv.FormatInt(topicId, 36))
	f.Set("nUser", strconv.FormatInt(userId, 36))
	f.Set("timestamp", ts)

	// template for new slide form
	f.AddTemplate(len(slides))

	// add slides to form
	for i, sl := range slides {
		image := uploader.NameFromFile(sl.Image)
		f.Add(i, sl.ShowOrder, sl.Title, image, sl.Caption, 0)
	}

	return
}

// Get data to edit topics

func (s *GalleryState) ForEditTopics(tok string) (f *form.SlideshowsForm) {

	// serialisation
	defer s.updatesNone()()

	// get topics
	topics := s.app.SlideshowStore.AllTopicsEditable()

	// form
	var d = make(url.Values)
	f = form.NewSlideshows(d, tok)

	// add template and slideshows to form
	f.AddTemplate()
	for i, sh := range topics {
		f.Add(i, sh.Id, 0, sh.Visible, sh.Shared != 0, sh.Title, "")
	}

	return
}

// OnEditTopics processes changes when topics are modified.
// It returns an extended transaction ID if there are no errors.
func (s *GalleryState) OnEditTopics(rsSrc []*form.SlideshowFormData) (int, etx.TxId) {

	// ## should combine with OnEditSlideshows, since they are so similar. Or even all of the multi-item forms?

	// serialisation
	defer s.updatesGallery()()

	// start extended transaction
	tx := s.app.tm.Begin()

	now := time.Now()

	// compare modified slideshows against current ones, and update
	rsDest := s.app.SlideshowStore.AllTopicsEditable()

	nSrc := len(rsSrc)
	nDest := len(rsDest)

	// skip template
	iSrc := 1
	var iDest int

	for iSrc < nSrc || iDest < nDest {

		if iSrc == nSrc {
			// no more source topics - delete from destination
			if err := s.removeSlideshow(tx, rsDest[iDest], true); err != nil {
				return s.rollback(http.StatusBadRequest, err), 0
			}
			iDest++

		} else if iDest == nDest {

			// set creation date just once, when published
			var created time.Time
			visible := rsSrc[iSrc].Visible
			if visible > models.SlideshowPrivate {
				created = now
			}

			// no more destination slideshows - add new one
			r := models.Slideshow{
				GalleryOrder: 5, // default order
				Access:       visible,
				Visible:      visible,
				Created:      created,
				Shared:       shareCode(rsSrc[iSrc].IsShared, 0),
				Revised:      now,
				Title:        s.sanitize(rsSrc[iSrc].Title, ""),
			}
			s.app.SlideshowStore.Update(&r)
			iSrc++

		} else {
			ix := rsSrc[iSrc].ChildIndex
			if ix > iDest {
				// source slideshow removed - delete from destination
				if err := s.removeSlideshow(tx, rsDest[iDest], true); err != nil {
					return s.rollback(http.StatusBadRequest, err), 0
				}
				iDest++

			} else if ix == iDest {
				// check if details changed
				rSrc := rsSrc[iSrc]
				rDest := rsDest[iDest]

				if rSrc.Visible != rDest.Visible ||
					rSrc.Title != rDest.Title ||
					rSrc.IsShared != (rDest.Shared > 0) {

					if err := s.setVisible(tx, rDest, rSrc.Visible); err != nil {
						return s.rollback(http.StatusBadRequest, err), 0
					}

					rDest.Shared = shareCode(rSrc.IsShared, rDest.Shared)
					rDest.Title = s.sanitize(rSrc.Title, rDest.Title)

					// set creation date just once, when published
					if rSrc.Visible > models.SlideshowPrivate && rDest.Created.IsZero() {
						rDest.Created = now
						rDest.Revised = now

						// needs a media file before it will appear on home page
						if err := s.app.tm.AddNext(tx, s, OpTopic, &OpUpdateTopic{
							TopicId: rDest.Id,
							Revised: false,
						}); err != nil {
							return s.rollback(http.StatusInternalServerError, err), 0
						}
					}

					s.app.SlideshowStore.Update(rDest)
				}
				iSrc++
				iDest++

			} else {
				// out of sequence slideshow index
				return s.rollback(http.StatusBadRequest, nil), 0

			}
		}
	}

	return 0, tx
}

// ForEditGallery returns a competition entry form.
func (s *GalleryState) forEnterComp(classId int64, tok string) (status int, f *form.PublicCompForm, title string, caption string) {

	// serialisation
	defer s.updatesGallery()()

	// get the class topic
	show, err := s.app.SlideshowStore.Get(classId)
	if err != nil {
		status = s.rollback(http.StatusInternalServerError, err)
		return
	} else if show == nil {
		status = s.rollback(http.StatusNotFound, nil)
		return
	}

	// start multi-step transaction for uploaded files
	var ts string
	ts, err = s.app.uploader.Begin()
	if err != nil {
		return s.rollback(http.StatusInternalServerError, err), nil, "", ""
	}

	// initial data
	var d = make(url.Values)
	f = form.NewPublicComp(d, 1, tok)
	f.Set("class", strconv.FormatInt(show.Id, 36))
	f.Set("timestamp", ts)
	title = show.Title
	caption = show.Caption

	return
}

// onEnterComp processes a competition entry.
// It returns 0 and a validation code on success, or an HTTP status code.
// The validation code is non-zero for auto-validation, 0 for validation by email.
func (s *GalleryState) onEnterComp(classId int64, tx etx.TxId, name string, email string, location string, title string, caption string, image string, nAgreed int) (status int, vc int64) {

	// serialisation
	defer s.updatesGallery()()

	// check for a request that has been running so long that we have discarded the uploads
	if err := s.app.uploader.Commit(tx); err != nil {
		return s.rollback(http.StatusRequestTimeout, err), -1
	}

	// create user for entry
	u, err := s.app.userStore.GetNamed(email)
	if err != nil && !s.app.userStore.IsNoRecord(err) {
		return s.rollback(http.StatusInternalServerError, err), -1
	}
	if u == nil {
		u = &users.User{
			Username: email,
			Name:     name,
			Password: []byte(""),
			Created:  time.Now(),
		}
		if err = s.app.userStore.Update(u); err != nil {
			return s.rollback(http.StatusInternalServerError, err), -1
		}
	}

	// generate validation code for public entry
	vc, err = picinch.SecureCode(8)
	if err != nil {
		return s.rollback(http.StatusInternalServerError, err), -1
	}

	// create slideshow for entry
	t := time.Now()
	show := &models.Slideshow{
		User:    sql.NullInt64{Int64: u.Id, Valid: true},
		Access:  models.SlideshowClub,
		Visible: models.SlideshowClub, // ## Private would be better, but needs something else for judges to view.
		Shared:  vc,
		Topic:   classId,
		Created: t,
		Revised: t,
		Title:   s.sanitize(title, ""),
		Caption: s.sanitize(location, ""),
		Format:  "E",
	}
	if err = s.app.SlideshowStore.Update(show); err != nil {
		return s.rollback(http.StatusBadRequest, err), -1
	}

	// must be an acceptable file type
	// (it should have been validated when the file was uploaded)
	image = uploader.CleanName(image)
	sf := slideMedia(s.app.uploader.MediaType(image))
	if sf == 0 {
		return s.rollback(http.StatusBadRequest, err), -1
	}
	if caption != "" {
		sf += models.SlideCaption
	}

	// create slide for media file
	// ## a future version will allow multiple slides
	slide := &models.Slide{
		Slideshow: show.Id,
		Format:    sf,
		Revised:   time.Now(),
		Caption:   s.sanitize(caption, ""),
		Image:     uploader.FileFromName(tx, 1, image),
	}

	if err = s.app.SlideStore.Update(slide); err != nil {
		return s.rollback(http.StatusInternalServerError, err), -1
	}

	// tag entry as unvalidated
	s.app.tagger.SetTagRef(show.Id, 0, "validate", 0, "")

	// tag agreements (This is a legal requirement, so we make the logic as explicit as possible.)
	if nAgreed > 0 {
		s.app.tagger.SetTagRef(show.Id, 0, "agreements", 0, strconv.Itoa(nAgreed))
	}

	// request worker to generate media version and remove unused images
	if err := s.app.tm.AddNext(tx, s, OpShow, &OpUpdateShow{
		ShowId:  show.Id,
		Revised: false,
	}); err != nil {
		return s.rollback(http.StatusInternalServerError, err), -1
	}

	if s.app.emailer != nil {
		// request worker to send validation email
		if err := s.app.tm.AddNext(tx, s, OpComp, &OpValidate{ShowId: show.Id}); err != nil {
			return s.rollback(http.StatusInternalServerError, err), -1
		}

		// auto validation is not needed if we can send emails
		vc = 0
	}

	return 0, vc
}

// onRemoveUser removes a user's media files from the system.
func (s *GalleryState) onRemoveUser(tx etx.TxId, user *users.User) {

	// all slideshows for user
	shows := s.app.SlideshowStore.ForUser(user.Id, models.SlideshowTopic)
	for _, show := range shows {

		if err := s.app.galleryState.removeSlideshow(tx, show, false); err != nil {
			s.app.log(err)
		}
	}

	// set deletion in progress
	user.Status = users.UserRemoved
	user.Role = models.UserUnknown
	if err := s.app.userStore.Update(user); err != nil {
		s.app.log(err)
	}

	// request delayed deletion
	if err := s.app.tm.AddTimed(tx, s, OpDropUser, &OpDrop{
		Id:     user.Id,
		Access: user.Status,
	}, s.app.cfg.DropDelay); err != nil {
		s.app.log(err)
	}
}

// onUpdateUser updates topics when a user is suspended.
func (s *GalleryState) onUpdateUser(tx etx.TxId, from *users.User, to *users.User) {

	if from.Status == users.UserSuspended || to.Status != users.UserSuspended {
		return // no action needed
	}

	// all topic slideshows for user
	shows := s.app.SlideshowStore.ForUser(to.Id, models.SlideshowTopic)
	for _, show := range shows {

		// request to change topic thumbnail
		err := s.app.tm.AddNext(tx, s, OpShow, &OpUpdateTopic{TopicId: show.Topic, Revised: false})
		if err != nil {
			s.app.log(err)
		}
	}
}

// Get user's display name

func (s *GalleryState) UserDisplayName(userId int64) string {

	// serialisation
	defer s.updatesNone()()

	u := s.app.getUserIf(userId)
	if u == nil {
		return ""
	}

	return u.Name
}

// removeSlideshow hides a page, slideshow or topic, initiates cleanup, and optionally requests deferred deletion.
func (s *GalleryState) removeSlideshow(tx etx.TxId, slideshow *models.Slideshow, delete bool) error {

	// set deletion in progress
	slideshow.Access = slideshow.Visible
	slideshow.Visible = models.SlideshowRemoved
	if err := s.app.SlideshowStore.Update(slideshow); err != nil {
		return err
	}

	// request to change topic thumbnail
	if slideshow.Topic != 0 {
		if err := s.app.tm.AddNext(tx, s, OpShow, &OpUpdateTopic{
			TopicId: slideshow.Topic,
			Revised: false,
		}); err != nil {
			return err
		}
	}

	if delete {
		// release slideshows back to users
		if !slideshow.User.Valid {
			s.app.releaseSlideshows(slideshow)
		}

		// request delayed deletion
		if err := s.app.tm.AddTimed(tx, s, OpDropShow, &OpDrop{
			Id:     slideshow.Id,
			Access: slideshow.Visible,
		}, s.app.cfg.DropDelay); err != nil {
			return err
		}
	}

	return nil
}

// validate tags an entry as validated and returns 0, a template and data to confirm a validated entry on success.
func (s *GalleryState) validate(code int64) (int, string, *dataValidated) {

	defer s.updatesGallery()()

	// check if code is valid
	show := s.app.SlideshowStore.GetIfShared(code)
	if show == nil {
		return s.rollback(http.StatusBadRequest, nil), "", nil
	}

	// remove any other entries for this topic
	deleted := 0
	warn := ""
	rivals := s.app.SlideshowStore.ForTopicUserAll(show.Topic, show.User.Int64)
	for _, r := range rivals {
		if r.Id != show.Id {
			s.app.SlideshowStore.DeleteId(r.Id)
			deleted++
		}
	}
	if deleted == 1 {
		warn = "A duplicate entry has been cancelled"
	} else if deleted > 1 {
		warn = strconv.Itoa(deleted) + " duplicate entries have been cancelled"
	}

	// remove validate tag, which triggers addition of successor tags
	// (succeeds if the tag has already been removed)
	if !s.app.tagger.DropTagRef(show.Id, 0, "validate", 0) {
		return s.rollback(http.StatusInternalServerError, nil), "", nil
	}

	// get confirmation details
	u := s.app.getUserIf(show.User.Int64)
	if u == nil {
		return s.rollback(http.StatusInternalServerError, nil), "", nil
	}

	t := s.app.SlideshowStore.GetIf(show.Topic)
	if t == nil {
		return s.rollback(http.StatusInternalServerError, nil), "", nil
	}

	// validated
	return 0, "validated.page.tmpl", &dataValidated{

		Name:  u.Name,
		Email: u.Username,
		Class: t.Title,
		Title: show.Title,
		Warn:  warn,
	}
}

// INTERNAL FUNCTIONS

// editTags recursively processes tag changes.
func (app *Application) editTags(f *multiforms.Form, userId int64, slideshowId int64, tags []*tags.ItemTag) bool {
	for _, tag := range tags {

		// name for form input and element ID
		// (We just use it to identify the field, and don't trust it as a database ID)
		nm := strconv.FormatInt(tag.Id, 36)

		// One of the rubbish parts of HTML. For a checkbox, the name is the tag and any value indicates set.
		// For a radio button, the name is the parent tag and the value is the set tag.
		src := false
		switch tag.Format {
		case "C":
			if f.Get(nm) != "" {
				src = true // any value indicates set, "on" is default value
			}

		case "R":
			radio := strconv.FormatInt(tag.Parent, 36)
			if f.Get(radio) == nm {
				src = true
			}

		default:
			src = tag.Set // not editable
		}

		if src && !tag.Set {

			// set tag reference, and do any corresponding actions
			if !app.tagger.SetTagRef(slideshowId, tag.Parent, tag.Name, userId, "") {
				return false
			}

		} else if !src && tag.Set {

			// drop tag reference, and do any corresponding actions
			if !app.tagger.DropTagRef(slideshowId, tag.Parent, tag.Name, userId) {
				return false
			}
		}

		if !app.editTags(f, userId, slideshowId, tag.Children) {
			return false
		}
	}
	return true
}

// eventFormat returns an auto-format for an event slide.
func (app *Application) eventFormat(e *form.EventFormData) int {

	var f int
	if len(e.Title) > 0 {
		f = models.SlideTitle
	}
	if len(e.Caption) > 0 {
		f = f + models.SlideCaption
	}

	return f
}

// releaseSlideshows releases the contributing slideshows for a topic back to the users.
func (app *Application) releaseSlideshows(t *models.Slideshow) {

	// give the users back their own slideshows
	store := app.SlideshowStore
	slideshows := store.ForTopic(t.Id)
	for _, s := range slideshows {
		s.Title = t.Title // with current topic title
		s.Access = models.SlideshowPrivate
		s.Visible = models.SlideshowPrivate
		store.Update(s)
	}
}

// sanitize returns HTML safe for display, assuming the current value is safe.
func (s *GalleryState) sanitize(new string, current string) string {
	if new == current {
		return current
	}

	return s.publicPages.Sanitize(new)
}

// sanitizeUnless sanitizes HTML, unless it is markdown, in which case the cached version will be sanitised.
func (s *GalleryState) sanitizeUnless(markdown bool, new string, current string) string {
	if markdown {
		return new
	}
	return s.sanitize(new, current)
}

// setVisible changes the visibility of a slideshow.
// If visibility is being reduced, access is left unchanged and a request logged to reduce access later.
func (s *GalleryState) setVisible(tx etx.TxId, show *models.Slideshow, visible int) error {

	if visible >= show.Visible {
		// increase access immediately
		show.Access = visible
		show.Visible = visible

	} else {
		// drop visibility but leave access unchanged for now
		show.Visible = visible

		// request delayed access drop
		if err := s.app.tm.AddTimed(tx, s, OpDropShow, &OpDrop{
			Id:     show.Id,
			Access: visible,
		}, s.app.cfg.DropDelay); err != nil {
			return err
		}
	}
	return nil
}

// shareCode returns an access code for a shared slideshow or topic.
// ## This func seemed like a good idea, got out of hand, :-(
func shareCode(isShared bool, hasCode int64) int64 {
	if isShared {
		if hasCode == 0 {
			c, err := picinch.SecureCode(8)
			if err != nil {
				return 0
			} else {
				return c
			}
		} else {
			return hasCode
		}
	} else {
		return 0
	}
}

// slideFormat returns a format for a slide.
func (app *Application) slideFormat(slide *form.SlideFormData, page bool) int {

	// auto-format
	var f int
	if len(slide.Title) > 0 {
		f = models.SlideTitle
	}
	if len(slide.MediaName) > 0 {
		f = f + slideMedia(app.uploader.MediaType(slide.MediaName))
	}
	if len(slide.Caption) > 0 {
		f = f + models.SlideCaption
	}

	// // validate and add non-default manual format
	if page {
		fm := slide.Format
		// validate and add non-default
		if fm > 0 && fm <= models.SlideFormatMax {
			f = f + fm<<models.SlideFormatShift
		}
	}

	return f
}

// slide media returns the slide type for the specified media type.
func slideMedia(mediaType int) int {

	switch mediaType {
	case uploader.MediaImage:
		return models.SlideImage
	case uploader.MediaVideo:
		return models.SlideVideo
	default:
		return 0 // invalid media type
	}
}
