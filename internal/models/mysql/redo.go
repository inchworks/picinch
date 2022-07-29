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

// SQL operations on redo log.

import (
	"log"

	"inchworks.com/picinch/internal/models"

	"github.com/inchworks/webparts/etx"
	"github.com/jmoiron/sqlx"
)

const (
	redoDelete = `DELETE FROM redo WHERE id = ?`

	redoInsert = `
		INSERT INTO redo (id, manager, optype, operation) VALUES (:id, :manager, :optype, :operation)`

	redoUpdate = `
		UPDATE redo
		SET manager=:manager, optype=:optype, operation=:operation
		WHERE id=:id
	`
)

const (
	redoCount = `SELECT COUNT(*) FROM redo`

	redoSelect  = `SELECT * FROM redo`
	redoOrderId = ` ORDER BY id`

	redoWhereId = redoSelect + ` WHERE id = ?`

	redoById               = redoSelect + redoOrderId
	redoWhereManagerBefore = redoSelect + ` WHERE manager = ? AND id < ?` + redoOrderId
)

type RedoStore struct {
	store
}

func NewRedoStore(db *sqlx.DB, tx **sqlx.Tx, errorLog *log.Logger) *RedoStore {

	return &RedoStore{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  errorLog,
			sqlDelete: redoDelete,
			sqlInsert: redoInsert,
			sqlUpdate: redoUpdate,
		},
	}
}

// All returns redo records, in ID order
func (st *RedoStore) All() []*etx.Redo {

	var rs []*etx.Redo

	if err := st.DBX.Select(&rs, redoById); err != nil {
		st.logError(err)
		return nil
	}
	return rs
}

// Count returns the number of redo records.
// (It is used just as a convenient way to check if the redo table exists.)
func (st *RedoStore) Count() (int, error) {
	var n int

	return n, st.DBX.Get(&n, redoCount)
}

// ForManager returns old redo records for the specified resource manager.
func (st *RedoStore) ForManager(rm string, before int64) []*etx.Redo {

	var rs []*etx.Redo

	if err := st.DBX.Select(&rs, redoWhereManagerBefore, rm, before); err != nil {
		st.logError(err)
		return nil
	}

	return rs
}

// Get redo record, if it exists.
func (st *RedoStore) GetIf(id int64) (*etx.Redo, error) {

	var r etx.Redo

	if err := st.DBX.Get(&r, redoWhereId, id); err != nil {
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

// Insert adds a new redo record.
func (st *RedoStore) Insert(r *etx.Redo) error {

	tx := *st.ptx
	if tx == nil {
		panic("Transaction not begun")
	}

	// insert
	if _, err := tx.NamedExec(st.sqlInsert, r); err != nil {
		return st.logError(err)
	} else {
		return nil
	}
}

// Update modifies an existing redo record.
func (st *RedoStore) Update(r *etx.Redo) error {

	tx := *st.ptx
	if tx == nil {
		panic("Transaction not begun")
	}

	if _, err := tx.NamedExec(st.sqlUpdate, r); err != nil {
		st.logError(err)
		return st.convertError(err)
	} else {
		return nil
	}
}
