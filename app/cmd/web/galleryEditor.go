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

	"inchworks.com/picinch/pkg/form"
	"inchworks.com/picinch/pkg/models"
	"inchworks.com/picinch/pkg/picinch"

	"github.com/inchworks/webparts/etx"
	"github.com/inchworks/webparts/multiforms"
	"github.com/inchworks/webparts/uploader"
	"github.com/inchworks/webparts/users"
	"inchworks.com/picinch/pkg/tags"
)

type userTags struct {
	id   int64
	name string
	tags []*tags.ItemTag
}

// Get data to assign slideshows to topics

func (s *GalleryState) ForAssignShows(tok string) (f *form.SlideshowsForm) {

	// serialisation
	defer s.updatesNone()()

	// get slideshows
	slideshows := s.app.SlideshowStore.AllForUsers()

	// form
	var d = make(url.Values)
	f = form.NewSlideshows(d, tok)

	// add template and slideshows to form
	f.AddTemplate()
	for i, sh := range slideshows {
		f.Add(i, sh.Id, sh.Topic, sh.Visible, sh.Shared != 0, sh.Title, s.app.userStore.Name(sh.User.Int64))
	}

	return
}

// Processing when slideshows are assigned to topics.
//
// Returns true if no client errors.

// ## I don't like using database IDs in a form, because it exposes us to a user that manipulates the form.
// ## In this case the user has to be authorised as a curator, and (I think) they can only make changes
// ## that the form allows anyway. Still, I'd like an alternative :-(.

func (s *GalleryState) OnAssignShows(rsSrc []*form.SlideshowFormData) bool {

	// serialisation
	defer s.updatesGallery()()

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
			// normalise topic and visibility
			if rSrc.NTopic != 0 {
				rSrc.Visible = models.SlideshowTopic
			} else if rSrc.Visible == models.SlideshowTopic {
				rSrc.Visible = models.SlideshowPrivate
			}

			// check if details changed
			if rSrc.Visible != rDest.Visible ||
				rSrc.Title != rDest.Title ||
				rSrc.NTopic != rDest.Topic {

				if rSrc.NTopic != 0 && s.app.SlideshowStore.GetIf(rSrc.NTopic) == nil {
					nConflicts++ // another curator deleted the topic!

				} else {
					rDest.Visible = rSrc.Visible
					rDest.Title = rSrc.Title
					rDest.Topic = rSrc.NTopic

					s.app.SlideshowStore.Update(rDest)
				}
			}
		}
		i++
	}

	return nConflicts == 0
}

// Get data to edit gallery

func (s *GalleryState) ForEditGallery(tok string) (f *multiforms.Form) {

	// serialisation
	defer s.updatesNone()()

	// current data
	var d = make(url.Values)
	f = multiforms.New(d, tok)
	f.Set("organiser", s.gallery.Organiser)
	f.Set("nMaxSlides", strconv.Itoa(s.gallery.NMaxSlides))
	f.Set("nShowcased", strconv.Itoa(s.gallery.NShowcased))

	return
}

// Processing when gallery is modified.
//
// Returns HTTP status or 0.

func (s *GalleryState) OnEditGallery(organiser string, nMaxSlides int, nShowcased int) int {

	// serialisation
	defer s.updatesGallery()()

	// save changes via cache (conversions already checked)
	s.gallery.Organiser = organiser
	s.gallery.NMaxSlides = nMaxSlides
	s.gallery.NShowcased = nShowcased
	if err := s.app.GalleryStore.Update(s.gallery); err != nil {
		return s.rollback(http.StatusBadRequest, err)
	}

	return 0
}

// Get data to edit a slideshow

func (s *GalleryState) ForEditSlideshow(showId int64, tok string) (status int, f *form.SlidesForm, show *models.Slideshow) {

	// serialisation
	defer s.updatesGallery()()

	// title and slides
	show = s.app.SlideshowStore.GetIf(showId)
	if show == nil {
		status = s.rollback(http.StatusNotFound, nil); return
	}
	slides := s.app.SlideStore.ForSlideshow(show.Id, 100)

	// start multi-step transaction for uploaded files
	ts, err := s.app.uploader.Begin()
	if err != nil {
		status = s.rollback(http.StatusInternalServerError, err); return
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
		_, image, _ := uploader.NameFromFile(sl.Image)
		f.Add(i, sl.ShowOrder, sl.Title, image, sl.Caption)
	}

	return
}

// OnEditSlideshow processes the modification of a slideshow. It returns 0 and the user ID on success, or an HTTP status code.
// topicId and userId are needed only for a new slideshow for a topic. Otherwise we prefer to trust the database.
func (s *GalleryState) OnEditSlideshow(showId int64, topicId int64, tx etx.TxId, userId int64, qsSrc []*form.SlideFormData) (int, int64) {

	// serialisation
	defer s.updatesGallery()()

	// check for a request that has been running so long that we have discarded the uploads
	if !s.app.uploader.ValidCode(tx) {
		return s.rollback(http.StatusRequestTimeout, nil), 0
	}

	now := time.Now()
	nSrc := len(qsSrc)
	revised := false

	if showId != 0 {
		// slideshow already exists
		show := s.app.SlideshowStore.GetIf(showId)
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
		show := s.app.SlideshowStore.ForTopicUserIf(topicId, userId)
		if show == nil {

			// create a new slideshow from the topic details
			show = &models.Slideshow{
				GalleryOrder: 5, // default
				Visible:      models.SlideshowTopic,
				User:         sql.NullInt64{Int64: userId, Valid: true},
				Topic:        topicId,
				Created:      now,
				Revised:      now,
				Title:        topic.Title,
			}
		}
		s.app.SlideshowStore.Update(show)
		showId = show.Id
	}

	// compare modified slides against current slides, and update
	qsDest := s.app.SlideStore.ForSlideshow(showId, 100)

	updated := false
	nDest := len(qsDest)

	iSrc := 1 // skip template slide
	var iDest int

	for iSrc < nSrc || iDest < nDest {

		if iSrc == nSrc {
			// no more source slides - delete from destination
			s.app.SlideStore.DeleteId(qsDest[iDest].Id)
			updated = true
			iDest++

		} else if iDest == nDest {
			// no more destination slides - add new one
			imageName := uploader.CleanName(qsSrc[iSrc].ImageName)
			qd := models.Slide{
				Slideshow: showId,
				Format:    s.app.slideFormat(qsSrc[iSrc]),
				ShowOrder: qsSrc[iSrc].ShowOrder,
				Created:   now,
				Revised:   now,
				Title:     s.sanitize(qsSrc[iSrc].Title, ""),
				Caption:   s.sanitize(qsSrc[iSrc].Caption, ""),
				Image:     uploader.FileFromName(tx, imageName),
			}
			// only a new media file is counted as a revision to the slideshow
			if imageName != "" {
				revised = true
			}

			s.app.SlideStore.Update(&qd)
			updated = true
			iSrc++

		} else {
			ix := qsSrc[iSrc].ChildIndex
			if ix > iDest {
				// source slide removed - delete from destination
				s.app.SlideStore.DeleteId(qsDest[iDest].Id)
				updated = true
				iDest++

			} else if ix == iDest {
				// check if details changed
				// (checking media name at this point, version change will be handled later)
				imageName := uploader.CleanName(qsSrc[iSrc].ImageName)
				qDest := qsDest[iDest]
				_, dstName, _ := uploader.NameFromFile(qDest.Image)
				if qsSrc[iSrc].ShowOrder != qDest.ShowOrder ||
					qsSrc[iSrc].Title != qDest.Title ||
					qsSrc[iSrc].Caption != qDest.Caption ||
					imageName != dstName {

					qDest.Format = s.app.slideFormat(qsSrc[iSrc])
					qDest.ShowOrder = qsSrc[iSrc].ShowOrder
					qDest.Revised = now
					qDest.Title = s.sanitize(qsSrc[iSrc].Title, qDest.Title)
					qDest.Caption = s.sanitize(qsSrc[iSrc].Caption, qDest.Caption)

					// If the media name hasn't changed, leave the old version in use for now,
					// so that the slideshow still works. We'll detect a version change later.
					if imageName != dstName {
						qDest.Image = uploader.FileFromName(tx, imageName)
					}

					s.app.SlideStore.Update(qDest)
					updated = true
				}
				iSrc++
				iDest++

			} else {
				// out of sequence question index
				return s.rollback(http.StatusBadRequest, nil), 0
			}
		}
	}

	// re-sequence slides, removing missing or duplicate orders
	// If two slides have the same order, the later update comes first
	if updated {

		// ## think I have to commit changes for them to appear in a new query
		// ## but this makes the unsequenced changes visible briefly, or would if I weren't serialising at server level
		s.save()

		sls := s.app.SlideStore.ForSlideshow(showId, 100)

		for ix, sl := range sls {
			nOrder := ix + 1
			if sl.ShowOrder != nOrder {

				// update sequence
				sl.ShowOrder = nOrder
				s.app.SlideStore.Update(sl)
			}
		}
	}

	// Note that if showId is still 0 at this point, the user submitted a slideshow with no images for a topic.
	// We'll ignore it. The uploader's timeout operation will be called via uploader.DoNext.

	if showId != 0 {
		// request worker to generate media versions, and remove unused images
		if err := s.txShow(
			&OpUpdateShow{
				ShowId:  showId,
				TopicId: topicId,
				tx:      tx,
				Revised: revised,
			},
			OpShow); err != nil {
			return s.rollback(http.StatusInternalServerError, err), 0
		}
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
			if err := s.onRemoveSlideshow(tx, rsDest[iDest]); err != nil {
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
				Visible:      visible,
				User:         sql.NullInt64{Int64: userId, Valid: true},
				Created:      created,
				Revised:      now,
				Title:        s.sanitize(rsSrc[iSrc].Title, ""),
			}
			s.app.SlideshowStore.Update(&r)
			iSrc++

		} else {
			ix := rsSrc[iSrc].ChildIndex
			if ix > iDest {
				// source slideshow removed - delete from destination
				if err := s.onRemoveSlideshow(tx, rsDest[iDest]); err != nil {
					return s.rollback(http.StatusBadRequest, err), 0
				}
				iDest++

			} else if ix == iDest {
				// check if details changed
				rSrc := rsSrc[iSrc]
				rDest := rsDest[iDest]

				if rSrc.Visible != rDest.Visible ||
					rSrc.Title != rDest.Title {

					rDest.Visible = rSrc.Visible
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
				// out of sequence round index
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
			return
		}
		title = topic.Title

	} else {
		// user's existing contribution to topic
		showId = show.Id
		title = show.Title
		slides = s.app.SlideStore.ForSlideshow(showId, 100)
	}

	// start multi-step transaction for uploaded files
	ts, err := s.app.uploader.Begin()
	if err != nil {
		status = s.rollback(http.StatusInternalServerError, err); return
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
		_, image, _ := uploader.NameFromFile(sl.Image)
		f.Add(i, sl.ShowOrder, sl.Title, image, sl.Caption)
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
			// no more source slideshows - delete from destination
			s.onRemoveTopic(rsDest[iDest])
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
				s.onRemoveTopic(rsDest[iDest])
				iDest++

			} else if ix == iDest {
				// check if details changed
				rSrc := rsSrc[iSrc]
				rDest := rsDest[iDest]

				if rSrc.Visible != rDest.Visible ||
					rSrc.Title != rDest.Title ||
					rSrc.IsShared != (rDest.Shared > 0) {

					rDest.Visible = rSrc.Visible
					rDest.Shared = shareCode(rSrc.IsShared, rDest.Shared)
					rDest.Title = s.sanitize(rSrc.Title, rDest.Title)

					// set creation date just once, when published
					if rSrc.Visible > models.SlideshowPrivate && rDest.Created.IsZero() {
						rDest.Created = now
						rDest.Revised = now

						// needs a media file before it will appear on home page
						if err := s.txBeginTopic(tx, &OpUpdateTopic{
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
				// out of sequence index
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
		status = s.rollback(http.StatusInternalServerError, err); return
	} else if show == nil {
		status = s.rollback(http.StatusNotFound, nil); return
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
	if !s.app.uploader.ValidCode(tx) {
		return s.rollback(http.StatusRequestTimeout, nil), -1
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
		Image:     uploader.FileFromName(tx, image),
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

	// request worker to generate media version, remove unused images, and send validation email
	if err := s.txShow(&OpUpdateShow{ShowId: show.Id, tx: tx, Revised: false}, OpComp); err != nil {
		return s.rollback(http.StatusInternalServerError, err), -1
	}

	// auto validation is not needed if we can send emails
	if s.app.cfg.EmailHost != "" {
		vc = 0
	}
	return 0, vc
}

// OnRemoveUser removes a user's media files from the system.
func (s *GalleryState) OnRemoveUser(tx etx.TxId, user *users.User) {

	// all slideshow IDs for user
	shows := s.app.SlideshowStore.ForUser(user.Id, models.SlideshowTopic)
	for _, show := range shows {
		s.txBeginShow(tx, &OpUpdateShow{
			ShowId:  show.Id,
			TopicId: show.Topic,
			Revised: false})
	}

	// slideshows and slides will be removed by cascade delete in caller
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

// onRemoveSlideshow does cleanup when a slideshow is removed.
func (s *GalleryState) onRemoveSlideshow(tx etx.TxId, slideshow *models.Slideshow) error {

	topicId := slideshow.Topic

	// slides will be removed by cascade delete
	s.app.SlideshowStore.DeleteId(slideshow.Id)

	// request worker to remove media files, and change topic image
	return s.txBeginShow(tx, &OpUpdateShow{
		ShowId:  slideshow.Id,
		TopicId: topicId,
		tx:      0,
		Revised: false},
	)
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

// onRemoveTopic releases the contributing slideshows back to the users, and deletes the topic.
func (s *GalleryState) onRemoveTopic(t *models.Slideshow) {

	// give the users back their own slideshows
	store := s.app.SlideshowStore
	slideshows := store.ForTopic(t.Id)
	for _, s := range slideshows {
		s.Topic = 0
		s.Title = t.Title // with current topic title
		s.Visible = models.SlideshowPrivate
		store.Update(s)
	}

	s.app.SlideshowStore.DeleteId(t.Id)
}

// Sanitize HTML for reuse

func (s *GalleryState) sanitize(new string, current string) string {
	if new == current {
		return current
	}

	return s.app.sanitizer.Sanitize(new)
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

// slideFormat returns an auto-format for a slide.
func (app *Application) slideFormat(slide *form.SlideFormData) int {

	var f int
	if len(slide.Title) > 0 {
		f = models.SlideTitle
	}
	if len(slide.ImageName) > 0 {
		f = f + slideMedia(app.uploader.MediaType(slide.ImageName))
	}
	if len(slide.Caption) > 0 {
		f = f + models.SlideCaption
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

// txBeginTopic requests a topic update as a new extended transaction.
func (s *GalleryState) txBeginTopic(tx etx.TxId, req *OpUpdateTopic) error {

	// ## could log error
	return s.app.tm.BeginNext(tx, s, OpTopic, req)
}

// txBeginShow requests a show update as a new extended transaction.
func (s *GalleryState) txBeginShow(tx etx.TxId, req *OpUpdateShow) error {

	// ## could log error
	return s.app.tm.BeginNext(tx, s, OpShow, req)
}

// txShow requests a show update as a transaction, so that it will be done even if the server restarts.
func (s *GalleryState) txShow(req *OpUpdateShow, opType int) error {
	return s.app.tm.SetNext(req.tx, s, opType, req)
}
