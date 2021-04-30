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

func (app *Application) Routes() http.Handler {

	commonHs := alice.New(secureHeaders, app.noQuery, wwwRedirect)
	dynHs := alice.New(app.limitPage, app.session.Enable, noSurf, app.authenticate, app.logRequest) // dynamic page handlers
	staticHs := alice.New(app.limitFile)

	// access to page
	adminHs := dynHs.Append(app.requireAdmin)
	authHs := dynHs.Append(app.requireAuthentication) // friend authenticated, may be further restriction by application logic
	curatorHs := dynHs.Append(app.requireCurator)
	ownerHs := dynHs.Append(app.requireOwner)
	publicHs := dynHs.Append(app.public)
	sharedHs := dynHs.Append(app.shared)

	// HttpRouter wrapped to allow middleware handlers
	router := httprouter.New()

	// panic handler
	router.PanicHandler = app.recoverPanic()

	// log rejected routes
	router.NotFound = app.routeNotFound()

	// public pages
	router.Handler("GET", "/", publicHs.ThenFunc(app.home))
	router.Handler("GET", "/about/:page", publicHs.ThenFunc(app.about))

	// public competition
	if app.cfg.Options == "main-comp" {
		router.Handler("GET", "/classes", publicHs.ThenFunc(app.classes))
		router.Handler("GET", "/enter-comp/:nCategory", publicHs.ThenFunc(app.getFormEnterComp))
		router.Handler("POST", "/enter-comp", publicHs.ThenFunc(app.postFormEnterComp))
		router.Handler("GET", "/validate/:code", sharedHs.ThenFunc(app.validate))
	}

	// pages shared with an access code
	router.Handler("GET", "/shared/:code/:seq", sharedHs.ThenFunc(app.slideshowShared))

	// embedding
	router.Handler("GET", "/highlight/:prefix/:nImage", publicHs.ThenFunc(app.highlight))
	router.Handler("GET", "/highlights/:nImages", publicHs.ThenFunc(app.highlights))

	// setup
	router.Handler("GET", "/setup", adminHs.ThenFunc(app.getFormGallery))
	router.Handler("POST", "/setup", adminHs.ThenFunc(app.postFormGallery))

	router.Handler("GET", "/edit-topics", curatorHs.ThenFunc(app.getFormTopics))
	router.Handler("POST", "/edit-topics", curatorHs.ThenFunc(app.postFormTopics))

	// edit slideshows
	router.Handler("GET", "/edit-slides/:nShow", authHs.ThenFunc(app.getFormSlides))
	router.Handler("POST", "/edit-slides", authHs.ThenFunc(app.postFormSlides))
	router.Handler("GET", "/edit-slideshows/:nUser", ownerHs.ThenFunc(app.getFormSlideshows))
	router.Handler("POST", "/edit-slideshows/:nUser", ownerHs.ThenFunc(app.postFormSlideshows))

	// edit topics
	router.Handler("GET", "/assign-slideshows", curatorHs.ThenFunc(app.getFormAssignShows))
	router.Handler("POST", "/assign-slideshows", curatorHs.ThenFunc(app.postFormAssignShows))
	router.Handler("GET", "/edit-topic/:nShow/:nUser", ownerHs.ThenFunc(app.getFormTopic))

	// upload image
	router.Handler("POST", "/upload/:timestamp", publicHs.ThenFunc(app.postFormImage))

	// displays
	router.Handler("GET", "/slideshow/:nShow/:seq", dynHs.ThenFunc(app.slideshow))
	router.Handler("GET", "/contributors", authHs.ThenFunc(app.contributors))
	router.Handler("GET", "/contributor/:nUser", authHs.ThenFunc(app.contributor))
	router.Handler("GET", "/my-slideshows", authHs.ThenFunc(app.slideshowsOwn))
	router.Handler("GET", "/slideshows-user/:nUser", authHs.ThenFunc(app.slideshowsUser))
	router.Handler("GET", "/topic-user/:nShow/:nUser", dynHs.ThenFunc(app.topicUser))
	router.Handler("GET", "/topic-contributors/:nTopic", dynHs.ThenFunc(app.topicContributors))
	router.Handler("GET", "/topics", curatorHs.ThenFunc(app.topics))
	router.Handler("GET", "/usage-days", adminHs.ThenFunc(app.usageDays))
	router.Handler("GET", "/usage-months", adminHs.ThenFunc(app.usageMonths))
	router.Handler("GET", "/users-curator", curatorHs.ThenFunc(app.usersCurator))

	// user management
	router.Handler("GET", "/edit-users", adminHs.ThenFunc(app.users.GetFormEdit))
	router.Handler("POST", "/edit-users", adminHs.ThenFunc(app.users.PostFormEdit))

	// user authentication
	router.Handler("GET", "/user/login", dynHs.ThenFunc(app.users.GetFormLogin))
	router.Handler("POST", "/user/login", dynHs.Append(app.limitLogin).ThenFunc(app.users.PostFormLogin))
	router.Handler("POST", "/user/logout", authHs.ThenFunc(app.logout))
	router.Handler("GET", "/user/signup", dynHs.ThenFunc(app.users.GetFormSignup))
	router.Handler("POST", "/user/signup", dynHs.Append(app.limitLogin).ThenFunc(app.users.PostFormSignup))

	// files that must be in root
	router.GET("/apple-touch-icon.png", rootImagesHandler)
	router.GET("/favicon.ico", rootImagesHandler)
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

	// serve static files and content (Alice doesn't seem to simplify these)
	router.Handler("GET", path.Join(misc, "*filepath"), staticHs.Then(http.StripPrefix(misc, app.fileServer(fsMisc))))
	router.Handler("GET", "/static/*filepath", staticHs.Then(http.StripPrefix("/static", app.fileServer(fsStatic))))
	router.Handler("GET", "/photos/*filepath", staticHs.Then(http.StripPrefix("/photos", app.fileServer(fsPhotos))))

	// return 'standard' middleware chain followed by router
	return commonHs.Then(router)
}

// Special handling for files that must be in root

func robotsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.ServeFile(w, r, filepath.Join(UIPath, "static/robots.txt"))
}

func rootImagesHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	file := path.Base(r.URL.Path)
	http.ServeFile(w, r, filepath.Join(UIPath, "static/images", file))
}
