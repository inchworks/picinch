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

package models

// Database models PicInch.

import (
	"database/sql"
	"errors"
	"html/template"
	"strconv"
	"strings"
	"time"
)

// Database field names are the same as structure names, with lower case first letter.

const (
	// These must match the database, so prefer specified values to iota.

	// page formats
	PageDiary = 1
	PageHome  = 2
	PageInfo  = 3

	// slide formats
	SlideTitle   = 1
	SlideImage   = 2
	SlideCaption = 4
	SlideVideo   = 8

	SlideFormatShift = 8 // shift for manual formats (below)

	SlideAbove      = 0 // image above text
	SlideBelow      = 1 // image below text
	SlideCard       = 2 // one of a grid of cards
	SlideLeft       = 3 // image left of text
	SlideRight      = 4 // image right of text
	SlideEvents     = 5 // next events
	SlideHighlights = 6 // Lightbox
	SlideSlideshows = 7 // recent slideshows
	SlideSubPages   = 8 // sub-pages
	SlideFormatMax  = 8 // manual formats are [0..max] shifted

	// slideshow type and visibility
	SlideshowRemoved = -10 // deletion in progress but cached access allowed
	SlideshowTopic   = -1  // slideshow for a topic
	SlideshowPrivate = 0
	SlideshowClub    = 1 // club or friends
	SlideshowPublic  = 2

	// user roles
	UserUnknown = 0
	UserFriend  = 1
	UserMember  = 2
	UserCurator = 3
	UserAdmin   = 4
	UserSystem  = 10

	// user status, not supported by webparts/v2/users
	UserSysInfo = -5
	UserSysSolo = -1

	// field sizes
	MaxName     = 60
	MaxTitle    = 128
	MaxDetail   = 512
	MaxMarkdown = 4096
)

var (
	ErrNoRecord           = errors.New("models: no matching record found")
	ErrInvalidCredentials = errors.New("models: invalid credentials")
	ErrDuplicateEmail     = errors.New("models: duplicate email")
)

var FormatOpts = []string{"above", "below", "card", "left", "right", "events", "highlights", "slideshows", "subpages"}

var VisibleOpts = []string{"none", "club", "public"}

type Gallery struct {
	Id      int64
	Version int

	// parameters
	Organiser  string // website name
	Title      string // site name appended to page titles
	Events     string // redundant
	NMaxSlides int    `db:"n_max_slides"`
	NShowcased int    `db:"n_showcased"`

	// announcements
	NoticePublic string // appears on home page
	NoticeUsers  string // appears on contributor's page
}

type Page struct {
	Id          int64
	Slideshow   int64
	Format      int
	Name        string `db:"menu"`
	Description string // for <meta>
	NoIndex     bool
	Title       string // for <title>
}

type Slide struct {
	Id        int64
	Slideshow int64
	Format    int
	ShowOrder int `db:"show_order"`
	Created   time.Time
	Revised   time.Time
	Title     string // sanitized HTML
	Caption   string // sanitized HTML
	Image     string
}

type Slideshow struct {
	Id           int64
	Gallery      int64
	GalleryOrder int           `db:"gallery_order"`
	Access       int           // permitted access (changes deferred for caching)
	Visible      int           // visible for listing (changes immediate)
	User         sql.NullInt64 // null for a topic
	Shared       int64         // link for external access
	Topic        int64         // parent topic, 0 for a normal slideshow
	Created      time.Time
	Revised      time.Time
	Title        string
	Caption      string // sanitized HTML
	Format       string
	Image        string
	ETag         string `db:"etag"` // latent support: entity tag for caching
}

type Tag struct {
	Id      int64
	Gallery int64
	Parent  int64 // 0 for a top level tag
	Name    string
	Action  string
	Format  string
}

type TagRef struct {
	Id     int64
	Item   sql.NullInt64 // null for a user permission tag
	Tag    int64
	User   sql.NullInt64 // null for a system tag
	Added  time.Time
	Detail string
}

// Join results

type PageSlideshow struct {
	PageId      int64
	PageFormat  int
	Name        string `db:"menu"`
	Description string
	MetaTitle   string
	NoIndex     bool
	Slideshow
}

type SlideRank struct {
	Slide
	Rank int
}

type SlideRevision struct {
	Slide
	ShowTitle string
	Visible   int
	Name      string
}

type SlideshowTagRef struct {
	Slideshow
	TagRefId int64
}

type SlideshowUser struct {
	Id     int64
	Title  string
	Image  string
	UserId int64
	Name   string // user's display name
}

type SubPage struct {
	Name    string `db:"menu"` // from page
	Title   string // from slideshow
	Caption string // from 1st section?
	Image   string // from 1st section
}

type TagItem struct {
	Tag
	ItemId int64
}

type TagUser struct {
	Tag
	UserId    int64
	UsersName string
}

type TopicSlide struct {
	Format  int
	Title   string
	Caption string
	Image   string
	Name    string
}

// Fields with newlines replaced by breaks, and HTML formatting allowed.
// ## If source is untrusted, could return a slice of lines and use range to add breaks in template.

func (p *Slide) TitleBr() template.HTML {
	return Nl2br(p.Title)
}

func (p *Slide) CaptionBr() template.HTML {
	return Nl2br(p.Caption)
}

func Nl2br(str string) template.HTML {
	return template.HTML(strings.Replace(str, "\n", "<br>", -1))
}

// Code to string conversions

func (s *Slideshow) VisibleStr() string {

	return VisibleOpts[s.Visible]
}

// ManualFormat returns the manual component of the slide format
func (s *Slide) ManualFormat() int {
	return s.Format >> SlideFormatShift
}

// ParseFormat returns the slideshow format and maximum number of slides.
func (t *Slideshow) ParseFormat(defaultMax int) (fmt string, max int) {

	var err error
	ss := strings.Split(t.Format, ".")

	switch len(ss) {
	case 0:
		fmt = "S" // regular slideshow
		max = defaultMax

	case 1:
		fmt = ss[0]
		max = defaultMax

	default:
		fmt = ss[0]
		max, err = strconv.Atoi(ss[1])
		if err != nil {
			max = defaultMax
		} // default
	}

	return
}
