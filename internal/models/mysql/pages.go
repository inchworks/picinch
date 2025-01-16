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

package mysql

import (
	"log"

	"github.com/jmoiron/sqlx"

	"inchworks.com/picinch/internal/models"
)

const (
	pageDelete = `DELETE FROM page WHERE id = ?`

	pageInsert = `
		INSERT INTO page (slideshow, format, menu, description, noindex, title)
		VALUES (:slideshow, :format, :menu, :description, :noindex, :title)`

	pageUpdate = `
		UPDATE page
		SET slideshow=:slideshow, format=:format, menu=:menu, description=:description, noindex=:noindex, title=:title
		WHERE id = :id
	`
)

const (
	pageSelect = `SELECT * FROM page`

	// note that ID is included for stable ordering of selections for editing
	pageShowSelect = `
		SELECT page.id AS pageid, page.format AS pageformat, page.menu, page.description, page.noindex, page.title AS metatitle, slideshow.* FROM page
		JOIN slideshow ON slideshow.id = page.slideshow
	`

	pageOrderMenu  = ` ORDER BY page.menu, page.id`
	pageOrderTitle = ` ORDER BY page.title, page.id`

	pageWhereId = pageSelect + ` WHERE page.id = ?`
	pageWhereShowId = pageShowSelect + ` WHERE slideshow.id = ?`

	pagesWhereFormat  = pageShowSelect + ` WHERE slideshow.gallery = ? AND slideshow.visible >=  0 AND page.format = ?` + pageOrderTitle
	pagesWhereVisible = pageShowSelect + ` WHERE slideshow.gallery = ? AND slideshow.visible = ?`
)

type PageStore struct {
	GalleryId      int64
	SlideshowStore *SlideshowStore
	store
}

func NewPageStore(db *sqlx.DB, tx **sqlx.Tx, log *log.Logger) *PageStore {

	return &PageStore{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  log,
			sqlDelete: pageDelete,
			sqlInsert: pageInsert,
			sqlUpdate: pageUpdate,
		},
	}
}

// AllVisible returns a list of pages with specified visibility.
func (st *PageStore) AllVisible(visible int) []*models.PageSlideshow {
	var pages []*models.PageSlideshow

	if err := st.DBX.Select(&pages, pagesWhereVisible, st.GalleryId, visible); err != nil {
		st.logError(err)
		return nil
	}
	return pages
}

// ForFormat returns a list of pages of the specified format.
func (st *PageStore) ForFormat(fmt int) []*models.PageSlideshow {
	var pages []*models.PageSlideshow

	if err := st.DBX.Select(&pages, pagesWhereFormat, st.GalleryId, fmt); err != nil {
		st.logError(err)
		return nil
	}
	return pages
}

// GetIf returns the page if it exists.
func (st *PageStore) GetIf(id int64) *models.Page {

	var r models.Page

	if err := st.DBX.Get(&r, pageWhereId, id); err != nil {
		if st.convertError(err) != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}

	return &r
}

// GetByShowIf returns the page by slideshow ID.
func (st *PageStore) GetByShowIf(showId int64) *models.PageSlideshow {

	var r models.PageSlideshow

	if err := st.DBX.Get(&r, pageWhereShowId, showId); err != nil {
		if st.convertError(err) != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}

	return &r
}

// Update inserts or updates a page. The slideshow ID must be set.
func (st *PageStore) Update(p *models.Page) error {

	return st.updateData(&p.Id, p)
}

// UpdateFrom inserts or updates a page from the join to a slideshow.
func (st *PageStore) UpdateFrom(pg *models.PageSlideshow) error {

	// insert/update the page joined to the slideshow
	p := models.Page{
		Id:          pg.PageId,
		Slideshow:   pg.Slideshow.Id,
		Format:      pg.PageFormat,
		Menu:        pg.Menu,
		Description: pg.Description,
		NoIndex:     pg.NoIndex,
		Title:       pg.MetaTitle,
	}

	return st.updateData(&pg.PageId, p)
}

// UpdateWith inserts or updates a page with its slideshow.
func (st *PageStore) UpdateWith(pg *models.PageSlideshow) error {

	// insert/update the slideshow for the page
	if err := st.SlideshowStore.Update(&pg.Slideshow); err != nil {
		return err
	}

	// insert/update the page joined to the slideshow
	return st.UpdateFrom(pg)
}
