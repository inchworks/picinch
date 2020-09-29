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
	"net/http"
	"path/filepath"
	"strings"

	"inchworks.com/picinch/pkg/limiter"
	"inchworks.com/picinch/pkg/models"
	"inchworks.com/picinch/pkg/usage"

	"github.com/justinas/nosurf"
)

// Authenticate user ID against database (i.e. still a valid user since last login)

func (app *Application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// check for authenticated user in session
		exists := app.session.Exists(r, "authenticatedUserID")
		if !exists {
			next.ServeHTTP(w, r)
			return
		}

		// check user against database
		user, err := app.UserStore.Get(app.session.Get(r, "authenticatedUserID").(int64))
		if errors.Is(err, models.ErrNoRecord) || user.Status < models.UserActive {
			app.session.Remove(r, "authenticatedUserID")
			next.ServeHTTP(w, r)
			return
		} else if err != nil {
			app.serverError(w, err)
			return
		}

		// copy the request with indicator that user is authenticated
		auth := AuthenticatedUser{
			id:     user.Id,
			status: user.Status,
		}
		ctx := context.WithValue(r.Context(), contextKeyUser, auth)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Limit login (and signup) rates
// 50s per attempt, with an initial burst of 20, banned after 10 rejects
// (Fail2Ban defaults are to jail for 10 minutes, ban after just 3 attempts within 10 minutes)

func (app *Application) limitLogin(next http.Handler) http.Handler {
	h := limiter.New("P", 0.02, 20, 10, next)

	h.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		http.Error(w, "Too many failed attempts - wait a few minutes", http.StatusTooManyRequests)
	}))

	h.SetReportHandler(func(status string, r *http.Request) {

		// try to get the username
		username := "unknown"
		if r.ParseForm() == nil {
			username = r.PostForm.Get("username")
		}

		app.threatLog.Printf("Login rate %s for %s, user \"%s\"", status, r.RemoteAddr, username)
	})

	return h
}

// Limit web request rate
// 1 per second with burst of 5, banned after 20 rejects

func (app *Application) limitWeb(next http.Handler) http.Handler {
	limitHandler := limiter.New("W", 1, 5, 20, next)

	return limitHandler
}

// Logging

func (app *Application) logNotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.threat("bad URL", r)

		http.NotFound(w, r)
	})
}

func (app *Application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// anonymise request
		// ## could be more selective and include some IDs
		request := strings.SplitN(r.URL.RequestURI(), "/", 3)
		if len(request) < 2 {
			request[1] = "nil"
		}

		// app.infoLog.Printf("%s %s /%s", r.Proto, r.Method, request[1])

		// usage statistics
		app.usage.Count(request[1], "page")
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

// File system that blocks browse access to folder. Allows index.html to be served as default.
//
// From https://www.alexedwards.net/blog/disable-http-fileserver-directory-listings

type noDirFileSystem struct {
    fs http.FileSystem
}

func (nfs noDirFileSystem) Open(path string) (http.File, error) {
    f, err := nfs.fs.Open(path)
    if err != nil {
        return nil, err
    }

    s, err := f.Stat()
    if s.IsDir() {
        index := filepath.Join(path, "index.html")
        if _, err := nfs.fs.Open(index); err != nil {
            closeErr := f.Close()
            if closeErr != nil {
                return nil, closeErr
            }

            return nil, err
        }
    }

    return f, nil
}    

// Block probes with random query parameters
// (mainly so we don't count them as valid visitors)

func (app *Application) noQuery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			app.threat("bad query", r)
			http.Error(w, "Query parameters not accepted", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CSRF protection

func noSurf(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   true,
	})

	return csrfHandler
}

// Set headers for public web pages and resources

func (app *Application) public(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// set canical URL for search engines, if we accept more than one domain
		if len(app.cfg.Domains) > 1 {
			u := *r.URL
			u.Host = app.cfg.Domains[1] // first listed domain
			u.Scheme = "https"
			w.Header().Set("Link", `<` + u.String() + `>; rel="canonical"`)
		}

		w.Header().Set("Cache-Control", "public, max-age=600")
		next.ServeHTTP(w, r)
	})
}

// Middleware handler: recover from panic
// ## Not used, as httprouter has panic handling built-in.

func (app *Application) recoverPanic0(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.serverError(w, fmt.Errorf("%s", err))
			}
		}()

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

// Require authentication for access to page

func (app *Application) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.isAuthenticated(r) {
			app.session.Put(r, "redirectPathAfterLogin", r.URL.Path)
			http.Redirect(w, r, "/user/login", http.StatusSeeOther)
			return
		}

		// pages that require authentication should not be cached by browser
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// Add HTTP headers for security against XSS and Clickjacking.

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Frame-Options", "deny")

		next.ServeHTTP(w, r)
	})
}

// Note attempted intrusion

func (app *Application) threat(event string, r *http.Request) {
	app.threatLog.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())

	rec := app.usage
	rec.Count(event, "threat")
	rec.Seen(usage.FormatIP(r.RemoteAddr), "suspect")
}

// Redirect www.domain to domain

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
