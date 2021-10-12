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

package main

// Requests for gallery display pages

import (
	"net/http"
	"strconv"

	"github.com/inchworks/webparts/multiforms"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/nosurf"
)

// getFormSelectSlideshow displays a form to select a slideshow by ID.
func (app *Application) getFormSelectSlideshow(w http.ResponseWriter, r *http.Request) {

	f := app.galleryState.forSelectSlideshow(nosurf.Token(r))

	// display form
	app.render(w, r, "select-slideshow.page.tmpl", &simpleFormData{
		Form: f,
	})
}

// postFormSelectSlideshow validates a slideshow selection and displays the slideshow.
func (app *Application) postFormSelectSlideshow(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	f := multiforms.New(r.PostForm, nosurf.Token(r))
	f.Required("nShow")

	nShow, err := strconv.ParseInt(f.Get("nShow"), 10, 64)
	if err != nil {
		f.Errors.Add("nShow", "Must be a number")
	} else {
		// check if slideshow exists
		if !app.galleryState.onSelectSlideshow(nShow) {
			f.Errors.Add("nShow", "No such slideshow")
		}
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "select-slideshow.page.tmpl", &simpleFormData{
			Form: f,
		})
		return
	}

	// display slideshow
	http.Redirect(w, r, "/entry/"+strconv.FormatInt(nShow, 10)+"/1", http.StatusSeeOther)
}

// slideshowsTagged handles a request to view tagged slideshows for a topic.
func (app *Application) slideshowsTagged(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	topicId, _ := strconv.ParseInt(ps.ByName("nTopic"), 10, 64)
	rootId, _ := strconv.ParseInt(ps.ByName("nRoot"), 10, 64)
	tagId, _ := strconv.ParseInt(ps.ByName("nTag"), 10, 64)
	nMax, _ := strconv.ParseInt(ps.ByName("nMax"), 10, 32)
	userId := app.authenticatedUser(r)


	// template and data for slides
	data := app.galleryState.displayTagged(topicId, rootId, tagId, userId, int(nMax))
	if data == nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// display page
	app.render(w, r, "tagged.page.tmpl", data)
}

// userTags handles a request to view tags assigned to the user.
func (app *Application) userTags(w http.ResponseWriter, r *http.Request) {

	userId := app.authenticatedUser(r)
			
	data := app.galleryState.displayUserTags(userId)

	// display page
	app.render(w, r, "user-tags.page.tmpl", data)
}
