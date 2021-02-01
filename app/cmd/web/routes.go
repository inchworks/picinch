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

import (
	"net/http"
	"path"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

// Register handlers for routes

func (app *Application) routes() http.Handler {

	commonHandlers := alice.New(secureHeaders, app.noQuery, wwwRedirect)             // ## removed app.recoverPanic
	dynHs := alice.New(app.session.Enable, noSurf, app.authenticate, app.logRequest) // dynamic page handlers

	// HttpRouter wrapped to allow middleware handlers
	router := httprouter.New()

	// panic handler
	router.PanicHandler = app.recoverPanic()

	// log rejected routes
	router.NotFound = app.logNotFound()

	// public pages
	router.Handler("GET", "/", dynHs.Append(app.public).ThenFunc(app.home))
	router.Handler("GET", "/about", dynHs.Append(app.public).ThenFunc(app.about))

	// embedding
	router.Handler("GET", "/highlight/:prefix/:nImage", dynHs.Append(app.public).ThenFunc(app.highlight))
	router.Handler("GET", "/highlights/:nImages", dynHs.Append(app.public).ThenFunc(app.highlights))

	// setup
	router.Handler("GET", "/setup", dynHs.Append(app.requireAuthentication).ThenFunc(app.getFormGallery))
	router.Handler("POST", "/setup", dynHs.Append(app.requireAuthentication).ThenFunc(app.postFormGallery))

	router.Handler("GET", "/edit-topics", dynHs.Append(app.requireAuthentication).ThenFunc(app.getFormTopics))
	router.Handler("POST", "/edit-topics", dynHs.Append(app.requireAuthentication).ThenFunc(app.postFormTopics))

	router.Handler("GET", "/edit-users", dynHs.Append(app.requireAuthentication).ThenFunc(app.getFormUsers))
	router.Handler("POST", "/edit-users", dynHs.Append(app.requireAuthentication).ThenFunc(app.postFormUsers))

	// edit slideshows
	router.Handler("GET", "/edit-slides/:nShow", dynHs.Append(app.requireAuthentication).ThenFunc(app.getFormSlides))
	router.Handler("POST", "/edit-slides/:nShow", dynHs.Append(app.requireAuthentication).ThenFunc(app.postFormSlides))
	router.Handler("GET", "/edit-slideshows/:nUser", dynHs.Append(app.requireAuthentication).ThenFunc(app.getFormSlideshows))
	router.Handler("POST", "/edit-slideshows/:nUser", dynHs.Append(app.requireAuthentication).ThenFunc(app.postFormSlideshows))

	// edit topics
	router.Handler("GET", "/assign-slideshows", dynHs.Append(app.requireAuthentication).ThenFunc(app.getFormAssignShows))
	router.Handler("POST", "/assign-slideshows", dynHs.Append(app.requireAuthentication).ThenFunc(app.postFormAssignShows))
	router.Handler("GET", "/edit-topic/:nShow/:nUser", dynHs.Append(app.requireAuthentication).ThenFunc(app.getFormTopic))

	// upload image
	router.Handler("POST", "/upload/:nShow", dynHs.Append(app.requireAuthentication).ThenFunc(app.postFormImage))

	// displays
	router.Handler("GET", "/slideshow/:nShow/:seq", dynHs.ThenFunc(app.slideshow))
	router.Handler("GET", "/contributors", dynHs.Append(app.requireAuthentication).ThenFunc(app.contributors))
	router.Handler("GET", "/contributor/:nUser", dynHs.Append(app.requireAuthentication).ThenFunc(app.contributor))
	router.Handler("GET", "/my-slideshows", dynHs.Append(app.requireAuthentication).ThenFunc(app.slideshowsOwn))
	router.Handler("GET", "/slideshows-user/:nUser", dynHs.Append(app.requireAuthentication).ThenFunc(app.slideshowsUser))
	router.Handler("GET", "/topic-user/:nShow/:nUser", dynHs.ThenFunc(app.topicUser))
	router.Handler("GET", "/topic-contributors/:nTopic", dynHs.ThenFunc(app.topicContributors))
	router.Handler("GET", "/topics", dynHs.Append(app.requireAuthentication).ThenFunc(app.topics))
	router.Handler("GET", "/usage-days", dynHs.Append(app.requireAuthentication).ThenFunc(app.usageDays))
	router.Handler("GET", "/usage-months", dynHs.Append(app.requireAuthentication).ThenFunc(app.usageMonths))
	router.Handler("GET", "/users-curator", dynHs.Append(app.requireAuthentication).ThenFunc(app.usersCurator))

	// user authentication
	router.Handler("GET", "/user/login", dynHs.ThenFunc(app.getFormLogin))
	router.Handler("POST", "/user/login", dynHs.Append(app.limitLogin).ThenFunc(app.postFormLogin))
	router.Handler("POST", "/user/logout", dynHs.Append(app.requireAuthentication).ThenFunc(app.logout))
	router.Handler("GET", "/user/signup", dynHs.ThenFunc(app.getFormSignup))
	router.Handler("POST", "/user/signup", dynHs.Append(app.limitLogin).ThenFunc(app.postFormSignup))

	// files that must be in root
	router.GET("/apple-touch-icon.png", appleTouchHandler)
	router.GET("/favicon.ico", faviconHandler)
	router.GET("/robots.txt", robotsHandler)

	// these are just a courtesy, say no immediately instead of redirecting to "/path/" first
	misc := path.Join("/", app.cfg.MiscName)
	router.Handler("GET", misc, http.NotFoundHandler())
	router.Handler("GET", "/photos", http.NotFoundHandler())
	router.Handler("GET", "/static", http.NotFoundHandler())

	// file systems that block directory listing
	fsStatic := noDirFileSystem{http.Dir(filepath.Join(UIPath, "static"))}
	fsPhotos := noDirFileSystem{http.Dir(ImagePath)}
	fsMisc := noDirFileSystem{http.Dir(MiscPath)}

	// serve static files and content
	router.Handler("GET", path.Join(misc, "*filepath"), http.StripPrefix(misc, http.FileServer(fsMisc)))
	router.Handler("GET", "/static/*filepath", http.StripPrefix("/static", http.FileServer(fsStatic)))
	router.Handler("GET", "/photos/*filepath", http.StripPrefix("/photos", http.FileServer(fsPhotos)))

	// return 'standard' middleware chain followed by router
	return commonHandlers.Then(router)
}

// Special handling for files that must be in root

func appleTouchHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.ServeFile(w, r, filepath.Join(SitePath, "images/apple-touch-icon.png"))
}

func faviconHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.ServeFile(w, r, filepath.Join(SitePath, "images/favicon.ico"))
}

func robotsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.ServeFile(w, r, filepath.Join(UIPath, "static/robots.txt"))
}
