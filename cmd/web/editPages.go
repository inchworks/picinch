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

// Processing for event and page editing.
//
// These functions may modify application state.

import (
	"database/sql"
	"net/http"
	"net/url"
	"time"

	"inchworks.com/picinch/internal/form"
	"inchworks.com/picinch/internal/models"

	"github.com/inchworks/webparts/v2/etx"
)

// ForEditDiary returns data to edit events.
func (s *GalleryState) ForEditDiary(tok string) (f *form.DiaryForm) {

	// serialisation
	defer s.updatesNone()()

	// get events
	events := s.app.SlideStore.AllEvents(s.app.publicPages.Diaries[1].Id)

	// form
	var d = make(url.Values)
	f = form.NewEvents(d, 10, tok)

	// add template and events to form
	f.AddTemplate(len(events))
	for i, e := range events {
		f.Add(i, e.Created, e.Revised, e.Title, e.Caption)
	}

	return
}

// OnEditDiary processes changes when diary events are modified.
// It returns 0 or an HTTP status code.
func (s *GalleryState) OnEditDiary(rsSrc []*form.EventFormData) (int, etx.TxId) {

	// serialisation
	defer s.updatesGallery()()

	// start extended transaction
	tx := s.app.tm.Begin()

	// compare modified slideshows against current ones, and update
	rsDest := s.app.SlideStore.AllEvents(s.app.publicPages.Diaries[1].Id)
	nSrc := len(rsSrc)
	nDest := len(rsDest)

	// skip template
	iSrc := 1
	var iDest int

	for iSrc < nSrc || iDest < nDest {

		if iSrc == nSrc {
			// no more source events - delete from destination
			s.app.SlideStore.DeleteId(rsDest[iDest].Id)
			iDest++

		} else if iDest == nDest {
			// no more destination events - add new one
			qd := models.Slide{
				Slideshow: s.app.publicPages.Diaries[1].Id,
				Format:    s.app.eventFormat(rsSrc[iSrc]),
				Created:   rsSrc[iSrc].Publish,
				Revised:   rsSrc[iSrc].Start,
				Title:     s.sanitize(rsSrc[iSrc].Title, ""),
				Caption:   s.sanitize(rsSrc[iSrc].Caption, ""),
			}

			s.app.SlideStore.Update(&qd)
			iSrc++

		} else {
			ix := rsSrc[iSrc].ChildIndex
			rDest := rsDest[iDest]

			if ix > iDest {
				// source event removed - delete from destination
				s.app.SlideStore.DeleteId(rDest.Id)
				iDest++

			} else if ix == iDest {
				// check if details changed
				if rsSrc[iSrc].Publish != rDest.Created ||
					rsSrc[iSrc].Start != rDest.Revised ||
					rsSrc[iSrc].Title != rDest.Title ||
					rsSrc[iSrc].Caption != rDest.Caption {

					rDest.Format = s.app.eventFormat(rsSrc[iSrc])
					rDest.Created = rsSrc[iSrc].Publish
					rDest.Revised = rsSrc[iSrc].Start
					rDest.Title = s.sanitize(rsSrc[iSrc].Title, rDest.Title)
					rDest.Caption = s.sanitize(rsSrc[iSrc].Caption, rDest.Caption)

					s.app.SlideStore.Update(rDest)
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

// ForEditPages returns the form data to edit all information pages.
func (s *GalleryState) ForEditPages(tok string) (f *form.PagesForm) {

	// serialisation
	defer s.updatesNone()()

	// get pages
	pages := s.app.PageStore.ForFormat(models.PageInfo)

	// form
	var d = make(url.Values)
	f = form.NewPages(d, tok)

	// add template and pages to form
	f.AddTemplate()
	for i, pg := range pages {
		f.Add(i, pg.Menu, pg.Title)
	}

	return
}

// OnEditPages processes updates when slideshows are modified.
// It returns an extended transaction ID if there are no client errors.
func (s *GalleryState) OnEditPages(rsSrc []*form.PageFormData) (int, etx.TxId) {

	// #### can I extract this pattern somehow?

	// serialisation
	defer s.updatesGallery()()

	// start extended transaction
	tx := s.app.tm.Begin()

	now := time.Now()

	// compare modified slideshows against current ones, and update
	rsDest := s.app.PageStore.ForFormat(models.PageInfo)

	nSrc := len(rsSrc)
	nDest := len(rsDest)

	// skip template
	iSrc := 1
	var iDest int

	for iSrc < nSrc || iDest < nDest {

		if iSrc == nSrc {
			// no more source pages - delete from destination (deletes linked page)
			if err := s.removeSlideshow(tx, &rsDest[iDest].Slideshow, true); err != nil {
				return s.rollback(http.StatusBadRequest, err), 0
			}
			iDest++

		} else if iDest == nDest {

			// no more destination pages - add new one
			r := models.PageSlideshow{
				Menu: rsSrc[iSrc].Menu,
				PageFormat: models.PageInfo,
				PageTitle: rsSrc[iSrc].Title, // default
				Slideshow: models.Slideshow{
					Access:       models.SlideshowPublic,
					Visible:      models.SlideshowPublic,
					User:         sql.NullInt64{Int64: s.app.userStore.Info.Id, Valid: true},
					Created:      now,
					Revised:      now,
					Title:        rsSrc[iSrc].Title,
				},
			}
			s.app.PageStore.UpdateWith(&r)
			iSrc++

		} else {
			ix := rsSrc[iSrc].ChildIndex
			if ix > iDest {
				// source page removed - delete from destination
				if err := s.removeSlideshow(tx, &rsDest[iDest].Slideshow, true); err != nil {
					return s.rollback(http.StatusBadRequest, err), 0
				}
				iDest++

			} else if ix == iDest {
				// check if details changed
				rSrc := rsSrc[iSrc]
				rDest := rsDest[iDest]

				if rSrc.Menu != rDest.Menu || rSrc.Title != rDest.Title {
					rDest.Menu = rSrc.Menu
					rDest.Title = rSrc.Title
					rDest.Revised = now
					s.app.PageStore.UpdateWith(rDest)
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