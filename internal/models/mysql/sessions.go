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

// SQL operations on sessions table.
//
// Note that this is a trivial implemention because the table used only by scs.
// However it is easier to match the implementation of the other tables than make something specific.

import (
	"log"

	"github.com/jmoiron/sqlx"
)

const (
	sessionCount = `SELECT COUNT(*) FROM sessions`
)

type SessionStore struct {
	GalleryId int64
	store
}

func NewSessionStore(db *sqlx.DB, tx **sqlx.Tx, log *log.Logger) *SessionStore {

	return &SessionStore{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  log,
		},
	}
}

// Count returns the number of sessions.
// (It is used just as a convenient way to check if the sessions table exists.)
func (st *SessionStore) Count() (int, error) {
	var n int

	return n, st.DBX.Get(&n, sessionCount)
}