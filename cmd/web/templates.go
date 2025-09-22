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
// MERCHANTABILITY orBoo FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with PicInch.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"html/template"
	"net/http"
	"time"

	"codeberg.org/inchworks/webparts/multiforms"
	"codeberg.org/inchworks/webparts/uploader"
	"codeberg.org/inchworks/webparts/usage"
	"codeberg.org/inchworks/webstarter/users"
	"github.com/justinas/nosurf"

	"inchworks.com/picinch/internal/cache"
	"inchworks.com/picinch/internal/form"
	"inchworks.com/picinch/internal/models"
)

// Template data for all pages - implements TemplateData interface so we can add data without knowing
// which template we have

type TemplateData interface {
	addDefaultData(app *Application, r *http.Request, name string, addSite bool)
}

type DataCommon struct {
	Canonical  string // canonical domain
	CSRFToken  string
	Flash      string // flash message
	ParentHRef string

	// To configure menus, this is NOT to check authorisation
	IsAdmin         bool // user is administrator
	IsAuthenticated bool // user authenticated
	IsCompetition   bool // competitions enabled
	IsCurator       bool // user is curator
	IsFriend        bool // user is friend
	IsGallery       bool // gallery with contributors
	IsMember        bool // user is member

	Menus     []*cache.MenuItem
	Page      string // unused, kept for version compatibility
	SiteTitle string // appended to page titles
}

func (d *DataCommon) addDefaultData(app *Application, r *http.Request, page string, addSite bool) {

	d.CSRFToken = nosurf.Token(r)
	d.Flash = app.session.PopString(r.Context(), "flash")
	d.IsAdmin = app.isAuthenticated(r, models.UserAdmin)
	d.IsAuthenticated = app.isAuthenticated(r, models.UserFriend)
	d.IsCompetition = (app.cfg.Options == "main-comp")
	d.IsCurator = app.isAuthenticated(r, models.UserCurator)
	d.IsFriend = app.isAuthenticated(r, models.UserFriend)
	d.IsGallery = true // ## no non-gallery configuration yet
	d.IsMember = app.isAuthenticated(r, models.UserMember)

	if addSite && app.galleryState.gallery.Title != "" {
		d.SiteTitle = " " + app.galleryState.gallery.Title
	}

	d.Menus = app.galleryState.publicPages.MainMenu
	d.Page = page
}

// metadata for diary and information pages
type DataMeta struct {
	Title       string
	Description string
	NoIndex     bool
}

// template data for display pages

type dataCompetition struct {
	Categories []*DataPublished
	DataCommon
}

type DataContributor struct {
	DisplayName string
	Highlights  []*DataSlide
	Slideshows  []*DataPublished
	DataCommon
}

type DataDiary struct {
	Meta    DataMeta
	Title   string
	Caption template.HTML
	Events  []*DataEvent
	DataCommon
}

type DataEvent struct {
	Start   string
	Title   template.HTML
	Details template.HTML
	Diary   string
}

type DataInfo struct {
	Meta     DataMeta
	Title    string
	Sections []*DataSection
	DataCommon
}

type DataMyGallery struct {
	NUser       int64
	DisplayName string
	Slideshows  []*DataMySlideshow
	Topics      []*DataMySlideshow
	DataCommon
}

type DataMySlideshow struct {
	NShow   int64
	Ref     string
	Title   string
	Visible string
	Shared  string
}

type DataPage struct {
	NPage int64
	Title string
	Name  string
}

type DataPages struct {
	Diaries []*DataPage
	Home    []*DataPage
	Pages   []*DataPage
	DataCommon
}

type DataPublished struct {
	Id          int64
	Ref         string
	Title       string
	Caption     template.HTML
	NTopic      int64
	NUser       int64
	DisplayName string
	Image       string
	NTagRef     int64
	DataCommon
}

type DataSection struct {
	cache.Section

	// sub-pages for page, if section includes them
	SubPages []*cache.SubPage

	// updated with live data
	Events     []*DataEvent     // if section includes next events
	Highlights []*DataSlide     // if section includes highlights
	Slideshows []*DataPublished // if section includes slideshows
}

type DataSlideshow struct {
	Title       string
	Caption     template.HTML
	DisplayName string
	Reference   string
	AfterHRef   string
	BeforeHRef  string
	Single      string
	Slides      []*DataSlide
	DataCommon
}

type DataSlideshows struct {
	Title      string
	Slideshows []*DataPublished
	DataCommon
}

type DataSlide struct {
	Title       template.HTML
	Caption     template.HTML
	DisplayName string
	Image       string
	Format      int
}

type DataSlideshowPage struct {
	Page  string // "" if same as previous slideshow
	User  string
	NShow int64
	Title string
}

type DataSlideshowsByPage struct {
	Slideshows []*DataSlideshowPage
	Users      []*models.UserSummary
	DataCommon
}

type DataTagged struct {
	NRoot      int64
	NUser      int64
	Parent     string
	Tag        string
	Topic      string
	Slideshows []*DataPublished
	DataCommon
}

type DataTags struct {
	Tags []*DataTag
	DataCommon
}

type DataTag struct {
	NRoot   int64
	NTag    int64
	ForUser int64
	Name    string
	Count   string
	Disable string
	Indent  string
}

type DataUsagePeriods struct {
	Title string
	Usage []*DataUsage
	DataCommon
}

type DataUsage struct {
	Date  string
	Stats []*usage.Statistic
}

type DataUsers struct {
	Users []*users.User
	DataCommon
}

type dataValidated struct {
	Name  string
	Email string
	Class string
	Title string
	Warn  string
	DataCommon
}

// template data for forms

type assignToPagesFormData struct {
	Form *form.AssignToPagesForm
	DataCommon
}

type dataUpdating struct {
	Status string
	Title  string
	User   string
}

type assignToTopicsFormData struct {
	Form     *form.AssignShowsForm
	Updating []*dataUpdating
	DataCommon
}

type compFormData struct {
	Form      *form.PublicCompForm
	Class     string
	Caption   template.HTML
	Accept    string
	MaxUpload int // in MB
	DataCommon
}

type diaryFormData struct {
	Form  *form.DiaryForm
	Title string
	DataCommon
}

type metaFormData struct {
	Form  *multiforms.Form
	Title string
	DataCommon
}

type pagesFormData struct {
	Form     *form.PagesForm
	Action   string
	Heading  string
	HomeName string
	HomePage string // page ID, base 36, not trusted and only for a URL
	DataCommon
}

type simpleFormData struct {
	Form *multiforms.Form
	DataCommon
}

type slidesFormData struct {
	Form      *form.SlidesForm
	Title     string
	Accept    string
	IsHome    bool
	MaxUpload int // in MB
	DataCommon
}

type slideshowPageFormData struct {
	Form  *multiforms.Form
	Title string
	User  string
	DataCommon
}

type slideshowsFormData struct {
	Form  *form.SlideshowsForm
	User  string
	NUser int64
	DataCommon
}

type tagsFormData struct {
	Form  *multiforms.Form
	Title string
	Users []*tagUser
	DataCommon
}

type tagFormData struct {
	Form  *multiforms.Form
	Title string
	Tags  []*tagData
	DataCommon
}

type tagData struct {
	tagId   int64
	TagHTML template.HTML
	Tags    []*tagData
}

type tagUser struct {
	Name string
	Tags []*tagData
}

type usersFormData struct {
	Users interface{}
	DataCommon
}

// Context for functions called from template.

type templateCtx struct {
	uploader *uploader.Uploader
}

// setTemplateCtx adds template functions that need a context.
func setTemplateCtx(up *uploader.Uploader) {

	// "method values" are preferable to saving the context as a global value.
	ctx := templateCtx{uploader: up}
	templateFuncs["isWorking"] = ctx.isWorking
	templateFuncs["thumbnail"] = ctx.thumbnail
	templateFuncs["viewable"] = ctx.viewable
}

// Define functions callable from a template

var templateFuncs = template.FuncMap{
	"cardCols":     cardCols,
	"checked":      checked,
	"htmlDate":     htmlDate,
	"htmlDateTime": htmlDateTime,
	"humanDate":    humanDate,
	"userStatus":   userStatus,
}

// cardCols returns the column classes for a row of cards.
func cardCols(nCards int) string {

	switch nCards {
	case 0:
		return "" // not expected
	case 1:
		return "row-cols-1"
	case 2:
		return "row-cols-1 row-cols-sm-1 row-cols-md-2"
	case 3:
		return "row-cols-1 row-cols-sm-1 row-cols-md-2 row-cols-lg-3"
	case 4:
		// pairs, never 3+1
		return "row-cols-1 row-cols-sm-1 row-cols-md-2 row-cols-lg-2 row-cols-xxl-4"
	case 5:
		// 3+2, never 4+1
		return "row-cols-1 row-cols-sm-1 row-cols-md-2 row-cols-lg-3"
	default:
		return "row-cols-1 row-cols-sm-1 row-cols-md-2 row-cols-lg-3 row-cols-xxl-4"
	}
}

// checked returns "checked" if the parameter is true, for use with a form checkbox.
func checked(isChecked bool) string {

	if isChecked {
		return "checked"
	} else {
		return ""
	}
}

// htmlDate returns the date in HTML input type format.
func htmlDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Local().Format("2006-01-02")
}

// htmlDateTime returns the date-time in HTML input type format.
func htmlDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Local().Format("2006-01-02T15:04")
}

// humanDate returns the date in a user-friendly format.
func humanDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Local().Format("02 Jan 2006 at 15:04")
}

// isWorking returns true if a media file is not ready to be viewed.
func (ctx *templateCtx) isWorking(image string) bool {
	return ctx.uploader.Status(image) < 100
}

// thumbnail returns a path to a thumbnail image
func (ctx *templateCtx) thumbnail(image string) string {

	s := ctx.uploader.Status(image)

	if s == 0 {
		return "/static/images/no-photos.jpg"
	} else if s < 100 {
		return "/static/images/working.jpg"
	} else {
		return "/photos/" + ctx.uploader.Thumbnail(image)
	}
}

// userStatus returns a user's status as a string
func userStatus(n int) (s string) {

	switch n {
	case users.UserSuspended:
		s = "suspended"

	case users.UserKnown:
		s = "-"

	case users.UserActive:
		s = "signed-up"

	default:
		s = "??"
	}

	return
}

// viewable returns the version of a media file that is ready to be viewed.
func (ctx *templateCtx) viewable(image string) string {

	s := ctx.uploader.Status(image)

	if s == 0 {
		return "/static/images/no-photos.jpg" // not expected
	} else if s < 100 {
		return "/static/images/working-lg.jpg"
	} else {
		return "/photos/" + image
	}
}
