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

package main

// Form handling for gallery setup

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/inchworks/webparts/etx"
	"github.com/inchworks/webparts/multiforms"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/nosurf"

	"inchworks.com/picinch/pkg/form"
	"inchworks.com/picinch/pkg/models"
	"inchworks.com/picinch/pkg/tags"
)

type RepUpload struct {
	Error string `json:"error"`
}

// Form to assign slideshows to topics

func (app *Application) getFormAssignShows(w http.ResponseWriter, r *http.Request) {

	f := app.galleryState.ForAssignShows(nosurf.Token(r))
	if f == nil {
		app.clientError(w, http.StatusInternalServerError)
		return
	}

	// display form
	app.render(w, r, "assign-slideshows.page.tmpl", &slideshowsFormData{
		Form: f,
	})
}

func (app *Application) postFormAssignShows(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	f := form.NewSlideshows(r.PostForm, nosurf.Token(r))
	slideshows, err := f.GetSlideshows(true)
	if err != nil {
		app.errorLog.Print(err.Error())
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.errorLog.Print(f.Errors)
		app.errorLog.Print(f.ChildErrors)

		app.render(w, r, "assign-slideshows.page.tmpl", &slideshowsFormData{
			Form: f,
		})
		return
	}

	// save changes
	if app.galleryState.OnAssignShows(slideshows) {
		app.session.Put(r, "flash", "Topic assignments saved.")

	} else {
		app.session.Put(r, "flash", "Slideshow or topic deleted - check.")
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
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

	f, c, cap, err := app.galleryState.forEnterComp(id, nosurf.Token(r))
	if err != nil {
		app.serverError(w, err)
		return
	}

	// display form
	app.render(w, r, "enter-comp-public.page.tmpl", &compFormData{
		Form:     f,
		Category: c,
		Caption:  models.Nl2br(cap),
		MaxUpload: app.cfg.MaxUpload,
	})
}

// postFormEnterComp handles a request to enter a competition.
// ## This version allows only one media file.
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

	// expect one slide with a media file
	slides, err := f.GetSlides(app.validTypeCheck())
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

	// transaction Id, to associate uploaded images
	tx, err := etx.Id(f.Get("timestamp"))
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "enter-comp-public.page.tmpl", &compFormData{
			Form:     f,
			Category: show.Title,
		})
		return
	}

	// save changes
	code := app.galleryState.onEnterComp(id, tx, f.Get("name"), f.Get("email"), f.Get("location"),
		slides[0].Title, slides[0].Caption, slides[0].ImageName, nAgreed)

	if code >= 0 {
		// bind updated media, now that update is committed
		app.uploader.DoNext(tx)
	}

	if code == 0 {

		app.session.Put(r, "flash", "Competition entry saved - check your email.")
		http.Redirect(w, r, "/", http.StatusSeeOther)

	} else if code > 0 {
		// auto validation
		app.galleryState.validate(code)

		app.session.Put(r, "flash", "Competition entry accepted.")
		http.Redirect(w, r, "/", http.StatusSeeOther)

	} else {
		app.clientError(w, http.StatusBadRequest)
	}
}

// Main form to setup gallery

func (app *Application) getFormGallery(w http.ResponseWriter, r *http.Request) {

	f := app.galleryState.ForEditGallery(nosurf.Token(r))

	// display form
	app.render(w, r, "edit-gallery.page.tmpl", &simpleFormData{
		Form: f,
	})
}

func (app *Application) postFormGallery(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	f := multiforms.New(r.PostForm, nosurf.Token(r))
	f.Required("organiser", "nMaxSlides")
	f.MaxLength("organiser", 60)
	nMaxSlides := f.Positive("nMaxSlides")
	nShowcased := f.Positive("nShowcased")

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "edit-gallery.page.tmpl", &simpleFormData{
			Form: f,
		})
		return
	}

	// save changes
	// // ## could save organiser from MaxLength
	if app.galleryState.OnEditGallery(f.Get("organiser"), nMaxSlides, nShowcased) {
		app.session.Put(r, "flash", "Gallery settings saved.")
		http.Redirect(w, r, "/", http.StatusSeeOther)

	} else {
		app.clientError(w, http.StatusBadRequest)
	}
}

// postFormImage handles an uploaded media file
func (app *Application) postFormMedia(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	timestamp := ps.ByName("timestamp")

	// multipart form
	// (The limit, 10 MB, is just for memory use, not the size of the upload)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// get file returned with form
	f := r.MultipartForm.File["image"]
	if f == nil || len(f) == 0 {
		// ## don't know how we can get a form without a file, but we do
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// check file size, rounded to nearest MB
	// (Our client script checks file sizes, so we needn't send a nice error.)
	fh := f[0]
	sz := (fh.Size + (1 << 19)) >> 20
	if sz > int64(app.cfg.MaxUpload) {
		app.clientError(w, http.StatusRequestEntityTooLarge)
		return
	}

	// schedule upload to be saved as a file
	id, err := etx.Id(timestamp)
	if err != nil {
		app.clientError(w, http.StatusInternalServerError)
	}

	err, byUser := app.uploader.Save(fh, id)
	var s string
	if err != nil {
		if byUser {
			s = err.Error()

		} else {
			// server error
			app.log(err)
			app.clientError(w, http.StatusInternalServerError)
			return
		}
	}

	// return response
	app.reply(w, RepUpload{Error: s})
}

// Form to set slides for slideshow

func (app *Application) getFormSlides(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	showId, _ := strconv.ParseInt(ps.ByName("nShow"), 10, 64)

	// allow access to show?
	if !app.allowUpdateShow(r, showId) {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	f, slideshow := app.galleryState.ForEditSlideshow(showId, nosurf.Token(r))
	if f == nil {
		app.clientError(w, http.StatusInternalServerError)
		return
	}

	// display form
	app.render(w, r, "edit-slides.page.tmpl", &slidesFormData{
		Form:      f,
		Title:     slideshow.Title, // ## could be in form, to allow editing
		MaxUpload: app.cfg.MaxUpload,
	})
}

func (app *Application) postFormSlides(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	f := form.NewSlides(r.PostForm, 10, nosurf.Token(r))
	slides, err := f.GetSlides(app.validTypeCheck())
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	nShow, err := strconv.ParseInt(f.Get("nShow"), 36, 64)
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
	}
	nUser, err := strconv.ParseInt(f.Get("nUser"), 36, 64)
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
	}
	tx, err := etx.Id(f.Get("timestamp"))
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
	}

	// allow access to slideshow?
	if nShow != 0 && !app.allowUpdateShow(r, nShow) {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// need topic if there is no slideshow (otherwise we prefer to trust the database)
	var nTopic int64
	if nShow == 0 {
		nTopic, _ = strconv.ParseInt(f.Get("nTopic"), 36, 64)

		if nTopic == 0 {
			app.clientError(w, http.StatusBadRequest)
			return
		}

		// allow access for user?
		if !app.allowAccessUser(r, nUser) {
			app.clientError(w, http.StatusUnauthorized)
		}
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.errorLog.Print(f.Errors)
		app.errorLog.Print(f.ChildErrors)

		t := app.galleryState.SlideshowTitle(nShow)
		app.render(w, r, "edit-slides.page.tmpl", &slidesFormData{
			Form:      f,
			Title:     t,
			MaxUpload: app.cfg.MaxUpload,
		})
		return
	}

	// save changes
	status, userId := app.galleryState.OnEditSlideshow(nShow, nTopic, tx, nUser, slides)
	if status == 0 {
		// bind updated media, now that update is committed
		app.uploader.DoNext(tx)

		app.session.Put(r, "flash", "Slide changes saved.")
		http.Redirect(w, r, "/slideshows-user/"+strconv.FormatInt(userId, 10), http.StatusSeeOther)

	} else {
		app.clientError(w, status)
	}
}

// Form to setup slideshows

func (app *Application) getFormSlideshows(w http.ResponseWriter, r *http.Request) {

	// requested user
	ps := httprouter.ParamsFromContext(r.Context())
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	f, user := app.galleryState.ForEditSlideshows(userId, nosurf.Token(r))
	if f == nil || user == nil {
		app.clientError(w, http.StatusInternalServerError)
		return
	}

	// display form
	app.render(w, r, "edit-slideshows.page.tmpl", &slideshowsFormData{
		Form:  f,
		User:  user.Name,
		NUser: user.Id,
	})
}

func (app *Application) postFormSlideshows(w http.ResponseWriter, r *http.Request) {

	// requested user
	ps := httprouter.ParamsFromContext(r.Context())
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	err := r.ParseForm()
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	f := form.NewSlideshows(r.PostForm, nosurf.Token(r))
	slideshows, err := f.GetSlideshows(false)
	if err != nil {
		app.errorLog.Print(err.Error())
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.errorLog.Print(f.Errors)
		app.errorLog.Print(f.ChildErrors)

		n := app.galleryState.UserDisplayName(userId)
		app.render(w, r, "edit-slideshows.page.tmpl", &slideshowsFormData{
			Form:  f,
			User:  n,
			NUser: userId,
		})
		return
	}

	// save changes
	if tx := app.galleryState.OnEditSlideshows(userId, slideshows); tx != 0 {

		// bind updated media, now that update is committed
		app.tm.DoNext(tx)

		app.session.Put(r, "flash", "Slideshow changes saved.")
		http.Redirect(w, r, "/slideshows-user/"+strconv.FormatInt(userId, 10), http.StatusSeeOther)

	} else {
		app.clientError(w, http.StatusBadRequest)
	}
}

// getFormTagSlideshow serves a form to change slideshow tags.
func (app *Application) getFormTagSlideshow(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	showId, _ := strconv.ParseInt(ps.ByName("nShow"), 10, 64)
	rootId, _ := strconv.ParseInt(ps.ByName("nRoot"), 10, 64)
	userId := app.authenticatedUser(r)

	// get slideshow tags for all users
	f, t, users := app.galleryState.forEditSlideshowTags(showId, rootId, userId, nosurf.Token(r))
	if f == nil {
		app.clientError(w, http.StatusBadRequest)
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

func (app *Application) postFormTagSlideshow(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	// ## Validation needed?
	f := multiforms.New(r.PostForm, nosurf.Token(r))
	nShow, _ := strconv.ParseInt(f.Get("nShow"), 36, 64)
	nRoot, _ := strconv.ParseInt(f.Get("nRoot"), 36, 64)

	// save changes
	if app.galleryState.onEditSlideshowTags(nShow, nRoot, app.authenticatedUser(r), f) {
		app.session.Put(r, "flash", "Tag changes saved.")
		http.Redirect(w, r, "/", http.StatusSeeOther)

	} else {
		app.clientError(w, http.StatusBadRequest)
	}

}

// getFormTags serves the form to edit tag definitions.
func (app *Application) getFormTags(w http.ResponseWriter, r *http.Request) {

	app.session.Put(r, "flash", "Not implemented yet.")
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

// Form to set user's slides for topic

func (app *Application) getFormTopic(w http.ResponseWriter, r *http.Request) {

	// requested topic and user
	ps := httprouter.ParamsFromContext(r.Context())
	topicId, _ := strconv.ParseInt(ps.ByName("nShow"), 10, 64)
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	f, title := app.galleryState.ForEditTopic(topicId, userId, nosurf.Token(r))

	// display form
	app.render(w, r, "edit-slides.page.tmpl", &slidesFormData{
		Form:      f,
		Title:     title,
		MaxUpload: app.cfg.MaxUpload,
	})
}

// Form to setup topics

func (app *Application) getFormTopics(w http.ResponseWriter, r *http.Request) {

	f := app.galleryState.ForEditTopics(nosurf.Token(r))
	if f == nil {
		app.clientError(w, http.StatusInternalServerError)
		return
	}

	// display form (reusing the slideshows form, as it is so similar)
	app.render(w, r, "edit-topics.page.tmpl", &slideshowsFormData{
		Form:  f,
		User:  "Topics",
		NUser: 0,
	})
}

func (app *Application) postFormTopics(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	f := form.NewSlideshows(r.PostForm, nosurf.Token(r))
	slideshows, err := f.GetSlideshows(false)
	if err != nil {
		app.errorLog.Print(err.Error())
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.errorLog.Print(f.Errors)
		app.errorLog.Print(f.ChildErrors)

		app.render(w, r, "edit-topics.page.tmpl", &slideshowsFormData{
			Form:  f,
			User:  "Topics",
			NUser: 0,
		})
		return
	}

	// save changes
	if tx := app.galleryState.OnEditTopics(slideshows); tx != 0 {
		app.tm.DoNext(tx)
		app.session.Put(r, "flash", "Topic changes saved.")
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
