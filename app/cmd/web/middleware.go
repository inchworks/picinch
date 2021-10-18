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
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/inchworks/usage"
	"github.com/inchworks/webparts/users"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/nosurf"

	"inchworks.com/picinch/pkg/models"
)

// HANDLERS.

// codeNotFound returns a handler that logs and rate limits HTTP requests to non-existent codes.
// Typically these are intrusion attempts. Not called for non-existent files :-).
func (app *Application) codeNotFound() http.Handler {

	// allow 1 every 10 minutes, burst of 10, banned after 1 rejection,
	// (typically probing for vulnerable PHP files).
	lim := app.lhs.New("S", 10*time.Minute, 10, 1, "P", nil)

	lim.SetReportHandler(func(r *http.Request, addr string, status string) {

		app.threatLog.Printf("%s - %s for bad code requests, after %s", addr, status, r.RequestURI)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, status := lim.Allow(r)
		if ok {
			app.threat("bad access code", r)
			http.NotFound(w, r)
		} else {
			http.Error(w, "Intrusion attempt suspected", status)
		}
	})
}

// authenticate returns a handler to check if this is an authenticated user or not.
// It checks any ID against the database, to see if this is still a valid user since the last login.
func (app *Application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// check for authenticated user in session
		exists := app.session.Exists(r, "authenticatedUserID")
		if !exists {
			next.ServeHTTP(w, r)
			return
		}

		// check user against database
		user, err := app.userStore.Get(app.session.Get(r, "authenticatedUserID").(int64))
		if errors.Is(err, models.ErrNoRecord) || user.Status < users.UserActive {
			app.session.Remove(r, "authenticatedUserID")
			next.ServeHTTP(w, r)
			return
		} else if err != nil {
			app.serverError(w, err)
			return
		}

		// copy the request with indicator that user is authenticated
		auth := AuthenticatedUser{
			id:   user.Id,
			role: user.Role,
		}
		ctx := context.WithValue(r.Context(), contextKeyUser, auth)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// fileServer returns a handler that serves files.
// It wraps http.File server with a limit on the number of bad requests accepted.
// (Thanks https://stackoverflow.com/questions/34017342/log-404-on-http-fileserver.)
func (app *Application) fileServer(root http.FileSystem, banBad bool) http.Handler {

	fs := http.FileServer(root)

	var ban int
	if banBad {
		ban = 1  // banned after 2 rejections
	} else {
		ban = math.MaxInt32  // never ban
	}

	// limit bad file requests to one per second, burst of 5
	// (probably probing to guess file names, but we should allow for a few missing files that are our fault).
	lim := app.lhs.New("N", time.Second, 10, ban, "F,P", nil)

	lim.SetReportHandler(func(r *http.Request, addr string, status string) {

		app.threatLog.Printf("%s - %s for bad file names", addr, status)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// use a response writer that saves status
		sw := &statusWriter{ResponseWriter: w}

		// serve file request
		fs.ServeHTTP(sw, r)
		if sw.status == http.StatusNotFound {
			// Log threat. Limiter will ban user if there are too many.
			if ok, _ := lim.Allow(r); ok {
				app.threat("bad file", r)
			}
		}
	})
}

// limitFile returns a handler to limit file requests, per user.
func (app *Application) limitFile(next http.Handler) http.Handler {

	// no limit - but can be set to block all file requests after other bad requests
	lh := app.lhs.New("F", 0, 0, 20, "", next)

	lh.SetReportHandler(func(r *http.Request, addr string, status string) {

		app.threatLog.Printf("%s - %s file requests, too many after %s", addr, status, r.RequestURI)
	})

	return lh
}

// limitLogin returns a handler to restrict user login (and signup) rates, per-user.
func (app *Application) limitLogin(next http.Handler) http.Handler {

	// 1 minute per attempt, with an initial burst of 5, banned after 15 rejects (20 attempts, total)
	lh := app.lhs.New("L", time.Minute, 5, 15, "", next)

	lh.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		http.Error(w, "Too many failed attempts - wait a few minutes", http.StatusTooManyRequests)
	}))

	lh.SetReportHandler(func(r *http.Request, addr string, status string) {

		// try to get the username
		username := "unknown"
		if r.ParseForm() == nil {
			username = r.PostForm.Get("username")
		}

		app.threatLog.Printf("%s - %s login, too many for user \"%s\"", addr, status, username)
	})

	return lh
}

// limitPage returns a handler to limit web page requests, per user.
func (app *Application) limitPage(next http.Handler) http.Handler {

	// 1 per second with burst of 5, banned after 20 rejects,
	// (This is too restrictive to be applied to file requests.)
	lim := app.lhs.New("P", time.Second, 5, 20, "", next)

	lim.SetReportHandler(func(r *http.Request, addr string, status string) {

		app.threatLog.Printf("%s - %s page requests, too many after %s", addr, status, r.RequestURI)
	})

	return lim
}

// logRequest records an HTTP request.
func (app *Application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// anonymise request
		// ## could be more selective and include some IDs
		request := strings.SplitN(r.URL.RequestURI(), "/", 3)
		var req string
		if len(request) < 2 {
			req = "nil"
		} else {
			// remove any query parameter values, but keep first query name (e.g. fbclid)
			req = strings.SplitN(request[1], "=", 2)[0]
		}

		// app.infoLog.Printf("%s %s /%s", r.Proto, r.Method, request[1])

		// usage statistics
		app.usage.Count(req, "page")
		userId := app.authenticatedUser(r)
		if userId != 0 {
			app.usage.Seen(app.usage.FormatID("U", userId), "user")
		} else {
			if ip := usage.FormatIP(r.RemoteAddr); ip != "" {
				app.usage.Seen(ip, "visitor")
			}
		}

		next.ServeHTTP(w, r)
	})
}

// noQuery returns a handler that blocks probes with random query parameters
// (mainly so we don't count them as valid visitors).
func (app *Application) noQuery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			qs := r.URL.Query()
			if !qs.Has("fbclid") {
				app.threat("bad query", r)
				http.Error(w, "Query parameters not accepted", http.StatusBadRequest)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// noSurf returns a handler that implements CSRF protection,
func noSurf(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   true,
	})

	return csrfHandler
}

// pubic returns a handler that sets headers for public web pages and resources.
func (app *Application) public(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// set canical URL for search engines, if we accept more than one domain
		if len(app.cfg.Domains) > 1 {
			u := *r.URL
			u.Host = app.cfg.Domains[0] // first listed domain
			u.Scheme = "https"
			w.Header().Set("Link", `<`+u.String()+`>; rel="canonical"`)
		}

		w.Header().Set("Cache-Control", "public, max-age=600")
		next.ServeHTTP(w, r)
	})
}

// publicComp sets headers for user-specific competition pages
func (app *Application) publicComp(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// Recover from panic, set in httprouter
func (app *Application) recoverPanic() func(http.ResponseWriter, *http.Request, interface{}) {

	return func(w http.ResponseWriter, r *http.Request, err interface{}) {
		w.Header().Set("Connection", "close")
		app.serverError(w, fmt.Errorf("%s", err))
	}
}

// reqAuth returns a handler that checks if the user has at least the specified role, or is owner of the data requested.
// It also sets cache control, so should not be bypassed on any successful authentications.
// ## Is it?
func (app *Application) reqAuth(minRole int, orUser int, next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var ok bool
		if app.isAuthenticated(r, minRole) {
			ok = true

		} else if orUser > 0 && app.isAuthenticated(r, orUser) {
			// owner of this path?
			u := httprouter.ParamsFromContext(r.Context()).ByName("nUser")
			userId, _ := strconv.ParseInt(u, 10, 64)
			auth, okCast := r.Context().Value(contextKeyUser).(AuthenticatedUser)
			if okCast && auth.id == userId {
				ok = true
			}
		}

		// reject access, or ask for login 
		if !ok {
			if app.isAuthenticated(r, models.UserUnknown) {
				http.Error(w, "User is not authorised for role", http.StatusUnauthorized)

			} else {
				app.session.Put(r, "redirectPathAfterLogin", r.URL.Path)
				http.Redirect(w, r, "/user/login", http.StatusSeeOther)
			}
			return
		}

		// pages that require authentication should not be cached by browser
		w.Header().Set("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}

// requireAdmin specifies that administrator authentication is needed for access to this page.
func (app *Application) requireAdmin(next http.Handler) http.Handler {

	return app.reqAuth(models.UserAdmin, 0, next)
}


// requireAuthentication specifies that minimum authentication is needed, for access to a page,
// or to log out.
func (app *Application) requireAuthentication(next http.Handler) http.Handler {

	return app.reqAuth(models.UserFriend, 0, next)
}

// requireCurator specifies that curator authentication is needed for access to this page.
func (app *Application) requireCurator(next http.Handler) http.Handler {

	return app.reqAuth(models.UserCurator, 0, next)
}

// requireOwner specifies that the page is for a specified member, otherwise curator authentication is needed.
func (app *Application) requireOwner(next http.Handler) http.Handler {

	return app.reqAuth(models.UserCurator, models.UserMember, next)
}

// routeNotFound returns a handler that logs and rate limits HTTP requests to non-existent routes.
// Typically these are intrusion attempts. Not called for non-existent files :-).
func (app *Application) routeNotFound() http.Handler {

	// allow 1 every 10 minutes, burst of 3, banned after 1 rejection,
	// (typically probing for vulnerable PHP files).
	lim := app.lhs.New("R", 10*time.Minute, 3, 1, "F,P", nil)

	lim.SetReportHandler(func(r *http.Request, addr string, status string) {

		app.threatLog.Printf("%s - %s for bad requests, after %s", addr, status, r.RequestURI)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// ignore some common bad requests, so we don't ban unreasonably
		d, f := path.Split(r.URL.Path)
		if d =="/" && path.Ext(f) == ".png" { 
			app.threat("no favicon", r)
			http.NotFound(w, r)  // possibly a favicon for an ancient mobile device
			return

		} else if d == "/shared/" {
			app.threat("old slideshow", r)
			http.NotFound(w, r)  // ## temporary - had these on the home page, picked up by search engines
			return
		}

		ok, status := lim.Allow(r)
		if ok {
			app.threat("bad URL", r)
			http.NotFound(w, r)
		} else {
			http.Error(w, "Intrusion attempt suspected", status)
		}
	})
}

// secureHeaders adds HTTP headers for security against XSS and Clickjacking.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Frame-Options", "deny")

		next.ServeHTTP(w, r)
	})
}

// shared sets headers for shared topic and slideshows
func (app *Application) shared(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=600")
		next.ServeHTTP(w, r)
	})
}

// wwwRedirect redirects a request for the www sub-domain to the parent domain.
func wwwRedirect(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if host := strings.TrimPrefix(r.Host, "www."); host != r.Host {
			// Request host has www. prefix. Redirect to host with www. trimmed.
			u := *r.URL
			u.Host = host
			u.Scheme = "https"
			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// HELPER FUNCTIONS.

// A noDirFileSystem blocks browsing of directories.
// It avoids the need to install copies of index.html but allows index.html to be served if there is one.
// From https://www.alexedwards.net/blog/disable-http-fileserver-directory-listings.
type noDirFileSystem struct {
	http.FileSystem
}

func (nfs noDirFileSystem) Open(path string) (http.File, error) {
	fs := nfs.FileSystem

	f, err := fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if s.IsDir() {
		index := filepath.Join(path, "index.html")
		if _, err := fs.Open(index); err != nil {
			closeErr := f.Close()
			if closeErr != nil {
				return nil, closeErr
			}

			return nil, err
		}
	}

	return f, nil
}

// A statusWriter is a ResponseWriter that saves the response status.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
    w.status = status
    w.ResponseWriter.WriteHeader(status)
}

// threat records an attempted intrusion
func (app *Application) threat(event string, r *http.Request) {
	app.threatLog.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())

	rec := app.usage
	rec.Count(event, "threat")
	rec.Seen(usage.FormatIP(r.RemoteAddr), "suspect")
}
