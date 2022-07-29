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

	"inchworks.com/picinch/internal/models"
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
	tagSelect = `SELECT * FROM tag`

	tagOrderName     = ` ORDER BY name`
	tagOrderNameUser = ` ORDER BY name, usersname`

	tagCount = `SELECT COUNT(*) FROM tag`

	tagWhereId        = tagSelect + ` WHERE id = ?`
	tagRootWhereName  = tagSelect + ` WHERE gallery = ? AND parent = 0 AND name = ?`
	tagChildWhereName = tagSelect + ` WHERE parent = ? AND name = ?`

	tagsWherePermission = `
		SELECT tag.*
		FROM tagref
		JOIN tag ON tag.id = tagref.tag
		WHERE tagref.user = ? AND tagref.item IS NULL AND tag.parent = 0
	`

	tagsWhereTeam = `
		SELECT tag.*, tagref.user AS userid, user.name AS usersname
		FROM tag
		JOIN tagref on tagref.tag = tag.id
		JOIN user on user.id = tagref.user
		WHERE tag.id = ? AND tagref.item IS NULL` + tagOrderNameUser

	tagsWhereRoot = `
		SELECT tag.*, tagref.user AS userid, user.name AS usersname
		FROM tagref
		JOIN tag on tag.id = tagref.tag
		JOIN user on user.id = tagref.user
		WHERE tag.gallery = ? AND tagref.item IS NULL AND tag.parent = 0` + tagOrderNameUser

	tagsWhereSystem = `
		SELECT DISTINCT tag.*
		FROM tag
		JOIN tagref on tag.id = tagref.tag
		WHERE tag.gallery = ? AND tagref.item IS NOT NULL AND tag.parent = 0` + tagOrderName

	tagsWhereParent = tagSelect + ` WHERE parent = ?` + tagOrderName
)

type TagStore struct {
	GalleryId int64
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

// AllRoot returns all root tags.
func (st *TagStore) AllRoot() []*models.TagUser {

	var tags []*models.TagUser

	if err := st.DBX.Select(&tags, tagsWhereRoot, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}

	return tags
}

// Count returns the number of tags.
// (It is used just as a convenient way to check if the tag table exists.)
func (st *TagStore) Count() (int, error) {
	var n int

	return n, st.DBX.Get(&n, tagCount)
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

// ForSystem returns root tags assigned by system to slideshows.
func (st *TagStore) ForSystem() []*models.Tag {

	var tags []*models.Tag

	if err := st.DBX.Select(&tags, tagsWhereSystem, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}

	return tags
}

// ForTeam returns root tags for all users assigned a permission tag.
func (st *TagStore) ForTeam(id int64) []*models.TagUser {

	var tags []*models.TagUser

	if err := st.DBX.Select(&tags, tagsWhereTeam, id); err != nil {
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

// Update inserts or updates a tag.
func (st *TagStore) Update(r *models.Tag) error {
	r.Gallery = st.GalleryId
	return st.updateData(&r.Id, r)
}
