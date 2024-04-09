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
	"io/fs"
	"net/http"
	"path"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

// Note that for caching we're using a few different patterns.
//
// 0: Specify nothing, leaving it to the browser, because I don't know what else to do.
// Used for favicons, files with standard names that must be in root, and embedded images.
// Also used for CSS and JS, so that site customisation can overlay them without them being immutable.
//
// 1: Immutable content. One year max-age and "immutable" set.
// Used for images.
//
// 2: Mutable content with references (contributors and slideshows) that may be deleted, always server-revalidated.
// "no-store" for editing and configuration pages that are likely to be different on every access.
// "no-cache, private" on pages with access controls that don't change often.
// Also used for a user's own slideshows, so that they see updates immediately.
// "no-cache" on mutable public content: (home, contributor and topic-contributors pages).
//
// 3: Mutable content with only references to images and other slideshows.
// Configurable "max-age", default 1 hour, for slideshows. "private" for slideshows with access controls.
// Needs care when referenced content (that might not be cached) is deleted.
// - Image deletions are deferred for longer than the page max-age.
// - "from", "prev" and "next" could be incorrect. Missing pages (for slideshow, contributor 
// and topic-contributors) are reported politely.
//
// For patterns 0, 2 and 3, "If-Modified-Since" is checked on requests, and a StatusNotModified header
// returned if there have been no changes to the gallery.

// ## For content with significant confidentiality, "no-store" should be used instead of "no-cache, private",
// ## so that the browser cache stays clean, but we don't currently have any such content.

// Register handlers for routes

func (app *Application) Routes() http.Handler {

	commonHs := alice.New(secureHeaders, app.noBanned, app.geoBlock, app.noQuery, wwwRedirect)
	dynHs := alice.New(app.limitPage, app.session.Enable, noSurf, app.authenticate, app.logRequest) // dynamic page handlers
	staticHs := alice.New(app.limitFile)

	// access to page
	adminHs := dynHs.Append(app.requireAdmin)
	authHs := dynHs.Append(app.requireAuthentication) // friend authenticated, may be further restriction by application logic
	curatorHs := dynHs.Append(app.requireCurator)
	ownerHs := dynHs.Append(app.requireOwner)
	
	sharedHs := dynHs.Append(app.ccCache)

	// cache-control settings
	adminCacheHs := adminHs.Append(app.ccPrivateCache)
	adminNoStoreHs := adminHs.Append(app.ccNoStore)
	authNoCacheHs := authHs.Append(app.ccPrivateNoCache)
	authNoStoreHs := authHs.Append(app.ccNoStore)
	compNoStoreHs := dynHs.Append(app.ccNoStore)
	curatorNoCacheHs := curatorHs.Append(app.ccPrivateNoCache)
	curatorNoStoreHs := curatorHs.Append(app.ccNoStore)
	ownerNoCacheHs := ownerHs.Append(app.ccPrivateNoCache)
	ownerNoStoreHs := ownerHs.Append(app.ccNoStore)
	publicCacheHs := dynHs.Append(app.public, app.ccCache)
	publicNoCacheHs := dynHs.Append(app.public, app.ccNoCache)
	specificHs := dynHs.Append(app.public, app.ccSpecific)

	// files
	immutableHs := staticHs.Append(app.ccImmutable)

	// HttpRouter wrapped to allow middleware handlers
	router := httprouter.New()

	// panic handler
	router.PanicHandler = app.recoverPanic()

	// log rejected routes
	router.NotFound = app.routeNotFound()

	// public pages
	router.Handler("GET", "/", publicNoCacheHs.ThenFunc(app.home))
	router.Handler("GET", "/info/:page", publicCacheHs.ThenFunc(app.info))

	// public competition
	if app.cfg.Options == "main-comp" {
		router.Handler("GET", "/classes", publicCacheHs.ThenFunc(app.classes))
		router.Handler("GET", "/enter-comp/:nClass", publicCacheHs.ThenFunc(app.getFormEnterComp))
		router.Handler("POST", "/enter-comp", dynHs.ThenFunc(app.postFormEnterComp))
		router.Handler("GET", "/validate/:code", compNoStoreHs.ThenFunc(app.validate))
	}

	// pages shared with an access code
	router.Handler("GET", "/shared/:code/:seq", sharedHs.ThenFunc(app.slideshowShared))

	// embedding
	router.Handler("GET", "/highlight/:prefix/:nImage", publicCacheHs.ThenFunc(app.highlight))
	router.Handler("GET", "/highlights/:nImages", publicCacheHs.ThenFunc(app.highlights))

	// setup
	router.Handler("GET", "/setup", adminNoStoreHs.ThenFunc(app.getFormGallery))
	router.Handler("POST", "/setup", adminHs.ThenFunc(app.postFormGallery))
	router.Handler("GET", "/edit-tags", adminNoStoreHs.ThenFunc(app.getFormTags))
	router.Handler("POST", "/edit-tags", adminHs.ThenFunc(app.postFormTags))

	router.Handler("GET", "/edit-topics", curatorNoStoreHs.ThenFunc(app.getFormTopics))
	router.Handler("POST", "/edit-topics", curatorHs.ThenFunc(app.postFormTopics))

	// edit slideshows
	router.Handler("GET", "/edit-slides/:nShow", authNoStoreHs.ThenFunc(app.getFormSlides))
	router.Handler("POST", "/edit-slides", authHs.ThenFunc(app.postFormSlides))
	router.Handler("GET", "/edit-slideshows/:nUser", ownerNoStoreHs.ThenFunc(app.getFormSlideshows))
	router.Handler("POST", "/edit-slideshows/:nUser", ownerHs.ThenFunc(app.postFormSlideshows))

	// edit topics
	router.Handler("GET", "/assign-slideshows", curatorNoStoreHs.ThenFunc(app.getFormAssignShows))
	router.Handler("POST", "/assign-slideshows", curatorHs.ThenFunc(app.postFormAssignShows))
	router.Handler("GET", "/edit-topic/:nShow/:nUser", ownerNoStoreHs.Append(app.ccNoStore).ThenFunc(app.getFormTopic))

	// upload media files
	router.Handler("POST", "/upload", dynHs.ThenFunc(app.postFormMedia))

	// displays
	router.Handler("GET", "/slideshow/:nShow/:seq", specificHs.ThenFunc(app.slideshow))
	router.Handler("GET", "/contributors", authNoCacheHs.ThenFunc(app.contributors))
	router.Handler("GET", "/contributor/:nUser", authNoCacheHs.ThenFunc(app.contributor))
	router.Handler("GET", "/entry/:nShow", authNoCacheHs.ThenFunc(app.entry))
	router.Handler("GET", "/my-slideshow/:nShow/:seq", ownerNoCacheHs.ThenFunc(app.slideshow))
	router.Handler("GET", "/my-slideshows", ownerNoCacheHs.ThenFunc(app.slideshowsOwn))
	router.Handler("GET", "/slideshows-user/:nUser", curatorNoCacheHs.ThenFunc(app.slideshowsUser))
	router.Handler("GET", "/topic-user/:nShow/:nUser", publicNoCacheHs.ThenFunc(app.topicUser))
	router.Handler("GET", "/topic-contributors/:nTopic", publicNoCacheHs.ThenFunc(app.topicContributors))
	router.Handler("GET", "/topics", curatorNoStoreHs.ThenFunc(app.topics))
	router.Handler("GET", "/usage-days", adminCacheHs.ThenFunc(app.usageDays))
	router.Handler("GET", "/usage-months", adminCacheHs.ThenFunc(app.usageMonths))
	router.Handler("GET", "/users-curator", curatorNoCacheHs.ThenFunc(app.usersCurator))

	// selections
	router.Handler("GET", "/select-slideshow", authNoStoreHs.ThenFunc(app.getFormSelectSlideshow))
	router.Handler("POST", "/select-slideshow", authHs.ThenFunc(app.postFormSelectSlideshow))
	router.Handler("GET", "/slideshows-tagged/:nTopic/:nRoot/:nTag/:nUser/:nMax", authNoStoreHs.ThenFunc(app.slideshowsTagged))
	router.Handler("GET", "/user-tags", authNoCacheHs.ThenFunc(app.userTags))

	// set tags
	router.Handler("GET", "/tag-slideshow/:nShow/:nRoot/:nUser", authNoStoreHs.ThenFunc(app.getFormTagSlideshow))
	router.Handler("POST", "/tag-slideshow", authHs.ThenFunc(app.postFormTagSlideshow))

	// user management
	router.Handler("GET", "/edit-users", adminNoStoreHs.ThenFunc(app.users.GetFormEdit))
	router.Handler("POST", "/edit-users", adminHs.ThenFunc(app.users.PostFormEdit))

	// user authentication
	router.Handler("GET", "/user/login", publicCacheHs.ThenFunc(app.users.GetFormLogin))
	router.Handler("POST", "/user/login", dynHs.Append(app.limitLogin).ThenFunc(app.users.PostFormLogin))
	router.Handler("POST", "/user/logout", authHs.ThenFunc(app.logout))
	router.Handler("GET", "/user/signup", publicCacheHs.ThenFunc(app.users.GetFormSignup))
	router.Handler("POST", "/user/signup", dynHs.Append(app.limitLogin).ThenFunc(app.users.PostFormSignup))

	// these are just a courtesy, say no immediately instead of redirecting to "/path/" first
	misc := path.Join("/", app.cfg.MiscName)
	router.Handler("GET", misc, http.NotFoundHandler())
	router.Handler("GET", "/photos", http.NotFoundHandler())
	router.Handler("GET", "/static", http.NotFoundHandler())

	// file systems that block directory listing
	fsStatic := noDirFileSystem{http.FS(app.staticFS)}
	fsPhotos := noDirFileSystem{http.Dir(ImagePath)}
	fsMisc := noDirFileSystem{http.Dir(MiscPath)}

	// serve static files and content (Alice doesn't seem to simplify these)
	ban := app.cfg.BanBadFiles 
	router.Handler("GET", path.Join(misc, "*filepath"), staticHs.Then(http.StripPrefix(misc, app.fileServer(fsMisc, true, app.cfg.MiscName))))
	router.Handler("GET", "/static/*filepath", staticHs.Then(http.StripPrefix("/static", app.fileServer(fsStatic, ban, ""))))
	router.Handler("GET", "/photos/*filepath", immutableHs.Then(http.StripPrefix("/photos", app.fileServer(fsPhotos, ban, ""))))

	// files that must be in root
	fsImages, _ := fs.Sub(app.staticFS, "images")
	fsRoot := http.FS(fsImages)
	router.Handler("GET", "/robots.txt", staticHs.Then(app.fileServer(fsStatic, false, "")))
	router.Handler("GET", "/apple-touch-icon.png", staticHs.Then(app.fileServer(fsRoot, false, "")))
	router.Handler("GET", "/favicon.ico", staticHs.Then(app.fileServer(fsRoot, false, "")))
	
	// return 'standard' middleware chain followed by router
	return commonHs.Then(router)
}
