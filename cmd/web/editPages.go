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
	"strconv"
	"time"

	"inchworks.com/picinch/internal/form"
	"inchworks.com/picinch/internal/models"

	"codeberg.org/inchworks/webparts/etx"
	"codeberg.org/inchworks/webparts/multiforms"
)

// ForAssignToPages returns a form with data to assign slideshows to pages.
func (s *GalleryState) ForAssignToPages(tok string) (f *form.AssignToPagesForm) {

	// serialisation
	defer s.updatesNone()()

	// get slideshows
	slideshows := s.app.SlideshowStore.AllForPages()

	// form
	var d = make(url.Values)
	f = form.NewAssignToPages(d, tok)

	// add slideshows to form
	for i, sh := range slideshows {
		f.Add(i, sh.Id, sh.Format, sh.Title, sh.Name)
	}

	return
}

// OnAssignToPages processes updates when slideshows are assigned to pages.
func (s *GalleryState) OnAssignToPages(rsSrc []*form.AssignToPagesFormData) int {

	// #### Validate that slideshows are ones that can be assigned (format and not a topic).

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
			// check if details changed
			if rSrc.Page != rDest.Format {

				// #### sanitise format (i.e. no "$")

				rDest.Format = rSrc.Page
				s.app.SlideshowStore.Update(rDest) // #### handle error
			}
		}
		i++
	}

	if nConflicts > 0 {
		return http.StatusConflict
	} else {
		return 0
	}
}

// ForEditDiary returns data to edit events.
func (s *GalleryState) ForEditDiary(diaryId int64, tok string) (f *form.DiaryForm, diary *models.PageSlideshow) {

	// serialisation
	defer s.updatesNone()()

	// diary
	diary = s.app.PageStore.GetByShowIf(diaryId)
	if diary == nil || diary.PageFormat != models.PageDiary {
		return nil, nil // not a diary
	}

	// get events
	events := s.app.SlideStore.AllEvents(diaryId)

	// form
	var d = make(url.Values)
	f = form.NewEvents(d, 10, tok)
	f.Set("nDiary", strconv.FormatInt(diaryId, 36))
	f.Set("diaryCaption", diary.Caption)

	// add template and events to form
	f.AddTemplate(len(events))
	for i, e := range events {
		f.Add(i, e.Created, e.Revised, e.Title, e.Caption)
	}

	return
}

// OnEditDiary processes changes when diary events are modified.
// It returns 0 or an HTTP status code.
func (s *GalleryState) OnEditDiary(diaryId int64, caption string, rsSrc []*form.EventFormData) int {

	// serialisation
	defer s.updatesGallery()()

	// ## handle uploaded images here

	// update caption
	updated := false
	show := s.app.PageStore.GetByShowIf(diaryId)
	if show == nil {
		return s.rollback(http.StatusBadRequest, nil)
	}
	if caption != show.Caption {
		show.Caption = caption
		if err := s.app.SlideshowStore.Update(&show.Slideshow); err != nil {
			return s.rollback(http.StatusBadRequest, err)
		}
		updated = true
	}

	// compare modified slides against current ones, and update
	rsDest := s.app.SlideStore.AllEvents(diaryId)
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
				Slideshow: diaryId,
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
				return s.rollback(http.StatusBadRequest, nil)
			}
		}
	}

	// update cached page
	if updated {
		s.publicPages.SetDiary(show, nil)
	}

	return 0
}

// ForEditMetadata returns data to edit information page metadata.
func (s *GalleryState) ForEditMetadata(pageId int64, tok string) (f *multiforms.Form, page *models.PageSlideshow) {

	// serialisation
	defer s.updatesNone()()

	// page
	page = s.app.PageStore.GetByShowIf(pageId)
	if page == nil {
		return // not a page
	}

	// form
	var d = make(url.Values)
	f = multiforms.New(d, tok)

	f.Set("nPage", strconv.FormatInt(pageId, 36))
	f.Set("title", page.MetaTitle)
	f.Set("desc", page.Description)
	f.Set("noIndex", checked(page.NoIndex))

	return
}

// OnEditMetadata processes changes when the metadata for an information page is modified.
// It returns -1 (no update), 0, or an HTTP status code, and the path to the next page.
func (s *GalleryState) OnEditMetadata(showId int64, title string, desc string, noIndex bool) (int, string) {

	// serialisation
	defer s.updatesGallery()()

	// updates
	updated := false
	page := s.app.PageStore.GetByShowIf(showId)
	if page == nil {
		return s.rollback(http.StatusBadRequest, nil), ""
	}

	var path string
	switch page.PageFormat {
	case models.PageDiary:
		path = "/edit-diaries"
	case models.PageHome:
		path = s.app.authHome
	case models.PageInfo:
		path = "/edit-infos"
	default:
		path = s.app.authHome
	}

	if title != page.MetaTitle {
		page.MetaTitle = title
		updated = true
	}
	if desc != page.Description {
		page.Description = desc
		updated = true
	}
	if noIndex != page.NoIndex {
		page.NoIndex = noIndex
		updated = true
	}

	if !updated {
		return -1, path
	}

	// update just the page record, not the join
	if err := s.app.PageStore.UpdateFrom(page); err != nil {
		return s.rollback(http.StatusBadRequest, err), ""
	}

	// update cached page metadata
	if updated {
		s.publicPages.SetMetadata(page)
	}

	return 0, path
}

// ForEditPages returns the form data to edit all information pages.
func (s *GalleryState) ForEditPages(fmt int, tok string) (f *form.PagesForm) {

	// serialisation
	defer s.updatesNone()()

	// get pages
	pages := s.app.PageStore.ForFormat(fmt)

	// form
	var d = make(url.Values)
	f = form.NewPages(d, tok)

	// add template and pages to form
	f.AddTemplate()
	for i, pg := range pages {
		f.Add(i, pg.Name, pg.Title, pg.Id)
	}

	return
}

// OnEditPages processes updates when page definitions are modified.
// It returns -1 if there are no updates, 0 on success, or an HTTP status code.
// It also returns an extended transaction ID if there are updates.
func (s *GalleryState) OnEditPages(fmt int, rsSrc []*form.PageFormData) (int, etx.TxId) {

	updated := false

	// serialisation
	defer s.updatesGallery()()

	// start extended transaction
	tx := s.app.tm.Begin()

	now := time.Now()

	// compare modified pages against current ones, and update
	rsDest := s.app.PageStore.ForFormat(fmt)

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
			updated = true
			iDest++

		} else if iDest == nDest {

			// no more destination pages - add new one
			r := models.PageSlideshow{
				Name:       rsSrc[iSrc].Name,
				PageFormat: fmt,
				Slideshow: models.Slideshow{
					Access:  models.SlideshowPublic,
					Visible: models.SlideshowPublic,
					User:    sql.NullInt64{Int64: s.app.userStore.Info.Id, Valid: true},
					Created: now,
					Revised: now,
					Title:   rsSrc[iSrc].Title,
				},
			}
			s.app.PageStore.UpdateWith(&r)
			updated = true
			iSrc++

		} else {
			ix := rsSrc[iSrc].ChildIndex
			if ix > iDest {
				// source page removed - delete from destination
				if err := s.removeSlideshow(tx, &rsDest[iDest].Slideshow, true); err != nil {
					return s.rollback(http.StatusBadRequest, err), 0
				}
				updated = true
				iDest++

			} else if ix == iDest {
				// check if details changed
				rSrc := rsSrc[iSrc]
				rDest := rsDest[iDest]

				if rSrc.Name != rDest.Name || rSrc.Title != rDest.Title {
					rDest.Name = rSrc.Name
					rDest.Title = rSrc.Title
					rDest.Revised = now
					s.app.PageStore.UpdateWith(rDest)
					updated = true
				}
				iSrc++
				iDest++

			} else {
				// out of sequence slideshow index
				return s.rollback(http.StatusBadRequest, nil), 0
			}
		}
	}

	if !updated {
		// no updates
		return s.rollback(-1, nil), 0
	}

	return 0, tx
}
