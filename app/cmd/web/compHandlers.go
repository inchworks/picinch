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
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/nosurf"
	// "github.com/sqs/goreturns/returns"

	"github.com/inchworks/webparts/multiforms"

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

// getFormTags serves the form to edit tag definitions.
func (app *Application) getFormTags(w http.ResponseWriter, r *http.Request) {

	// ## temporary, until we implement tag editing
	if !app.galleryState.addUserTagsAll() {
		app.clientError(w, http.StatusInternalServerError)
		return
	}

	app. session.Put(r, "flash", "Tags set for users.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// postFormTags handles submission of a form to edit tag definitions.
func (app *Application) postFormTags(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	app.session.Put(r, "flash", "Not implemented yet.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// getFormTagSlideshow serves a form to change slideshow tags.
func (app *Application) getFormTagSlideshow1(w http.ResponseWriter, r *http.Request) {

	// ## OK to trust the tagRef, because we show tags only belonging to the real user?
	ps := httprouter.ParamsFromContext(r.Context())
	tagRefId, _ := strconv.ParseInt(ps.ByName("nTagRef"), 10, 64)
	userId := app.authenticatedUser(r)

	f, t, tags := app.galleryState.forEditSlideshowTags1(userId, tagRefId, nosurf.Token(r))
	if f == nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// display form
	// ## template accepts just two levels of tags
	app.render(w, r, "edit-tag-slideshow.page.tmpl", &tagFormData{
		Form:  f,
		Title: t,
		Tags: tagHTML(tags),
	})
}

// getFormTagSlideshow serves a form to change slideshow tags.
func (app *Application) getFormTagSlideshow(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	showId, _ := strconv.ParseInt(ps.ByName("nShow"), 10, 64)
	userTagId, _ := strconv.ParseInt(ps.ByName("nUserTag"), 10, 64)
	userId := app.authenticatedUser(r)

	// #### Validate tag is for user here? Needs reading of rag record.

	// get slideshow tags for all users
	f, t, users := app.galleryState.forEditSlideshowTags(showId, userTagId, userId, nosurf.Token(r))
	if f == nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// tag values and fields
	var tus []*tagUser
	for _, u := range users {
		tu := &tagUser {
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

// tagChecks returns the template data for a set of tag checkboxes.
func tagHTML(tags []*slideshowTag) []*tagData {

	var tcs []*tagData
	for _, tag := range tags {
		var html string

		if tag.edit {

			const inputHtml = `
				<div class="form-check">
				<input class="form-check-input" type="%s" name="%s" value="%s" id="F%s" %s>
				<label class="form-check-label" for="F%s">%s</label>
				</div>
			`

			// names for form input and element ID
			radio := strconv.FormatInt(tag.parent, 36)
			nm := strconv.FormatInt(tag.id, 36)

			var checked string
			if tag.set {
				checked = "checked"
			}

			switch tag.format {
			case "C":
				html = fmt.Sprintf(inputHtml, "checkbox", nm, "on", nm, checked, nm, tag.name)

			case "R":
				html = fmt.Sprintf(inputHtml, "radio", radio, nm, nm, checked, nm, tag.name)

			default:
				html = fmt.Sprintf("<label>%s</label>", tag.name)
			}
		} else {
			html = fmt.Sprintf("<label>%s</label>", tag.name)
		}

		tc := &tagData{
			tagId: tag.id,
			TagHTML: template.HTML(html),
			Tags: tagHTML(tag.children),
		}

		tcs = append(tcs, tc)
	}
	return tcs
}

// tagged handles a request to view tagged slideshows for a topic.
func (app *Application) tagged(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	topicId, _ := strconv.ParseInt(ps.ByName("nTopic"), 10, 64)
	parentId, _ := strconv.ParseInt(ps.ByName("nParent"), 10, 64)
	nMax, _ := strconv.ParseInt(ps.ByName("nMax"), 10, 32)

	// template and data for slides
	template, data := app.galleryState.DisplayTagged(topicId, parentId, ps.ByName("tag"), int(nMax))

	// display page
	app.render(w, r, template, data)
}

// toDo handles a request to view tagged slideshows for a topic.
func (app *Application) toDo(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	topicId, _ := strconv.ParseInt(ps.ByName("nTopic"), 10, 64)
	userTagId, _ := strconv.ParseInt(ps.ByName("nUserTag"), 10, 64)
	parentId, _ := strconv.ParseInt(ps.ByName("nParent"), 10, 64)
	nMax, _ := strconv.ParseInt(ps.ByName("nMax"), 10, 32)
	userId := app.authenticatedUser(r)


	// template and data for slides
	template, data := app.galleryState.DisplayToDo(topicId, userTagId, parentId, ps.ByName("tag"), userId, int(nMax))

	// display page
	app.render(w, r, template, data)
}

func (app *Application) postFormTagSlideshow(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	// OK to trust the tagRef, because we update tags only belonging to the real user.
	// ## However we can't validate the slideshow specified by the tagref,
	// ## unless we were to check that the user has a selecting tag for it.
	f := multiforms.New(r.PostForm, nosurf.Token(r))
	nShow, _ := strconv.ParseInt(f.Get("nShow"), 36, 64)
	nUserTag, _ := strconv.ParseInt(f.Get("nUserTag"), 36, 64)

	// save changes
	if app.galleryState.onEditSlideshowTags(nShow, nUserTag, app.authenticatedUser(r), f) {
		app.session.Put(r, "flash", "Tag changes saved.")
		http.Redirect(w, r, "/", http.StatusSeeOther)

	} else {
		app.clientError(w, http.StatusBadRequest)
	}

}

// userTags handles a request to view tags assigned to the user.
func (app *Application) userTags(w http.ResponseWriter, r *http.Request) {

	userId := app.authenticatedUser(r)
			
	data := app.galleryState.userTags(userId)

	// display page
	app.render(w, r, "user-tags.page.tmpl", data)
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
