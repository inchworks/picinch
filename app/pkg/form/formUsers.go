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

package form

import (
	"net/url"

	"inchworks.com/gallery/pkg/models"
)

type UsersForm struct {
	Form
	StatusOpts []string
	Children   []*UserFormData
}

type UserFormData struct {
	Child
	Username    string
	DisplayName string
	NUser       int64
	Status      int
}

var statusOpts = []string{"suspended", "known", "active", "curator", "admin"}

// Users form

func NewUsers(data url.Values) *UsersForm {
	return &UsersForm{
		Form:       Form{data, make(map[string][]string), make(map[string]map[int][]string)},
		StatusOpts: statusOpts,
		Children:   make([]*UserFormData, 0, 16),
	}
}

// Add user to form

func (f *UsersForm) Add(index int, u *models.User) {

	f.Children = append(f.Children, &UserFormData{
		Child:       Child{parent: &f.Form, ChildIndex: index},
		Username:    u.Username,
		DisplayName: u.Name,
		NUser:       u.Id,
		Status:      u.Status,
	})
}

// Add user form template

func (f *UsersForm) AddTemplate() {

	f.Children = append(f.Children, &UserFormData{
		Child: Child{parent: &f.Form, ChildIndex: -1},
		Status: models.UserKnown,
	})
}

// Get users structs. They are sent as arrays of values for each field name.

func (f *UsersForm) GetUsers() (items []*UserFormData, err error) {

	nItems := f.NChildItems()

	for i := 0; i < nItems; i++ {

		ix, err := f.ChildIndex("index", i)
		if err != nil {
			return nil, err
		}

		status, err := f.ChildSelect("status", i, len(statusOpts))
		if err != nil {
			return nil, err
		}

		items = append(items, &UserFormData{
			Child:       Child{parent: &f.Form, ChildIndex: ix},
			Username:    f.ChildRequired("username", i, ix),
			DisplayName: f.ChildRequired("displayName", i, ix),
			Status:      status,
		})
	}

	// Add the child items back into the form, in case we need to redisplay it
	f.Children = items

	return items, nil
}
