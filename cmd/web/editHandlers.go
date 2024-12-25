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
	"net/http"
	"strconv"

	"github.com/inchworks/webparts/v2/etx"
	"github.com/inchworks/webparts/v2/multiforms"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/nosurf"

	"inchworks.com/picinch/internal/form"
	"inchworks.com/picinch/internal/models"
)

type RepUpload struct {
	Error string `json:"error"`
}

// Form to assign slideshows to topics

func (app *Application) getFormAssignShows(w http.ResponseWriter, r *http.Request) {

	f := app.galleryState.ForAssignShows(nosurf.Token(r))
	if f == nil {
		httpServerError(w)
		return
	}

	// display form
	app.render(w, r, "assign-slideshows.page.tmpl", &assignShowsFormData{
		Form: f,
	})
}

func (app *Application) postFormAssignShows(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := form.NewAssignShows(r.PostForm, nosurf.Token(r))
	slideshows, err := f.GetAssignShows()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "assign-slideshows.page.tmpl", &assignShowsFormData{
			Form: f,
		})
		return
	}

	// save changes (no synchronous operations to be done)
	status, _ := app.galleryState.OnAssignShows(slideshows)
	switch status {
	case 0:
		app.redirectWithFlash(w, r, "/assign-slideshows", "Slideshow assignments saved.")

	case http.StatusConflict:
		app.redirectWithFlash(w, r, "/assign-slideshows", "Slideshow or topic deleted - check.")

	default:
		http.Error(w, http.StatusText(status), status)
	}
}

// getFormEnterComp serves the form to enter a competition.
func (app *Application) getFormEnterComp(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	// allow entry?
	id, _ := strconv.ParseInt(ps.ByName("nClass"), 10, 64)
	if app.allowEnterClass(r, id) == nil {
		httpUnauthorized(w)
		return
	}

	status, f, c, cap := app.galleryState.forEnterComp(id, nosurf.Token(r))
	if status != 0 {
		http.Error(w, http.StatusText(status), status)
		return
	}

	// display form
	app.render(w, r, "enter-comp-public.page.tmpl", &compFormData{
		Form:      f,
		Class:     c,
		Caption:   models.Nl2br(cap),
		Accept:    app.accept(),
		MaxUpload: app.cfg.MaxUpload,
	})
}

// postFormEnterComp handles a request to enter a competition.
// ## This version allows only one media file.
func (app *Application) postFormEnterComp(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := form.NewPublicComp(r.PostForm, 1, nosurf.Token(r))
	f.Required("class", "timestamp", "name", "email", "location")
	f.MaxLength("name", models.MaxName)
	f.MaxLength("email", models.MaxName)
	f.MaxLength("location", models.MaxName)

	f.MatchesPattern("email", multiforms.EmailRX)

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
		app.httpBadRequest(w, err)
		return
	}
	if len(slides) != 1 {
		app.httpBadRequest(w, errors.New("Wrong number of slides for competition."))
		return
	}

	// allow entry?
	id, _ := strconv.ParseInt(f.Get("class"), 10, 64)
	show := app.allowEnterClass(r, id)
	if show == nil {
		httpUnauthorized(w)
		return
	}

	// transaction Id, to associate uploaded images
	tx, err := etx.Id(f.Get("timestamp"))
	if err != nil {
		app.httpBadRequest(w, errors.New("Wrong number of slides for competition."))
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "enter-comp-public.page.tmpl", &compFormData{
			Form:      f,
			Class:     show.Title,
			Caption:   models.Nl2br(show.Caption),
			Accept:    app.accept(),
			MaxUpload: app.cfg.MaxUpload,
		})
		return
	}

	// save changes
	email := f.Get("email")
	status, code := app.galleryState.onEnterComp(id, tx, f.Get("name"), email, f.Get("location"),
		slides[0].Title, slides[0].Caption, slides[0].MediaName, nAgreed)

	if status == 0 {
		// claim updated media, now that update is committed
		app.tm.Do(tx)

		if code == 0 {
			app.redirectWithFlash(w, r, "/", "Competition entry saved - please check your email to confirm your address: "+email+".")

		} else {
			// auto validation
			if status, _, _ = app.galleryState.validate(code); status == 0 {
				app.redirectWithFlash(w, r, "/", "Competition entry accepted.")
			}
		}
	}

	if status != 0 {
		http.Error(w, http.StatusText(status), status)
	}
}

// getFormDiary displays a form to edit diary events.
func (app *Application) getFormDiary(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	diaryId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	f, page := app.galleryState.ForEditDiary(diaryId, nosurf.Token(r))
	if f == nil {
		httpNotFound(w)
		return
	}

	// display form
	app.render(w, r, "edit-diary.page.tmpl", &diaryFormData{
		Title: page.Title,
		Form:  f,
	})
}

// postFormDiary handles a request to update diary events.
func (app *Application) postFormDiary(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := form.NewEvents(r.PostForm, 10, nosurf.Token(r))
	events, err := f.GetEvents(app.validTypeCheck())
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	nDiary, err := strconv.ParseInt(f.Get("nDiary"), 36, 64)
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	f.MaxLength("caption", models.MaxMarkdown)

	// redisplay form if data invalid
	if !f.Valid() {
		t := app.galleryState.SlideshowTitle(nDiary)
		app.render(w, r, "edit-diary.page.tmpl", &diaryFormData{
			Title: t,
			Form:  f,
		})
		return
	}

	// save changes
	status := app.galleryState.OnEditDiary(nDiary, f.Get("caption"), events)
	if status == 0 {
		app.redirectWithFlash(w, r, "/members", "Event changes saved.")

	} else {
		http.Error(w, http.StatusText(status), status)
	}
}

// Form to setup diaries
func (app *Application) getFormDiaries(w http.ResponseWriter, r *http.Request) {

	f := app.galleryState.ForEditPages(models.PageDiary, nosurf.Token(r))
	if f == nil {
		httpNotFound(w)
		return
	}

	// display form
	app.render(w, r, "edit-diaries.page.tmpl", &pagesFormData{
		Form: f,
	})
}

func (app *Application) postFormDiaries(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := form.NewPages(r.PostForm, nosurf.Token(r))
	pages, err := f.GetPages()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "edit-diaries.page.tmpl", &pagesFormData{
			Form: f,
		})
		return
	}

	// save changes
	status, tx := app.galleryState.OnEditPages(models.PageDiary, pages)
	if status == 0 {

		// rebuild page cache
		warn := app.galleryState.cachePages()

		// claim updated media, now that update is committed
		app.tm.Do(tx)

		app.redirectWithFlash(w, r, "/members", warnings(
			"Page changes saved.",
			"Conflicting page menu items:",
			warn))

	} else {
		http.Error(w, http.StatusText(status), status)
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
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := multiforms.New(r.PostForm, nosurf.Token(r))
	f.Required("organiser", "nMaxSlides")
	f.MaxLength("title", models.MaxName)
	f.MaxLength("organiser", models.MaxName)
	f.MaxLength("events", models.MaxTitle)
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
	status := app.galleryState.OnEditGallery(f.Get("organiser"), f.Get("title"), f.Get("events"), nMaxSlides, nShowcased)
	if status == 0 {
		app.redirectWithFlash(w, r, "/members", "Gallery settings saved.")

	} else {
		http.Error(w, http.StatusText(status), status)
	}
}

// postFormImage handles an uploaded media file
func (app *Application) postFormMedia(w http.ResponseWriter, r *http.Request) {

	timestamp := r.FormValue("timestamp")

	vs := r.FormValue("version")
	v, err := strconv.Atoi(vs)
	if err != nil {
		app.httpBadRequest(w, errors.New("Bad media version."))
		return
	}

	// multipart form
	// (The limit, 10 MB, is just for memory use, not the size of the upload)
	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// get file returned with form
	f := r.MultipartForm.File["media"]
	if f == nil || len(f) == 0 {
		// ## don't know how we can get a form without a file, but we do
		app.httpBadRequest(w, errors.New("Upload received without file."))
		return
	}

	// check file size, rounded to nearest MB
	// (Our client script checks file sizes, so we needn't send a nice error.)
	fh := f[0]
	sz := (fh.Size + (1 << 19)) >> 20
	if sz > int64(app.cfg.MaxUpload) {
		app.reply(w, RepUpload{Error: "Upload too large at server."})
		return
	}

	// schedule upload to be saved as a file
	id, err := etx.Id(timestamp)
	if err != nil {
		app.log(err)
		httpServerError(w)
		return
	}

	var s string
	err, byUser := app.uploader.Save(fh, id, v)
	if err != nil {
		if byUser {
			s = err.Error()

		} else {
			// server error
			app.log(err)
			httpServerError(w)
			return
		}
	}

	// return response
	app.reply(w, RepUpload{Error: s})
}

// Form to edit page metadata
func (app *Application) getFormMetadata(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	pageId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	f := app.galleryState.ForEditMetadata(pageId, nosurf.Token(r))

	// display form
	app.render(w, r, "edit-metadata.page.tmpl", &simpleFormData{
		Form: f,
	})
}

func (app *Application) postFormMetadata(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := multiforms.New(r.PostForm, nosurf.Token(r))
	pageId, err := strconv.ParseInt(f.Get("nPage"), 36, 64)
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	f.MaxLength("title", models.MaxTitle)
	f.MaxLength("desc", models.MaxTitle)
	noIndex := f.Get("noIndex") != "" // any non-empty string means true

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "edit-gallery.page.tmpl", &simpleFormData{
			Form: f,
		})
		return
	}

	// save changes
	status := app.galleryState.OnEditMetadata(pageId, f.Get("title"), f.Get("description"), noIndex)
	if status == 0 {
		app.redirectWithFlash(w, r, "/setup", "Page metadata saved.")

	} else {
		http.Error(w, http.StatusText(status), status)
	}
}

// Form to edit the content of an information page.

func (app *Application) getFormPage(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	pageId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// editing identical to a slideshow
	status, f, slideshow := app.galleryState.ForEditSlideshow(pageId, nosurf.Token(r))
	if status != 0 {
		http.Error(w, http.StatusText(status), status)
		return
	}

	// display form (data identical to a slideshow)
	app.render(w, r, "edit-page.page.tmpl", &slidesFormData{
		Form:      f,
		Title:     slideshow.Title,
		Accept:    app.accept(),
		IsHome:    app.isHome(pageId),
		MaxUpload: app.cfg.MaxUpload,
	})
}

func (app *Application) postFormPage(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := form.NewSlides(r.PostForm, 10, nosurf.Token(r))
	slides, err := f.GetSlides(app.validTypeCheck())
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	nPage, err := strconv.ParseInt(f.Get("nShow"), 36, 64)
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}
	tx, err := etx.Id(f.Get("timestamp"))
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		t := app.galleryState.SlideshowTitle(nPage)
		app.render(w, r, "edit-page.page.tmpl", &slidesFormData{
			Form:      f,
			Title:     t,
			Accept:    app.accept(),
			IsHome:    app.isHome(nPage),
			MaxUpload: app.cfg.MaxUpload,
		})
		return
	}

	// save changes
	status, _ := app.galleryState.OnEditSlideshow(nPage, 0, tx, app.userStore.Info.Id, slides, true)
	if status == 0 {

		// claim updated media, now that update is committed
		app.tm.Do(tx)
		app.redirectWithFlash(w, r, "/pages", "Page content saved.")

	} else {
		http.Error(w, http.StatusText(status), status)
	}
}

// Form to setup information pages
func (app *Application) getFormPages(w http.ResponseWriter, r *http.Request) {

	f := app.galleryState.ForEditPages(models.PageInfo, nosurf.Token(r))
	if f == nil {
		httpNotFound(w)
		return
	}

	// display form
	app.render(w, r, "edit-pages.page.tmpl", &pagesFormData{
		Form: f,
	})
}

func (app *Application) postFormPages(w http.ResponseWriter, r *http.Request) {

	// page for detail editing
	ps := httprouter.ParamsFromContext(r.Context())
	pageId, _ := strconv.ParseInt(ps.ByName("nPage"), 36, 64)

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := form.NewPages(r.PostForm, nosurf.Token(r))
	pages, err := f.GetPages()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "edit-pages.page.tmpl", &pagesFormData{
			Form: f,
		})
		return
	}

	// page to be edited next?
	var url string
	if pageId != 0 {
		url = fmt.Sprintf("/edit-metadata/%d", pageId)
	} else {
		url = "/members"
	}

	// save changes
	status, tx := app.galleryState.OnEditPages(models.PageInfo, pages)
	if status == 0 {

		// rebuild page cache
		warn := app.galleryState.cachePages()

		// claim any updated media, now that update is committed
		app.tm.Do(tx)

		app.redirectWithFlash(w, r, url, warnings(
			"Page changes saved.",
			"Conflicting page menu items:",
			warn))

	} else if status < 0 {

		// no changes - just end transaction
		app.tm.Do(tx)

		http.Redirect(w, r, url, http.StatusSeeOther)

	} else {
		http.Error(w, http.StatusText(status), status)
	}
}

// Form to set slides for slideshow

func (app *Application) getFormSlides(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	showId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// allow access to show?
	if !app.allowUpdateShow(r, showId) {
		httpUnauthorized(w)
		return
	}

	status, f, slideshow := app.galleryState.ForEditSlideshow(showId, nosurf.Token(r))
	if status != 0 {
		http.Error(w, http.StatusText(status), status)
		return
	}

	// display form
	app.render(w, r, "edit-slides.page.tmpl", &slidesFormData{
		Form:      f,
		Title:     slideshow.Title, // ## could be in form, to allow editing
		Accept:    app.accept(),
		MaxUpload: app.cfg.MaxUpload,
	})
}

func (app *Application) postFormSlides(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := form.NewSlides(r.PostForm, 10, nosurf.Token(r))
	slides, err := f.GetSlides(app.validTypeCheck())
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	nShow, err := strconv.ParseInt(f.Get("nShow"), 36, 64)
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}
	nUser, err := strconv.ParseInt(f.Get("nUser"), 36, 64)
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}
	tx, err := etx.Id(f.Get("timestamp"))
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// allow access to slideshow?
	if nShow != 0 && !app.allowUpdateShow(r, nShow) {
		httpUnauthorized(w)
		return
	}

	// need topic if there is no slideshow (otherwise we prefer to trust the database)
	var nTopic int64
	if nShow == 0 {
		nTopic, _ = strconv.ParseInt(f.Get("nTopic"), 36, 64)

		if nTopic == 0 {
			httpNotFound(w)
			return
		}

		// allow access for user?
		if !app.allowAccessUser(r, nUser, true) {
			httpUnauthorized(w)
			return
		}
	}

	// redisplay form if data invalid
	if !f.Valid() {
		t := app.galleryState.SlideshowTitle(nShow)
		app.render(w, r, "edit-slides.page.tmpl", &slidesFormData{
			Form:      f,
			Title:     t,
			Accept:    app.accept(),
			MaxUpload: app.cfg.MaxUpload,
		})
		return
	}

	// save changes
	status, userId := app.galleryState.OnEditSlideshow(nShow, nTopic, tx, nUser, slides, false)
	if status == 0 {

		// claim updated media, now that update is committed
		app.tm.Do(tx)

		if app.allowAccessUser(r, userId, false) {
			app.redirectWithFlash(w, r, "/my-slideshows", "Slide changes saved.")
		} else {
			app.redirectWithFlash(w, r, "/slideshows-user/"+strconv.FormatInt(userId, 10), "Slide changes saved.")
		}

	} else {
		http.Error(w, http.StatusText(status), status)
	}
}

// Form to setup slideshows
func (app *Application) getFormSlideshows(w http.ResponseWriter, r *http.Request) {

	// requested user
	ps := httprouter.ParamsFromContext(r.Context())
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	f, user := app.galleryState.ForEditSlideshows(userId, nosurf.Token(r))
	if f == nil || user == nil {
		httpNotFound(w)
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
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := form.NewSlideshows(r.PostForm, nosurf.Token(r))
	slideshows, err := f.GetSlideshows()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		n := app.galleryState.UserDisplayName(userId)
		app.render(w, r, "edit-slideshows.page.tmpl", &slideshowsFormData{
			Form:  f,
			User:  n,
			NUser: userId,
		})
		return
	}

	// save changes
	status, tx := app.galleryState.OnEditSlideshows(userId, slideshows)
	if status == 0 {

		// claim updated media, now that update is committed
		app.tm.Do(tx)

		if app.allowAccessUser(r, userId, false) {
			app.redirectWithFlash(w, r, "/my-slideshows", "Slideshow changes saved.")
		} else {
			app.redirectWithFlash(w, r, "/slideshows-user/"+strconv.FormatInt(userId, 10), "Slideshow changes saved.")
		}

	} else {
		http.Error(w, http.StatusText(status), status)
	}
}

// Form to set user's slides for topic

func (app *Application) getFormTopic(w http.ResponseWriter, r *http.Request) {

	// requested topic and user
	ps := httprouter.ParamsFromContext(r.Context())
	topicId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	st, f, title := app.galleryState.ForEditTopic(topicId, userId, nosurf.Token(r))
	if st != 0 {
		http.Error(w, http.StatusText(st), st)
		return
	}

	// display form
	app.render(w, r, "edit-slides.page.tmpl", &slidesFormData{
		Form:      f,
		Title:     title,
		Accept:    app.accept(),
		MaxUpload: app.cfg.MaxUpload,
	})
}

// Form to setup topics

func (app *Application) getFormTopics(w http.ResponseWriter, r *http.Request) {

	f := app.galleryState.ForEditTopics(nosurf.Token(r))
	if f == nil {
		httpServerError(w)
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
		app.httpBadRequest(w, err)
		return
	}

	// process form data
	f := form.NewSlideshows(r.PostForm, nosurf.Token(r))
	slideshows, err := f.GetSlideshows()
	if err != nil {
		app.httpBadRequest(w, err)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.render(w, r, "edit-topics.page.tmpl", &slideshowsFormData{
			Form:  f,
			User:  "Topics",
			NUser: 0,
		})
		return
	}

	// save changes
	status, tx := app.galleryState.OnEditTopics(slideshows)
	if status == 0 {
		app.tm.Do(tx)
		app.redirectWithFlash(w, r, "/topics", "Topic changes saved.")

	} else {
		http.Error(w, http.StatusText(status), status)
	}
}

// accept returns the HTML specification of acceptable file types.
func (app *Application) accept() string {

	a := "image/*"
	if len(app.cfg.VideoTypes) > 0 {
		a = a + ",video/*"
	}

	return a
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
	status, template, data := app.galleryState.validate(code)
	if status == http.StatusBadRequest {
		app.wrongCode.ServeHTTP(w, r)

	} else if status != 0 {
		http.Error(w, http.StatusText(status), status)

	} else {
		// display page
		app.render(w, r, template, data)
	}
}

// warnings returns a string of warnings for a flash message.
func warnings(ok string, warn string, warns []string) string {

	if len(warns) == 0 {
		return ok
	}
	for _, w := range warns {
		warn += "\n\t" + w
	}
	return warn + "."
}
