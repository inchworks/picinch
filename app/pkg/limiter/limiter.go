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

package limiter

// Rate limiter for HTTP requests.
//
// Based on https://www.alexedwards.net/blog/how-to-rate-limit-http-requests.
// Interface model loosely from https://github.com/justinas/nosurf.

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ## Suggested changes
// o sync.RWMutex for reduced contention on map

type LimitHandler struct {

	// handlers wrapped
	success http.Handler
	failure http.Handler
	report func(string, *http.Request)

	// parameters
	name  string
	rate  rate.Limit  // max. requests per second
	burst int  // allowed burst
	ban int  // rejects until banned

	// internal data
	visitors map[string]*visitor
}

// rate limiter for each visitor
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
	rejects  int
}

var visitors = make(map[string]*visitor)
var mu sync.Mutex

// background goroutine to remove old entries from the visitors map
func Init(cleanup time.Duration) {
	go cleanupVisitors(cleanup)
}

// Construct a new LimitHandler that calls the next handler if specified rate is acceptable

func New(n string, r rate.Limit, b int, ban int, next http.Handler) *LimitHandler {

	return &LimitHandler{
		name:     n,
		rate:     r,
		burst:    b,
		ban:	ban,
		success:  next,
		failure:  http.HandlerFunc(defaultFailureHandler),
		report:  defaultReportHandler,
		visitors: visitors,
	}
}

// Limit request rate

func (h *LimitHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// visitor address
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// limiter for this limit and visitor
	id := h.name + ip
	lim, banned := h.getVisitor(id)
	if banned || lim.Allow() == false {

		// count rejections and report first one
		s := h.setReject(id)
		if s != "" {
			h.report(s, r)
		} 

		h.failure.ServeHTTP(w, r)
		return
	}

	h.success.ServeHTTP(w, r)
}

// Set handler to call when rate limit exceeded

func (h *LimitHandler) SetFailureHandler(handler http.Handler) {
	h.failure = handler
}

// Set handler to report rate limiting

func (h *LimitHandler) SetReportHandler(handler func(status string, r *http.Request)) {
	h.report = handler
}

// check the map for visitors that haven't been seen for more than 3 intervals and delete the entries
// ## parameterise by limit

func cleanupVisitors(d time.Duration) {
	for {
		time.Sleep(d)

		mu.Lock()
		for id, v := range visitors {
			if time.Since(v.lastSeen) > d*3 {
				delete(visitors, id)
			}
		}
		mu.Unlock()
	}
}

// default handler for failures

func defaultFailureHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(429), http.StatusTooManyRequests)
}

// default handle to log rejections

func defaultReportHandler(string, *http.Request) {}

// get rate limiter for visitor

func (h *LimitHandler) getVisitor(id string) (*rate.Limiter, bool) {
	mu.Lock()
	defer mu.Unlock()

	v, exists := visitors[id]
	if !exists {

		// limiter for new visitor
		limiter := rate.NewLimiter(h.rate, h.burst)
		visitors[id] = &visitor{limiter, time.Now(), 0}
		return limiter, false
	}

	// last seen time for the visitor
	v.lastSeen = time.Now()

	return v.limiter, v.rejects > h.ban
}

// count rejections for visitor

func (h *LimitHandler) setReject(id string) (status string) {

	mu.Lock()
	defer mu.Unlock()

	v, exists := visitors[id]
	if !exists {
		return "error"  // should never happen - they were there a moment ago!
	}

	v.rejects += 1
	if v.rejects == 1 {
		status = "rejection"  // limit reached for first time
	} else if v.rejects == h.ban {
		status = "ban"  // ban limit reached for first time
	}

	return status
}