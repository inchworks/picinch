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
	"time"

	"inchworks.com/picinch/internal/models"
)

// Copyright © Rob Burke inchworks.com, 2020.

// DisplayClasses returns data for competition classes.
func (s *GalleryState) DisplayClasses(_ bool) *dataCompetition {

	defer s.updatesNone()()

	a := s.app

	// ## restrict to published categories
	// ## could have unrestricted list for members
	dShows := s.dataShowsPublished(
		a.SlideshowStore.AllTopicsFormatted("C%"), a.cfg.MaxSlideshowsPublic, a.cfg.MaxSlideshowsTotal)

	// template and its data
	return &dataCompetition{
		Categories: dShows,
	}
}

// DisplayContributor returns a list of published slideshows for a user.
func (s *GalleryState) DisplayContributor(userId int64, member bool) *DataHome {

	defer s.updatesNone()()

	// user
	user := s.app.getUserIf(userId)
	if user == nil {
		return nil
	}

	// show all published or just public ones?
	var visible int
	if member {
		visible = models.SlideshowClub
	} else {
		visible = models.SlideshowPublic
	}

	// highlights
	var dHighlights []*DataSlide
	show := s.app.SlideshowStore.ForTopicUserVisibleIf(s.app.SlideshowStore.HighlightsId, user.Id, visible)
	if show != nil {
		dHighlights = s.dataHighlightsUser(show.Id, s.app.cfg.MaxHighlightsTotal)
	}

	// slideshows
	slideshows := s.app.SlideshowStore.ForUserPublished(user.Id, visible)
	var dShows []*DataPublished

	for _, show := range slideshows {

		var href string
		if show.Visible == models.SlideshowTopic {
			href = fmt.Sprintf("/for-topic/%d/%d", user.Id, show.Topic)
		} else {
			href = fmt.Sprintf("/for-show/%d", show.Id)
		}

		dShows = append(dShows, &DataPublished{
			Id:    show.Id,
			Ref:   href,
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

// DisplayHighlights returns data for highlights of a topic.
func (s *GalleryState) DisplayHighlights(
	id int64,
	forPath func(s *models.Slideshow) string,
) (data *DataSlideshow) {

	defer s.updatesNone()()

	// get topic and section
	topic := s.app.SlideshowStore.GetIf(id)
	if topic == nil || topic.User.Valid {
		return
	}

	// ### check format

	// handler's interpetation of the slideshow
	from := forPath(topic)
	if from == "" {
		return // no access to show
	}

	fmt, max := topic.ParseFormat(s.app.cfg.MaxSlides)
	if fmt != "H" {
		return // topic doesn't have highlights
	}

	// data for slides
	data = s.dataHighlightSlides(topic, from, max)
	return
}

// DisplayGallery returns the data for a member's or curator's view of their gallery.
func (s *GalleryState) DisplayGallery(userId int64) *DataMyGallery {

	// serialisation
	defer s.updatesNone()()

	// get user
	user := s.app.getUserIf(userId)
	if user == nil {
		return nil
	}

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

// DisplayHome returns the home page with slideshows
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

// DisplayShared returns the data for a shared slideshow.
func (s *GalleryState) DisplayShared(code int64) (data *DataSlideshow, id int64) {

	defer s.updatesNone()()

	// check if code is valid
	slideshow := s.app.SlideshowStore.GetIfShared(code)
	if slideshow == nil || !slideshow.User.Valid {
		return
	}

	// return to this page at the end, as we have nowhere else to go.
	from := href("shared-", true, slideshow, 0)

	// this is a single slideshow
	return s.dataSlides(slideshow, 0, from, "", s.app.cfg.MaxSlides), slideshow.Id
}

// DisplaySharedSlides returns the data for a section of a shared topic.
func (s *GalleryState) DisplaySharedSlides(code int64, secId int64) (data *DataSlideshow, id int64) {

	defer s.updatesNone()()

	// check if code is valid
	topic := s.app.SlideshowStore.GetIfShared(code)
	if topic == nil || topic.User.Valid {
		return
	}

	sec := s.app.SlideshowStore.GetIf(secId)
	if sec == nil || sec.Topic != topic.Id {
		return
	}

	// return to this topic at the end, as we have nowhere else to go.
	from := href("shared-", true, topic, 0)

	// contribution to topic
	fmt, max := topic.ParseFormat(s.app.cfg.MaxSlides)
	return s.dataSection(topic, sec, "shared-", true, from, fmt, max), sec.Id
}

// DisplaySharedTopic returns the data for a shared topic header.
func (s *GalleryState) DisplaySharedTopic(code int64) (data *DataSlideshow, id int64) {

	defer s.updatesNone()()

	// check if code is valid
	topic := s.app.SlideshowStore.GetIfShared(code)
	if topic == nil || topic.User.Valid {
		return
	}

	// return to this topic at the end, as we have nowhere else to go.
	from := href("shared-", true, topic, 0)

	// next is first user's slides
	after, before := s.topicHRefs(topic, time.Time{}, true, "shared-", true, from)

	// home page for topic
	// Note that the slideshow must have at least two slides,
	// otherwise Bootstrap Carousel doesn't give any events to trigger loading of the first user's slideshow.
	return &DataSlideshow{
		Title:      topic.Title,
		Caption:    s.app.galleryState.gallery.Organiser,
		AfterHRef:  after,
		BeforeHRef: before,
		Single:     "Y",
		DataCommon: DataCommon{
			ParentHRef: from,
		},
	}, topic.Id
}

// DisplaySlides returns data for section of a topic, displayed as part of the topic.
func (s *GalleryState) DisplaySlides(
	id int64, secId int64, origin string,
	forPath func(s *models.Slideshow, fmt string) string,
) (data *DataSlideshow) {

	defer s.updatesNone()()

	// get topic and section
	topic := s.app.SlideshowStore.GetIf(id)
	if topic == nil || topic.User.Valid {
		return
	}
	sec := s.app.SlideshowStore.GetIf(secId)
	if sec == nil || sec.Topic != topic.Id || !sec.User.Valid {
		return // ## something worse if user ID not valid?
	}

	// handler's interpetation of the slideshow
	fmt, max := topic.ParseFormat(s.app.cfg.MaxSlides)
	from := forPath(topic, fmt)
	if from == "" {
		return // no access to show
	}

	// data for slides
	data = s.dataSection(topic, sec, origin, false, from, fmt, max)

	// parent title overrides user's own
	data.Title = topic.Title
	return
}

// DisplaySlideshow returns data for single-user slideshow.
func (s *GalleryState) DisplaySlideshow(
	id int64, forRole int,
	forPath func(s *models.Slideshow, userId int64) string,
) (data *DataSlideshow) {

	defer s.updatesNone()()

	// get slideshow and section
	show := s.app.SlideshowStore.GetIf(id)
	if show == nil || !show.User.Valid {
		return
	}

	// handler's interpretation of the slideshow
	from := forPath(show, show.User.Int64)
	if from == "" {
		return // no access to show
	}

	// data for slides
	fmt, max := show.ParseFormat(s.app.cfg.MaxSlides)
	data = s.dataSlides(show, forRole, from, fmt, max)
	return
}

// DisplayTopic returns data for topic header.
func (s *GalleryState) DisplayTopic(
	id int64, origin string,
	forPath func(s *models.Slideshow, userId int64) string,
) (data *DataSlideshow) {

	defer s.updatesNone()()

	// get topic
	show := s.app.SlideshowStore.GetIf(id)
	if show == nil || show.User.Valid {
		return
	}

	// handler's interpetation of the slideshow
	from := forPath(show, show.Id)
	if from == "" {
		return // no access to show
	}

	// data for topic
	data = s.dataTopic(show, origin, from)
	return
}

// DisplayTopics returns the data for a curator's view of all topics (similar to user's view of gallery)
func (s *GalleryState) DisplayTopics() *DataMyGallery {

	// serialisation
	defer s.updatesNone()()

	// get topics
	topics := s.app.SlideshowStore.AllTopics()
	var dataShows []*DataMySlideshow

	for _, topic := range topics {
		fmt, _ := topic.ParseFormat(s.app.cfg.MaxSlides)

		d := DataMySlideshow{
			NShow:   topic.Id,
			Title:   topic.Title,
			Visible: topic.VisibleStr(),
			Shared:  s.formatShared(topic.Shared),
		}
		if fmt == "H" {
			d.Ref = "/rev-hilites/" + strconv.FormatInt(topic.Id, 10)
		} else {
			d.Ref = "/rev-topic/" + strconv.FormatInt(topic.Id, 10)
		}

		dataShows = append(dataShows, &d)
	}

	return &DataMyGallery{
		DisplayName: "Topics",
		Topics:      dataShows,
	}
}

// DisplayTopicContributors returns the slideshows contributed to a topic.
func (s *GalleryState) DisplayTopicContributors(id int64, forPath func(s *models.Slideshow) string) *DataSlideshows {

	defer s.updatesNone()()

	topic := s.app.SlideshowStore.GetIf(id)
	if topic == nil || topic.User.Valid {
		return nil // no topic or not a topic
	}

	// caller sets caching, now we can read the topic
	if forPath(topic) == "" {
		return nil // no access
	}

	// show latest highlights first, other topics in published order
	latest := false
	ft, _ := topic.ParseFormat(0)
	if ft == "H" {
		latest = true
	}

	// get published slideshows for topic
	slideshows := s.app.SlideshowStore.ForTopicPublished(id, latest)
	var dShows []*DataPublished

	for _, s := range slideshows {
		dShows = append(dShows, &DataPublished{
			NTopic:      id,
			Id:          s.Id,
			Ref:         fmt.Sprintf("/topic-user/%d/%d", id, s.UserId),
			Title:       s.Title,
			NUser:       s.UserId,
			Image:       s.Image,
			DisplayName: s.Name,
		})
	}

	return &DataSlideshows{
		Title:      topic.Title,
		Slideshows: dShows,
	}
}

// DisplayUserTopic returns slides for a user's contribution to a topic, shown separately.
func (s *GalleryState) DisplayUserTopic(
	userId int64, topicId int64,
	forPath func(t *models.Slideshow, fmt string, sId int64) string,
) *DataSlideshow {

	defer s.updatesNone()()

	// get topic and section
	topic := s.app.SlideshowStore.GetIf(topicId)
	if topic == nil {
		return nil
	}
	sec := s.app.SlideshowStore.ForTopicUserIf(topicId, userId)
	if sec == nil {
		return nil
	}
	fmt, max := topic.ParseFormat(s.app.cfg.MaxSlides)

	// parent path (and set appropriate caching)
	from := forPath(topic, fmt, sec.Id)
	if from == "" {
		return nil // no access
	}

	// .. and slides
	return s.dataSlides(sec, 0, from, fmt, max)
}

// DisplayUsers returns the data for a curator's view of the users.
func (s *GalleryState) DisplayUsers() *DataUsers {

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

// SlideshowTitle returns the title for a slideshow.
func (s *GalleryState) SlideshowTitle(showId int64) string {

	// serialisation
	defer s.updatesNone()()

	r := s.app.SlideshowStore.GetIf(showId)
	if r == nil {
		return ""
	}

	return r.Title
}

// dataHighlightSlides returns the latest slides for a topic.
func (s *GalleryState) dataHighlightSlides(topic *models.Slideshow, from string, perUser int) *DataSlideshow {

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
	return &DataSlideshow{
		Title:      topic.Title,
		AfterHRef:  from,
		BeforeHRef: from,
		Slides:     dataSlides,
		DataCommon: DataCommon{
			ParentHRef: from,
		},
	}
}

// dataHighlights returns data for highlight images on the home page or am embedded page.
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

// dataHighlightsUser returns highlight slides for a user.
func (s *GalleryState) dataHighlightsUser(showId int64, max int) []*DataSlide {

	// get slides for highlights topic s.highlightsId
	slides := s.app.SlideStore.ForSlideshowOrdered(showId, true, max)

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

// dataSection returns data for a section of a topic.
// It is called for topics on the home page and for shared topics. from specifies the parent URL.
func (s *GalleryState) dataSection(topic *models.Slideshow, section *models.Slideshow, origin string, code bool, from string, fmt string, max int) *DataSlideshow {

	if section.Topic != topic.Id {
		// doesn't belong to topic?
		// #### if is there one that does - redirect
		return nil // slideshow removed
	}

	// slides and user
	slides := s.app.SlideStore.ForSlideshowOrdered(section.Id, fmt == "H", max)
	user := s.app.getUserIf(section.User.Int64)
	if user == nil {
		return nil
	}

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
	after, before := s.topicHRefs(topic, section.Revised, false, origin, code, from)

	// template and its data
	return &DataSlideshow{
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

// dataShowsPublished returns public or club slideshows and topics for home page.
// It is also called for competition classes.
func (s *GalleryState) dataShowsPublished(shows []*models.Slideshow, maxUser int, maxTotal int) []*DataPublished {

	count := make(map[int64]int, 16) // count slideshows per-user

	var data []*DataPublished
	var total int

	for _, show := range shows {

		fmt, _ := show.ParseFormat(s.app.cfg.MaxSlides)
	
		if show.User.Valid {
			// slideshow - check if user's limit reached
			userId := show.User.Int64
			if count[userId] < maxUser {

				// contributor of slideshow
				user := s.app.getUserIf(userId)
				if user == nil {
					return nil
				}

				// data for display
				data = append(data, &DataPublished{
					Id:          show.Id,
					Ref:         "/show/" + strconv.FormatInt(show.Id, 10),
					Title:       show.Title,
					Image:       show.Image,
					NUser:       userId,
					DisplayName: user.Name,
				})

				// count for user
				count[userId]++
			}
		} else if fmt == "H" {
			// highlights - data for display
			data = append(data, &DataPublished{
				Id:      show.Id,
				Ref:     "/hilites/" + strconv.FormatInt(show.Id, 10),
				Title:   show.Title,
				Caption: models.Nl2br(show.Caption),
				Image:   show.Image,
			})
		} else {
			// topic - data for display
			data = append(data, &DataPublished{
				Id:      show.Id,
				Ref:     "/topic/" + strconv.FormatInt(show.Id, 10),
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

// dataSlides returns data for slides, not as part of topic.
func (s *GalleryState) dataSlides(show *models.Slideshow, forRole int, from string, fmt string, max int) *DataSlideshow {

	slides := s.app.SlideStore.ForSlideshowOrdered(show.Id, fmt == "H", max)
	user := s.app.getUserIf(show.User.Int64)
	if user == nil {
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

	// template and its data
	return data
}

// dataTopic returns the data for a topic.
func (s *GalleryState) dataTopic(topic *models.Slideshow, origin string, from string) *DataSlideshow {

	// special selection for highlights
	var after, before string
	fm, _ := topic.ParseFormat(s.app.cfg.MaxSlides)
	if fm == "H" {
		// next is all the highlights
		before = from
		after = fmt.Sprintf("/%sslides/%d", origin, topic.Id)
	} else {
		// next is first user's slides
		after, before = s.topicHRefs(topic, time.Time{}, true, origin, false, from)
	}

	// home page for topic
	// Note that the slideshow must have at least two slides,
	// otherwise Bootstrap Carousel doesn't give any events to trigger loading of the first user's slideshow.
	return &DataSlideshow{
		Title:      topic.Title,
		Caption:    topic.Caption,
		AfterHRef:  after,
		BeforeHRef: before,
		Single:     "Y",
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

// href returns the path to a slideshow, topic or slides.
func href(origin string, code bool, slideshow *models.Slideshow, secId int64) string {

	var id string
	if code {
		// base-36 access code
		id = strconv.FormatInt(slideshow.Shared, 36)
	} else {
		// decimal topic number
		id = strconv.FormatInt(slideshow.Id, 10)
	}

	if secId != 0 {
		return fmt.Sprintf("/%sslides/%s/%d", origin, id, secId)
	} else if !slideshow.User.Valid {
		return fmt.Sprintf("/%stopic/%s", origin, id)
	} else {
		return fmt.Sprintf("/%sshow/%s", origin, id)
	}
}

// topicHRefs returns links to the next and previous slideshows for a topic.
func (s *GalleryState) topicHRefs(topic *models.Slideshow, current time.Time, first bool, origin string, code bool, from string) (after string, before string) {

	// next user's slides
	id := s.app.SlideshowStore.ForTopicSeq(topic.Id, current, true)
	if id != 0 {
		after = href(origin, code, topic, id)
	} else {
		after = from // parent
	}

	// previous user's slides
	if first {
		before = from // parent
	} else {
		id = s.app.SlideshowStore.ForTopicSeq(topic.Id, current, false)
		if id != 0 {
			before = href(origin, code, topic, id)
		} else {
			before = href(origin, code, topic, 0) // header
		}
	}
	return
}
