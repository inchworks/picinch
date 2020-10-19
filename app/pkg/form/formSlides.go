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
)

type SlidesForm struct {
	Form
	Children []*SlideFormData
}

type SlideFormData struct {
	Child
	ShowOrder int
	Title     string
	Caption   string
	ImageName string
}

// Slides form with expected capacity.

func NewSlides(data url.Values, nSlides int) *SlidesForm {
	return &SlidesForm{
		Form:     Form{data, make(map[string][]string), make(map[string]map[int][]string)},
		Children: make([]*SlideFormData, 0, nSlides+1),
	}
}

// Add slide to form

func (f *SlidesForm) Add(index int, showOrder int, title string, imageName string, caption string) {

	f.Children = append(f.Children, &SlideFormData{
		Child:     Child{parent: &f.Form, ChildIndex: index},
		ShowOrder: showOrder,
		Title:     title,
		ImageName: imageName,
		Caption:   caption,
	})
}

// Add slide template form
//
// Sets slide order in template to the end of the show.

func (f *SlidesForm) AddTemplate(nSlides int) {

	f.Children = append(f.Children, &SlideFormData{
		Child:     Child{parent: &f.Form, ChildIndex: -1},
		ShowOrder: nSlides + 1,
	})
}

// Get slides as structs. They are sent as arrays of values for each field name.

func (f *SlidesForm) GetSlides() (items []*SlideFormData, err error) {

	nItems := f.NChildItems()

	for i := 0; i < nItems; i++ {

		ix, err := f.ChildIndex("index", i)
		if err != nil {
			return nil, err
		}

		items = append(items, &SlideFormData{
			Child:     Child{parent: &f.Form, ChildIndex: ix},
			ShowOrder: f.ChildMin("showOrder", i, ix, 1),
			Title:     f.ChildTrimmed("title", i),
			ImageName: f.ChildImage("imageName", i, ix),
			Caption:   f.ChildGet("caption", i),
		})
	}

	// Add the child items back into the form, in case we need to redisplay it
	f.Children = items

	return items, nil
}
