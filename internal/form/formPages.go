// Copyright © Rob Burke inchworks.com, 2024.

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
	"strconv"

	"github.com/inchworks/webparts/v2/multiforms"

	"inchworks.com/picinch/internal/models"
)

type PagesForm struct {
	*multiforms.Form
	VisibleOpts []string
	Children    []*PageFormData
}

type PageFormData struct {
	multiforms.Child
	Name  string
	Title string
	Page  string // page ID, base 36, not trusted and only for a URL 
}

// NewPages returns a form to edit pages.
func NewPages(data url.Values, token string) *PagesForm {
	return &PagesForm{
		Form:        multiforms.New(data, token),
		VisibleOpts: models.VisibleOpts,
		Children:    make([]*PageFormData, 0, 16),
	}
}

// Add appends a page to the form.
func (f *PagesForm) Add(index int, name string, title string, pageId int64) {

	f.Children = append(f.Children, &PageFormData{
		Child: multiforms.Child{Parent: f.Form, ChildIndex: index},
		Name:  name,
		Title: title,
		Page:  strconv.FormatInt(pageId, 36),
	})
}

// AddTemplate appends the template for new pages.
func (f *PagesForm) AddTemplate() {

	f.Children = append(f.Children, &PageFormData{
		Child:   multiforms.Child{Parent: f.Form, ChildIndex: -1},
	})
}

// GetPages returns pages as structs. They are sent as arrays of values for each field name.
func (f *PagesForm) GetPages() (items []*PageFormData, err error) {

	nItems := f.NChildItems()

	for i := 0; i < nItems; i++ {

		ix, err := f.ChildIndex("index", i)
		if err != nil {
			return nil, err
		}

		items = append(items, &PageFormData{
			Child: multiforms.Child{Parent: f.Form, ChildIndex: ix},
			Name:  f.ChildText("name", i, ix, 1, models.MaxTitle),
			Title: f.ChildText("title", i, ix, 1, models.MaxTitle),
			Page: f.ChildText("page", i, ix, 0, 12),
		})
	}

	// Add the child items back into the form, in case we need to redisplay it
	f.Children = items

	return items, nil
}
