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

	tagrefCountForTag = `SELECT COUNT(*) FROM tagref WHERE tag = ?`

	tagrefDeleteAll = `
		DELETE tagref FROM tagref
		INNER JOIN tag ON tag.id = tagref.tag
		WHERE tag.parent = ? AND tag.name = ? AND tagref.slideshow = ? AND tag.user <> 0
	`
	tagrefDeleteWhere = `
		DELETE FROM tagref
		WHERE slideshow = ? AND tag = ?
	`
	tagrefDeleteWhereTag = `
		DELETE tagref FROM tagref
		INNER JOIN tag ON tag.id = tagref.tag
		WHERE tag.parent = ? AND tag.name = ? AND tag.user = ? AND tagref.slideshow = ?
	`

	tagrefExists = `SELECT EXISTS(SELECT * FROM tagref WHERE slideshow = ? AND tag = ?)`
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
func (st *TagRefStore) CountForTag(tagId int64) int {
	var n int

	if err := st.DBX.Get(&n, tagrefCountForTag, tagId); err != nil {
		st.logError(err)
		return 0
	}

	return n
}

// DeleteAll removes user-specific tag references for all users.
func (st *TagRefStore) DeleteAll(parent int64, name string, slideshow int64) error {

	if _, err := st.DBX.Exec(tagrefDeleteAll, parent, name, slideshow); err != nil {
		return st.logError(err)
	}

	return nil
}

// DeleteIf deletes a tag reference with a specified tag ID.
func (st *TagRefStore) DeleteIf(slideshow int64, tag int64) error {

	if _, err := st.DBX.Exec(tagrefDeleteWhere, slideshow, tag); err != nil {
		return st.logError(err)
	}

	return nil
}

// DeleteIfTag deletes a tag reference with a specified tag name.
func (st *TagRefStore) DeleteIfTag0(parent int64, name string, forUser int64, slideshow int64) error {

	if _, err := st.DBX.Exec(tagrefDeleteWhereTag, parent, name, forUser, slideshow); err != nil {
		return st.logError(err)
	}

	return nil
}

// Exists returns true if a slideshow has the specfied tag.
func (st *TagRefStore) Exists(slideshow int64, tag int64) bool {
	var e bool

	if err := st.DBX.Get(&e, tagrefExists, slideshow, tag); err != nil {
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

// Update inserts or updates a tag reference.
func (st *TagRefStore) Update(r *models.TagRef) error {
	return st.updateData(&r.Id, r)
}
