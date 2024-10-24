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

	"github.com/inchworks/usage"
	"github.com/inchworks/webparts/v2/multiforms"
	"github.com/inchworks/webparts/v2/uploader"
	"github.com/inchworks/webparts/v2/users"
	"github.com/justinas/nosurf"

	"inchworks.com/picinch/internal/form"
	"inchworks.com/picinch/internal/models"
)

// Template data for all pages - implements TemplateData interface so we can add data without knowing
// which template we have

type TemplateData interface {
	addDefaultData(app *Application, r *http.Request, name string)
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
	IsMember        bool // user is member

	Page string
}

func (d *DataCommon) addDefaultData(app *Application, r *http.Request, page string) {

	d.CSRFToken = nosurf.Token(r)
	d.Flash = app.session.PopString(r.Context(), "flash")
	d.IsAdmin = app.isAuthenticated(r, models.UserAdmin)
	d.IsAuthenticated = app.isAuthenticated(r, models.UserFriend)
	d.IsCompetition = (app.cfg.Options == "main-comp")
	d.IsCurator = app.isAuthenticated(r, models.UserCurator)
	d.IsFriend = app.isAuthenticated(r, models.UserFriend)
	d.IsMember = app.isAuthenticated(r, models.UserMember)
	d.Page = page
}

// template data for display pages

type dataCompetition struct {
	Categories []*DataPublished
	DataCommon
}

type DataHome struct {
	DisplayName string
	Highlights  []*DataSlide
	Slideshows  []*DataPublished
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

type DataSlideshow struct {
	Title       string
	Caption     string
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

type assignShowsFormData struct {
	Form  *form.AssignShowsForm
	User  string
	NUser int64
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

type simpleFormData struct {
	Form *multiforms.Form
	DataCommon
}

type slidesFormData struct {
	Form      *form.SlidesForm
	Title     string
	Accept    string
	MaxUpload int // in MB
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

// Define functions callable from a template

var templateFuncs = template.FuncMap{
	"checked":    checked,
	"isWorking":  isWorking,
	"humanDate":  humanDate,
	"thumbnail":  thumbnail,
	"userStatus": userStatus,
	"viewable":   viewable,
}

// checked returns "checked" if the parameter is true, for use with a form checkbox.
func checked(isChecked bool) string {

	if isChecked {
		return "checked"
	} else {
		return ""
	}
}

// humanDate returns the date in a user-friendly format.
func humanDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.UTC().Format("02 Jan 2006 at 15:04")
}

// isWorking returns true if a media file is not ready to be viewed.
func isWorking(image string) bool {
	return uploader.Status(image) < 100
}

// thumbnail returns a path to a thumbnail image
func thumbnail(image string) string {

	s := uploader.Status(image)

	if s == 0 {
		return "/static/images/no-photos.jpg"
	} else if s < 100 {
		return "/static/images/working.jpg"
	} else {
		return "/photos/" + uploader.Thumbnail(image)
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
func viewable(image string) string {

	s := uploader.Status(image)

	if s == 0 {
		return "/static/images/no-photo.jpg" // not expected
	} else if s < 100 {
		return "/static/images/working-lg.jpg"
	} else {
		return "/photos/" + image
	}
}
