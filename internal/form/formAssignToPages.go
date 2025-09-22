// Copyright © Rob Burke inchworks.com, 2025.

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

	"codeberg.org/inchworks/webparts/multiforms"

	"inchworks.com/picinch/internal/models"
)

type AssignToPagesForm struct {
	*multiforms.Form
	Children []*AssignToPagesFormData
}

type AssignToPagesFormData struct {
	// form fields
	multiforms.Child
	NShow int64
	Page  string

	// display fields
	Title string
	User  string
}

// NewAssignToPages returns a form to assign slideshows to pages.
func NewAssignToPages(data url.Values, token string) *AssignToPagesForm {
	return &AssignToPagesForm{
		Form:     multiforms.New(data, token),
		Children: make([]*AssignToPagesFormData, 0, 16),
	}
}

// Add appends a slideshow entry to the assignment form.
func (f *AssignToPagesForm) Add(index int, id int64, page string, title string, user string) {

	if user == "" {
		user = "*topic*"
	}

	f.Children = append(f.Children, &AssignToPagesFormData{
		Child: multiforms.Child{Parent: f.Form, ChildIndex: index},
		NShow: id,
		Page:  page,
		Title: title,
		User:  user,
	})
}

// GetAssignShows returns form data as structs. They are sent as arrays of values for each field name.
func (f *AssignToPagesForm) GetAssignToPages() (items []*AssignToPagesFormData, err error) {

	nItems := f.NChildItems()

	for i := 0; i < nItems; i++ {

		ix, err := f.ChildIndex("index", i)
		if err != nil {
			return nil, err
		}

		// optional topic assignment with show ID
		showId := int64(f.ChildPositive("nShow", i, ix))

		items = append(items, &AssignToPagesFormData{

			Child: multiforms.Child{Parent: f.Form, ChildIndex: ix},
			NShow: showId,
			Page:  f.ChildText("page", i, ix, 1, models.MaxName),
		})
	}

	// Add the child items back into the form, in case we need to redisplay it
	f.Children = items

	return items, nil
}
