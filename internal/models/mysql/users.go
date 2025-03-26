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

// SQL operations on user table.

import (
	"log"

	"github.com/inchworks/webparts/v2/users"
	"github.com/jmoiron/sqlx"

	"inchworks.com/picinch/internal/models"
)

const (
	userDelete = `DELETE FROM user WHERE id = ?`

	userInsert = `
		INSERT INTO user (parent, username, name, role, status, password, created) VALUES (:parent, :username, :name, :role, :status, :password, :created)`

	userUpdate = `
		UPDATE user
		SET username=:username, name=:name, role=:role, status=:status, password=:password, created=:created
		WHERE id=:id
	`
)

const (
	// note that ID is included for stable ordering of selections for editing
	userSelect    = `SELECT * FROM user`
	userOrderName = ` ORDER BY name, id`

	userWhereId     = userSelect + ` WHERE id = ?`
	userWhereName   = userSelect + ` WHERE parent = ? AND username = ? AND status >= 0`
	userWhereSystem = userSelect + ` WHERE parent = ? AND username = ? AND role = 10`

	usersWhereGallery = userSelect + ` WHERE parent = ? AND status >=0`

	usersByName = usersWhereGallery + userOrderName

	userCount = `SELECT COUNT(*) FROM user WHERE parent = ? AND status >= 0`

	usersHavingTags = `
		SELECT * FROM user
		WHERE status > -10 AND EXISTS ( SELECT * FROM tag WHERE tag.gallery = ? AND tag.user = user.id )
	`

	// Users ordered by most recent published slideshow, excluding suspended users.
	// This is tricky. First get all slideshows, partition them by user and sort within users by date.
	// Then take the first ranked ones, and join the users.
	// https://dev.mysql.com/doc/refman/8.0/en/example-maximum-column-group-row.html

	usersByLatestSlideshow = `
		WITH s1 AS (
			SELECT contrib.user AS userId, contrib.created,
				RANK() OVER (PARTITION BY userId
									ORDER BY contrib.created DESC
							) AS rnk
			FROM slideshow AS contrib
			LEFT JOIN slideshow AS topic ON topic.id = contrib.topic
			WHERE contrib.gallery = ? AND contrib.image <> "" AND (contrib.visible >= ? OR (contrib.visible = -1 AND topic.visible >= ?))
		)
		SELECT user.*
		FROM s1
		JOIN user ON userId = user.id
		WHERE rnk = 1 AND user.status > 0
		ORDER BY s1.created DESC
	`

	usersWhereTag = `
		SELECT user.* FROM tagref
		JOIN user ON user.id = tagref.user
		WHERE tagref.tag = ? AND tagref.item IS NULL AND user.status >= 0
	`

	usersWhereTagName = `
	    SELECT user.* FROM tag
		JOIN tagref ON tagref.tag = tag.id 
		JOIN user ON user.id = tagref.user
		WHERE tag.gallery = ? AND tag.name = ? AND tag.parent = 0 AND tagref.item IS NULL
		AND user.status >= 0
	`
)

type UserStore struct {
	GalleryId int64

	// cached system users
	Info *users.User
	Solo *users.User

	threatLog *log.Logger
	store
}

func NewUserStore(db *sqlx.DB, tx **sqlx.Tx, errorLog *log.Logger) *UserStore {

	return &UserStore{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  errorLog,
			sqlDelete: userDelete,
			sqlInsert: userInsert,
			sqlUpdate: userUpdate,
		},
	}
}

// All returns all users for all galleries, unordered and including removed users.
// It is used only for migrations.
func (st *UserStore) All() []*users.User {

	var users []*users.User

	if err := st.DBX.Select(&users, userSelect); err != nil {
		st.logError(err)
		return nil
	}
	return users
}

// All users, in name order

func (st *UserStore) ByName() []*users.User {

	var users []*users.User

	if err := st.DBX.Select(&users, usersByName, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}
	return users
}

// Contributors returns all users with published slideshows, ordered by latest slideshow.
func (st *UserStore) Contributors(visible int) []*users.User {

	var users []*users.User

	if err := st.DBX.Select(&users, usersByLatestSlideshow, st.GalleryId, visible, visible); err != nil {
		st.logError(err)
		return nil
	}
	return users
}

// Count of users

func (st *UserStore) Count() int {

	var n int

	if err := st.DBX.Get(&n, userCount, st.GalleryId); err != nil {
		st.logError(err)
		return 0
	}

	return n
}

// ForTag returns all users with the specified root tag.
func (st *UserStore) ForTag(tagId int64) []*users.User {

	var users []*users.User

	if err := st.DBX.Select(&users, usersWhereTag, tagId); err != nil {
		st.logError(err)
		return nil
	}

	return users
}

// ForTagName returns all users with the root tag specified by name.
func (st *UserStore) ForTagName(name string) []*users.User {

	var users []*users.User

	if err := st.DBX.Select(&users, usersWhereTagName, st.GalleryId, name); err != nil {
		st.logError(err)
		return nil
	}

	return users
}

// Get user

func (st *UserStore) Get(id int64) (*users.User, error) {

	var t users.User

	if err := st.DBX.Get(&t, userWhereId, id); err != nil {
		// unknown user ID is possible, not logged as an error
		return nil, st.convertError(err)
	}

	return &t, nil
}

// Get user ID for username

func (st *UserStore) GetNamed(username string) (*users.User, error) {

	var t users.User

	if err := st.DBX.Get(&t, userWhereName, st.GalleryId, username); err != nil {
		// unknown users are expected, not logged as an error
		return nil, st.convertError(err)
	}

	return &t, nil
}

// InitSystem finds the system users that owns information and solo slideshows.
func (st *UserStore) InitSystem() error {
	var err error
	st.Info, err = st.getSystem("SystemInfo")
	if err == nil {
		st.Solo, err = st.getSystem("SystemSolo")
	}
	return err
}

// IsNoRecord returns true if error is "record not found"
func (st *UserStore) IsNoRecord(err error) bool {
	return err == models.ErrNoRecord
}

// Convenience function for user's name

func (st *UserStore) Name(id int64) string {

	u, err := st.Get(id)

	if err != nil {
		return ""
	} else {
		return u.Name
	}
}

// Redundant function, never used.
func (st *UserStore) Rollback() {
}

// Users that can set tags.
func (st *UserStore) Taggers() []*users.User {

	var users []*users.User

	if err := st.DBX.Select(&users, usersHavingTags, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}
	return users
}

// Insert or update user

func (st *UserStore) Update(u *users.User) error {

	u.Parent = st.GalleryId

	return st.updateData(&u.Id, u)
}

// getSystem looks up a system user.
func (st *UserStore) getSystem(username string) (*users.User, error) {

	var u users.User

	if err := st.DBX.Get(&u, userWhereSystem, st.GalleryId, username); err != nil {
		return nil, st.convertError(err)
	}
	return &u, nil
}

// getSystemTx looks up a system user, as part of a transaction.
func (st *UserStore) getSystemTx(username string) (*users.User, error) {

	var u users.User


	if err := (*st.ptx).Get(&u, userWhereSystem, st.GalleryId, username); err != nil {
		return nil, st.convertError(err)
	}
	return &u, nil
}
