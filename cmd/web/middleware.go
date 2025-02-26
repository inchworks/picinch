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
	"net"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/inchworks/usage"
	"github.com/inchworks/webparts/v2/server"
	"github.com/inchworks/webparts/v2/users"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/nosurf"

	"inchworks.com/picinch/internal/models"
)

// HTTP REQUEST HANDLERS.

const (
	// Ignore user mistakes at less than one in 10 minutes. This should hold back probes to guess even easy passwords.
	mistakeRate = 10 * time.Minute

	// Ignore threats at less than one every 4 hours. Most baddies seem to try more often than this.
	threatRate = 4 * time.Hour
)

// These rate limiters are used:
// B - extended escalating ban on all accesses.
// F - file requests.
// L - login requests.
// N - non-existent file accesses.
// P - page requests.
// Q - bad query URLs.
// S - wrong shared access codes.

// authenticate returns a handler to check if this is an authenticated user or not.
// It checks any ID against the database, to see if this is still a valid user since the last login.
func (app *Application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// check for authenticated user in session
		id := app.session.GetInt64(r.Context(), "authenticatedUserID")
		if id == 0 {
			next.ServeHTTP(w, r)
			return
		}

		// check user against database
		user, err := app.userStore.Get(id)
		if errors.Is(err, models.ErrNoRecord) || user.Status < users.UserActive {
			app.session.Remove(r.Context(), "authenticatedUserID")
			next.ServeHTTP(w, r)
			return
		} else if err != nil {
			app.log(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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

// ccCache returns a handler that sets Cache-Control for a resource that may be cached by the client for all users.
// It might also be cached by an intermediate in the chain to the client browser.
func (app *Application) ccCache(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// maximum age
		w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(int(app.cfg.MaxCacheAge.Seconds())))

		// check if client's version is up to date
		if app.unmodified(r, false) {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Last-Modified", app.galleryState.lastModifiedS)
		next.ServeHTTP(w, r)
	})
}

// ccImmutable returns a handler that sets Cache-Control for a resource that never changes.
func (app *Application) ccImmutable(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Cache-Control", "immutable, max-age=31536000") // 1 year
		next.ServeHTTP(w, r)
	})
}

// ccNoCache returns a handler that sets Cache-Control for a resource that should not be cached.
// Note that the client may still have a copy, but it will be checked on every reference.
func (app *Application) ccNoCache(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Cache-Control", "no-cache")

		// check if client's version is up to date
		if app.unmodified(r, false) {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Last-Modified", app.galleryState.lastModifiedS)
		next.ServeHTTP(w, r)
	})
}

// ccNoStore returns a handler that sets Cache-Control for a resource that must not be stored by the client.
// This is for resources that are either confidential, or certain to be different next time.
func (app *Application) ccNoStore(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// maximum age
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// ccPrivate returns a handler that sets Cache-Control for a resource that may be cached by the client,
// but restricted to the current user.
func (app *Application) ccPrivateCache(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// maximum age
		w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(int(app.cfg.MaxCacheAge.Seconds()))+", private")

		// check if client's version is up to date
		if app.unmodified(r, false) {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Last-Modified", app.galleryState.lastModifiedS)
		next.ServeHTTP(w, r)
	})
}

// ccNoCache returns a handler that sets Cache-Control for a resource that should not be cached.
// Note that the client may still have a private copy, to be checked on every reference.
func (app *Application) ccPrivateNoCache(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Cache-Control", "no-cache, private")

		// check if client's version is up to date
		if app.unmodified(r, false) {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Last-Modified", app.galleryState.lastModifiedS)
		next.ServeHTTP(w, r)
	})
}

// ccSlideshow returns a handler for a resource where Cache-Control depends on the specific slideshow.
// It check if a resource might have been modified since the time specified in the request.
func (app *Application) ccSlideshow(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// lock cache and check if client's version is up to date
		if app.unmodified(r, true) {
			gs := &app.galleryState

			// cache control depends on the specific slideshow, and we should have that cached
			ps := httprouter.ParamsFromContext(r.Context())
			id, _ := strconv.ParseInt(ps.ByName("nId"), 10, 64)
			public, ok := gs.publicSlideshow[id]

			// deserialise
			gs.muCache.RUnlock()

			if ok {

				cc := "max-age=" + strconv.Itoa(int(app.cfg.MaxCacheAge.Seconds()))
				if !public {
					cc += ", private"
				}
				w.Header().Set("Cache-Control", cc)
				w.WriteHeader(http.StatusNotModified)
				return
			} else {
				app.usage.Count("unknown", "cache")
			}
		}

		// non-cached read
		w.Header().Set("Last-Modified", app.galleryState.lastModifiedS)
		next.ServeHTTP(w, r)
	})
}

// codeNotFound returns a handler that logs and rate limits HTTP requests to non-existent codes.
// Typically these are intrusion attempts.
func (app *Application) codeNotFound(next http.Handler) http.Handler {

	// allow an initial burst of 10, banned after 5 rejections
	lim := app.lhs.New("S", mistakeRate, 10, 5, "B", next)

	lim.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		http.Error(w, "Too many wrong access codes - wait an hour", http.StatusTooManyRequests)
	}))

	lim.SetReportHandler(func(r *http.Request, ip string, status string) {

		app.blocked(r, ip, status, "for bad code requests, after "+sanitise(r.RequestURI))
	})

	return lim
}

// fileServer returns a handler that serves files.
// It wraps http.File server with a limit on the number of bad requests accepted.
// (Thanks https://stackoverflow.com/questions/34017342/log-404-on-http-fileserver.)
func (app *Application) fileServer(root http.FileSystem, banBad bool, event string) http.Handler {

	fs := http.FileServer(root)

	var ban int
	if banBad {
		ban = 1 // banned after rejection
	} else {
		ban = math.MaxInt32 // never ban
	}

	// limit bad file requests to bursts of 10
	// (probably probing to guess file names, but we should allow for a few missing files that are our fault).
	lim := app.lhs.New("N", threatRate, 10, ban, "B", nil)

	lim.SetReportHandler(func(r *http.Request, ip string, status string) {

		app.blocked(r, ip, status, "for bad file names after "+sanitise(r.RequestURI))
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
		} else if event != "" {
			app.usage.Count(event, "file") // count significant file reads
		}
	})
}

// geoBlock returns a handler to block IPs for some locations.
func (app *Application) geoBlock(next http.Handler) http.Handler {

	// Use the limiter ban mechanism,
	// because that allows us to dismiss repeated requests efficiently with a single lookup.
	lh := app.lhs.New("G", threatRate, 3, 1, "B", nil)

	lh.SetReportHandler(func(r *http.Request, ip string, status string) {

		app.blocked(r, ip, status, "from blocked country, after "+sanitise(r.RequestURI))
	})

	app.geoblocker.Reporter = func(r *http.Request, location string, _ net.IP) string {

		if ip := usage.FormatIPSeen("B", r.RemoteAddr); ip != "" {
			app.usage.Seen(ip, "blocked-"+location)
		}

		// allow a few unsuccessful attempts and then switch to a general ban on the IP
		lh.Allow(r)

		return ""
	}

	return app.geoblocker.GeoBlock(next)
}

// limitFile returns a handler to limit file requests, per user.
func (app *Application) limitFile(next http.Handler) http.Handler {

	// allow 10/second with a burst of 100, banned after 100 rejects
	lh := app.lhs.New("F", 100*time.Millisecond, 100, 100, "", next)

	lh.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		http.Error(w, "Too many file requests - wait a minute", http.StatusTooManyRequests)
	}))

	lh.SetReportHandler(func(r *http.Request, ip string, status string) {

		app.blocked(r, ip, status, "file requests, too many after "+sanitise(r.RequestURI))
	})

	return lh
}

// limitLogin returns a handler to restrict user login (and signup) rates, per-user.
func (app *Application) limitLogin(next http.Handler) http.Handler {

	// allow an initial burst of 10, banned after 5 rejects (15 attempts, total)
	lh := app.lhs.New("L", mistakeRate, 10, 5, "", next)

	lh.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		http.Error(w, "Too many login requests - wait an hour", http.StatusTooManyRequests)
	}))

	lh.SetReportHandler(func(r *http.Request, ip string, status string) {

		// try to get the username
		username := "anon"
		if r.ParseForm() == nil {
			username = sanitise(r.PostForm.Get("username"))
		}

		app.blocked(r, ip, status, "login, too many for user "+username)
	})

	return lh
}

// limitPage returns a handler to limit web page requests, per user.
func (app *Application) limitPage(next http.Handler) http.Handler {

	// 1 per second with burst of 5, banned after 20 rejects,
	// (This is too restrictive to be applied to file requests.)
	lim := app.lhs.New("P", time.Second, 5, 20, "", next)

	lim.SetReportHandler(func(r *http.Request, ip string, status string) {

		app.blocked(r, ip, status, "page requests, too many after "+sanitise(r.RequestURI))
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
			if ip := usage.FormatIPSeen("V", r.RemoteAddr); ip != "" {
				app.usage.Seen(ip, "visitor-"+server.Country(r))
			}
		}

		next.ServeHTTP(w, r)
	})
}

// noBanned returns a handler to reject banned IP addresses.
func (app *Application) noBanned(next http.Handler) http.Handler {

	// no limit - but set to escalate blocking of repeated offenders.
	lh := app.lhs.NewUnlimited("B", "B", next)

	lh.SetReportHandler(func(r *http.Request, ip string, status string) {

		app.blocked(r, ip, status, "on "+sanitise(r.RequestURI))
	})

	return lh
}

// noQuery returns a handler that blocks probes with random query parameters
func (app *Application) noQuery(next http.Handler) http.Handler {

	// banned after 1 rejection
	// (typically probing for well-known PHP vulnerabilities).
	lim := app.lhs.New("Q", threatRate, 1, 0, "B", nil)

	lim.SetReportHandler(func(r *http.Request, ip string, status string) {

		app.blocked(r, ip, status, "for bad queries, after "+sanitise(r.RequestURI))
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery == "" {
			next.ServeHTTP(w, r) // ok, no query
			return
		}

		// reject queries with semicolon separators, just so that we don't log a server error
		// (see golang.org/issue/25192 - it's a mess)
		if !strings.Contains(r.URL.RawQuery, ";") {

			// allow some single query names, such as "fbclid" from Facebook
			qs := r.URL.Query()
			nOK := 0
			for q := range qs {
				for _, nm := range app.cfg.AllowedQueries {
					if q == nm {
						nOK++ // query name allowed
						break
					}
				}
				break // just the first one allowed
			}

			if nOK == len(qs) {
				next.ServeHTTP(w, r) // allowed query name
				return
			}
		}

		// limit the rate of bad queries
		ok, status := lim.Allow(r)
		if ok {
			app.threat("bad query", r)
			http.Error(w, "Query parameters not accepted", http.StatusBadRequest)
		} else {
			http.Error(w, "Intrusion attempt suspected", status)
		}
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

		next.ServeHTTP(w, r)
	})
}

// Recover from panic, set in httprouter
func (app *Application) recoverPanic() func(http.ResponseWriter, *http.Request, interface{}) {

	return func(w http.ResponseWriter, r *http.Request, err interface{}) {
		w.Header().Set("Connection", "close")
		app.log(fmt.Errorf("%s", err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// reqAuth returns a handler that checks if the user has at least the specified role, or is owner of the data requested.
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
				app.session.Put(r.Context(), "afterLogin", r.URL.Path)
				http.Redirect(w, r, "/user/login", http.StatusSeeOther)
			}
			return
		}

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

// requireMember specifies that member authentication is needed for access to this page.
func (app *Application) requireMember(next http.Handler) http.Handler {

	return app.reqAuth(models.UserMember, 0, next)
}

// requireOwner specifies that the page is for a specified member, otherwise curator authentication is needed.
func (app *Application) requireOwner(next http.Handler) http.Handler {

	return app.reqAuth(models.UserCurator, models.UserMember, next)
}

// routeNotFound returns a handler that logs and rate limits HTTP requests to non-existent routes.
// Typically these are intrusion attempts. Not called for non-existent files.
func (app *Application) routeNotFound() http.Handler {

	// burst of 3, in case it is a user mistyping, banned after 1 rejection,
	// (typically probing for vulnerable PHP files).
	lim := app.lhs.New("R", threatRate, 3, 1, "B", nil)

	lim.SetReportHandler(func(r *http.Request, ip string, status string) {

		app.blocked(r, ip, status, "for bad requests, after "+sanitise(r.RequestURI))
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// ignore some common bad requests, so we don't ban unreasonably
		d, f := path.Split(r.URL.Path)
		if (d == "/" && path.Ext(f) == ".png") || (strings.HasPrefix(f, "favicon")) {
			app.optional("no favicon", r)
			http.NotFound(w, r) // possibly a favicon for an ancient mobile device
			return
		}
		if d == "/" && strings.HasPrefix(f, "sitemap") {
			app.optional("no sitemap", r)
			http.NotFound(w, r) // some people ask for sitemap, sitemap.txt, sitemap.xml
			return
		}
		if r.URL.Path == "/.well-known/security.txt" {
			app.optional("no security.txt", r)
			http.NotFound(w, r) // not sure if there is a good reason for asking :-)
			return
		}

		ok, status := lim.Allow(r)
		if ok {
			app.threat("bad path", r)
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

// timeout specifies a time limit on request + response handling, to mitigate DoS attacks, such as R.U.D.Y.
// It must be called before any handler that extends ResponseWriter, such as a session manager.
func (app *Application) timeout(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Method == "POST" && r.URL.Path == "/upload" {
			rc := http.NewResponseController(w)

			// need more read time for file uploads
			if err := rc.SetReadDeadline(time.Now().Add(app.cfg.TimeoutUpload)); err != nil {
				app.log(err)
				httpServerError(w)
				return
			}

			// write time includes read time for HTTPS
			if err := rc.SetWriteDeadline(time.Now().Add(app.cfg.TimeoutUpload).Add(app.cfg.TimeoutWeb)); err != nil {
				app.log(err)
				httpServerError(w)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// timeoutMedia specifies a default time limit on media download requests
// It must be called before any handler that extends ResponseWriter, such as a session manager.
func (app *Application) timeoutMedia(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		rc := http.NewResponseController(w)

		// need more write time for file downloads
		if err := rc.SetWriteDeadline(time.Now().Add(app.cfg.TimeoutWeb).Add(app.cfg.TimeoutDownload)); err != nil {
			app.log(err)
			httpServerError(w)
			return
		}

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

// blocked records the blocking or banning of an IP address
func (app *Application) blocked(r *http.Request, ip string, status string, reason string) {

	if status != "" {
		loc := server.Country(r)
		if loc == "" {
			// on an early rejection we haven't located the address yet
			loc, _, _ = app.geoblocker.Locate(ip)
		}

		// report changes in status
		app.threatLog.Printf("%s %s - %s %s", loc, ip, status, reason)
	}
}

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

// optional records a legitimate request for something we don't have
func (app *Application) optional(_ string, _ *http.Request) {

	app.usage.Count("allowed", "bad-req")
}

// sanitise prevents user input from creating fake log entries
func sanitise(s string) string {

	// remove line breaks, and quote the string
	safe := strings.Replace(s, "\n", "", -1)
	safe = `"` + strings.Replace(safe, "\r", "", -1) + `"`
	return safe
}

// threat records a suspected intrusion attempt
func (app *Application) threat(event string, r *http.Request) {

	// log location and threat
	loc := server.Country(r)
	ip := server.RemoteIP(r)
	app.threatLog.Printf("%s %s - %s %s %s %s", loc, ip, event, r.Proto, r.Method, sanitise(r.URL.RequestURI()))

	// count requests
	app.usage.Count("suspect", "bad-req")

	// count suspects
	if ip := usage.FormatIPSeen("S", r.RemoteAddr); ip != "" {
		app.usage.Seen(ip, "suspect-"+server.Country(r))
	}
}

// unmodified returns true if the gallery has not been modified later than the client's version.
// Optionally the cache is locked for the caller to read more.
func (app *Application) unmodified(r *http.Request, lock bool) bool {

	// Implementation adapted from http.fs.

	if r.Method != "GET" && r.Method != "HEAD" {
		return false
	}
	ims := r.Header.Get("If-Modified-Since")
	if ims == "" {
		return false
	}
	t, err := http.ParseTime(ims)
	if err != nil {
		return false
	}

	// serialised
	gs := &app.galleryState
	gs.muCache.RLock()

	// lastModified must be specified without sub-second precision, otherwise equality comparison will usually fail.
	if ret := app.galleryState.lastModified.Compare(t); ret <= 0 {
		if !lock {
			gs.muCache.RUnlock() // nothing more from cache
		}
		app.usage.Count("not-modified", "cache")
		return true
	}
	gs.muCache.RUnlock()
	return false
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
