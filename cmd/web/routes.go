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

// For caching we're using a few different patterns.
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

// There are many similar slideshow paths. They have been determined by these intentions:
// - Make each path meaningful for user’s intention, not what is delivered.
// - Path is specific to the return path when the slideshow ends.
// - User/topic preferred to topic/section so that user might have multiple contributions in future.
// - Topic ID included where a segment could have moved to another topic.
// - User and curator must access sections by topic not ID, since there needn’t be a contribution.
// - Highlights is a regular topic with a special section.
//   In future, we could have multiple highlight topics, or a highlights view of a normal topic.
// - Multiple sections could be used to construct a large show, in future.

// Register handlers for routes

func (app *Application) Routes() http.Handler {

	commonHs := alice.New(secureHeaders, app.noBanned, app.geoBlock, app.noQuery, wwwRedirect)
	dynHs := alice.New(app.timeout, app.limitPage, app.session.LoadAndSave, noSurf, app.authenticate, app.logRequest) // dynamic page handlers
	immutableHs := alice.New(app.timeoutMedia, app.limitFile, app.ccImmutable)
	staticHs := alice.New(app.timeout, app.limitFile)

	// access to page
	adminHs := dynHs.Append(app.requireAdmin)
	authHs := dynHs.Append(app.requireAuthentication) // friend authenticated, may be further restriction by application logic
	curatorHs := dynHs.Append(app.requireCurator)
	memberHs := dynHs.Append(app.requireMember)
	ownerHs := dynHs.Append(app.requireOwner) // checks :nUser in path

	sharedHs := dynHs.Append(app.ccCache)

	// cache-control settings
	adminNoCacheHs := adminHs.Append(app.ccNoCache)
	adminNoStoreHs := adminHs.Append(app.ccNoStore)
	authCacheHs := authHs.Append(app.ccPrivateCache)
	authNoCacheHs := authHs.Append(app.ccPrivateNoCache)
	authNoStoreHs := authHs.Append(app.ccNoStore)
	compNoStoreHs := dynHs.Append(app.ccNoStore)
	curatorNoCacheHs := curatorHs.Append(app.ccPrivateNoCache)
	curatorNoStoreHs := curatorHs.Append(app.ccNoStore)
	memberNoCacheHs := memberHs.Append(app.ccNoCache)
	memberNoStoreHs := memberHs.Append(app.ccNoStore)
	ownerNoStoreHs := ownerHs.Append(app.ccNoStore)
	publicCacheHs := dynHs.Append(app.public, app.ccCache)
	publicNoCacheHs := dynHs.Append(app.public, app.ccNoCache)
	publicNoStoreHs := dynHs.Append(app.public, app.ccNoStore)

	slideshowHs := dynHs.Append(app.public, app.ccSlideshow) // caching varies with slideshow

	// HttpRouter wrapped to allow middleware handlers
	router := httprouter.New()

	// panic handler
	router.PanicHandler = app.recoverPanic()

	// log rejected routes
	router.NotFound = app.routeNotFound()

	// public pages
	router.Handler("GET", "/", publicCacheHs.ThenFunc(app.homePublic))
	router.Handler("GET", "/contributor/:nUser", publicCacheHs.ThenFunc(app.contributorPublic))
	router.Handler("GET", "/contributors", publicCacheHs.ThenFunc(app.contributorsPublic))
	router.Handler("GET", "/diary/:page", publicCacheHs.ThenFunc(app.diary))
	router.Handler("GET", "/info/:page", publicCacheHs.ThenFunc(app.info))
	router.Handler("GET", "/msg", publicNoStoreHs.ThenFunc(app.homePublic))

	// public competition
	if app.cfg.Options == "main-comp" {
		router.Handler("GET", "/classes", publicCacheHs.ThenFunc(app.classes))
		router.Handler("GET", "/enter-comp/:nClass", publicCacheHs.ThenFunc(app.getFormEnterComp))
		router.Handler("POST", "/enter-comp", dynHs.ThenFunc(app.postFormEnterComp))
		router.Handler("GET", "/validate/:code", compNoStoreHs.ThenFunc(app.validate))
	}

	// pages shared with an access code
	router.Handler("GET", "/shared/:code", sharedHs.ThenFunc(app.sharedSlideshow))
	router.Handler("GET", "/shared-topic/:code", sharedHs.ThenFunc(app.sharedTopic))
	router.Handler("GET", "/shared-slides/:code/:nSec", sharedHs.ThenFunc(app.sharedSlides))

	// embedding
	router.Handler("GET", "/highlight/:prefix/:nImage", publicCacheHs.ThenFunc(app.embedded))
	router.Handler("GET", "/highlights/:nImages", publicCacheHs.ThenFunc(app.embeddedImages))

	// setup
	router.Handler("GET", "/setup", adminNoStoreHs.ThenFunc(app.getFormGallery))
	router.Handler("POST", "/setup", adminHs.ThenFunc(app.postFormGallery))
	router.Handler("GET", "/edit-tags", adminNoStoreHs.ThenFunc(app.getFormTags))
	router.Handler("POST", "/edit-tags", adminHs.ThenFunc(app.postFormTags))

	router.Handler("GET", "/edit-diary", curatorNoStoreHs.ThenFunc(app.getFormDiary))
	router.Handler("POST", "/edit-diary", curatorHs.ThenFunc(app.postFormDiary))
	router.Handler("GET", "/edit-topics", curatorNoStoreHs.ThenFunc(app.getFormTopics))
	router.Handler("POST", "/edit-topics", curatorHs.ThenFunc(app.postFormTopics))

	// edit slideshows
	router.Handler("GET", "/edit-slides/:nId", authNoStoreHs.ThenFunc(app.getFormSlides))
	router.Handler("POST", "/edit-slides", authHs.ThenFunc(app.postFormSlides))
	router.Handler("GET", "/edit-slideshows/:nUser", ownerNoStoreHs.ThenFunc(app.getFormSlideshows))
	router.Handler("POST", "/edit-slideshows/:nUser", ownerHs.ThenFunc(app.postFormSlideshows))

	// edit topics
	router.Handler("GET", "/assign-slideshows", curatorNoStoreHs.ThenFunc(app.getFormAssignShows))
	router.Handler("POST", "/assign-slideshows", curatorHs.ThenFunc(app.postFormAssignShows))
	router.Handler("GET", "/edit-topic/:nId/:nUser", ownerNoStoreHs.ThenFunc(app.getFormTopic))

	// upload media files
	router.Handler("POST", "/upload", dynHs.ThenFunc(app.postFormMedia))

	// displays - general
	router.Handler("GET", "/contrib-members", authNoCacheHs.ThenFunc(app.contributorsMembers))
	router.Handler("GET", "/contrib-member/:nUser", authNoCacheHs.ThenFunc(app.contributorMembers))
	router.Handler("GET", "/entry/:nId", authNoCacheHs.ThenFunc(app.entry))
	router.Handler("GET", "/members", authCacheHs.ThenFunc(app.homeMembers)) // home page for members
	router.Handler("GET", "/members-msg", authNoStoreHs.ThenFunc(app.homeMembers))
	router.Handler("GET", "/my-slideshows", memberNoCacheHs.ThenFunc(app.slideshowsOwn))
	router.Handler("GET", "/my-slideshows-msg", memberNoStoreHs.ThenFunc(app.slideshowsOwn))
	router.Handler("GET", "/next", authNoStoreHs.ThenFunc(app.next))
	router.Handler("GET", "/slideshows-user/:nUser", curatorNoCacheHs.ThenFunc(app.slideshowsUser))
	router.Handler("GET", "/topic-contributors/:nId", slideshowHs.ThenFunc(app.topicContributors))
	router.Handler("GET", "/topics", curatorNoCacheHs.ThenFunc(app.topics))
	router.Handler("GET", "/usage-days", adminNoCacheHs.ThenFunc(app.usageDays))
	router.Handler("GET", "/usage-months", adminNoCacheHs.ThenFunc(app.usageMonths))
	router.Handler("GET", "/users-curator", curatorNoCacheHs.ThenFunc(app.usersCurator))

	// slideshows
	router.Handler("GET", "/show/:nId", slideshowHs.ThenFunc(app.slideshow))
	router.Handler("GET", "/slides/:nId/:nSec", slideshowHs.ThenFunc(app.slides))
	router.Handler("GET", "/topic/:nId", slideshowHs.ThenFunc(app.topic))
	router.Handler("GET", "/for-show/:nId", slideshowHs.ThenFunc(app.forShow))
	router.Handler("GET", "/for-topic/:nUser/:nId", slideshowHs.ThenFunc(app.forTopic))
	router.Handler("GET", "/topic-user/:nId/:nUser", slideshowHs.ThenFunc(app.topicUser))
	router.Handler("GET", "/my-show/:nId", memberNoCacheHs.ThenFunc(app.ownShow))
	router.Handler("GET", "/my-topic/:nId", memberNoCacheHs.ThenFunc(app.ownTopic))
	router.Handler("GET", "/hilites/:nId", slideshowHs.ThenFunc(app.highlights))
	router.Handler("GET", "/user-show/:nUser/:nId", authNoCacheHs.ThenFunc(app.userShow))
	router.Handler("GET", "/user-topic/:nUser/:nId", authNoCacheHs.ThenFunc(app.userTopic))
	router.Handler("GET", "/rev-hilites/:nId", authNoCacheHs.ThenFunc(app.reviewHighlights))
	router.Handler("GET", "/rev-slides/:nId/:nSec", authNoCacheHs.ThenFunc(app.reviewSlides))
	router.Handler("GET", "/rev-topic/:nId", authNoCacheHs.ThenFunc(app.reviewTopic))

	// redirect
	router.Handler("GET", "/slideshow/:nId/:nSeq", publicNoCacheHs.ThenFunc(app.slideshowOld))

	// selections
	router.Handler("GET", "/select-slideshow", authNoCacheHs.ThenFunc(app.getFormSelectSlideshow))
	router.Handler("POST", "/select-slideshow", authHs.ThenFunc(app.postFormSelectSlideshow))
	router.Handler("GET", "/slideshows-tagged/:nId/:nRoot/:nTag/:nUser/:nMax", authNoStoreHs.ThenFunc(app.slideshowsTagged))
	router.Handler("GET", "/user-tags", authNoCacheHs.ThenFunc(app.userTags))

	// set tags
	router.Handler("GET", "/tag-slideshow/:nId/:nRoot/:nUser", authNoStoreHs.ThenFunc(app.getFormTagSlideshow))
	router.Handler("POST", "/tag-slideshow", authHs.ThenFunc(app.postFormTagSlideshow))

	// user management
	router.Handler("GET", "/edit-users", adminNoStoreHs.ThenFunc(app.users.GetFormEdit))
	router.Handler("POST", "/edit-users", adminHs.ThenFunc(app.users.PostFormEdit))

	// user authentication
	router.Handler("GET", "/user-login", publicCacheHs.ThenFunc(app.users.GetFormLogin))
	router.Handler("GET", "/user/login", publicNoStoreHs.ThenFunc(app.users.GetFormLogin)) // with flash
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
	router.Handler("GET", path.Join(misc, "*filepath"), staticHs.Then(http.StripPrefix(misc, app.fileServer(fsMisc, false, app.cfg.MiscName))))
	router.Handler("GET", "/static/*filepath", staticHs.Then(http.StripPrefix("/static", app.fileServer(fsStatic, true, ""))))
	router.Handler("GET", "/photos/*filepath", immutableHs.Then(http.StripPrefix("/photos", app.fileServer(fsPhotos, app.cfg.BanBadFiles, ""))))

	// files that must be in root
	fsImages, _ := fs.Sub(app.staticFS, "images")
	fsRoot := http.FS(fsImages)
	router.Handler("GET", "/robots.txt", staticHs.Then(app.fileServer(fsStatic, false, "")))
	router.Handler("GET", "/apple-touch-icon.png", staticHs.Then(app.fileServer(fsRoot, false, "")))
	router.Handler("GET", "/favicon.ico", staticHs.Then(app.fileServer(fsRoot, false, "")))

	// return 'standard' middleware chain followed by router
	return commonHs.Then(router)
}
