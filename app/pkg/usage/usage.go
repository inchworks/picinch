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

package usage

// Website usage statistics
//
// Note:
//  o Time periods are UTC.

import (
	"math/rand"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	tickInterval = time.Minute      // determines accuracy of periods
	saveInterval = time.Minute * 10 // determines data loss on server crash
)

// Period values

const (
	Seen  = 0
	Base  = 1
	Day   = 2
	Month = 3
	Year  = 4 // reserved, not used
	Mark  = 5
)

type Statistic struct {
	Id       int64
	Event    string
	Category string
	Count    int
	Start    time.Time
	Period   int
}

// User must implement this interface for storage and update of usage statistics

type StatisticStore interface {
	BeforeByCategory(before time.Time, period int) []*Statistic // ordered by category and time
	BeforeByEvent(before time.Time, period int) []*Statistic    // ordered by event and time
	BeforeByTime(before time.Time, period int) []*Statistic     // ordered by time and perferred display order (e.g. count descending)
	DeleteId(id int64) error
	DeleteIf(before time.Time, period int) error
	GetEvent(event string, start time.Time, period int) *Statistic
	GetMark(event string) *Statistic
	Transaction() func()
	Update(s *Statistic) error
}

// Usage recorder

type Recorder struct {
	store StatisticStore

	// parameters
	basePeriod  int           // Base or Day
	base        time.Duration // e.g. an hour or a day
	keepBase    int           // in days
	keepDays    int           // in days
	keepDetails int           // in months
	keepMonths  int           // in months

	chSaver *time.Ticker
	chDone  chan bool

	// operation times
	periodStart time.Time
	now         time.Time // (set on each tick, for constency and to allow accelerated testing)
	volatileEnd time.Time
	periodEnd   time.Time
	dayEnd      time.Time

	// volatile stats
	mu       sync.Mutex
	count    map[string]int
	seen     map[string]bool
	category map[string]string
	salt     uint32 // used to anonymise IDs such as user IDs
}

// Start recorder

func New(st StatisticStore, base time.Duration, baseDays int, days int, detailMonths int, months int) (*Recorder, error) {

	// override silly parameters with defaults
	if base < time.Hour || base > time.Hour*24 {
		base = time.Hour * 24
	}
	if baseDays < 1 {
		baseDays = 1
	}
	if days < 1 {
		days = 7
	}
	if detailMonths < 1 {
		detailMonths = 3
	}
	if months < 1 {
		months = 24
	}

	r := &Recorder{
		store:       st,
		base:        base,
		keepBase:    baseDays,
		keepDays:    days,
		keepDetails: detailMonths,
		keepMonths:  months,

		now: time.Now().UTC(),

		chSaver: time.NewTicker(tickInterval),
		chDone:  make(chan bool, 1),

		count:    make(map[string]int),
		seen:     make(map[string]bool),
		category: make(map[string]string),
		salt:     rand.Uint32(),
	}

	if r.base != time.Hour*24 {
		r.basePeriod = Base
	} else {
		r.basePeriod = Day
	}

	// start of statistics recording
	s := st.GetMark("goLive")
	if s == nil {
		s = &Statistic{
			Event:    "goLive",
			Category: "timeline",
			Start:    r.now,
			Period:   Mark,
		}

		defer st.Transaction()()
		if err := st.Update(s); err != nil {
			return nil, err
		}
	}

	// next operations
	r.periodStart = r.start(base)
	r.volatileEnd = r.next(saveInterval)
	r.periodEnd = r.next(base)
	r.dayEnd = r.start(time.Hour * 24)

	// start saver
	go r.saver(r.chSaver.C, r.chDone)

	return r, nil
}

// Count event

func (r *Recorder) Count(event string, category string) {

	if event == "" {
		event = "#"
	} // something searchable for home page

	r.mu.Lock()
	defer r.mu.Unlock()

	r.count[event]++             // count events
	r.category[event] = category // ## note aggregate - inefficient
}

// Format ID for recording, anonymised
// Note that the salt is changed daily, so anonymisation is genuine.
// Needs AES-128 encryption to prevent reversal using a known ID for the day, but the
// extra cost isn't worth it for this purpose.
//
// ## The salt is changed on restart, so IDs after a restart will be seen as different.
// ## Could save/erase the salt across a restart - but not with the stats records!

func (r *Recorder) FormatID(prefix string, id int64) string {

	// reduce ID to 32 bits, to keep resulting string small, and add salt
	id32 := uint32(id) ^ uint32(id>>32) ^ r.salt

	return prefix + strconv.FormatUint(uint64(id32), 36)
}

// Format IP address for recording, anonymised
//
// This is more complex than I hoped, because the format of the IP address is "IP:port"
// and we might not have a valid address.

func FormatIP(addr string) string {

	ipStr, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}

	// convert IP address to slice of 16 bytes
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	// attempt to reduce it to 4 bytes
	ip4 := ip.To4()

	// anonymise address
	// (ICANN method 4, https://www.icann.org/en/system/files/files/rssac-040-07aug18-en.pdf)
	if ip4 != nil {
		ip4[3] = 0
		ipStr = ip4.String()

	} else if len(ip) == 16 {
		for i := 6; i < 16; i++ {
			ip[i] = 0
		}
		ipStr = ip.String()

	} else {
		return "" // unknown - cannot anonymise
	}

	return ipStr
}

// Get statistics for days or months

func Get(st StatisticStore, period int) [][]*Statistic {

	sPeriods := make([][]*Statistic, 0, 16)
	var stats []*Statistic
	var before time.Time

	// rollup lower level stats, and split into start periods
	switch period {
	case Month:
		// days into months
		// ## we're missing the base periods
		statsDays := st.BeforeByEvent(time.Now().UTC(), period-1)
		stats, before = getRollup(st, statsDays, forMonth, Month)
		sPeriods = split(sPeriods, stats)

	case Day:
		// base periods into days
		statsBase := st.BeforeByEvent(time.Now().UTC(), period-1)
		stats, before = getRollup(st, statsBase, forDay, Day)
		sPeriods = split(sPeriods, stats)

	default:
		before = time.Now().UTC()
	}

	// add in the remaining periods, for which rollup wasn't needed
	stats = st.BeforeByTime(before, period)
	sPeriods = split(sPeriods, stats)

	// replace seen counts by daily average
	if period == Month {
		average(st, sPeriods)
	}

	return sPeriods
}

// Mark event

func (r *Recorder) Mark(event string, category string) error {

	s := &Statistic{
		Event:    event,
		Category: category,
		Start:    time.Now().UTC(),
		Period:   Mark,
	}

	defer r.store.Transaction()()
	return r.store.Update(s)
}

// Count distinct events seen (e.g. visitors)

func (r *Recorder) Seen(event string, category string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seen[event] = true         // event occured
	r.category[event] = category // ## note aggregate - inefficient
}

// Stop recording

func (r *Recorder) Stop() {

	r.chSaver.Stop()
	r.chDone <- true
}

// Aggregate detail or seen events into parent category

func (r *Recorder) aggregate(stats []*Statistic, newCategory string, period int) {

	st := r.store

	var category string
	var start time.Time
	var newEvent string
	var next bool
	var total *Statistic

	for _, s := range stats {

		// exclude events that have already been aggregated
		// i.e. distinct events, already aggregated, and lower level aggregates
		if s.Event[:1] != "<" {

			count := s.Count
			if count == 0 {
				count = 1 // not set for seen events
			}

			// aggregate events for same category and time
			if category != s.Category {
				category = s.Category
				newEvent = "<" + s.Category + ">"
				next = true
			}

			if start != s.Start {
				start = s.Start
				next = true
			}

			if next {
				// save previous total
				if total != nil {
					st.Update(total)
				}

				// next total
				total = st.GetEvent(newEvent, start, Day)
				if total == nil {
					// new total
					total = &Statistic{
						Event:    newEvent,
						Category: newCategory,
						Start:    start,
						Period:   period,
					}
				}
				next = false
			}

			// add event to total and drop event
			total.Count += count
			st.DeleteId(s.Id)
		}
	}
	if total != nil {
		st.Update(total) // save final total
	}
}

// Aggregate details into categories

func (r *Recorder) aggregateDetails(before time.Time, period int) {

	stats := r.store.BeforeByCategory(before, period) // ordered by category and time

	r.aggregate(stats, "total", period)
}

// Count discrete events seen for day
// Must include previous days, in case server wasn't running at day end.

func (r *Recorder) aggregateSeen() {

	stats := r.store.BeforeByCategory(r.dayEnd, Seen) // ordered by category and time

	r.aggregate(stats, "seen", Day)

}

// Convert seen counts to daily averages
//
// Because because we can't distinguish between vistors across days.
// ## Is there an event we could distinguish and should support? E.g. errors?

func average(st StatisticStore, sPeriods [][]*Statistic) {

	// first month of recording
	s := st.GetMark("goLive")
	if s == nil {
		return // conversion not possible
	}
	yLive := s.Start.Year()
	mLive := s.Start.Month()
	dLive := s.Start.Day()

	// current month of recording
	now := time.Now().UTC()
	yNow := now.Year()
	mNow := now.Month()
	dNow := now.Day()

	for _, ss := range sPeriods {

		// calculate no of days in period
		var days int
		y := ss[0].Start.Year()
		m := ss[0].Start.Month()

		if m == mNow && y == yNow {
			// current month
			days = dNow
		} else {
			// days in month
			days = daysIn(y, m) 
		}

		if m == mLive && y == yLive {
			// first month - this applies even when the first month is also the current month
			days = days - dLive + 1
		}

		for _, s := range ss {
			// convert seen counts to daily average
			if s.Category == "seen" {
				s.Category = "daily"
				s.Count = int(s.Count + (days + 1)/2 - 1) / days  // rounded nearest
			}
		}
	}
}

// Catch up on any missed processing

func (r *Recorder) catchUp() {

	r.doDaily()
}

// Days in month

func daysIn(year int, month time.Month) int {

	// works because values outside the normal range are normalised 
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day() 
}

// Processing at end of day

func (r *Recorder) doDaily() {

	// make changes as a transaction
	st := r.store
	defer st.Transaction()()

	// count discrete events seen during day
	r.aggregateSeen()

	// rollup base period into days - no-op if base is a day
	if r.basePeriod == Base {
		r.rollupPeriod(r.dayEnd.AddDate(0, 0, -r.keepBase), Base, forDay, Day)
	}

	// ## could aggregate days, if wanted

	// rollup days into months
	r.rollupPeriod(r.dayEnd.AddDate(0, 0, -r.keepDays), Day, forMonth, Month)

	// aggregate months
	r.aggregateDetails(r.dayEnd.AddDate(0, -r.keepDetails, 0), Month)

	// purge months
	st.DeleteIf(r.dayEnd.AddDate(0, -r.keepMonths, 0), Month)

	// next day
	r.dayEnd = r.dayEnd.AddDate(0, 0, 1)
}

// Processing at end of base period

func (r *Recorder) doPeriodically() {

	// nothing to do, except start saving for next period
	r.periodStart = r.start(r.base)
	r.periodEnd = r.next(r.base)
}

// Start times for day and month

func forDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func forMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Rollup events into parent period for display
// Returns slice of stats, sorted by time and count, and oldest start included
// ## similar pattern to aggregate() and rollup() - could it be combined?

func getRollup(st StatisticStore, stats []*Statistic, toStart func(time.Time) time.Time, toPeriod int) ([]*Statistic, time.Time) {

	ss := make([]*Statistic, 0, 32)

	var event string
	var start time.Time
	var next bool
	var total *Statistic

	for _, s := range stats {

		// next parent period?
		sStart := toStart(s.Start)
		if start != sStart {
			start = sStart
			next = true
		}

		// next event?
		if event != s.Event {
			event = s.Event
			next = true
		}

		if next {

			if total != nil {

				// add in parent period
				// ## not efficient - read and index them?
				if p := st.GetEvent(total.Event, total.Start, toPeriod); p != nil {
					total.Count += p.Count
				}

				// save previous total
				ss = append(ss, total)
			}

			// next total
			total = &Statistic{
				Event:    event,
				Category: s.Category,
				Start:    start,
				Period:   toPeriod,
			}
			next = false
		}

		// add event to total
		total.Count += s.Count
	}
	if total != nil {

		// add in parent period
		if p := st.GetEvent(total.Event, total.Start, toPeriod); p != nil {
			total.Count += p.Count
		}

		ss = append(ss, total) // save final total
	}

	// sort by time descending, count descending, event
	compare := func(i, j int) bool {
		if ss[i].Start == ss[j].Start {
			if ss[i].Count == ss[j].Count {
				return ss[i].Event < ss[j].Event
			} else {
				return ss[i].Count > ss[j].Count
			}
		} else {
			return ss[i].Start.After(ss[j].Start)
		}
	}
	sort.Slice(ss, compare)

	// processed period
	if len(ss) > 0 {
		start = ss[len(ss)-1].Start
	} else {
		start = time.Now().UTC()
	}

	return ss, start
}

// Time of next operation, UTC, aligned to interval

func (r *Recorder) next(interval time.Duration) time.Time {
	return r.now.Truncate(interval).Add(interval)
}

// Rollup events into parent period
// ## similar pattern to aggregate() - could it be combined?

func (r *Recorder) rollup(stats []*Statistic, toStart func(time.Time) time.Time, toPeriod int) {

	st := r.store

	var event string
	var start time.Time
	var next bool
	var total *Statistic

	for _, s := range stats {

		// aggregate events for parent period
		if event != s.Event {
			event = s.Event
			next = true
		}

		sStart := toStart(s.Start)
		if start != sStart {
			start = sStart
			next = true
		}

		if next {
			// save previous total
			if total != nil {
				st.Update(total)
			}

			// next total
			total = st.GetEvent(event, start, toPeriod)
			if total == nil {
				// new total
				total = &Statistic{
					Event:    event,
					Category: s.Category,
					Start:    start,
					Period:   toPeriod,
				}
			}
			next = false
		}

		// add event to total and drop event
		total.Count += s.Count
		st.DeleteId(s.Id)
	}
	if total != nil {
		st.Update(total) // save final total
	}
}

// Rollup to next level

func (r *Recorder) rollupPeriod(before time.Time, from int, toStart func(time.Time) time.Time, to int) {

	stats := r.store.BeforeByEvent(before, from)

	r.rollup(stats, toStart, to)
}

// Save event counts to database. Time in UTC, called at least once per hour, and on shutdown.

func (r *Recorder) save() {

	// copy the volatile counts and unlock them, so we don't block the user unnecessarily
	r.mu.Lock()

	count := make([]struct {
		evt string
		n   int
	}, len(r.count))
	i := 0
	for event, n := range r.count {
		count[i].evt = event
		count[i].n = n
		i++
	}

	seen := make([]string, len(r.seen))
	i = 0
	for evt, _ := range r.seen {
		seen[i] = evt
		i++
	}

	category := make(map[string]string, len(r.category))
	for evt, grp := range r.category {
		category[evt] = grp
	}

	// reset the volatile counts (deletions optimised by compiler)
	for k := range r.count {
		delete(r.count, k)
	}
	for k := range r.seen {
		delete(r.seen, k)
	}
	for k := range r.category {
		delete(r.category, k)
	}

	// change salt daily, so that our anonymisation is genuine
	if r.now.After(r.dayEnd) {
		r.salt = rand.Uint32()
	}

	r.mu.Unlock()

	// make database changes as a transaction
	st := r.store
	defer st.Transaction()()

	// save volatile counts
	for _, ec := range count {

		s := st.GetEvent(ec.evt, r.periodStart, r.basePeriod)
		if s == nil {
			// new statistic
			s = &Statistic{
				Event:    ec.evt,
				Category: category[ec.evt],
				Count:    ec.n,
				Start:    r.periodStart,
				Period:   r.basePeriod,
			}
		} else {
			// update statistic
			s.Count += ec.n
		}
		st.Update(s)
	}

	// save distinct events
	for _, evt := range seen {

		s := st.GetEvent(evt, r.periodStart, Seen)
		if s == nil {
			// new event seen
			s = &Statistic{
				Event:    evt,
				Category: category[evt],
				Count:    1,
				Start:    r.periodStart,
				Period:   Seen,
			}
			st.Update(s)
		}
	}

	// next save
	r.volatileEnd = r.next(saveInterval)
}

// Asynchronous save to database

func (r *Recorder) saver(chTick <-chan time.Time, chDone <-chan bool) {

	// do anything pending during shutdown
	// (also helps with testing)
	r.now = time.Now().UTC()
	r.catchUp()

	for {
		select {
		case t := <-chTick:
			r.now = t.UTC()

			if r.now.After(r.volatileEnd) {
				r.save()
			}
			if r.now.After(r.periodEnd) {
				r.doPeriodically()
			}
			if r.now.After(r.dayEnd) {
				r.doDaily()
			}

		case <-chDone:
			r.now = time.Now().UTC()
			r.save() // save volatile counts before shutdown
			return
		}
	}
}

// Split stats by start time. Input slice assumed to be sorted.

func split(sPeriods [][]*Statistic, stats []*Statistic) [][]*Statistic {

	var last time.Time
	var ss []*Statistic

	// split stats into periods
	for _, s := range stats {
		if s.Start != last {
			if len(ss) > 0 {
				sPeriods = append(sPeriods, ss)
			}
			ss = make([]*Statistic, 0, 32)
			last = s.Start
		}
		ss = append(ss, s)
	}

	// final period
	if len(ss) > 0 {
		sPeriods = append(sPeriods, ss)
	}

	return sPeriods
}

// Start of period, UTC, aligned to interval

func (r *Recorder) start(interval time.Duration) time.Time {
	return r.now.Truncate(interval)
}
