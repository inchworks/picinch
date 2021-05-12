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

// Requests for competition pages

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/nosurf"

	"inchworks.com/picinch/pkg/form"
	"inchworks.com/picinch/pkg/models"
)

// classes serves the home page for a competition.
func (app *Application) classes(w http.ResponseWriter, r *http.Request) {

	template, data := app.galleryState.displayClasses(app.isAuthenticated(r, models.UserFriend))
	if data == nil {
		app.clientError(w, http.StatusInternalServerError)
		return
	}

	app.render(w, r, template, data)
}

// getFormEnterComp serves the form to enter a competition.
func (app *Application) getFormEnterComp(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	// allow entry?
	id, _ := strconv.ParseInt(ps.ByName("nCategory"), 10, 64)
	if app.allowEnterCategory(r, id) == nil {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	f, c, err := app.galleryState.forEnterComp(id, nosurf.Token(r))
	if err != nil {
		app.serverError(w, err)
		return
	}

	// display form
	app.render(w, r, "enter-comp-public.page.tmpl", &compFormData{
		Form: f,
		Category: c,
	})
}

// postFormEnterComp handles a request to enter a competition.
// ## This version allows only one image.
func (app *Application) postFormEnterComp(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	f := form.NewPublicComp(r.PostForm, 1, nosurf.Token(r))
	f.Required("category", "timestamp", "name", "email", "location")
	f.MaxLength("name", 60)
	f.MaxLength("email", 60)
	f.MaxLength("location", 60)

	// agreements must be checked
	var nAgreed int
	for _, a := range []string{"agree1", "agree2"} {
		if f.Get(a) == "" {
			f.Errors.Add(a, "Agreement is required")
		} else {
			nAgreed++
		}
	}

	// expect one slide with an image
	slides, err := f.GetSlides()
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}
	if len(slides) != 1 {
		app.log(errors.New("Wrong number of slide for competition."))
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// allow entry?
	id, _ := strconv.ParseInt(f.Get("category"), 10, 64)
	show := app.allowEnterCategory(r, id)
	if show == nil {
		app.clientError(w, http.StatusUnauthorized)
		return
	}
	
	// timestamp, to associate uploaded images
	timestamp := f.Get("timestamp")

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "enter-comp-public.page.tmpl", &compFormData{
			Form: f,
			Category: show.Title,
		})
		return
	}

	// save changes
	code := app.galleryState.onEnterComp(id, timestamp, f.Get("name"), f.Get("email"), f.Get("location"),
			slides[0].Title, slides[0].Caption, slides[0].ImageName, nAgreed)
	if code != 0 {

		// #### temporary - auto validation
		app.galleryState.validate(code)

		app.session.Put(r, "flash", "Competition entry saved - check your email.")
		http.Redirect(w, r, "/", http.StatusSeeOther)

	} else {
		app.clientError(w, http.StatusBadRequest)
	}
}

// validate handles a request to validate a competition entry.
func (app *Application) validate(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	// validation code
	sc := ps.ByName("code")
	code, err := strconv.ParseInt(sc, 36, 64)
	if err != nil {
		app.wrongCode.ServeHTTP(w, r)
		return
	}

	// validate entry, get template and data for response
	template, data := app.galleryState.validate(code)
	if template == "" {
		app.wrongCode.ServeHTTP(w, r)
		return
	}

	// display page
	app.render(w, r, template, data)
}
