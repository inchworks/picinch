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

	id := s.publicPages.Pages[name]
	if id == 0 {
		return
	}

	// diary data
	ssEvents := s.publicPages.Diaries[id]
	return &DataDiary{
		Title:   ssEvents.Title,
		Caption: models.Nl2br(ssEvents.Caption),
		Events:  s.dataEvents(false, 100),
	}
}

// DisplayInfo returns the data for an information page.
func (s *GalleryState) DisplayInfo(name string) (template string, data TemplateData) {

	defer s.updatesNone()()

	name, _ = url.PathUnescape(name)

	// page cached from slideshow?
	pg := s.publicPages.Infos[name]
	if pg != nil {

		template = "info.page.tmpl"
		data = &DataInfo{
			Title:   pg.Title,
			Caption: pg.Caption,
			Sections: pg.Sections,
		}
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

	// get pages
	pages := s.app.PageStore.ForFormat(models.PageInfo)

	var dPages []*DataPage
	for _, pg := range pages {

		// add to template data
		d := DataPage{
			NPage: pg.Id, // slideshow ID to be edited, not page ID
			Title: pg.Title,
			Menu:  pg.Menu,
		}

		dPages = append(dPages, &d)
	}

	return &DataPages{
		Pages: dPages,
	}
}

// dataEvents returns diary events.
func (s *GalleryState) dataEvents(next bool, max int) []*DataEvent {

	var from time.Time

	if next {
		// start of today
		y, m, d := time.Now().Date()
		from = time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	}

	// get events following start time
	// ## defined order for multiple diaries
	var evs []*models.Slide
	for _, d := range s.publicPages.Diaries {
		evs = s.app.SlideStore.NextEvents(d.Id, from, max)
	}

	// replace slide data with HTML formatted fields
	var dataEvs []*DataEvent
	for _, ev := range evs {
		dataEvs = append(dataEvs, &DataEvent{
			Start:   ev.Revised.Format("2 January"),
			Title:   models.Nl2br(ev.Title),
			Details: models.Nl2br(ev.Caption),
		})
	}

	return dataEvs
}
