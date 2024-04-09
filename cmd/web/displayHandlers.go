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

// Requests for gallery display pages

import (
	"net/http"
	"strconv"

	"github.com/inchworks/usage"
	"github.com/julienschmidt/httprouter"

	"inchworks.com/picinch/internal/models"
	"inchworks.com/picinch/internal/picinch"
)

// classes serves the home page for a competition.
func (app *Application) classes(w http.ResponseWriter, r *http.Request) {

	data := app.galleryState.displayClasses(app.isAuthenticated(r, models.UserFriend))
	if data == nil {
		httpServerError(w)
		return
	}

	app.render(w, r, "classes.page.tmpl", data)
}

// contributor shows a page of contributions from a user (for other users to see)
func (app *Application) contributor(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	// template and data for contributor
	data := app.galleryState.DisplayContributor(userId, app.isAuthenticated(r, models.UserFriend))
	if data == nil {
		// polite rejection because this could have come from a cached slideshow.
		app.session.Put(r, "flash", "Contributor removed.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "contributor.page.tmpl", data)
}

// Contributors (for other users to see)

func (app *Application) contributors(w http.ResponseWriter, r *http.Request) {

	// template and contributors
	template, data := app.galleryState.DisplayContributors()

	// display page
	app.render(w, r, template, data)
}

// entry handles a request to view a competition entry
func (app *Application) entry(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	id, _ := strconv.ParseInt(ps.ByName("nShow"), 10, 64)

	// allow access to show?
	// ## reads show, and DisplaySlideshow will read it again
	isVisible, _, _ := app.allowViewShow(r, id)

	if !isVisible {
		httpUnauthorized(w)
		return
	}

	// template and data for slides
	data := app.galleryState.DisplaySlideshow(id, app.role(r), r.Referer())
	if data == nil {
		httpServerError(w)
		return
	}

	// display page
	app.render(w, r, "carousel-competition.page.tmpl", data)
}

// Highlighted image, to be embedded in parent website

func (app *Application) highlight(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	prefix := ps.ByName("prefix")
	n, _ := strconv.Atoi(ps.ByName("nImage"))

	// get highlighted image, with the file system that holds it
	fs, image := app.galleryState.Highlighted(prefix, n)

	// return image
	if image != "" {
		picinch.ServeFile(w, r, http.FS(fs), image)
	} else {
		httpNotFound(w)
	}
}

// Highlights, to be embedded in parent website

func (app *Application) highlights(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	nImages, _ := strconv.Atoi(ps.ByName("nImages"))

	data := app.galleryState.DisplayEmbedded(nImages)

	app.render(w, r, "highlights.page.tmpl", data)
}

// home serves the main page for the website.
func (app *Application) home(w http.ResponseWriter, r *http.Request) {

	hs := app.cfg.HomeSwitch
	if hs != "" {
		app.render(w, r, hs+".page.tmpl", nil)
		return
	}

	// default home page
	data := app.galleryState.DisplayHome(app.isAuthenticated(r, models.UserFriend))
	if data == nil {
		httpServerError(w)
		return
	}

	app.render(w, r, "home.page.tmpl", data)
}

// info returns a configurable static page for the website
func (app *Application) info(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	page := "info-" + ps.ByName("page") + ".page.tmpl"

	// check if page exists
	_, ok := app.templateCache[page]
	if !ok {
		httpNotFound(w)
		return
	}

	app.render(w, r, page, nil)
}

// Logout user

func (app *Application) logout(w http.ResponseWriter, r *http.Request) {

	// remove user ID from the session data
	app.session.Remove(r, "authenticatedUserID")

	// flash message to confirm logged out
	app.session.Put(r, "flash", "You are logged out")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// slideshow handles a request to view a slideshow, or a topic
func (app *Application) slideshow(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	id, _ := strconv.ParseInt(ps.ByName("nShow"), 10, 64)
	seq, _ := strconv.ParseInt(ps.ByName("seq"), 10, 32)

	// allow access to show?
	// ## reads show, and DisplaySlideshow will read it again
	isVisible, isPublic, isTopic := app.allowViewShow(r, id)

	if !isVisible {
		httpUnauthorized(w)
		return
	}

	// set caching, with limit to private cache for non-public pages
	maxAge := strconv.Itoa(int(app.cfg.MaxCacheAge.Seconds()))
	if isPublic {
		w.Header().Set("Cache-Control", "max-age="+maxAge)
	} else {
		w.Header().Set("Cache-Control", "max-age="+maxAge+", private")
	}

	var template string
	var data *DataSlideshow
	if isTopic {
		// template and data for topic
		template, data = app.galleryState.DisplayTopicHome(id, int(seq), "/")
		if data == nil {
			// polite rejection because this could have come from a cached slideshow.
			app.session.Put(r, "flash", "No contribution to this topic.")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	} else {
		// template and data for slides
		template = "carousel-default.page.tmpl"
		data = app.galleryState.DisplaySlideshow(id, 0, r.Referer())
		if data == nil {
			httpServerError(w)
			return
		}

		// topic title overrides user's own
		if data.Topic != "" {
			data.Title = data.Topic
		}
	}

	// display page
	app.render(w, r, template, data)
}

// slideshowsOwn handles a request by a member for their own slideshows.
func (app *Application) slideshowsOwn(w http.ResponseWriter, r *http.Request) {

	// user
	userId := app.authenticatedUser(r)
	if !app.isAuthenticated(r, models.UserMember) {
		httpUnauthorized(w)
		return
	}

	data := app.galleryState.ForMyGallery(userId)
	if data == nil {
		httpServerError(w)
		return
	}

	app.render(w, r, "my-gallery.page.tmpl", data)
}

func (app *Application) slideshowsUser(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	data := app.galleryState.ForMyGallery(userId)
	if data == nil {
		httpNotFound(w)
		return
	}

	app.render(w, r, "my-gallery.page.tmpl", data)
}

// slideshowShared handles a request to view a shared slideshow or topic.
func (app *Application) slideshowShared(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	// access is allowed to anyone with the sharing code
	sc := ps.ByName("code")
	code, err := strconv.ParseInt(sc, 36, 64)
	if err != nil {
		app.wrongCode.ServeHTTP(w, r)
		return
	}

	seq, _ := strconv.ParseInt(ps.ByName("seq"), 10, 32)

	// template and data for slides
	template, data := app.galleryState.DisplayShared(code, int(seq))
	if template == "" {
		app.wrongCode.ServeHTTP(w, r)
		return

	} else if data == nil {
		app.session.Put(r, "flash", "No contributions to this topic yet.")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, template, data)
}

// Topic slides for user

func (app *Application) topicUser(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	showId, _ := strconv.ParseInt(ps.ByName("nShow"), 10, 64)
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	// template and data for slides
	data := app.galleryState.DisplayTopicUser(showId, userId, r.Referer())
	if data == nil {
		app.session.Put(r, "flash", "No slides to this topic yet.")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "carousel-default.page.tmpl", data)
}

// Users slideshows for topic

func (app *Application) topicContributors(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	topicId, _ := strconv.ParseInt(ps.ByName("nTopic"), 10, 64)

	// template and data for slides
	data := app.galleryState.DisplayTopicContributors(topicId)
	if data == nil {
		// polite rejection because this could have come from a cached slideshow.
		app.session.Put(r, "flash", "Topic removed.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "topic-contributors.page.tmpl", data)
}

// Topics

func (app *Application) topics(w http.ResponseWriter, r *http.Request) {

	data := app.galleryState.ForTopics()

	app.render(w, r, "topics.page.tmpl", data)
}

// Usage statistics

func (app *Application) usageDays(w http.ResponseWriter, r *http.Request) {

	data := app.galleryState.ForUsage(usage.Day)

	app.render(w, r, "usage.page.tmpl", data)
}

func (app *Application) usageMonths(w http.ResponseWriter, r *http.Request) {

	data := app.galleryState.ForUsage(usage.Month)

	app.render(w, r, "usage.page.tmpl", data)
}

// For curator

func (app *Application) usersCurator(w http.ResponseWriter, r *http.Request) {

	data := app.galleryState.ForUsers()

	app.render(w, r, "users-curator.page.tmpl", data)
}
