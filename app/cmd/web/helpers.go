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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	//	"github.com/justinas/nosurf"

	"inchworks.com/picinch/pkg/models"
)

// allow access/update as requested user?

func (app *Application) allowAccessUser(r *http.Request, userId int64) bool {

	auth, ok := r.Context().Value(contextKeyUser).(AuthenticatedUser)
	if !ok {
		return false
	}

	// access allowed to own data, or by curator
	return auth.id == userId || auth.status >= models.UserCurator
}

// allow update to slideshow?

func (app *Application) allowUpdateShow(r *http.Request, showId int64) bool {

	// get user for show
	s, err := app.SlideshowStore.Get(showId)
	if err != nil {
		return false
	}

	return app.allowAccessUser(r, s.User)  // owner or curator
}

// allow show to be viewed

func (app *Application) allowViewShow(r *http.Request, id int64) bool {

	// get show user and visibility
	s, err := app.SlideshowStore.Get(id)
	if err != nil {
		return false
	}

	switch s.Visible {

	case models.SlideshowPublic:
		return true // everyone

	case models.SlideshowClub:
		if app.isAuthenticated(r) { return true } // all club members

	case models.SlideshowTopic:
		// depends on topic visibility
		t, err := app.TopicStore.Get(s.Topic)
		if err != nil { return false }
	
		switch t.Visible {

		case models.SlideshowPublic:
			return true

		case models.SlideshowClub:
			if app.isAuthenticated(r) { return true } // all club members
		}
	}

	return app.allowAccessUser(r, s.User) // owner or curator
}

// allow topic to be viewed

func (app *Application) allowViewTopic(r *http.Request, id int64) bool {

	// get show user and visibility
	s, err := app.TopicStore.Get(id)
	if err != nil {
		return false
	}

	if s.Visible == models.SlideshowPublic {
		return true // everyone

	} else if s.Visible == models.SlideshowClub && app.isAuthenticated(r) {
		return true // all club members
	}

	return app.isCurator(r) // curator or admin
}

// get authenticated user ID

func (app *Application) authenticatedUser(r *http.Request) int64 {

	auth, ok := r.Context().Value(contextKeyUser).(AuthenticatedUser)
	if !ok {
		return 0
	}

	// active user?
	if auth.status >= models.UserActive {
		return auth.id
	} else {
		return 0
	}
}

// Send a specific status code and corresponding description to the user

func (app *Application) clientError(w http.ResponseWriter, status int) {

	app.galleryState.rollback = true
	http.Error(w, http.StatusText(status), status)
}

// Date in user-friendly format

func humanDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.UTC().Format("02 Jan 2006 at 15:04")
}

// copy file

func copyFile(toDir, from string) error {
	var src *os.File
	var dst *os.File
	var err error

	if src, err = os.Open(from); err != nil {
		return err
	}
	defer src.Close()

	name := filepath.Base(from)

	if dst, err = os.Create(filepath.Join(toDir, name)); err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return nil
}

// Get integer value of form parameter
// ## Not used - is it useful?

func (app *Application) intParam(w http.ResponseWriter, r *http.Request, s string) (int, bool) {

	i, err := strconv.Atoi(r.FormValue(s))
	if err != nil {
		app.log(fmt.Errorf("bad param %s : %v", s, err))
		app.clientError(w, http.StatusBadRequest)
		return 0, false
	}

	return i, true
}

// Request by admin?

func (app *Application) isAdmin(r *http.Request) bool {

	auth, ok := r.Context().Value(contextKeyUser).(AuthenticatedUser)
	if !ok {
		return false
	}
	
	return auth.status == models.UserAdmin
}

// Check if request is by an authenticated active user (saved in context from session)

func (app *Application) isAuthenticated(r *http.Request) bool {

	auth, ok := r.Context().Value(contextKeyUser).(AuthenticatedUser)
	if !ok {
		return false
	}
	return auth.status > 0
}

// Request by curator (or admin)?

func (app *Application) isCurator(r *http.Request) bool {

	auth, ok := r.Context().Value(contextKeyUser).(AuthenticatedUser)
	if !ok {
		return false
	}
	
	return auth.status >= models.UserCurator
}

// Log an error for debugging

func (app *Application) log(err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	app.errorLog.Output(2, trace)
}

// Send 404 Not Found response to the user

func (app *Application) notFound(w http.ResponseWriter) {

	app.clientError(w, http.StatusNotFound)
}

// End transaction, release mutexes, and render template from cache.
//
// Note unspecified type of template data.

func (app *Application) render(w http.ResponseWriter, r *http.Request, name string, td TemplateData) {

	if td == nil {
		td = &DataCommon{}
	}

	td.addDefaultData(app, r)

	// Retrieve the appropriate template set from the cache based on the page name
	// (like `home.page.tmpl`).
	ts, ok := app.templateCache[name]
	if !ok {
		app.serverError(w, fmt.Errorf("The template %s does not exist", name))
		return
	}

	// write template via buffer, to catch any error instead of sending a part executed page
	buf := new(bytes.Buffer)

	// Execute the template set, passing in any dynamic data.
	// Note type assertion that td will be a pointer to DataCommon at runtime.
	err := ts.Execute(buf, td)
	if err != nil {
		app.serverError(w, err)
		return
	}

	// write the buffer (pass http.ResponseWriter to a func that takes an io.Writer)
	buf.WriteTo(w)
}

// Send JSON reply
// ## Not used.

func (app *Application) reply(w http.ResponseWriter, v interface{}) {

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(err)
	}

	// ## Need to send JSON response with error, not a normal HTTP error, instead of panic
}

// Write error message and stack trace to the errorLog. If possible, send 500 Internal Server Error response to the user

func (app *Application) serverError(w http.ResponseWriter, err error) {

	app.galleryState.rollback = true

	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())

	// trace from caller
	app.errorLog.Output(2, trace)

	if w != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// Setup cached context.
//
// Creates a gallery if we don't have one.

func (a *Application) setupCache() error {

	var err error

	// start transaction
	a.tx = a.db.MustBegin()

	defer func() {
		// end transaction
		if err != nil {
			a.tx.Rollback()
		} else {
			a.tx.Commit()
		}
	}()

	// get gallery record - only one supported
	q, err := a.GalleryStore.Get(1)
	if err == models.ErrNoRecord {
		// create new gallery
		q = &models.Gallery{Id: 1}
		if err = a.GalleryStore.Update(q); err != nil {
			return err
		}

	} else if err != nil {
		return err
	}

	// save gallery ID for stores that need it
	a.SlideshowStore.GalleryId = q.Id
	a.TopicStore.GalleryId = q.Id
	a.UserStore.GalleryId = q.Id

	return a.galleryState.Init(a, q)
}

// Referring page
// ## Not used - it is more complex than this. Must recognise own pages and handle "/userId" etc.

func fromPage(path string) string {

	// remove trailing forward slash.
	if strings.HasSuffix(path, "/") {
		nLastChar := len(path) - 1
		path = path[:nLastChar]
	}
	// get final element
	els := strings.Split(path, "/")
	final := els[len(els)-1]

	if final == "" {
		return "/"
	}
	return final
}
