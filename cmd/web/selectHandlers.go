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
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/inchworks/webparts/v2/multiforms"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/nosurf"

	"inchworks.com/picinch/internal/tags"
)

// getFormInspect serves a form to request slides for inspection.
func (app *Application) getFormInspection(w http.ResponseWriter, r *http.Request) {

	f := app.galleryState.forInspection(nosurf.Token(r))

	// display form
	app.render(w, r, "inspection.page.tmpl", &simpleFormData{
		Form: f,
	})
}

func (app *Application) postFormInspection(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := multiforms.New(r.PostForm, nosurf.Token(r))
	f.Required("from")
	from := f.Get("from")

	_, err = time.Parse("2006-01-02T15:04", from)
	if err != nil {
		f.Errors.Add("from", "Invalid date/time")
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "inspection.page.tmpl", &simpleFormData{
			Form: f,
		})
		return
	}

	// display slides
	http.Redirect(w, r, "/inspect/" + from + ":00/0", http.StatusSeeOther)
}

// getFormTagSlideshow serves a form to change slideshow tags.
func (app *Application) getFormTagSlideshow(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	showId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)
	rootId, _ := strconv.ParseInt(ps.ByName("nRoot"), 10, 64)
	forUserId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)
	byUserId := app.authenticatedUser(r)

	// get slideshow tags for all users
	st, f, t, users := app.galleryState.forEditSlideshowTags(showId, rootId, forUserId, byUserId, app.role(r), nosurf.Token(r))
	if st != 0 {
		http.Error(w, http.StatusText(st), st)
		return
	}

	// tag values and fields
	var tus []*tagUser
	for _, u := range users {
		tu := &tagUser{
			Name: u.name,
			Tags: tagHTML(u.tags),
		}
		tus = append(tus, tu)
	}

	// display form
	app.render(w, r, "edit-tags-slideshow.page.tmpl", &tagsFormData{
		Form:  f,
		Title: t,
		Users: tus,
	})
}

func (app *Application) postFormTagSlideshow(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	// ## Validation needed?
	f := multiforms.New(r.PostForm, nosurf.Token(r))
	showId, _ := strconv.ParseInt(f.Get("nShow"), 36, 64)
	rootId, _ := strconv.ParseInt(f.Get("nRoot"), 36, 64)
	forUserId, _ := strconv.ParseInt(f.Get("nUser"), 36, 64)

	// save changes
	status := app.galleryState.onEditSlideshowTags(showId, rootId, forUserId, app.authenticatedUser(r), app.role(r), f)
	if status == 0 {
		app.redirectWithFlash(w, r, "/members", "Tag changes saved.")

	} else {
		http.Error(w, http.StatusText(status), status)
	}
}

// getFormTags serves the form to edit tag definitions.
func (app *Application) getFormTags(w http.ResponseWriter, r *http.Request) {
	app.redirectWithFlash(w, r, "/members", "Not implemented yet.")
}

// postFormTags handles submission of a form to edit tag definitions.
func (app *Application) postFormTags(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}
	app.redirectWithFlash(w, r, "/members", "Not implemented yet.")
}

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
		app.httpBadRequest(w, err)
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
	http.Redirect(w, r, "/entry/"+strconv.FormatInt(nShow, 10), http.StatusSeeOther)
}

// inspect handles a request to inspect recently created or revised slides.
func (app *Application) inspect(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	// start time for next segment of inspection
	from, err := time.Parse("2006-01-02T15:04:05", ps.ByName("from"))
	if err != nil {
		app.log(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// last slide ID of previous segment
	lastId, _ := strconv.ParseInt(ps.ByName("nLast"), 36, 64)

	data, flash := app.galleryState.displayInspection(from, lastId)
	if data == nil {
		app.redirectWithFlash(w, r, "/", flash)
		return
	}

	// display page
	app.render(w, r, "carousel-section.page.tmpl", data)
}

// slideshowsTagged handles a request to view tagged slideshows for a topic.
func (app *Application) slideshowsTagged(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	topicId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)
	rootId, _ := strconv.ParseInt(ps.ByName("nRoot"), 10, 64)
	tagId, _ := strconv.ParseInt(ps.ByName("nTag"), 10, 64)
	forUserId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	nMax, _ := strconv.ParseInt(ps.ByName("nMax"), 10, 32)
	byUserId := app.authenticatedUser(r)

	// template and data for slides
	st, data := app.galleryState.displayTagged(topicId, rootId, tagId, forUserId, byUserId, app.role(r), int(nMax))
	if st == 0 {
		http.Error(w, http.StatusText(st), st)
		return
	}

	// display page
	app.render(w, r, "tagged.page.tmpl", data)
}

// userTags handles a request to view tags assigned to the user.
func (app *Application) userTags(w http.ResponseWriter, r *http.Request) {

	userId := app.authenticatedUser(r)

	data := app.galleryState.displayUserTags(userId, app.role(r))

	// display page
	app.render(w, r, "user-tags.page.tmpl", data)
}

// INTERNAL FUNCTIONS

// tagChecks returns the template data for a set of tag checkboxes.
func tagHTML(tags []*tags.ItemTag) []*tagData {

	var tcs []*tagData
	for _, t := range tags {
		var html string

		if t.Edit {

			const inputHtml = `
				<div class="form-check">
				<input class="form-check-input" type="%s" name="%s" value="%s" id="F%s" %s>
				<label class="form-check-label" for="F%s">%s</label>
				</div>
			`

			// names for form input and element ID
			radio := strconv.FormatInt(t.Parent, 36)
			nm := strconv.FormatInt(t.Id, 36)

			var checked string
			if t.Set {
				checked = "checked"
			}

			switch t.Format {
			case "C":
				html = fmt.Sprintf(inputHtml, "checkbox", nm, "on", nm, checked, nm, t.Name)

			case "R":
				html = fmt.Sprintf(inputHtml, "radio", radio, nm, nm, checked, nm, t.Name)

			default:
				html = fmt.Sprintf("<label>%s</label>", t.Name)
			}
		} else {
			html = fmt.Sprintf("<label>%s</label>", t.Name)
		}

		tc := &tagData{
			tagId:   t.Id,
			TagHTML: template.HTML(html),
			Tags:    tagHTML(t.Children),
		}

		tcs = append(tcs, tc)
	}
	return tcs
}
