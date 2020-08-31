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

	"github.com/jmoiron/sqlx"

	"inchworks.com/gallery/pkg/models"
)

const (
	userDelete = `DELETE FROM user WHERE id = ?`

	userInsert = `
		INSERT INTO user (gallery, username, name, status, password, created) VALUES (:gallery, :username, :name, :status, :password, :created)`

	userUpdate = `
		UPDATE user
		SET username=:username, name=:name, status=:status, password=:password, created=:created
		WHERE id=:id
	`
)

const (
	userSelect    = `SELECT * FROM user`
	userOrderName = ` ORDER BY name`

	userWhereId       = userSelect + ` WHERE id = ?`
	userWhereName     = userSelect + ` WHERE gallery = ? AND username = ?`
	usersWhereGallery = userSelect + ` WHERE gallery = ?`

	usersByName = usersWhereGallery + userOrderName

	userCount = `SELECT COUNT(*) FROM user WHERE gallery = ?`

	usersWithSlideshows0 = `
		SELECT user.id AS userid, user.name, slideshow.id AS slideshowid, slideshow.title as showtitle FROM user
			LEFT JOIN slideshow ON slideshow.user = user.id
			WHERE user.gallery = ?
			ORDER BY user.name ASC
	`

	usersHavingSlideshows = `
		SELECT * FROM user
			WHERE user.gallery = ? AND EXISTS
				  ( SELECT * FROM slideshow WHERE slideshow.user = user.id )
			ORDER BY user.name ASC
	`

	// Users ordered by most recent slideshow.
	// This is tricky. First get all slideshows, partition them by user and sort within users by date.
	// Then take the first ranked ones, and join the users.
	// https://dev.mysql.com/doc/refman/8.0/en/example-maximum-column-group-row.html

	usersByLatestSlideshow = `
		WITH s1 AS (
			SELECT user AS userId, created,
				RANK() OVER (PARTITION BY user
									ORDER BY created DESC, id
							) AS rnk
			FROM slideshow
			WHERE gallery = ? AND visible <> 0
		)
		SELECT user.*
		FROM s1
		LEFT JOIN user ON userId = user.id
		WHERE rnk = 1
		ORDER BY s1.created DESC
	`
)

type UserStore struct {
	GalleryId int64
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

// All users, unordered

func (st *UserStore) All() []*models.User {

	var users []*models.User

	if err := st.DBX.Select(&users, usersWhereGallery, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}
	return users
}

// All users, in name order

func (st *UserStore) ByName() []*models.User {

	var users []*models.User

	if err := st.DBX.Select(&users, usersByName, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}
	return users
}

// All users with published slideshows, ordered by latest slideshow

func (st *UserStore) Contributors() []*models.User {

	var users []*models.User

	if err := st.DBX.Select(&users, usersByLatestSlideshow, st.GalleryId); err != nil {
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

// Get user

func (st *UserStore) Get(id int64) (*models.User, error) {

	var t models.User

	if err := st.DBX.Get(&t, userWhereId, id); err != nil {
		return nil, st.logError(err)
	}

	return &t, nil
}

// Get user ID for username

func (st *UserStore) GetNamed(username string) (*models.User, error) {

	var t models.User

	if err := st.DBX.Get(&t, userWhereName, st.GalleryId, username); err != nil {
		// unknown users are expected, not logged as an error
		return nil, st.convertError(err)
	}

	return &t, nil
}

// Convenience function for user's name

func (st *UserStore) Name(id int64) string {

	u, err := st.Get(id)

	if err != nil { return "" } else { return u.Name }
}


// Insert or update user

func (st *UserStore) Update(u *models.User) error {

	u.Gallery = st.GalleryId

	return st.updateData(&u.Id, u)
}
