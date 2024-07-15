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

// SQL operations on V1 redo log.

import (
	"log"

	"inchworks.com/picinch/internal/models"

	"github.com/inchworks/webparts/v2/etx"
	"github.com/jmoiron/sqlx"
)

const (
	redoV1Delete = `DELETE FROM redo WHERE id = ?`
)

const (
	redoV1Count = `SELECT COUNT(*) FROM redo`

	redoV1Select  = `SELECT * FROM redo`
	redoV1OrderId = ` ORDER BY id`

	redoV1WhereId = redoSelect + ` WHERE id = ?`

	redoV1ById               = redoV1Select + redoV1OrderId
	redoV1WhereManagerBefore = redoV1Select + ` WHERE manager = ? AND id < ?` + redoV1OrderId
)

type RedoV1Store struct {
	store
}

func NewRedoV1Store(db *sqlx.DB, tx **sqlx.Tx, errorLog *log.Logger) *RedoV1Store {

	return &RedoV1Store{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  errorLog,
			sqlDelete: redoV1Delete,
		},
	}
}

// All returns redo records, in ID order
func (st *RedoV1Store) All() []*etx.RedoV1 {

	var rs []*etx.RedoV1

	if err := st.DBX.Select(&rs, redoV1ById); err != nil {
		st.logError(err)
		return nil
	}
	return rs
}

// Count returns the number of redo records.
// (It is used just as a convenient way to check if the redo table exists.)
func (st *RedoV1Store) Count() (int, error) {
	var n int

	return n, st.DBX.Get(&n, redoV1Count)
}

// ForManager returns old redo records for the specified resource manager.
func (st *RedoV1Store) ForManager(rm string, before int64) []*etx.RedoV1 {

	var rs []*etx.RedoV1

	if err := st.DBX.Select(&rs, redoV1WhereManagerBefore, rm, before); err != nil {
		st.logError(err)
		return nil
	}

	return rs
}

// Get redo record, if it exists.
func (st *RedoV1Store) GetIf(id int64) (*etx.RedoV1, error) {

	var r etx.RedoV1

	if err := st.DBX.Get(&r, redoV1WhereId, id); err != nil {
		// unknown transaction ID is possible, not logged as an error
		if st.convertError(err) == models.ErrNoRecord {
			return nil, nil
		} else {
			st.logError(err)
			return nil, err
		}
	}

	return &r, nil
}
