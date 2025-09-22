// Copyright © Rob Burke inchworks.com, 2024.

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

// Processing related to general web pages.
//
// These functions should not modify application state.

import (
	"net/url"
	"time"

	"inchworks.com/picinch/internal/models"
)

// DisplayDiary returns the data for a diary of events.
func (s *GalleryState) DisplayDiary(name string) (data *DataDiary) {

	defer s.updatesNone()()

	name, _ = url.PathUnescape(name)

	d := s.publicPages.Diaries[name]
	if d == nil {
		return
	}

	// diary data
	return &DataDiary{
		Meta: DataMeta{
			Title:       d.MetaTitle,
			Description: d.Description,
			NoIndex:     d.NoIndex,
		},
		Title:   d.Title,
		Caption: d.Caption,
		Events:  s.dataEvents(d.Id),
	}
}

// DisplayHome returns the home page with slideshows
func (s *GalleryState) DisplayHome(known bool) *DataInfo {

	defer s.updatesNone()()

	a := s.app

	// sections from system slideshow
	pg := s.publicPages.Infos["/"]
	if pg == nil {
		// #### what went wrong - can it be deleted?
	}

	// default title
	// ## cleaner if cached
	title := pg.MetaTitle
	if title == "" {
		title = s.gallery.Organiser
	}

	d := &DataInfo{
		Meta: DataMeta{
			Title:       title,
			Description: pg.Description,
			NoIndex:     pg.NoIndex,
		},
		Title:    s.gallery.Organiser,
		Sections: make([]*DataSection, len(pg.Sections)),
	}

	// add sections
	for i, sec := range pg.Sections {
		ds := &DataSection{
			Section: *sec,
		}
		d.Sections[i] = ds

		// add data for special sections
		switch ds.Layout {
		case models.SlideEvents:
			// next events
			ds.Events = s.dataEventsNext(a.cfg.MaxNextEvents)
			ds.Layout = models.SlideBelow

		case models.SlideHighlights:
			// highlight slides
			ds.Highlights = s.dataHighlights(a.cfg.MaxHighlightsTotal)
			ds.Layout = models.SlideBelow

		case models.SlideSlideshows:
			ds.Slideshows = s.dataPublished("", known)
			ds.Layout = models.SlideBelow

		case models.SlideSubPages:
			// sub-pages
			ds.SubPages = pg.SubPages
			ds.Layout = models.SlideSubPages

		case models.SlidePageShows:
			ds.Slideshows = s.dataPublished("Home", known)
			ds.Layout = models.SlideLeft
		}
	}
	return d
}

// DisplayInfo returns the data for an information page.
func (s *GalleryState) DisplayInfo(name string) (template string, data TemplateData) {

	defer s.updatesNone()()

	name, _ = url.PathUnescape(name)

	// page cached from slideshow?
	pg := s.publicPages.Infos[name]
	if pg != nil {

		template = "info.page.tmpl"
		d := &DataInfo{
			Meta: DataMeta{
				Title:       pg.MetaTitle,
				Description: pg.Description,
				NoIndex:     pg.NoIndex,
			},
			Title:    pg.Title,
			Sections: make([]*DataSection, len(pg.Sections)),
		}

		// add sections
		for i, sec := range pg.Sections {
			ds := &DataSection{
				Section: *sec,
			}
			d.Sections[i] = ds

			// add data for special sections
			a := s.app
			switch ds.Layout {
			case models.SlideEvents:
				// next events
				ds.Events = s.dataEventsNext(a.cfg.MaxNextEvents)
				ds.Layout = models.SlideBelow

			case models.SlideHighlights:
				// highlight slides
				ds.Highlights = s.dataHighlights(a.cfg.MaxHighlightsTotal)
				ds.Layout = models.SlideBelow

			case models.SlideSlideshows:
				// No option for members, unlike home page, because we don't have members versions of info pages.
				ds.Slideshows = s.dataPublished("", false)
				ds.Layout = models.SlideBelow

			case models.SlideSubPages:
				// sub-pages
				ds.SubPages = pg.SubPages
				ds.Layout = models.SlideBelow

			case models.SlidePageShows:
				ds.Slideshows = s.dataPublished(name, false)
				ds.Layout = models.SlideLeft
			}
		}

		data = d
		return
	}

	data = &DataCommon{}

	// menu page defined by template?
	page := s.publicPages.Files[name]
	if page != "" {
		return // assumed to be in template cache
	}

	// non-menu information page
	page = "info-" + name + ".page.tmpl"

	if _, ok := s.app.templateCache[page]; ok {
		template = page
	}
	return
}

// DisplayPages returns the data for a list of information pages.
func (s *GalleryState) DisplayPages() (data *DataPages) {

	defer s.updatesNone()()

	return &DataPages{
		Diaries: s.dataPages(models.PageDiary),
		Home:    s.dataPages(models.PageHome),
		Pages:   s.dataPages(models.PageInfo),
	}
}

// dataPages returns template data for pages of the specified type.
func (s *GalleryState) dataPages(fmt int) []*DataPage {

	// get pages
	pages := s.app.PageStore.ForFormat(fmt)

	var dPages []*DataPage
	for _, pg := range pages {

		// heading
		// ## untidy
		var title string
		if fmt == models.PageHome {
			title = s.gallery.Organiser
		} else {
			title = pg.Title
		}

		// add to template data
		d := DataPage{
			NPage: pg.Id, // slideshow ID to be edited, not page ID
			Title: title,
			Name:  pg.Name,
		}

		dPages = append(dPages, &d)
	}
	return dPages
}

// dataEvents returns events for a diary.
func (s *GalleryState) dataEvents(id int64) []*DataEvent {

	var evs []*models.Slide

	// all events for diary
	evs = s.app.SlideStore.AllEvents(id)

	// replace slide data with HTML formatted fields
	var dataEvs []*DataEvent
	for _, ev := range evs {
		dataEvs = append(dataEvs, &DataEvent{
			Start:   ev.Revised.Local().Format(s.app.cfg.DateFormat),
			Title:   models.Nl2br(ev.Title),
			Details: models.Nl2br(ev.Caption),
		})
	}

	return dataEvs
}

// dataEventsNext returns the next events from all diaries.
func (s *GalleryState) dataEventsNext(max int) []*DataEvent {

	var from time.Time
	var evs []*models.SlideRank

	if max > 0 {
		// start of today
		y, m, d := time.Now().Date()
		from = time.Date(y, m, d, 0, 0, 0, 0, time.Local)

		// get events following start time
		evs = s.app.SlideStore.NextEvents(models.SlideshowPublic, from, max)
	}

	// replace slide data with HTML formatted fields
	var dataEvs []*DataEvent
	for _, ev := range evs {
		dataEvs = append(dataEvs, &DataEvent{
			Start: ev.Revised.Local().Format(s.app.cfg.DateFormat),
			Title: models.Nl2br(ev.Title),
			Diary: s.publicPages.Paths[ev.Slideshow],
		})
	}

	return dataEvs
}

// dataPublished returns published slideshows
func (s *GalleryState) dataPublished(page string, knownUser bool) []*DataPublished {

	a := s.app
	if knownUser {
		return s.dataShowsPublished(
			a.SlideshowStore.RecentPublished(models.SlideshowClub, page, s.usersHidden, a.cfg.MaxSlideshowsClub),
			a.cfg.MaxSlideshowsClub, a.cfg.MaxSlideshowsTotal)
	} else {
		return s.dataShowsPublished(
			a.SlideshowStore.RecentPublished(models.SlideshowPublic, page, s.usersHidden, a.cfg.MaxSlideshowsPublic),
			a.cfg.MaxSlideshowsPublic, a.cfg.MaxSlideshowsTotal)
	}
}
