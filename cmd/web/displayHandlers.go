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

	data := app.galleryState.DisplayClasses(app.isAuthenticated(r, models.UserFriend))
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

	// data for contributor
	data := app.galleryState.DisplayContributor(userId, app.isAuthenticated(r, models.UserFriend))
	if data == nil {
		// polite rejection because this could have come from browser history or the current page read long ago.
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

// embedded returns a highlighted image, to be embedded in a parent website.
func (app *Application) embedded(w http.ResponseWriter, r *http.Request) {

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

// embeddedImages returns a page of highlights, to be embedded in a parent website.
func (app *Application) embeddedImages(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	nImages, _ := strconv.Atoi(ps.ByName("nImages"))

	data := app.galleryState.DisplayEmbedded(nImages)

	app.render(w, r, "highlights.page.tmpl", data)
}

// entry handles a request to view a competition entry
func (app *Application) entry(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// template and data for slides
	data := app.galleryState.DisplaySlideshow(id, app.role(r),
		func(s *models.Slideshow, _ int64) string {
			return r.Referer() // ## ok if we don't cache
		})

	if data == nil {
		httpUnauthorized(w) // ## just a guess
		return
	}

	// display page
	app.render(w, r, "carousel-competition.page.tmpl", data)
}

// forShow handles a request to view a contributor's slideshow, by any user or the public.
func (app *Application) forShow(w http.ResponseWriter, r *http.Request) {

	// ## just one line different to slideshow()
	ps := httprouter.ParamsFromContext(r.Context())
	id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// cached and returns to contributor
	data := app.galleryState.DisplaySlideshow(id, 0,
		func(s *models.Slideshow, userId int64) string {
			if app.allowViewShow(r, s) {
				app.setCache(w, s.Id, s.Access)
				return "/contributor/" + strconv.FormatInt(userId, 10)
			} else {
				return ""
			}
		})

	if data == nil {
		// polite rejection because this could have come from browser history or the current page read long ago.
		app.session.Put(r, "flash", "Slideshow removed.")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "carousel-default.page.tmpl", data)
}

// forTopic handles a request to view a contribution to a topic, by any user or the public.
func (app *Application) forTopic(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)
	topicId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// cached and returns to home page
	var tp string
	data := app.galleryState.DisplayUserTopic(userId, topicId,
		func(t *models.Slideshow, fmt string) string {
			if app.allowViewShow(r, t) {
				app.setCache(w, topicId, t.Access)
				if fmt == "H" {
					tp = "carousel-highlights.page.tmpl"
				} else {
					tp = "carousel-default.page.tmpl"
				}

				return "/contributor/" + strconv.FormatInt(userId, 10)
			} else {
				return ""
			}
		})

	if data == nil {
		// polite rejection because this could have come from browser history or the current page read long ago.
		app.session.Put(r, "flash", "Contribution removed from topic.")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, tp, data)
}

// highlights handles a request to view highlight slides for a topic, by any user or the public.
func (app *Application) highlights(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// cached and returns to home page
	data := app.galleryState.DisplayHighlights(id,
		func(t *models.Slideshow) string {
			if app.allowViewShow(r, t) {
				app.setCache(w, t.Id, t.Access)
				return app.toHome(r)
			} else {
				return ""
			}
		})

	if data == nil {
		// polite rejection because this could have come from browser history or the current page read long ago.
		app.session.Put(r, "flash", "No highlights.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "carousel-highlights.page.tmpl", data)
}

// home serves the main page for the public.
func (app *Application) home(w http.ResponseWriter, r *http.Request) {

	member := app.isAuthenticated(r, models.UserFriend)
	if member {
		// show members home page if logged in
		http.Redirect(w, r, "/members", http.StatusSeeOther)
		return
	}

	hs := app.cfg.HomeSwitch
	if hs != "" {
		app.render(w, r, hs+".page.tmpl", nil)
		return
	}

	// default home page
	data := app.galleryState.DisplayHome(false)
	if data == nil {
		httpServerError(w)
		return
	}

	app.render(w, r, "home.page.tmpl", data)
}

// homeMembers serves the main page for members.
func (app *Application) homeMembers(w http.ResponseWriter, r *http.Request) {

	member := app.isAuthenticated(r, models.UserFriend)
	if !member {
		// show public home page if logged out
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// default home page
	data := app.galleryState.DisplayHome(true)
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

// ownShow handles a request by a member to view their own slideshow
func (app *Application) ownShow(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// user
	userId := app.authenticatedUser(r)
	if !app.isAuthenticated(r, models.UserMember) {
		httpUnauthorized(w)
		return
	}

	// not cached so that changes are visible immediately, and returns to user's list
	data := app.galleryState.DisplaySlideshow(id, 0,
		func(s *models.Slideshow, ownerId int64) string {
			if userId != ownerId {
				return ""
			} else if app.allowViewShow(r, s) {
				return "/my-slideshows"
			} else {
				return ""
			}
		})

	if data == nil {
		// unlikely unless user saved a link to own slideshow or changed ID
		app.session.Put(r, "flash", "Slideshow not known.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "carousel-default.page.tmpl", data)
}

// ownSlides handles a request by a member to view their own section of a topic.
// They need not have a contribution to make the request.
func (app *Application) ownTopic(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	topicId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// user
	userId := app.authenticatedUser(r)
	if !app.isAuthenticated(r, models.UserMember) {
		httpUnauthorized(w)
		return
	}

	// template and data for slides
	var tp string
	data := app.galleryState.DisplayUserTopic(userId, topicId,
		func(_ *models.Slideshow, fmt string) string {
			if fmt == "H" {
				tp = "carousel-highlights.page.tmpl"
			} else {
				tp = "carousel-default.page.tmpl"
			}
			return "/my-slideshows"
		})
	if data == nil {
		app.session.Put(r, "flash", "No slides to this topic yet.")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, tp, data)
}

// reviewHighlights handles a request by the curator to view highlight slides for a topic.
func (app *Application) reviewHighlights(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// not cached, and returns to curator's list of topics
	data := app.galleryState.DisplayHighlights(id,
		func(t *models.Slideshow) string {
			if app.allowViewShow(r, t) {
				return "/topics"
			} else {
				return ""
			}
		})

	if data == nil {
		// ## Shouldn't ever fail
		app.session.Put(r, "flash", "No highlights.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "carousel-highlights.page.tmpl", data)
}

// reviewSlides handles a request by the curator to view a section of a topic.
func (app *Application) reviewSlides(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)
	sec, _ := strconv.ParseInt(ps.ByName("nSec"), 10, 64)

	// template and data for slides
	var tp string
	data := app.galleryState.DisplaySlides(id, sec, "rev-",
		func(_ *models.Slideshow, fmt string) string {
			if fmt == "H" {
				tp = "carousel-highlights.page.tmpl"
			} else {
				tp = "carousel-section.page.tmpl"
			}
			return "/topics"
		})
	if data == nil {
		// ## shouldn't ever fail
		app.session.Put(r, "flash", "Contribution removed from topic.")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, tp, data)
}

// reviewTopic handles a request to view a topic header, by the curator.
func (app *Application) reviewTopic(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// not cached, and returns to curator's list of topics
	data := app.galleryState.DisplayTopic(id, "rev-",
		func(t *models.Slideshow, _ int64) string {
			if app.allowViewShow(r, t) {
				return "/topics"
			} else {
				return ""
			}
		})

	if data == nil {
		// ## Shouldn't ever fail
		app.session.Put(r, "flash", "Topic removed.")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "carousel-topic.page.tmpl", data)
}

// slides handles a request to view a section of a shared topic.
func (app *Application) sharedSlides(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	// access is allowed to anyone with the sharing code
	sc := ps.ByName("code")
	code, err := strconv.ParseInt(sc, 36, 64)
	if err != nil {
		app.wrongCode.ServeHTTP(w, r)
		return
	}
	sec, _ := strconv.ParseInt(ps.ByName("nSec"), 10, 64)

	// cached and returns to home page
	data, id := app.galleryState.DisplaySharedSlides(code, sec)
	if data == nil {
		// polite rejection because code may have been shared long ago.
		app.session.Put(r, "flash", "Contribution removed from shared topic.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	app.setCache(w, id, models.SlideshowPublic)

	// display page
	app.render(w, r, "carousel-section.page.tmpl", data)
}

// slideshowShared handles a request to view a shared slideshow.
func (app *Application) sharedSlideshow(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	// access is allowed to anyone with the sharing code
	sc := ps.ByName("code")
	code, err := strconv.ParseInt(sc, 36, 64)
	if err != nil {
		app.wrongCode.ServeHTTP(w, r)
		return
	}

	// data for slides
	data, id := app.galleryState.DisplayShared(code)
	if data == nil {
		// polite rejection because code may have been shared long ago.
		app.session.Put(r, "flash", "Shared slideshow not available.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	app.setCache(w, id, models.SlideshowPublic)

	// display page
	// ## needs a better one
	app.render(w, r, "carousel-shared.page.tmpl", data)
}

// topic handles a request to view the header for a shared topic.
func (app *Application) sharedTopic(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	// access is allowed to anyone with the sharing code
	sc := ps.ByName("code")
	code, err := strconv.ParseInt(sc, 36, 64)
	if err != nil {
		app.wrongCode.ServeHTTP(w, r)
		return
	}

	// cached and returns to home page
	data, id := app.galleryState.DisplaySharedTopic(code)
	if data == nil {
		// polite rejection because code may have been shared long ago.
		app.session.Put(r, "flash", "Shared topic not available.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	app.setCache(w, id, models.SlideshowPublic)

	// display page
	app.render(w, r, "carousel-shared-topic.page.tmpl", data)
}

// slides handles a request to view a section of a topic, by any user or the public.
func (app *Application) slides(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	topicId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)
	secId, _ := strconv.ParseInt(ps.ByName("nSec"), 10, 64)

	// cached and returns to home page
	data := app.galleryState.DisplaySlides(topicId, secId, "",
		func(t *models.Slideshow, _ string) string {
			if app.allowViewShow(r, t) {
				app.setCache(w, topicId, t.Access)
				return app.toHome(r)
			} else {
				return ""
			}
		})

	if data == nil {
		// polite rejection because this could have come from browser history or the current page read long ago.
		app.session.Put(r, "flash", "Contribution removed from topic.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "carousel-section.page.tmpl", data)
}

// slideshow handles a request to view a single-user slideshow, by any user or the public.
func (app *Application) slideshow(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// cached and returns to home page
	data := app.galleryState.DisplaySlideshow(id, 0,
		func(s *models.Slideshow, _ int64) string {
			if app.allowViewShow(r, s) {
				app.setCache(w, s.Id, s.Access)
				return app.toHome(r)
			} else {
				return ""
			}
		})

	if data == nil {
		// polite rejection because this could have come from browser history or the current page read long ago.
		app.session.Put(r, "flash", "Slideshow removed.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "carousel-default.page.tmpl", data)
}

// slideshowsOwn handles a request by a member for their own slideshows.
func (app *Application) slideshowsOwn(w http.ResponseWriter, r *http.Request) {

	// user
	userId := app.authenticatedUser(r)
	if !app.isAuthenticated(r, models.UserMember) {
		httpUnauthorized(w)
		return
	}

	data := app.galleryState.DisplayGallery(userId)
	if data == nil {
		httpServerError(w)
		return
	}

	app.render(w, r, "my-gallery.page.tmpl", data)
}

// slideshowsUser handles a request by the curator for a member's slideshows.
func (app *Application) slideshowsUser(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	data := app.galleryState.DisplayGallery(userId)
	if data == nil {
		httpNotFound(w)
		return
	}

	app.render(w, r, "user-gallery.page.tmpl", data)
}

// topic handles a request to view a topic header, by any user or the public.
func (app *Application) topic(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// cached and returns to home page
	data := app.galleryState.DisplayTopic(id, "",
		func(t *models.Slideshow, _ int64) string {
			if app.allowViewShow(r, t) {
				app.setCache(w, id, t.Access)
				return app.toHome(r)
			} else {
				return ""
			}
		})

	if data == nil {
		// polite rejection because this could have come from browser history or the current page read long ago.
		// ## could be more specific about what is missing
		app.session.Put(r, "flash", "Topic removed.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "carousel-topic.page.tmpl", data)
}

// topicContributors handles a request to see the contributors to a topic, by any user or the public.
func (app *Application) topicContributors(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())

	topicId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// template and data for slides
	data := app.galleryState.DisplayTopicContributors(topicId, func(t *models.Slideshow) string {
		if app.allowViewShow(r, t) {
			app.setCache(w, t.Id, t.Visible)
			return app.toHome(r)
		} else {
			return ""
		}
	})

	if data == nil {
		// polite rejection because this could have come from browser history or the current page read long ago.
		app.session.Put(r, "flash", "Topic removed.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "topic-contributors.page.tmpl", data)
}

// topicUser handles a request to view a contribution to topic, by any user or the public.
func (app *Application) topicUser(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	topicId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)

	// cached and returns to home page
	var tp string
	data := app.galleryState.DisplayUserTopic(userId, topicId,
		func(t *models.Slideshow, fmt string) string {
			if app.allowViewShow(r, t) {
				app.setCache(w, topicId, t.Access)
				if fmt == "H" {
					tp = "carousel-highlights.page.tmpl"
				} else {
					tp = "carousel-default.page.tmpl"
				}
				return "/topic-contributors/" + strconv.FormatInt(topicId, 10)
			} else {
				return ""
			}
		})

	if data == nil {
		// polite rejection because this could have come from browser history or the current page read long ago.
		app.session.Put(r, "flash", "Contribution removed from topic.")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, tp, data)
}

// topics handles a request by the curator to see the topics.
func (app *Application) topics(w http.ResponseWriter, r *http.Request) {

	data := app.galleryState.DisplayTopics()

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

// userShow handles a request by the curator to view a user's slideshow
func (app *Application) userShow(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)
	id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// not cached so that changes are visible immediately, and returns to curator's list
	data := app.galleryState.DisplaySlideshow(id, 0,
		func(s *models.Slideshow, ownerId int64) string {
			if userId != ownerId {
				return ""
			} else if app.allowViewShow(r, s) {
				return "/slideshows-user/" + strconv.FormatInt(userId, 10)
			} else {
				return ""
			}
		})

	if data == nil {
		// unlikely unless user saved a link to own slideshow or changed ID
		app.session.Put(r, "flash", "Slideshow not known.")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, "carousel-default.page.tmpl", data)
}

// userTopic handles a request by the curator to view a user's section of a topic.
// They need not have a contribution to make the request.
func (app *Application) userTopic(w http.ResponseWriter, r *http.Request) {

	ps := httprouter.ParamsFromContext(r.Context())
	userId, _ := strconv.ParseInt(ps.ByName("nUser"), 10, 64)
	topicId, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)

	// template and data for slides
	var tp string
	data := app.galleryState.DisplayUserTopic(userId, topicId,
		func(_ *models.Slideshow, fmt string) string {
			if fmt == "H" {
				tp = "carousel-highlights.page.tmpl"
			} else {
				tp = "carousel-default.page.tmpl"
			}
			return "/slideshows-user/" + strconv.FormatInt(userId, 10)
		})
	if data == nil {
		app.session.Put(r, "flash", "No slides to this topic yet.")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	// display page
	app.render(w, r, tp, data)
}

// For curator

func (app *Application) usersCurator(w http.ResponseWriter, r *http.Request) {

	data := app.galleryState.DisplayUsers()

	app.render(w, r, "users-curator.page.tmpl", data)
}
