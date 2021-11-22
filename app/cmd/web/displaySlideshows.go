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

// Processing related to slideshows
//
// These functions should not modify application state.

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"

	"inchworks.com/picinch/pkg/models"
)

// Copyright © Rob Burke inchworks.com, 2020.

// displayClasses returns data for competition classes.
func (s *GalleryState) displayClasses(member bool) *dataCompetition {

	defer s.updatesNone()()

	a := s.app

	// ## restrict to published categories
	dShows := s.dataShowsPublished(
		a.SlideshowStore.AllEditableTopics(), a.cfg.MaxSlideshowsPublic, a.cfg.MaxSlideshowsTotal)

	// template and its data
	return &dataCompetition{
		Categories: dShows,
	}
}

// List of published slideshows for a user

func (s *GalleryState) DisplayContributor(userId int64) *DataHome {

	defer s.updatesNone()()

	// user
	user, err := s.app.userStore.Get(userId)
	if err != nil {
		s.app.log(err)
		return nil
	}

	// highlights
	var dHighlights []*DataSlide
	show := s.app.SlideshowStore.ForTopicUserIf(s.app.SlideshowStore.HighlightsId, user.Id)
	if show != nil {
		dHighlights = s.dataSlides(show.Id, s.app.cfg.MaxHighlightsTotal)
	}

	// slideshows
	slideshows := s.app.SlideshowStore.ForUserPublished(user.Id)
	var dShows []*DataPublished

	for _, show := range slideshows {

		dShows = append(dShows, &DataPublished{
			Id:    show.Id,
			Title: show.Title,
			Image: show.Image,
		})
	}

	// template and its data
	return &DataHome{
		DisplayName: user.Name,
		Highlights:  dHighlights,
		Slideshows:  dShows,
	}
}

// Users that have slideshows

func (s *GalleryState) DisplayContributors() (string, *DataUsers) {

	defer s.updatesNone()()

	users := s.app.userStore.Contributors()
	if users == nil {
		return "no-contributors.page.tmpl", &DataUsers{}
	}

	return "contributors.page.tmpl", &DataUsers{
		Users: users,
	}
}

// Display highlights for embedded page

func (s *GalleryState) DisplayEmbedded(nImages int) *DataSlideshow {

	dataSlides := s.dataHighlights(nImages)

	// template and its data
	return &DataSlideshow{
		Slides: dataSlides,
	}
}

// Home page with slideshows

func (s *GalleryState) DisplayHome(member bool) *DataHome {

	defer s.updatesNone()()

	a := s.app

	// highlight slides
	dHighlights := s.dataHighlights(a.cfg.MaxHighlightsTotal)

	var dShows []*DataPublished
	if member {
		dShows = s.dataShowsPublished(
			a.SlideshowStore.RecentPublished(models.SlideshowClub, a.cfg.MaxSlideshowsClub), a.cfg.MaxSlideshowsClub, a.cfg.MaxSlideshowsTotal)
	} else {
		dShows = s.dataShowsPublished(
			a.SlideshowStore.RecentPublished(models.SlideshowPublic, a.cfg.MaxSlideshowsPublic), a.cfg.MaxSlideshowsPublic, a.cfg.MaxSlideshowsTotal)
	}

	// template and its data
	return &DataHome{
		Highlights: dHighlights,
		Slideshows: dShows,
	}
}

// DisplayTopicShared selects a template and data for a shared slideshow or topic
func (s *GalleryState) DisplayShared(code int64, seq int) (string, *DataSlideshow) {

	defer s.updatesNone()()

	// check if code is valid
	slideshow := s.app.SlideshowStore.GetIfShared(code)
	if slideshow == nil {
		return "", nil
	}

	// return to this page at the end, as we have nowhere else to go.
	from := href(true, slideshow, 0)

	if slideshow.User.Valid {

		// this is a single slideshow
		return "carousel-default.page.tmpl", s.displaySlides(slideshow, 0, from, s.app.cfg.MaxSlides)
	}

	// this is a topic
	topic := slideshow

	if seq == 0 {
		// next is first user's slides
		after, before := s.topicHRefs(topic, 0, true, from)

		// home page for topic
		// Annoyingly, the slideshow must have at least two slides,
		// otherwise Bootstrap Carousel doesn't give any events to trigger loading of the first user's slideshow.
		return "carousel-shared.page.tmpl", &DataSlideshow{
			Title:      topic.Title,
			Caption:    s.app.galleryState.gallery.Organiser,
			AfterHRef:  after,
			BeforeHRef: before,
			DataCommon: DataCommon{
				ParentHRef: from,
			},
		}
	} else {
		// contribution to topic
		return s.displayTopic(topic, true, seq, from)
	}
}

// DisplaySlideshow returns data for a slideshow.
func (s *GalleryState) DisplaySlideshow(id int64, forRole int, from string) *DataSlideshow {

	defer s.updatesNone()()

	// get slideshow ..
	show, err := s.app.SlideshowStore.Get(id)
	if err != nil {
		return nil
	}

	// .. and slides
	return s.displaySlides(show, forRole, from, s.app.cfg.MaxSlides)
}

// DisplayTopicHome returns a template and data for a topic shown on a website page.
func (s *GalleryState) DisplayTopicHome(id int64, seq int, from string) (string, *DataSlideshow) {

	defer s.updatesNone()()

	// check if topic ID is valid
	topic, _ := s.app.SlideshowStore.Get(id)
	if topic == nil {
		return "", nil
	}
	fmt, max := topic.ParseFormat()

	// special selection and ordering for highlights
	if fmt == "H" {
		return s.displayHighlights(topic, from, max)
	}

	return s.displayTopic(topic, false, seq, from)
}

// Slideshows for a topic

func (s *GalleryState) DisplayTopicContributors(id int64) *DataSlideshows {

	defer s.updatesNone()()

	topic, err := s.app.SlideshowStore.Get(id)
	if err != nil {
		return nil
	}

	// show latest highlights first, other topics in published order
	latest := false
	fmt, _ := topic.ParseFormat()
	if fmt == "H" {
		latest = true
	}

	// get published slideshows for topic
	slideshows := s.app.SlideshowStore.ForTopicPublished(id, latest)
	var dShows []*DataPublished

	for _, s := range slideshows {
		dShows = append(dShows, &DataPublished{
			Id:          s.Id,
			Title:       s.Title,
			Image:       s.Image,
			DisplayName: s.Name,
		})
	}

	return &DataSlideshows{
		Title:      topic.Title,
		Slideshows: dShows,
	}
}

// DisplayTopicUser returns slides for a user's contribution to a topic.
func (s *GalleryState) DisplayTopicUser(topicId int64, userId int64, from string) *DataSlideshow {

	defer s.updatesNone()()

	// get slideshow
	show := s.app.SlideshowStore.ForTopicUserIf(topicId, userId)
	if show == nil {
		return nil
	}

	// .. and slides
	return s.displaySlides(show, 0, from, s.app.cfg.MaxSlides)
}

// User's view of gallery - just their name, topics and own slideshows at present

func (s *GalleryState) ForMyGallery(userId int64) *DataMyGallery {

	// serialisation
	defer s.updatesNone()()

	// get user
	user, _ := s.app.userStore.Get(userId)

	// get slideshows
	slideshows := s.app.SlideshowStore.ForUser(userId, models.SlideshowPrivate)
	var dataShows []*DataMySlideshow

	for _, s := range slideshows {

		dataShows = append(dataShows, &DataMySlideshow{
			NShow:   s.Id,
			Title:   s.Title,
			Visible: s.VisibleStr(),
		})
	}

	// get topics
	topics := s.app.SlideshowStore.AllTopics()
	var dataTopics []*DataMySlideshow

	for _, t := range topics {

		dataTopics = append(dataTopics, &DataMySlideshow{
			NShow:   t.Id,
			NUser:   userId,
			Title:   t.Title,
			Visible: t.VisibleStr(),
		})
	}

	return &DataMyGallery{
		NUser:       user.Id,
		DisplayName: user.Name,
		Slideshows:  dataShows,
		Topics:      dataTopics,
	}
}

// Curator's view of topics (similar to user's view of gallery)

func (s *GalleryState) ForTopics() *DataMyGallery {

	// serialisation
	defer s.updatesNone()()

	// get topics
	topics := s.app.SlideshowStore.AllTopics()
	var dataShows []*DataMySlideshow

	for _, topic := range topics {

		dataShows = append(dataShows, &DataMySlideshow{
			NShow:   topic.Id,
			Title:   topic.Title,
			Visible: topic.VisibleStr(),
			Shared:  s.formatShared(topic.Shared),
		})
	}

	return &DataMyGallery{
		DisplayName: "Topics",
		Topics:      dataShows,
	}
}

// Curator's view of users

func (s *GalleryState) ForUsers() *DataUsers {

	// serialisation
	defer s.updatesNone()()

	// get all users
	users := s.app.userStore.ByName()

	return &DataUsers{
		Users: users,
	}
}

// Highlighted gets a highlighted image, for a parent website.
func (s *GalleryState) Highlighted(prefix string, nImage int) (fs.FS, string) {

	if nImage >= s.app.cfg.MaxHighlightsParent || nImage < 1 {
		return nil, ""
	} // silly image number

	// get cached image name
	if nImage <= len(s.highlights) {
		image := s.highlights[nImage-1]

		// with specified prefix as first character (main or thumbnail)
		image = prefix[:1] + image[1:]

		return os.DirFS(ImagePath), image

	} else {
		return s.app.staticFS, "images/no-photos-white.jpg"
	}
}

// Get slideshow title

func (s *GalleryState) SlideshowTitle(showId int64) string {

	// serialisation
	defer s.updatesNone()()

	r, _ := s.app.SlideshowStore.Get(showId)

	return r.Title
}

// Highlights for home page or embedded page

func (s *GalleryState) dataHighlights(nImages int) []*DataSlide {

	// get slides for highlights topic
	slides := s.app.SlideStore.RecentForTopic(s.app.SlideshowStore.HighlightsId, s.app.cfg.MaxHighlights, nImages)

	// replace slide data with HTML formatted fields
	var dataSlides []*DataSlide
	for _, slide := range slides {
		dataSlides = append(dataSlides, &DataSlide{
			Title:       models.Nl2br(slide.Title),
			Caption:     models.Nl2br(slide.Caption),
			DisplayName: slide.Name,
			Image:       slide.Image,
			Format:      slide.Format,
		})
	}

	return dataSlides
}

// Recent slides for a slideshow

// Highlights for home page or embedded page

func (s *GalleryState) dataSlides(showId int64, max int) []*DataSlide {

	// get slides for highlights topic s.highlightsId
	slides := s.app.SlideStore.RecentForSlideshow(showId, max)

	// replace slide data with HTML formatted fields
	var dataSlides []*DataSlide
	for _, slide := range slides {
		dataSlides = append(dataSlides, &DataSlide{
			Title:   slide.TitleBr(),
			Caption: slide.CaptionBr(),
			Image:   slide.Image,
			Format:  slide.Format,
		})
	}

	return dataSlides
}

// dataShowsPublished returns public or club slideshows and topics for home page.
// It is also called for competition classes.
func (s *GalleryState) dataShowsPublished(shows []*models.Slideshow, maxUser int, maxTotal int) []*DataPublished {

	a := s.app
	count := make(map[int64]int, 16) // count slideshows per-user

	var data []*DataPublished
	var total int

	for _, show := range shows {

		if show.User.Valid {

			// slideshow - check if user's limit reached
			userId := show.User.Int64
			if count[userId] < maxUser {

				// contributor of slideshow
				user, err := a.userStore.Get(userId)
				if err != nil {
					a.log(err)
					return nil
				}

				// data for display
				data = append(data, &DataPublished{
					Id:          show.Id,
					Title:       show.Title,
					Image:       show.Image,
					NUser:       userId,
					DisplayName: user.Name,
				})

				// count for user
				count[userId]++
			}
		} else {
			// topic - data for display
			data = append(data, &DataPublished{
				Id:      show.Id,
				Title:   show.Title,
				Caption: models.Nl2br(show.Caption),
				Image:   show.Image,
			})
		}

		// limit on total slideshows and topics
		total++
		if total == maxTotal {
			break
		}
	}
	return data
}

// Display highlights : latest slides

func (s *GalleryState) displayHighlights(topic *models.Slideshow, from string, perUser int) (string, *DataSlideshow) {

	// get slides for topic
	slides := s.app.SlideStore.RecentForTopic(topic.Id, perUser, s.app.cfg.MaxHighlightsTopic)

	// replace slide data with HTML formatted fields
	var dataSlides []*DataSlide
	for _, slide := range slides {
		dataSlides = append(dataSlides, &DataSlide{
			Title:       models.Nl2br(slide.Title),
			Caption:     models.Nl2br(slide.Caption),
			DisplayName: slide.Name,
			Image:       slide.Image,
			Format:      slide.Format,
		})
	}

	// template and its data
	return "carousel-highlights.page.tmpl", &DataSlideshow{
		Title:      topic.Title,
		AfterHRef:  from,
		BeforeHRef: from,
		Slides:     dataSlides,
		DataCommon: DataCommon{
			ParentHRef: from,
		},
	}
}

// displaySlides returns slides for a slideshow or a user's own view of a topic contribution.
func (s *GalleryState) displaySlides(show *models.Slideshow, forRole int, from string, max int) *DataSlideshow {

	slides := s.app.SlideStore.ForSlideshow(show.Id, max)
	user, err := s.app.userStore.Get(show.User.Int64)
	if err != nil {
		return nil
	}

	// replace slide data with HTML formatted fields
	var dataSlides []*DataSlide
	for _, slide := range slides {
		dataSlides = append(dataSlides, &DataSlide{
			Title:   slide.TitleBr(),
			Caption: slide.CaptionBr(),
			Image:   slide.Image,
			Format:  slide.Format,
		})
	}

	// competition reference
	var ref string
	switch forRole {
	case models.UserAdmin:
		ref = "#" + strconv.FormatInt(show.Id, 10) + " : " + user.Name + "<" + user.Username + ">"

	case models.UserMember:
		ref = "#" + strconv.FormatInt(show.Id, 10) + " : " + user.Name

	case models.UserFriend:
		ref = "#" + strconv.FormatInt(show.Id, 10)
	}

	data := &DataSlideshow{
		Title:       show.Title,
		Caption:     show.Caption,
		DisplayName: user.Name,
		Reference:   ref,
		AfterHRef:   from,
		BeforeHRef:  from,
		Slides:      dataSlides,
		DataCommon: DataCommon{
			ParentHRef: from,
		},
	}

	// use topic title for a topic contribution
	if show.Topic != 0 {
		topic, err := s.app.SlideshowStore.Get(show.Topic)
		if err != nil {
			return nil
		}
		data.Topic = topic.Title
	}

	// template and its data
	return data
}

// displayTopic returns a template and data for a section of a topic.
// It is called for topics on the home page and for shared topics. from specifies the parent URL.
func (s *GalleryState) displayTopic(topic *models.Slideshow, shared bool, seq int, from string) (string, *DataSlideshow) {

	id := topic.Id
	fmt, max := topic.ParseFormat()

	// special selection and ordering for highlights
	if fmt == "H" {
		return s.displayHighlights(topic, from, max)
	}

	// get N'th slideshow in sequence
	show := s.app.SlideshowStore.ForTopicSeq(id, seq-1)
	if show == nil {
		return "", nil // no contributions yet, ## or could be because user removed a slideshow
	}

	// slides and user
	slides := s.app.SlideStore.ForSlideshow(show.Id, max)
	user, _ := s.app.userStore.Get(show.User.Int64)

	// replace slide data with HTML formatted fields
	var dataSlides []*DataSlide
	for _, slide := range slides {
		dataSlides = append(dataSlides, &DataSlide{
			Title:       slide.TitleBr(),
			Caption:     slide.CaptionBr(),
			DisplayName: user.Name,
			Image:       slide.Image,
			Format:      slide.Format,
		})
	}

	// next and previous user's slides
	after, before := s.topicHRefs(topic, seq, shared, from)

	// select template
	var template string
	switch fmt {

	case "T":
		fallthrough

	default:
		template = "carousel-topic.page.tmpl"
	}

	// template and its data
	return template, &DataSlideshow{
		Title:       topic.Title,
		AfterHRef:   after,
		BeforeHRef:  before,
		DisplayName: user.Name,
		Slides:      dataSlides,
		DataCommon: DataCommon{
			ParentHRef: from,
		},
	}
}

// formatShared returns the displayable code for a shared slideshow or topic.
func (s *GalleryState) formatShared(code int64) string {
	if code == 0 {
		return "-"
	} else {
		return strconv.FormatInt(code, 36)
	}
}

// href returns the path to a slideshow.
func href(shared bool, slideshow *models.Slideshow, seq int) string {

	if shared {
		// base-36 access code
		return fmt.Sprintf("/shared/%s/%d", strconv.FormatInt(slideshow.Shared, 36), seq)
	} else {
		// decimal topic number
		return fmt.Sprintf("/slideshow/%d/%d", slideshow.Id, seq)
	}
}

// topicHRefs returns links to the next and previous slideshows for a topic.
func (s *GalleryState) topicHRefs(topic *models.Slideshow, seq int, shared bool, from string) (after string, before string) {

	// next user's slides
	if seq < s.app.SlideshowStore.CountForTopic(topic.Id) {
		after = href(shared, topic, seq+1)
	} else {
		after = from
	}

	// previous user's slides
	if seq == 0 {
		before = from
	} else {
		before = href(shared, topic, seq-1)
	}
	return
}
