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
	"net/http"
	"strconv"

	"github.com/inchworks/webparts/multiforms"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/nosurf"

	"inchworks.com/picinch/pkg/form"
	"inchworks.com/picinch/pkg/images"
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

// Upload image

func (app *Application) postFormImage(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	showId, err := strconv.ParseInt(ps.ByName("nShow"), 10, 64)
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
	}

	// allow access to slideshow?
	if !app.allowUpdateShow(r, showId) {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// multipart form, maximum upload of 32 MB of files.
	// ## Make configurable.
	err = r.ParseMultipartForm(32 << 20)
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// save image returned with form
	fh := r.MultipartForm.File["image"][0]

	err, byUser := images.Save(fh, showId, app.chImage)
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

	// display form
	app.render(w, r, "edit-slides.page.tmpl", &slidesFormData{
		Form:  f,
		NShow: showId,
		NUser: 0,
		Title: slideshow.Title, // ## could be in form, to allow editing
	})
}

func (app *Application) postFormSlides(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	showId, err := strconv.ParseInt(ps.ByName("nShow"), 10, 64)
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
	}

	// allow access to slideshow?
	if !app.allowUpdateShow(r, showId) {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	err = r.ParseForm()
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	f := form.NewSlides(r.PostForm, 10, nosurf.Token(r))
	slides, err := f.GetSlides()
	if err != nil {
		app.log(err)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.errorLog.Print(f.Errors)
		app.errorLog.Print(f.ChildErrors)

		t := app.galleryState.SlideshowTitle(showId)
		app.render(w, r, "edit-slides.page.tmpl", &slidesFormData{Form: f, NShow: showId, Title: t})
		return
	}

	// save changes
	ok, userId := app.galleryState.OnEditSlideshow(showId, slides)
	if ok {
		app.session.Put(r, "flash", "Slide changes saved.")
		http.Redirect(w, r, "/slideshows-user/"+strconv.FormatInt(userId, 10), http.StatusSeeOther)

	} else {
		app.clientError(w, http.StatusBadRequest)
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
	if app.galleryState.OnEditSlideshows(userId, slideshows) {
		app.session.Put(r, "flash", "Slideshow changes saved.")
		http.Redirect(w, r, "/slideshows-user/"+strconv.FormatInt(userId, 10), http.StatusSeeOther)

	} else {
		app.clientError(w, http.StatusBadRequest)
	}
}

// Form to set user's slides for topic

func (app *Application) getFormTopic(w http.ResponseWriter, r *http.Request) {

	// requested topic and user
	ps := httprouter.ParamsFromContext(r.Context())
	topicId, _ := strconv.ParseInt(ps.ByName("nShow"), 10, 64)
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	f, show := app.galleryState.ForEditTopic(topicId, userId, nosurf.Token(r))

	// display form
	app.render(w, r, "edit-slides.page.tmpl", &slidesFormData{
		Form:  f,
		NShow: show.Id,
		NUser: userId,
		Title: show.Title,
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
	if app.galleryState.OnEditTopics(slideshows) {
		app.session.Put(r, "flash", "Topic changes saved.")
		http.Redirect(w, r, "/", http.StatusSeeOther)

	} else {
		app.clientError(w, http.StatusBadRequest)
	}
}
