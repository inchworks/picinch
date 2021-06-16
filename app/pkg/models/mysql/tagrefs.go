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
		INSERT INTO tagref (item, tag, user, added, detail) VALUES (:item, :tag, :user, :added, :detail)`

	tagrefUpdate = `
		UPDATE tagref
		SET added=:added, detail=:detail
		WHERE id=:id
	`
)

const (
	tagrefSelect = `SELECT * FROM tagref`

	tagrefWhereId = tagrefSelect + ` WHERE id = ?`

	tagrefCountItems = `SELECT COUNT(*) FROM tagref WHERE tag = ? AND user = ? AND item IS NOT NULL`

	tagrefDeleteWhere = `
		DELETE FROM tagref
		WHERE item = ? AND tag = ? AND user = ?
	`
	tagrefDeleteWhereTag = `
		DELETE tagref FROM tagref
		INNER JOIN tag ON tag.id = tagref.tag
		WHERE tag.parent = ? AND tag.name = ? AND tag.user = ? AND tagref.item = ?
	`

	tagrefExists = `SELECT EXISTS(SELECT * FROM tagref WHERE item = ? AND tag = ? AND user = ?)`

	tagrefPermission = `SELECT EXISTS(SELECT * FROM tagref WHERE user = ? AND tag = ? AND item IS NULL)`
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

// Count returns the number of references for a tag.
func (st *TagRefStore) CountItems(tag int64, user int64) int {
	var n int

	if err := st.DBX.Get(&n, tagrefCountItems, tag, user); err != nil {
		st.logError(err)
		return 0
	}

	return n
}

// DeleteIf deletes a tag reference with a specified tag ID.
func (st *TagRefStore) DeleteIf(item int64, tag int64, user int64) error {

	if _, err := st.DBX.Exec(tagrefDeleteWhere, item, tag, user); err != nil {
		return st.logError(err)
	}

	return nil
}

// DeleteIfName deletes a tag reference with a specified tag name.
func (st *TagRefStore) DeleteIfName0(parent int64, name string, forUser int64, item int64) error {

	if _, err := st.DBX.Exec(tagrefDeleteWhereTag, parent, name, forUser, item); err != nil {
		return st.logError(err)
	}

	return nil
}

// Exists returns true if an item has the specfied tag and user.
func (st *TagRefStore) Exists(item int64, tag int64, user int64) bool {
	var e bool

	if err := st.DBX.Get(&e, tagrefExists, item, tag, user); err != nil {
		st.logError(err)
		return false
	}

	return e
}

// GetIf returns a tag by ID, if it exists.
func (st *TagRefStore) GetIf(id int64) *models.TagRef {

	var r models.TagRef

	if err := st.DBX.Get(&r, tagrefWhereId, id); err != nil {
		if st.convertError(err) != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}

	return &r
}

// HasPermission returns true if a user has a permission reference for the specified tag.
func (st *TagRefStore) HasPermission(tag int64, user int64) bool {
	var e bool

	if err := st.DBX.Get(&e, tagrefPermission, user, tag); err != nil {
		st.logError(err)
		return false
	}

	return e
}

// Update inserts or updates a tag reference.
func (st *TagRefStore) Update(r *models.TagRef) error {
	return st.updateData(&r.Id, r)
}
