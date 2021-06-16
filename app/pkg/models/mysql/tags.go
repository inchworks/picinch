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
		INSERT INTO tag (gallery, parent, name, action, format) VALUES (:gallery, :parent, :name, :action, :format)`

	tagUpdate = `
		UPDATE tag
		SET action=:action, format=:format
		WHERE id=:id
	`
)

const (
	tagSelect       = `SELECT * FROM tag`

	tagOrderName = ` ORDER BY name`

	tagWhereId         = tagSelect + ` WHERE id = ?`
	tagRootWhereName  = tagSelect + ` WHERE gallery = ? AND parent = 0 AND name = ?`
	tagChildWhereName  = tagSelect + ` WHERE parent = ? AND name = ?`

	tagsWherePermission = `
		SELECT tag.*
		FROM tagref
		JOIN tag ON tag.id = tagref.tag
		WHERE tagref.user = ? AND tagref.slideshow IS NULL AND tag.parent = 0
	`

	tagWhereRef = `
	 	SELECT tag.*, tagref.slideshow AS slideshowid
		FROM tagref
		JOIN tag ON tag.id = tagref.tag
		WHERE tagref.id = ?
	`

	tagsWhereParent = tagSelect + ` WHERE parent = ?` + tagOrderName
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

// ForParent returns all tags specific to a user.
func (st *TagStore) ForParent(parent int64) []*models.Tag {

	var tags []*models.Tag

	if err := st.DBX.Select(&tags, tagsWhereParent, parent); err != nil {
		st.logError(err)
		return nil
	}

	return tags
}

// ForUser returns all root tags for which a user has permission.
func (st *TagStore) ForUser(user int64) []*models.Tag {

	var tags []*models.Tag

	if err := st.DBX.Select(&tags, tagsWherePermission, user); err != nil {
		st.logError(err)
		return nil
	}

	return tags
}

// GetIf returns a tag by ID, if it exists.
func (st *TagStore) GetIf(id int64) *models.Tag {

	var r models.Tag

	if err := st.DBX.Get(&r, tagWhereId, id); err != nil {
		if st.convertError(err) != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}

	return &r
}

// GetNamed returns a tag selected by parent tag and name.
func (st *TagStore) GetNamed(parent int64, name string) *models.Tag {

	var t models.Tag

	var err error
	if parent == 0 {
		err = st.DBX.Get(&t, tagRootWhereName, st.GalleryId, name)
	} else {
		err = st.DBX.Get(&t, tagChildWhereName, parent, name)
	}

	if err != nil {
		if st.convertError(err) != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}
	
	return &t
}

// ForReference returns a tag and slideshow ID for a reference.
func (st *TagStore) ForReference(tagRef int64) *models.TagSlideshow {

	var r models.TagSlideshow

	if err := st.DBX.Get(&r, tagWhereRef, tagRef); err != nil {
		if st.convertError(err) != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}

	return &r
}

// Update inserts or updates a tag.
func (st *TagStore) Update(r *models.Tag) error {
	r.Gallery = st.GalleryId
	return st.updateData(&r.Id, r)
}
