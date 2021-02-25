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
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	"github.com/inchworks/usage"
	"github.com/inchworks/webparts/multiforms"
	"github.com/inchworks/webparts/users"
	"github.com/justinas/nosurf"

	"inchworks.com/picinch/pkg/form"
	"inchworks.com/picinch/pkg/images"
	"inchworks.com/picinch/pkg/models"
)

// Template data for all pages - implements TemplateData interface so we can add data without knowing
// which template we have

type TemplateData interface {
	addDefaultData(app *Application, r *http.Request)
}

type DataCommon struct {
	Canonical  string // canonical domain
	CSRFToken  string
	Flash      string // flash message
	ParentHRef string

	// To configure menus, this is NOT to check authorisation
	IsAdmin         bool // user is administrator
	IsAuthenticated bool // user authenticated
	IsCurator       bool // user is curator
}

func (d *DataCommon) addDefaultData(app *Application, r *http.Request) {

	d.CSRFToken = nosurf.Token(r)
	d.Flash = app.session.PopString(r, "flash")
	d.IsAdmin = app.isAdmin(r)
	d.IsAuthenticated = app.isAuthenticated(r, models.UserFriend)
	d.IsCurator = app.isCurator(r)
}

// template data for display pages

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
	NUser   int64
	Title   string
	Visible string
	Shared  string
}

type DataPublished struct {
	Id          int64
	Title       string
	NUser       int64
	DisplayName string
	Image       string
	DataCommon
}

type DataSlideshow struct {
	Topic       string
	Title       string
	DisplayName string
	AfterHRef   string
	BeforeHRef  string
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

// template data for gallery editing

type simpleFormData struct {
	Form *multiforms.Form
	DataCommon
}

type slidesFormData struct {
	Form  *form.SlidesForm
	NShow int64
	Title string
	DataCommon
}

type slideshowsFormData struct {
	Form  *form.SlideshowsForm
	User  string
	NUser int64
	DataCommon
}

type usersFormData struct {
	Users interface{}
	DataCommon
}

// Define functions callable from a template

var functions = template.FuncMap{
	"checked":    checked,
	"humanDate":  humanDate,
	"thumbnail":  thumbnail,
	"userStatus": userStatus,
}

// Template cache
//
// Code modified from Let's Go (to add sub-directories)

func newTemplateCache(forApp string, forSite string) (map[string]*template.Template, error) {

	// Initialize a new map to act as the cache.
	cache := map[string]*template.Template{}

	// Add templates from sub-directories
	err := filepath.Walk(forApp, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return addTemplates(forApp, path, forSite, cache)
		}
		return nil // ignore page templates in root
	})

	// return the map
	return cache, err
}

func addTemplates(root string, dir string, site string, cache map[string]*template.Template) error {

	// Use the filepath.Glob function to get a slice of all filepaths with
	// the extension '.page.tmpl'. This essentially gives us a slice of all the
	// 'page' templates for the application.
	pages, err := filepath.Glob(filepath.Join(dir, "*.page.tmpl"))
	if err != nil {
		return err
	}

	// Loop through the pages one-by-one.
	for _, page := range pages {

		// Extract the file name (e.g. 'home.page.tmpl') from the full file path
		name := filepath.Base(page)

		// The template.FuncMap must be registered with the template set before you
		// call the ParseFiles() method. So we create an empty template set, use the
		// Funcs() method to register the map, and then parse the file as normal.

		// Parse the page template file in to a template set.
		ts, err := template.New(name).Funcs(functions).ParseFiles(page)
		if err != nil {
			return err
		}

		// Add any 'layout' templates to the template set.
		if ts, err = parseGlobIf(ts, filepath.Join(dir, "*.layout.tmpl")); err != nil {
			return err
		}

		// Add root templates (assumes order of template types doesn't matter)
		if ts, err = parseGlobIf(ts, filepath.Join(root, "*.tmpl")); err != nil {
			return err
		}

		// Add any 'partial' templates to the template set
		if ts, err = parseGlobIf(ts, filepath.Join(dir, "*.partial.tmpl")); err != nil {
			return err
		}

		// Add any site templates
		if ts, err = parseGlobIf(ts, filepath.Join(site, "*.tmpl")); err != nil {
			return err
		}

		// Add the template set to the cache, using the name of the page
		// (like 'home.page.tmpl') as the key.
		cache[name] = ts
	}

	return nil
}

// add any optional templates to set

func parseGlobIf(ts *template.Template, pattern string) (*template.Template, error) {

	// ## ParseGlob fails if there are no matches. I can't find out how to test for that error :-(.
	m, err := filepath.Glob(pattern)
	if len(m) > 0 {
		ts, err = ts.ParseGlob(pattern)
		if err != nil {
			return nil, err
		}
	}
	return ts, err
}

// checked returns "checked" if the parameter is true, for use with a form checkbox.
func checked(isChecked bool) string {

	if isChecked {
		return "checked"
	} else {
		return ""
	}
}

// get thumbnail image

func thumbnail(image string) string {

	if image == "" {
		return "/static/images/no-photos.jpg"
	} else {
		return "/photos/" + images.Thumbnail(image)
	}

}

// userRole returns a user's role as a string
func userRole(n int) (s string) {

	switch n {
	// user status
	case models.UserFriend:
		s = "friend"

	case models.UserMember:
		s = "member"

	case models.UserCurator:
		s = "curator"

	case models.UserAdmin:
		s = "administrator"

	default:
		s = "??"
	}

	return
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
