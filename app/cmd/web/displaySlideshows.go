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
	"path/filepath"

	"inchworks.com/picinch/pkg/models"
)

// Copyright © Rob Burke inchworks.com, 2020.

// List of published slideshows for a user

func (s *GalleryState) DisplayContributor(userId int64) (string, *DataHome) {

	defer s.updatesNone()()

	// user
	user, err := s.app.UserStore.Get(userId)
	if err != nil {
		s.app.log(err)
		return "", nil
	}

	// highlights
	var dHighlights []*DataSlide
	show := s.app.SlideshowStore.ForTopicUser(s.app.TopicStore.HighlightsId, user.Id)
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
	return "contributor.page.tmpl", &DataHome{
		DisplayName:    user.Name,
		Highlights:     dHighlights,
		SlideshowsClub: dShows,
	}
}

// Users that have slideshows

func (s *GalleryState) DisplayContributors() (string, *DataUsers) {

	defer s.updatesNone()()

	users := s.app.UserStore.Contributors()
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

func (s *GalleryState) DisplayHome(member bool) (string, *DataHome) {

	defer s.updatesNone()()

	a := s.app

	// no of highlight slides
	dHighlights := s.dataHighlights(a.cfg.MaxHighlightsTotal)

	dTopicsPublic := s.dataTopicsPublished(a.TopicStore.Published(models.SlideshowPublic))

	// slideshows shown as public
	shown := make(map[int64]bool, 16)

	dShowsPublic := s.dataShowsPublished(
		a.SlideshowStore.RecentPublished(models.SlideshowPublic, a.cfg.MaxSlideshowsPublic), a.cfg.MaxSlideshowsPublic, shown)

	var dTopicsClub []*DataPublished
	var dShowsClub []*DataPublished
	if member {
		dTopicsClub = s.dataTopicsPublished(a.TopicStore.Published(models.SlideshowClub))

		// we include public slideshows, so that extra public slideshows can be seen by club
		dShowsClub = s.dataShowsPublished(
			a.SlideshowStore.RecentPublished(models.SlideshowClub, a.cfg.MaxSlideshowsPublic+a.cfg.MaxSlideshowsClub), a.cfg.MaxSlideshowsClub, shown)
	}

	// template and its data
	return "home.page.tmpl", &DataHome{
		Highlights:       dHighlights,
		TopicsPublic:     dTopicsPublic,
		SlideshowsPublic: dShowsPublic,
		TopicsClub:       dTopicsClub,
		SlideshowsClub:   dShowsClub,
	}
}

// Slideshow
//
// Returns slides

func (s *GalleryState) DisplaySlideshow(id int64, from string) (string, *DataSlideshow) {

	defer s.updatesNone()()

	// get slideshow ..
	show, err := s.app.SlideshowStore.Get(id)
	if err != nil {
		return "", nil
	}

	// .. and slides
	return s.displaySlides(show, from, 100)
}

// Topic with all contributions

func (s *GalleryState) DisplayTopic(id int64, seq int, from string) (string, *DataSlideshow) {

	defer s.updatesNone()()

	topic, _ := s.app.TopicStore.Get(id)
	fmt, max := topic.ParseFormat()

	// special selection and ordering for highlights
	if fmt == "H" {
		return s.displayHighlights(topic, from, max)
	}

	// get N'th slideshow in sequence
	show := s.app.SlideshowStore.ForTopicSeq(id, seq)
	if show == nil {
		return "", nil // no contributions yet, ## or could be because user removed a slideshow
	}

	// slides and user
	slides := s.app.SlideStore.ForSlideshow(show.Id, max)
	user, _ := s.app.UserStore.Get(show.User)

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

	// show user's title, if different from topic
	var title string
	if show.Title != topic.Title {
		title = show.Title
	}

	// next user's slides
	var after string
	if seq < s.app.SlideshowStore.CountForTopic(id)-1 {
		after = href(id, seq+1)
	} else {
		after = from
	}

	// previous user's slides
	var before string
	if seq == 0 {
		before = from
	} else {
		before = href(id, seq-1)
	}

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
		Topic:       topic.Title,
		AfterHRef:   after,
		BeforeHRef:  before,
		DisplayName: user.Name,
		Title:       title,
		Slides:      dataSlides,
		DataCommon: DataCommon{
			ParentHRef: from,
		},
	}
}

// Slideshows for a topic

func (s *GalleryState) DisplayTopicContributors(id int64) (string, *DataSlideshows) {

	defer s.updatesNone()()

	topic, _ := s.app.TopicStore.Get(id)

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

	return "topic-contributors.page.tmpl", &DataSlideshows{
		Title:      topic.Title,
		Slideshows: dShows,
	}
}

// Topic for a user
//
// Returns slides

func (s *GalleryState) DisplayTopicUser(topicId int64, userId int64, from string) (string, *DataSlideshow) {

	defer s.updatesNone()()

	// get slideshow
	show := s.app.SlideshowStore.ForTopicUser(topicId, userId)
	if show == nil {
		return "", nil
	}

	// .. and slides
	return s.displaySlides(show, from, 30)
}

// User's view of gallery - just their name and own slideshows at present

func (s *GalleryState) ForMyGallery(userId int64) *DataMyGallery {

	// serialisation
	defer s.updatesNone()()

	// get user
	user, _ := s.app.UserStore.Get(userId)

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
	topics := s.app.TopicStore.All()
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
	topics := s.app.TopicStore.All()
	var dataShows []*DataMySlideshow

	for _, topic := range topics {

		dataShows = append(dataShows, &DataMySlideshow{
			NShow:   topic.Id,
			Title:   topic.Title,
			Visible: topic.VisibleStr(),
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
	users := s.app.UserStore.ByName()

	return &DataUsers{
		Users: users,
	}
}

// Get highlighted image, for parent website

func (s *GalleryState) Highlighted(prefix string, nImage int) string {

	if nImage >= s.app.cfg.MaxHighlightsParent || nImage < 1 {
		return ""
	} // silly image number

	// get cached image name
	if nImage < len(s.highlights) {
		image := s.highlights[nImage-1]

		// with specified prefix as first character (main or thumbnail)
		image = prefix[:1] + image[1:]

		return filepath.Join(ImagePath, image)

	} else {
		return filepath.Join(SitePath, "images/no-photos-white.jpg")
	}
}

// Highlights for home page or embedded page

func (s *GalleryState) dataHighlights(nImages int) []*DataSlide {

	// get slides for highlights topic
	slides := s.app.SlideStore.RecentForTopic(s.app.TopicStore.HighlightsId, s.app.cfg.MaxHighlights, nImages)

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

// Public or club slideshows for home page

func (s *GalleryState) dataTopicsPublished(shows []*models.Topic) []*DataPublished {

	var data []*DataPublished

	for _, show := range shows {

		data = append(data, &DataPublished{
			Id:    show.Id,
			Title: show.Title,
			Image: show.Image,
		})
	}
	return data
}

// Public or club slideshows for home page

func (s *GalleryState) dataShowsPublished(shows []*models.Slideshow, max int, shown map[int64]bool) []*DataPublished {

	a := s.app
	public := len(shown) == 0        // empty map indicates no slideshows already shown
	count := make(map[int64]int, 16) // count slideshows per-user

	var data []*DataPublished

	for _, show := range shows {

		// check if slideshow already shown as public, or user's limit reached
		if (public || show.Visible != models.SlideshowPublic || !shown[show.Id]) &&
			count[show.User] < max {

			// contributor of slideshow
			user, err := a.UserStore.Get(show.User)
			if err != nil {
				a.log(err)
				return nil
			}

			// data for display
			data = append(data, &DataPublished{
				Id:          show.Id,
				Title:       show.Title,
				Image:       show.Image,
				NUser:       user.Id,
				DisplayName: user.Name,
			})

			// count for user
			count[show.User]++

			// add slideshow to public set
			if public {
				shown[show.Id] = true
			}
		}
	}
	return data
}

// Display highlights : latest slides

func (s *GalleryState) displayHighlights(topic *models.Topic, from string, perUser int) (string, *DataSlideshow) {

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

// User's slides for own slideshow or a topic

func (s *GalleryState) displaySlides(show *models.Slideshow, from string, max int) (string, *DataSlideshow) {

	slides := s.app.SlideStore.ForSlideshow(show.Id, max)
	user, _ := s.app.UserStore.Get(show.User)

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

	// template and its data
	return "carousel-default.page.tmpl", &DataSlideshow{
		Title:       show.Title,
		DisplayName: user.Name,
		AfterHRef:   from,
		BeforeHRef:  from,
		Slides:      dataSlides,
		DataCommon: DataCommon{
			ParentHRef: from,
		},
	}
}

// Path to previous or next show for topic

func href(id int64, seq int) string {

	return fmt.Sprintf("/topic/%d/%d", id, seq)
}
