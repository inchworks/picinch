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

package mysql

import (
	"log"

	"github.com/jmoiron/sqlx"

	"inchworks.com/picinch/internal/models"
)

const (
	galleryInsert = `
		INSERT INTO gallery
		(version, organiser, events, n_max_slides, n_showcased)
		VALUES (:version, :organiser, :n_max_slides, :n_showcased)
	`
	galleryUpdate = `
		UPDATE gallery
		SET version=:version, organiser=:organiser, events=:events, n_max_slides=:n_max_slides, n_showcased=:n_showcased
		WHERE id=:id
	`
)

type GalleryStore struct {
	store
}

func NewGalleryStore(db *sqlx.DB, tx **sqlx.Tx, log *log.Logger) *GalleryStore {

	return &GalleryStore{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  log,
			sqlInsert: galleryInsert,
			sqlUpdate: galleryUpdate,
		},
	}
}

// Get returns the gallery with specified ID.
// Unlike most store functions, it does not log an error.
func (st *GalleryStore) Get(id int64) (*models.Gallery, error) {

	q := &models.Gallery{}

	if err := st.DBX.Get(q, "SELECT * FROM gallery WHERE id = ?", id); err != nil {
		return nil, err
	}
	return q, nil
}

// Insert or update gallery

func (st *GalleryStore) Update(q *models.Gallery) error {

	q.Id = 1

	return st.updateData(&q.Id, q)
}
