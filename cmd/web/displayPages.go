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
		Events:  s.dataEvents(d.Id, 100),
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
			Meta: DataMeta{
				Title:       pg.MetaTitle,
				Description: pg.Description,
				NoIndex:     pg.NoIndex,
			},
			Title:    pg.Title,
			Caption:  pg.Caption,
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

	return &DataPages{
		Diaries: s.dataPages(models.PageDiary),
		Home:    s.dataPages(models.PageHome),
		Pages:   s.dataPages(models.PageInfo),
	}
}

// returns template data for pages of the specified type.
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
			Menu:  pg.Menu,
		}

		dPages = append(dPages, &d)
	}
	return dPages
}

// dataEvents returns diary events. If no diary ID is specified, next events from all diaries are returned.
func (s *GalleryState) dataEvents(id int64, max int) []*DataEvent {

	var from time.Time
	var evs []*models.Slide

	if id == 0 {
		if max > 0 {
			// start of today
			y, m, d := time.Now().Date()
			from = time.Date(y, m, d, 0, 0, 0, 0, time.Local)

			// get events following start time
			evs = s.app.SlideStore.NextEvents(models.SlideshowPublic, from, max)
		}

	} else {
		// all events for diary
		evs = s.app.SlideStore.AllEvents(id)
	}

	// replace slide data with HTML formatted fields
	var dataEvs []*DataEvent
	for _, ev := range evs {
		dataEvs = append(dataEvs, &DataEvent{
			Start:   ev.Revised.Format(s.app.cfg.DateFormat),
			Title:   models.Nl2br(ev.Title),
			Details: models.Nl2br(ev.Caption),
		})
	}

	return dataEvs
}
