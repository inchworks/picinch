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

	"github.com/inchworks/webparts/v2/multiforms"

	"inchworks.com/picinch/internal/models"
)

type AssignShowsForm struct {
	*multiforms.Form
	Children    []*AssignShowFormData
}

type AssignShowFormData struct {
	multiforms.Child
	IsShared    bool
	Title       string
	NShow       int64
	NTopic      int64
	DisplayName string
	Updating    string
}

// NewAssignShows returns a form to assign slideshows to topics.
func NewAssignShows(data url.Values, token string) *AssignShowsForm {
	return &AssignShowsForm{
		Form:        multiforms.New(data, token),
		Children:    make([]*AssignShowFormData, 0, 16),
	}
}

// Add appends a slideshow entry to the assignment form.
func (f *AssignShowsForm) Add(index int, id int64, topicId int64, isShared bool, title string, user string, isUpdating bool) {

	var updating string
	if isUpdating {
		updating = "Updating"
	} else {
		updating = "-"
	}
	f.Children = append(f.Children, &AssignShowFormData{
		Child:       multiforms.Child{Parent: f.Form, ChildIndex: index},
		IsShared:    isShared,
		Title:       title,
		NShow:       id,
		NTopic:      topicId,
		DisplayName: user,
		Updating:    updating,
	})
}


// GetAssignShows returns form data as structs. They are sent as arrays of values for each field name.
func (f *AssignShowsForm) GetAssignShows() (items []*AssignShowFormData, err error) {

	nItems := f.NChildItems()

	for i := 0; i < nItems; i++ {

		ix, err := f.ChildIndex("index", i)
		if err != nil {
			return nil, err
		}

		// optional topic assignment with show ID
		showId := int64(f.ChildPositive("nShow", i, ix))
		topicId := int64(f.ChildPositive("topic", i, ix))

		items = append(items, &AssignShowFormData{

			Child:    multiforms.Child{Parent: f.Form, ChildIndex: ix},
			IsShared: f.ChildBool("shared", ix),
			NShow:    showId,
			NTopic:   topicId,
			Title:    f.ChildText("title", i, ix, 1, models.MaxTitle),
		})
	}

	// Add the child items back into the form, in case we need to redisplay it
	f.Children = items

	return items, nil
}
