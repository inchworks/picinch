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

// Processing for gallery editing / setup.
//
// These functions may modify application state.

import (
	"net/url"
	"strconv"
	"time"

	"inchworks.com/picinch/pkg/form"
	"inchworks.com/picinch/pkg/images"
	"inchworks.com/picinch/pkg/models"
)

// Get data to assign slideshows to topics

func (s *GalleryState) ForAssignShows() (f *form.SlideshowsForm) {

	// serialisation
	defer s.updatesNone()()

	// get slideshows
	slideshows := s.app.SlideshowStore.All()

	// form
	var d = make(url.Values)
	f = form.NewSlideshows(d)

	// add template and slideshows to form
	f.AddTemplate()
	for i, sh := range slideshows {
		f.Add(i, sh.Id, sh.Topic, sh.Visible, sh.Title, s.app.UserStore.Name(sh.User))
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
			nConflicts++  // just deleted by user

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

				if rSrc.NTopic != 0 && s.app.TopicStore.GetIf(rSrc.NTopic) == nil {
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

func (s *GalleryState) ForEditGallery() (f *form.Form) {

	// serialisation
	defer s.updatesNone()()

	// current data
	var d = make(url.Values)
	f = form.New(d)
	f.Set("organiser", s.gallery.Organiser)
	f.Set("nMaxSlides", strconv.Itoa(s.gallery.NMaxSlides))
	f.Set("nShowcased", strconv.Itoa(s.gallery.NShowcased))

	return
}

// Processing when gallery is modified.
//
// Returns true if no client errors.

func (s *GalleryState) OnEditGallery(organiser string, nMaxSlides int, nShowcased int) bool {

	// serialisation
	defer s.updatesGallery()()

	// save changes via cache (conversions already checked)
	s.gallery.Organiser = organiser
	s.gallery.NMaxSlides = nMaxSlides
	s.gallery.NShowcased = nShowcased
	s.app.GalleryStore.Update(s.gallery)

	return true
}

// Get data to edit a slideshow

func (s *GalleryState) ForEditSlideshow(showId int64) (f *form.SlidesForm, show *models.Slideshow) {

	// serialisation
	defer s.updatesNone()()

	// title and slides
	show, _ = s.app.SlideshowStore.Get(showId)
	slides := s.app.SlideStore.ForSlideshow(show.Id, 100)

	// form
	var d = make(url.Values)
	f = form.NewSlides(d, len(slides))

	// template for new slide form
	f.AddTemplate(len(slides))

	// add slides to form
	for i, sl := range slides {
		_, image, _ := images.NameFromFile(sl.Image)
		f.Add(i, sl.ShowOrder, sl.Title, image, sl.Caption)
	}

	return
}

// Processing when slideshow is modified

func (s *GalleryState) OnEditSlideshow(showId int64, qsSrc []*form.SlideFormData) (ok bool, userId int64) {

	// serialisation
	defer s.updatesGallery()()

	now := time.Now()

	// compare modified slides against current slides, and update
	qsDest := s.app.SlideStore.ForSlideshow(showId, 100)

	updated := false
	nSrc := len(qsSrc)
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
			qd := models.Slide{
				Slideshow: showId,
				Format:    slideFormat(qsSrc[iSrc]),
				ShowOrder: qsSrc[iSrc].ShowOrder,
				Created:   now,
				Revised:   now,
				Title:     s.sanitize(qsSrc[iSrc].Title, ""),
				Caption:   s.sanitize(qsSrc[iSrc].Caption, ""),
				Image:     images.FileFromName(showId, qsSrc[iSrc].ImageName, 0),
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
				// (checking image name at this point, version change will be handled later)
				qDest := qsDest[iDest]
				_, dstName, _ := images.NameFromFile(qDest.Image)
				if qsSrc[iSrc].ShowOrder != qDest.ShowOrder ||
					qsSrc[iSrc].Title != qDest.Title ||
					qsSrc[iSrc].Caption != qDest.Caption ||
					qsSrc[iSrc].ImageName != dstName {

					qDest.Format = slideFormat(qsSrc[iSrc])
					qDest.ShowOrder = qsSrc[iSrc].ShowOrder
					qDest.Revised = now
					qDest.Title = s.sanitize(qsSrc[iSrc].Title, "")
					qDest.Caption = s.sanitize(qsSrc[iSrc].Caption, "")
					qDest.Image = images.FileFromName(showId, qsSrc[iSrc].ImageName, 0)

					s.app.SlideStore.Update(qDest)
					updated = true
				}
				iSrc++
				iDest++

			} else {
				// out of sequence question index
				return
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

	// request worker to generate image versions, and remove unused images
	s.app.chShowId <- showId

	// then worker should change the topic thumbnail, in case we just updated or removed the current one
	show, err := s.app.SlideshowStore.Get(showId)
	if err == nil && show.Topic != 0 {
		s.app.chTopicId <- show.Topic
	}

	ok = true
	userId = show.User
	return
}

// Get data to edit slideshows for a user

func (s *GalleryState) ForEditSlideshows(userId int64) (f *form.SlideshowsForm, user *models.User) {

	// serialisation
	defer s.updatesNone()()

	// get user
	user, _ = s.app.UserStore.Get(userId)

	// get slideshows
	slideshows := s.app.SlideshowStore.ForUser(userId, models.SlideshowPrivate)

	// form
	var d = make(url.Values)
	f = form.NewSlideshows(d)

	// add template and slideshows to form
	f.AddTemplate()
	for i, sh := range slideshows {
		f.Add(i, sh.Id, sh.Topic, sh.Visible, sh.Title, "")
	}

	return
}

// Processing when slideshows are modified.
//
// Returns true if no client errors.

func (s *GalleryState) OnEditSlideshows(userId int64, rsSrc []*form.SlideshowFormData) bool {

	// serialisation
	defer s.updatesGallery()()

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
			s.onRemoveSlideshow(rsDest[iDest])
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
				User:         userId,
				Created:      created,
				Revised:      now,
				Title:        rsSrc[iSrc].Title,
			}
			s.app.SlideshowStore.Update(&r)
			iSrc++

		} else {
			ix := rsSrc[iSrc].ChildIndex
			if ix > iDest {
				// source slideshow removed - delete from destination
				s.onRemoveSlideshow(rsDest[iDest])
				iDest++

			} else if ix == iDest {
				// check if details changed
				rSrc := rsSrc[iSrc]
				rDest := rsDest[iDest]

				if rSrc.Visible != rDest.Visible ||
					rSrc.Title != rDest.Title {

					rDest.Visible = rSrc.Visible
					rDest.Title = rSrc.Title

					// set creation date just once, when published
					if rSrc.Visible > models.SlideshowPrivate && rDest.Created.IsZero() {
						rDest.Created = now
					}
					rDest.Revised = now

					s.app.SlideshowStore.Update(rDest)
				}
				iSrc++
				iDest++

			} else {
				// out of sequence round index
				return false
			}
		}
	}

	return true
}

// Get data to edit a user's contribution to a topic

func (s *GalleryState) ForEditTopic(topicId int64, userId int64) (f *form.SlidesForm, show *models.Slideshow) {

	// serialisation
	defer s.updatesGallery()()

	// user's show for topic
	show = s.app.SlideshowStore.ForTopicUser(topicId, userId)
	if show == nil {
		topic, _ := s.app.TopicStore.Get(topicId)

		// create slideshow for topic
		now := time.Now()

		show = &models.Slideshow{
			GalleryOrder: 5, // default
			Visible:      models.SlideshowTopic,
			User:         userId,
			Topic:        topicId,
			Created:      now,
			Revised:      now,
			Title:        topic.Title,
		}
		s.app.SlideshowStore.Update(show)
	}

	// slides
	slides := s.app.SlideStore.ForSlideshow(show.Id, 100)

	// form
	var d = make(url.Values)
	f = form.NewSlides(d, len(slides))

	// template for new slide form
	f.AddTemplate(len(slides))

	// add slides to form
	for i, sl := range slides {
		_, image, _ := images.NameFromFile(sl.Image)
		f.Add(i, sl.ShowOrder, sl.Title, image, sl.Caption)
	}

	return
}

// Get data to edit topics

func (s *GalleryState) ForEditTopics() (f *form.SlideshowsForm) {

	// serialisation
	defer s.updatesNone()()

	// get topics
	topics := s.app.TopicStore.AllEditable()

	// form
	var d = make(url.Values)
	f = form.NewSlideshows(d)

	// add template and slideshows to form
	f.AddTemplate()
	for i, sh := range topics {
		f.Add(i, sh.Id, 0, sh.Visible, sh.Title, "")
	}

	return
}

// Processing when topics are modified.
//
// Returns true if no client errors.

func (s *GalleryState) OnEditTopics(rsSrc []*form.SlideshowFormData) bool {

	// ## should combine with OnEditSlideshows, since they are so similar. Or even all of the multi-item forms?

	// serialisation
	defer s.updatesGallery()()

	now := time.Now()

	// compare modified slideshows against current ones, and update
	rsDest := s.app.TopicStore.AllEditable()

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
			r := models.Topic{
				GalleryOrder: 5, // default order
				Visible:      visible,
				Created:      created,
				Revised:      now,
				Title:        rsSrc[iSrc].Title,
			}
			s.app.TopicStore.Update(&r)
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
					rSrc.Title != rDest.Title {

					rDest.Visible = rSrc.Visible
					rDest.Title = rSrc.Title

					// set creation date just once, when published
					if rSrc.Visible > models.SlideshowPrivate && rDest.Created.IsZero() {
						rDest.Created = now
					}
					rDest.Revised = now

					s.app.TopicStore.Update(rDest)
				}
				iSrc++
				iDest++

			} else {
				// out of sequence round index
				return false
			}
		}
	}

	return true
}

// Auto-format for slide

func slideFormat(slide *form.SlideFormData) int {

	var f int
	if len(slide.Title) > 0 {
		f = models.SlideT
	}
	if len(slide.ImageName) > 0 {
		f = f + models.SlideI
	}
	if len(slide.Caption) > 0 {
		f = f + models.SlideC
	}

	return f
}

// Processing when a slideshow is removed

func (s *GalleryState) onRemoveSlideshow(slideshow *models.Slideshow) {

	topicId := slideshow.Topic

	// slides will be removed by cascade delete
	s.app.SlideshowStore.DeleteId(slideshow.Id)

	// request worker to remove images, and change topic image
	s.app.chShowId <- slideshow.Id
	if topicId != 0 {
		s.app.chTopicId <- topicId
	}

}

// Processing when a topic is removed

func (s *GalleryState) onRemoveTopic(t *models.Topic) {

	// give the users back their own slideshows
	store := s.app.SlideshowStore
	slideshows := store.ForTopic(t.Id)
	for _, s := range slideshows {
		s.Topic = 0
		s.Visible = models.SlideshowPrivate
		store.Update(s)
	} 

	s.app.TopicStore.DeleteId(t.Id)
}

// Sanitize HTML for reuse

func (s *GalleryState) sanitize(new string, current string) string {
	if new == current {
		return current
	}

	return s.app.sanitizer.Sanitize(new)
}
