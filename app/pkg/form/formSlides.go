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

	"github.com/inchworks/webparts/multiforms"
)

type PublicCompForm struct {
	*multiforms.Form
	Children []*SlideFormData
}

type SlidesForm struct {
	*multiforms.Form
	Children []*SlideFormData
}

type SlideFormData struct {
	multiforms.Child
	ShowOrder int
	Title     string
	Caption   string
	ImageName string
	ImageType string
}

type ValidTypeFunc func(string) bool

// NewPublicComp returns a form for a public competition, with a set number of slides.
func NewPublicComp(data url.Values, nSlides int, token string) *PublicCompForm {

	f := &PublicCompForm{
		Form:     multiforms.New(data, token),
		Children: make([]*SlideFormData, nSlides),
	}
	for i := 0; i < nSlides; i++ {
		f.Children[i] = &SlideFormData{
			Child: multiforms.Child{Parent: f.Form, ChildIndex: i},
		}
	}
	return f
}

// NewSlides returns a form with the expected capacity.
func NewSlides(data url.Values, nSlides int, token string) *SlidesForm {
	return &SlidesForm{
		Form:     multiforms.New(data, token),
		Children: make([]*SlideFormData, 0, nSlides+1),
	}
}

// Add slide to form

func (f *SlidesForm) Add(index int, showOrder int, title string, imageName string, caption string) {

	f.Children = append(f.Children, &SlideFormData{
		Child:     multiforms.Child{Parent: f.Form, ChildIndex: index},
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
		Child:     multiforms.Child{Parent: f.Form, ChildIndex: -1},
		ShowOrder: nSlides + 1,
	})
}

// GetSlides returns slides as structs. They are sent as arrays of values for each field name.
func (f *PublicCompForm) GetSlides(vt ValidTypeFunc) (items []*SlideFormData, err error) {

	nItems := f.NChildItems()

	for i := 0; i < nItems; i++ {

		ix, err := f.ChildIndex("index", i)
		if err != nil {
			return nil, err
		}

		items = append(items, &SlideFormData{
			Child:     multiforms.Child{Parent: f.Form, ChildIndex: ix},
			Title:     f.ChildRequired("title", i, ix),
			ImageName: f.ChildFile("imageName", i, ix, vt),
			Caption:   f.ChildGet("caption", i),
		})

		// require an image for every name
		if len(items[i].ImageName) == 0 {
			f.ChildErrors.Add("imageName", ix, "No photo!")
		}
	}

	// Add the child items back into the form, in case we need to redisplay it
	f.Children = items

	return items, nil
}

// GetSlides returns slides as structs. They are sent as arrays of values for each field name.
func (f *SlidesForm) GetSlides(vt ValidTypeFunc) (items []*SlideFormData, err error) {

	nItems := f.NChildItems()

	for i := 0; i < nItems; i++ {

		ix, err := f.ChildIndex("index", i)
		if err != nil {
			return nil, err
		}

		items = append(items, &SlideFormData{
			Child:     multiforms.Child{Parent: f.Form, ChildIndex: ix},
			ShowOrder: f.ChildMin("showOrder", i, ix, 1),
			Title:     f.ChildTrimmed("title", i),
			ImageName: f.ChildFile("imageName", i, ix, vt),
			Caption:   f.ChildGet("caption", i),
		})
	}

	// Add the child items back into the form, in case we need to redisplay it
	f.Children = items

	return items, nil
}
