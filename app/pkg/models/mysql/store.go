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

// In general, queries that return multiple records don't return an error, as the caller will
// handle an empty set successfully.

import (
	"database/sql"
	"fmt"
	"log"
	"runtime/debug"

	"github.com/jmoiron/sqlx"

	"inchworks.com/gallery/pkg/models"
)

type store struct {
	DBX       	*sqlx.DB
	ptx       	**sqlx.Tx
	errorLog  	*log.Logger
	sqlDelete 	string
	sqlInsert   string
	sqlUpdate   string
}

// Delete object

func (st *store) DeleteId(id int64) error {

	tx := *st.ptx
	if tx == nil {
		panic("Transaction not begun")
	}

	if _, err := tx.Exec(st.sqlDelete, id); err != nil {
		return st.logError(err)
	}

	return nil
}

// convert errors from implementation to application

func (st *store) convertError(err error) error {

	switch err {
	case sql.ErrNoRows:
		return models.ErrNoRecord

	default:
		return err
	}
}

// Convert and log error

func (st *store) logError(err error) error {

	err = st.convertError(err)

	// trace from caller
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	st.errorLog.Output(2, trace)

	return err
}

// Update or insert data object

func (st *store) updateData(id *int64, args interface{}) error {

	tx := *st.ptx
	if tx == nil {
		panic("Transaction not begun")
	}

	if *id == 0 {
		// insert
		if result, err := tx.NamedExec(st.sqlInsert, args); err != nil {
			return st.logError(err)
		} else {
			*id, _ = result.LastInsertId()
			return nil
		}
	} else {
		// update
		if _, err := tx.NamedExec(st.sqlUpdate, args); err != nil {
			st.logError(err)
			return st.convertError(err)
		} else {
			return nil
		}
	}
}
