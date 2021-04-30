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

// SQL operations on tag table.

import (
	"log"

	"github.com/jmoiron/sqlx"

	"inchworks.com/picinch/pkg/models"
)

const (
	tagDelete = `DELETE FROM tag WHERE id = ?`

	tagInsert = `
		INSERT INTO tag (gallery, parent, name, action) VALUES (:gallery, :parent, :name, :action)`

	tagUpdate = `
		UPDATE tag
		SET parent=:parent, name=:name, action=:action
		WHERE id=:id
	`
)

const (
	tagSelect       = `SELECT * FROM tag`

	tagWhereId         = tagSelect + ` WHERE id = ?`
	tagRootWhereName  = tagSelect + ` WHERE gallery = ? AND parent = 0 AND name = ?`
	tagChildWhereName  = tagSelect + ` WHERE parent = ? AND name = ?`
)

type TagStore struct {
	GalleryId    int64
	store
}

func NewTagStore(db *sqlx.DB, tx **sqlx.Tx, log *log.Logger) *TagStore {

	return &TagStore{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  log,
			sqlDelete: tagDelete,
			sqlInsert: tagInsert,
			sqlUpdate: tagUpdate,
		},
	}
}

// GetNamed returns a tag selected by parent tag and name.
func (st *TagStore) GetNamed(parent int64, name string) (*models.Tag, error) {

	var t models.Tag

	var err error
	if parent == 0 {
		err = st.DBX.Get(&t, tagRootWhereName, st.GalleryId, name)
	} else {
		err = st.DBX.Get(&t, tagChildWhereName, parent, name)
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}


// Update inserts or updates a tag.
func (st *TagStore) Update(r *models.Tag) error {
	return st.updateData(&r.Id, r)
}
