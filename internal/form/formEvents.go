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
	"time"

	"github.com/inchworks/webparts/v2/multiforms"
	"inchworks.com/picinch/internal/models"
)

type EventsForm struct {
	*multiforms.Form
	Children []*EventFormData
}

type EventFormData struct {
	multiforms.Child
	Publish time.Time // date
	Start   time.Time // date-time
	Title   string
	Caption string
}

// NewSlides returns a form with the expected capacity.
func NewEvents(data url.Values, nSlides int, token string) *EventsForm {
	return &EventsForm{
		Form:     multiforms.New(data, token),
		Children: make([]*EventFormData, 0, nSlides+1),
	}
}

// Add appends an event to the form.
func (f *EventsForm) Add(index int, publish time.Time, start time.Time, title string, caption string) {

	f.Children = append(f.Children, &EventFormData{
		Child:   multiforms.Child{Parent: f.Form, ChildIndex: index},
		Publish: publish,
		Start:   start,
		Title:   title,
		Caption: caption,
	})
}

// AddTemplate adds a template for new events to the form.
func (f *EventsForm) AddTemplate(nSlides int) {

	f.Children = append(f.Children, &EventFormData{
		Child: multiforms.Child{Parent: f.Form, ChildIndex: -1},
	})
}

// GetSlides returns event slides as structs. They are sent as arrays of values for each field name.
func (f *EventsForm) GetEvents(vt ValidTypeFunc) (items []*EventFormData, err error) {

	now := time.Now()
	nItems := f.NChildItems()

	for i := 0; i < nItems; i++ {

		ix, err := f.ChildIndex("index", i)
		if err != nil {
			return nil, err
		}

		items = append(items, &EventFormData{
			Child:   multiforms.Child{Parent: f.Form, ChildIndex: ix},
			Publish: f.ChildTime("publish", i, ix, now, time.Local),
			Start:   f.ChildTime("start", i, ix, time.Time{}, time.Local),
			Title:   f.ChildText("title", i, ix, 0, models.MaxDetail), // allow long titles for slides
			Caption: f.ChildText("caption", i, ix, 0, models.MaxDetail),
		})
	}

	// Add the child items back into the form, in case we need to redisplay it
	f.Children = items

	return items, nil
}
