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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/inchworks/webparts/v2/users"

	"inchworks.com/picinch/internal/form"
	"inchworks.com/picinch/internal/models"
)

// allowAccessUser returns true if access/update to data owned by userId is allowed for the current user.
// If asCurator is true, access by curator is allowed.
func (app *Application) allowAccessUser(r *http.Request, userId int64, asCurator bool) bool {

	auth, ok := r.Context().Value(contextKeyUser).(AuthenticatedUser)
	if !ok {
		return false
	}

	// access allowed to own data, or by curator
	return auth.id == userId || (asCurator && auth.role >= models.UserCurator)
}

// allowEnterClass checks that a slideshow is a genuine competition class, available to the user, and returns the slideshow.
func (app *Application) allowEnterClass(r *http.Request, showId int64) *models.Slideshow {

	show := app.SlideshowStore.GetIf(showId)
	if show == nil {
		return nil
	}

	// check that slideshow really is a competition topic
	if show.Topic != 0 || show.Format != "C" {
		return nil
	}

	// check visibility for club or public
	if app.isAuthenticated(r, models.UserFriend) && show.Visible < models.SlideshowClub {
		return nil
	}
	if show.Visible != models.SlideshowPublic {
		return nil
	}

	return show
}

// allow update to slideshow?

func (app *Application) allowUpdateShow(r *http.Request, showId int64) bool {

	// get user for show
	s := app.SlideshowStore.GetIf(showId)
	if s == nil {
		return false
	}

	return app.allowAccessUser(r, s.User.Int64, true) // owner or curator
}

// allowViewShow returns whether the specified slideshow can be viewed by the current user,
// and whether it is a topic.
func (app *Application) allowViewShow(r *http.Request, showId int64) (canView bool, isPublic bool, isTopic bool) {

	// get show user and visibility
	s := app.SlideshowStore.GetIf(showId)
	if s == nil {
		return // no
	}

	// is this a topic
	isTopic = !s.User.Valid

	// checking Access not Visible allows viewing from cached pages
	switch s.Access {

	case models.SlideshowPublic:
		canView = true
		isPublic = true
		return // everyone

	case models.SlideshowClub:
		if app.isAuthenticated(r, models.UserFriend) {
			canView = true
			return // all club members and friends
		}
	}

	if isTopic {
		canView = app.isAuthenticated(r, models.UserCurator)
		return // curator or admin
	} else {
		canView = app.allowAccessUser(r, s.User.Int64, true)
		return // owner or curator
	}
}

// get authenticated user ID

func (app *Application) authenticatedUser(r *http.Request) int64 {

	auth, ok := r.Context().Value(contextKeyUser).(AuthenticatedUser)
	if !ok {
		return 0
	}

	// active user?
	if auth.role >= models.UserFriend {
		return auth.id
	} else {
		return 0
	}
}

// getUserIf returns the data for a user if it exists.
func (app *Application) getUserIf(id int64) *users.User {

	// This function exists to fix a mess. Most stores have a GetIf function that log database errors,
	// so that the caller needn't care why the data is missing. But I defined the webparts/users
	// package with a slightly different interface to the store :-(.

	u, err := app.userStore.Get(id)
	if err != nil && err != models.ErrNoRecord {
		app.log(err)
	}
	return u
}

// The following functions return status code and corresponding description HTTP client.
// They just make the code a bit easier to read.
// BadRequest and ServerError indicate faults with the PicInch software,
// on the client and server sides respectively, and so should be logged.
// The other errors should be detected and reported nicely when they are genuine user errors,
// but can occur from e.g. old URLs being re-requested and then a direct HTTP error is good enough.

func (app *Application) httpBadRequest(w http.ResponseWriter, err error) {

	app.log(err)
	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
}

func httpNotFound(w http.ResponseWriter) {

	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

func httpServerError(w http.ResponseWriter) {

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func httpTooLarge(w http.ResponseWriter) {

	http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
}

func httpUnauthorized(w http.ResponseWriter) {

	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

// isAuthenticated checks if the request is by an authenticated active user (saved in context from session),
// and that the user's role is sufficient.
func (app *Application) isAuthenticated(r *http.Request, minRole int) bool {

	auth, ok := r.Context().Value(contextKeyUser).(AuthenticatedUser)
	if !ok {
		return false
	}
	return auth.role >= minRole
}

// Log an error for debugging

func (app *Application) log(err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	app.errorLog.Output(2, trace)
}

// render fetches a template from the cache and writes the result as an HTTP response.
func (app *Application) render(w http.ResponseWriter, r *http.Request, name string, td TemplateData) {

	if td == nil {
		td = &DataCommon{}
	}

	td.addDefaultData(app, r, strings.SplitN(name, ".", 2)[0])

	// Retrieve the appropriate template set from the cache based on the page name
	// (like `home.page.tmpl`).
	ts, ok := app.templateCache[name]
	if !ok {
		app.log(fmt.Errorf("The template %s does not exist", name))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// write template via buffer, to catch any error instead of sending a part executed page
	buf := new(bytes.Buffer)

	// Execute the template set, passing in any dynamic data.
	// Note type assertion that td will be a pointer to DataCommon at runtime.
	err := ts.Execute(buf, td)
	if err != nil {
		app.log(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// write the buffer (pass http.ResponseWriter to a func that takes an io.Writer)
	buf.WriteTo(w)
}

// Send JSON reply.
func (app *Application) reply(w http.ResponseWriter, v interface{}) {

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(v); err != nil {
		// ## Need to send JSON response with error, not a normal HTTP error, instead of panic
		panic(err)
	}
}

// role returns the authenticated role of the active user (saved in context from session).
func (app *Application) role(r *http.Request) int {

	auth, ok := r.Context().Value(contextKeyUser).(AuthenticatedUser)
	if ok {
		return auth.role
	} else {
		return 0
	}
}

// validTypeCheck returns a function to check for acceptable file types
func (app *Application) validTypeCheck() form.ValidTypeFunc {

	return func(name string) bool {
		return app.uploader.MediaType(name) != 0
	}
}
