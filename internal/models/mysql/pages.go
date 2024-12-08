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
		INSERT INTO page (slideshow, format, menu, description, title)
		VALUES (:slideshow, :format, :menu, :description, :title)`

	pageUpdate = `
		UPDATE page
		SET slideshow=:slideshow, format=:format, menu=:menu, description=:description, title=:title
		WHERE id = :id
	`
)

const (
	// note that ID is included for stable ordering of selections for editing
	pageSelect         = `SELECT * FROM page`
	pageOrderTitle     = ` ORDER BY title, id`

	pageWhereId     = pageSelect + ` WHERE id = ?`

	// all pages + slideshows with specified visibilty
	pagesWhereVisible = `
		SELECT page.id AS pageid, page.format AS pageformat, page.menu, page.description, page.title AS pagetitle, slideshow.* FROM page
		JOIN slideshow ON slideshow.id = page.slideshow
		WHERE slideshow.gallery = ? AND slideshow.visible = ?
	`
)

type PageStore struct {
	GalleryId    int64
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

// ForFormat returns a list of pages of the specified format.
func (st *PageStore) AllVisible(visible int) []*models.PageSlideshow {
	var pages []*models.PageSlideshow

	if err := st.DBX.Select(&pages, pagesWhereVisible, st.GalleryId, visible); err != nil {
		st.logError(err)
		return nil
	}
	return pages
}

// Update inserts or updates a page. The slideshow ID must be set.
func (st *PageStore) Update(p *models.Page) error {

	return st.updateData(&p.Id, p)
}
