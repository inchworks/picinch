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

	app := s.app
	id := app.publicPages.Pages[name]
	if id == 0 {
		return
	}

	// diary data
	ssEvents := s.app.publicPages.Diaries[id]
	return &DataDiary{
		Title:   ssEvents.Title,
		Caption: ssEvents.Caption,
		Events:  s.dataEvents(false, 100),
	}
}

// DisplayInfo returns the data for an information page.
func (s *GalleryState) DisplayInfo(name string) (template string, data TemplateData) {

	defer s.updatesNone()()

	name, _ = url.PathUnescape(name)

	// page defined by slideshow?
	app := s.app
	id := app.publicPages.Pages[name]
	if id != 0 {

		pg := app.SlideshowStore.GetIf(id)
		if pg != nil {

			template = "info.page.tmpl"
			data = &DataInfo{
				Title:   pg.Title,
				Caption: pg.Caption,
				Divs:    s.dataDivs(pg),
			}
			return
		}
	}

	data = &DataCommon{}

	// menu page defined by template?
	page := s.app.publicPages.Files[name]
	if page != "" {
		return // assumed to be in template cache
	}

	// non-menu information page
	page = "info-" + name + ".page.tmpl"

	if _, ok := app.templateCache[page]; ok {
		template = page
	}
	return
}

// dataDivs returns sections for an information page.
func (s *GalleryState) dataDivs(page *models.Slideshow) []*DataInfoDiv {

	// get sections
	divs := s.app.SlideStore.ForSlideshowOrdered(page.Id, false, 10) // ## configure max

	// replace slide data with HTML formatted fields
	var dataDivs []*DataInfoDiv
	for _, div := range divs {
		dataDivs = append(dataDivs, &DataInfoDiv{
			Title: models.Nl2br(div.Title),
			Div:   models.Nl2br(div.Caption),
		})
	}

	return dataDivs
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
	for _, d := range s.app.publicPages.Diaries {
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
