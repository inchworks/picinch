// Copyright © Rob Burke inchworks.com, 2021.

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

// SQL operations on tagref table.

import (
	"log"

	"github.com/jmoiron/sqlx"

	"inchworks.com/picinch/pkg/models"
)

const (
	tagrefDelete = `DELETE FROM tagref WHERE id = ?`

	tagrefInsert = `
		INSERT INTO tagref (slideshow, tag, added, user, detail) VALUES (:slideshow, :tag, :added, :user, :detail)`

	tagrefUpdate = `
		UPDATE tagref
		SET added=:added, detail=:detail
		WHERE id=:id
	`
)

const (
	tagrefSelect = `SELECT * FROM tagref`

	tagrefWhereId = tagrefSelect + ` WHERE id = ?`

	tagrefDeleteIf = `
		DELETE tagref FROM tagref
		INNER JOIN tag ON tag.id = tagref.tag
		WHERE tag.parent = ? AND tag.name = ? AND tagref.slideshow = ?
	`
)

type TagRefStore struct {
	store
}

func NewTagRefStore(db *sqlx.DB, tx **sqlx.Tx, log *log.Logger) *TagRefStore {

	return &TagRefStore{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  log,
			sqlDelete: tagrefDelete,
			sqlInsert: tagrefInsert,
			sqlUpdate: tagrefUpdate,
		},
	}
}

// Remove deletes a tag reference.
func (st *TagRefStore) Remove(parent int64, name string, slideshow int64) error {

	if _, err := st.DBX.Exec(tagrefDeleteIf, parent, name, slideshow); err != nil {
		return st.logError(err)
	}

	return nil
}

// Update inserts or updates a tag reference.
func (st *TagRefStore) Update(r *models.TagRef) error {
	return st.updateData(&r.Id, r)
}
