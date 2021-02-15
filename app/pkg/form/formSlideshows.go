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

	"inchworks.com/picinch/pkg/models"
)

type SlideshowsForm struct {
	Form
	VisibleOpts []string
	Children    []*SlideshowFormData
}

type SlideshowFormData struct {
	Child
	Visible     int
	IsShared    bool
	Title       string
	NShow       int64
	NTopic      int64
	DisplayName string // not for editing
}

// Slideshows form

func NewSlideshows(data url.Values) *SlideshowsForm {
	return &SlideshowsForm{
		Form:        Form{data, make(map[string][]string), make(map[string]map[int][]string)},
		VisibleOpts: models.VisibleOpts,
		Children:    make([]*SlideshowFormData, 0, 16),
	}
}

// Add slideshow to form

func (f *SlideshowsForm) Add(index int, id int64, topicId int64, visible int, isShared bool, title string, user string) {

	f.Children = append(f.Children, &SlideshowFormData{
		Child:       Child{parent: &f.Form, ChildIndex: index},
		Visible:     visible,
		IsShared:    isShared,
		Title:       title,
		NShow:       id,
		NTopic:      topicId,
		DisplayName: user,
	})
}

// Add slideshow template form

func (f *SlideshowsForm) AddTemplate() {

	f.Children = append(f.Children, &SlideshowFormData{
		Child:   Child{parent: &f.Form, ChildIndex: -1},
		Visible: models.SlideshowClub,
	})
}

// Get slideshows as structs. They are sent as arrays of values for each field name.

func (f *SlideshowsForm) GetSlideshows(withTopics bool) (items []*SlideshowFormData, err error) {

	nItems := f.NChildItems()

	for i := 0; i < nItems; i++ {

		ix, err := f.ChildIndex("index", i)
		if err != nil {
			return nil, err
		}

		visible, err := f.ChildSelect("visible", i, len(models.VisibleOpts))
		if err != nil {
			return nil, err
		}

		// optional topic assignment with show ID
		var showId int64
		var topicId int64
		if withTopics {
			showId = int64(f.ChildPositive("nShow", i, ix))
			topicId = int64(f.ChildPositive("topic", i, ix))
		}

		items = append(items, &SlideshowFormData{

			Child:    Child{parent: &f.Form, ChildIndex: ix},
			Visible:  visible,
			IsShared: f.ChildBool("shared", ix),
			NShow:    showId,
			NTopic:   topicId,
			Title:    f.ChildRequired("title", i, ix),
		})
	}

	// Add the child items back into the form, in case we need to redisplay it
	f.Children = items

	return items, nil
}
