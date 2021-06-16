// Copyright © Rob Burke inchworks.com, 2021.

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

// Processing for workflow management using tags.

// These functions may modify application state.
// Parameters are defined in the order: slideshow, tag, user.

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"inchworks.com/picinch/pkg/models"
)

type action struct {
	code    string
	forUser int64
	path    []string
}

type slideshowTag struct {
	id       int64
	parent   int64
	name     string
	format   string
	children []*slideshowTag
	edit     bool
	set      bool
}

// childSlideshowTags returns a user's editable tags for a slideshow.
func (app *Application) childSlideshowTags(slideshow int64, parent int64, user int64, toEdit bool) []*slideshowTag {

	var fts []*slideshowTag

	ts := app.tagStore.ForParent(parent)
	for _, t := range ts {
		var isEdit, isSet bool
		var cts []*slideshowTag

		if isSet = app.tagRefStore.Exists(slideshow, t.Id, user); isSet {

			if toEdit {
				// get the tags that can be edited, instead of the one that is set
				ets := app.editableTags(t, user)
				if len(ets) > 0 {
					// editable tag (just one supported currently)
					t = ets[0]
					cts = app.childEditTags(slideshow, t.Id, user)
					isEdit = true
					isSet = false
				}
			}
		}

		if !isEdit {
			// child tags for non-editable tag
			cts = app.childSlideshowTags(slideshow, t.Id, user, toEdit)
		}

		if len(cts) == 1 && cts[0].id == t.Id {

			// A child tag may have returned this tag as editable. We don't need it twice.
			fts = append(fts, cts[0])

		} else if len(cts) > 0 {

			// include the tag if has referenced children
			fts = append(fts, &slideshowTag{
				id:       t.Id,
				parent:   t.Parent,
				name:     t.Name + " : ",
				format:   t.Format,
				children: cts,
				edit:     isEdit,
				set:      isSet,
			})

		} else if isSet {

			// include the tag if it is referenced
			fts = append(fts, &slideshowTag{
				id:     t.Id,
				parent: t.Parent,
				name:   t.Name,
				format: t.Format,
				edit:   isEdit,
				set:    true,
			})
		}
	}
	return fts
}

// dropTagRef removes a tag, and adds any successor tags. Returns false if the user lacks permission.
func (app *Application) dropTagRef(slideshow int64, parent int64, name string, user int64) bool {

	t := app.tagStore.GetNamed(parent, name)
	if t == nil {
		return false
	}

	// add successor tag references
	ok := true
	as := parseActions(t.Action, name, user)
	for _, a := range as {
		switch a.code {

		case ">":
			if !app.addTagRefAll(slideshow, a.path, a.forUser, "") {
				ok = false
			}
		}
	}

	// remove reference (OK if doesn't exist)
	if err := app.tagRefStore.DeleteIf(slideshow, t.Id, user); err != nil {
		app.log(err)
		return false
	}
	return ok
}

// formTags returns the tags to be changed on a form.
func (app *Application) formTags(slideshow int64, refTag *models.Tag, user int64) []*slideshowTag {

	var fts []*slideshowTag

	// actions specify the modifiable tags
	// ## only current user, so couldn't be called by a curator
	ets := app.editableTags(refTag, user)
	for _, t := range ets {

		ft := &slideshowTag{
			id:       t.Id,
			parent:   t.Parent,
			name:     t.Name,
			format:   t.Format,
			children: app.childEditTags(slideshow, t.Id, user),
			set:      app.tagRefStore.Exists(slideshow, t.Id, user),
		}
		fts = append(fts, ft)
	}

	return fts
}

// setTagRef adds a tag to a slideshow. Returns false if the user lacks permission. Errors are logged and ignored.
func (app *Application) setTagRef(slideshow int64, parent int64, name string, user int64, detail string) bool {

	// actions specified by parent
	if parent != 0 {
		p := app.tagStore.GetIf(parent)
		if p == nil {
			return false
		}

		if !app.doSetActions(p.Action, slideshow, name, user) {
			return false
		}
	}

	// actions specified by tag
	t := app.tagStore.GetNamed(parent, name)
	if t == nil {
		return false
	}
	if !app.doSetActions(t.Action, slideshow, name, user) {
		return false
	}

	// add tag reference
	ok := app.addTagRef(slideshow, t.Id, user, detail, true)
	return ok
}

// INTERNAL FUNCTIONS

// addTagRef adds a tag to a slideshow. Errors are logged and ignored. Returns false if the user lacks permission.
// user is 0 for a system tag.
func (app *Application) addTagRef(slideshow int64, tagId int64, user int64, detail string, create bool) bool {

	// is reference already set
	if app.tagRefStore.Exists(slideshow, tagId, user) {
		return true
	}

	// link tag to slideshow
	r := &models.TagRef{
		Slideshow: sql.NullInt64{Int64: slideshow, Valid: true},
		Tag:       tagId,
		Added:     time.Now(),
		Detail:    detail,
	}

	if user != 0 {
		r.User = sql.NullInt64{Int64: user, Valid: true}
	}

	err := app.tagRefStore.Update(r)
	if err != nil {
		app.log(err)
		return false
	}
	return true
}

// addTagRefAll adds a tags to a slideshow. A negative user ID of selects all users having the root tag, except the specified user.
// Errors are logged and ignored.
func (app *Application) addTagRefAll(slideshow int64, path []string, user int64, detail string) bool {

	// lookup tag
	t := app.getTag(path)
	if t == nil {
		return false
	}

	ok := true
	if user < 0 {
		// all users holding the root tag, except this one
		us := app.userStore.ForTagName(path[0])
		for _, u := range us {
			if u.Id != -user {
				if !app.addTagRef(slideshow, t.Id, u.Id, detail, false) {
					return false
				}
			}
		}

	} else {
		ok = app.addTagRef(slideshow, t.Id, user, detail, false)
	}
	return ok
}

// childFormTags returns the child tags for an edit tag.
func (app *Application) childEditTags(slideshow int64, parent int64, user int64) []*slideshowTag {

	var fts []*slideshowTag

	ts := app.tagStore.ForParent(parent)
	for _, t := range ts {

		ft := &slideshowTag{
			id:       t.Id,
			parent:   t.Parent,
			name:     t.Name,
			format:   t.Format,
			children: app.childEditTags(slideshow, t.Id, user),
			edit:     true,
			set:      app.tagRefStore.Exists(slideshow, t.Id, user),
		}
		fts = append(fts, ft)
	}
	return fts
}

// deleteTagRef finds and deletes the specified tag reference, if it exists.
func (app *Application) deleteTagRef(slideshow int64, path []string, user int64) error {

	// find tag
	t := app.getTag(path)
	if t == nil {
		return nil // ok
	}

	// delete reference
	return app.tagRefStore.DeleteIf(slideshow, t.Id, user)
}

// deleteTagRefAll finds and deletes any specified tag references. A negative user ID removes references for all users.
func (app *Application) deleteTagRefAll(slideshow int64, path []string, user int64) bool {

	if user < 0 {
		// all users with this root permission
		us := app.userStore.ForTagName(path[0])
		for _, u := range us {
			if app.deleteTagRef(slideshow, path, u.Id) != nil {
				return false
			}
		}
	} else {
		return app.deleteTagRef(slideshow, path, user) == nil
	}
	return true
}

// doSetActions does the actions needed when a tag is set.
func (app *Application) doSetActions(spec string, slideshowId int64, tag string, userId int64) bool {

	as := parseActions(spec, tag, userId)
	for _, a := range as {
		switch a.code {
		case "<":
			// remove predecessor(s)
			if !app.deleteTagRefAll(slideshowId, a.path, a.forUser) {
				return false
			}

		case "!":
			// add notification
			if !app.addTagRefAll(slideshowId, a.path, a.forUser, "") {
				return false
			}
		}
	}
	return true
}

// editableTag returns the editable tags corresponding to the specified tag.
func (app *Application) editableTags(tag *models.Tag, userId int64) []*models.Tag {

	var ets []*models.Tag

	// actions specify the editable tags
	as := parseActions(tag.Action, tag.Name, userId)
	for _, a := range as {

		if a.code == "$" {
			if a.path[0] == "#" {
				ets = append(ets, tag) // this tag

			} else if a.path[0] == "-" {
				ets = append(ets, app.tagStore.GetIf(tag.Parent)) // parent tag

			} else {
				ets = append(ets, app.getTag(a.path)) // tag specified by path
			}
		}
	}
	return ets
}

// getTag returns a tag for a path.
func (app *Application) getTag(path []string) *models.Tag {

	// lookup path
	var t *models.Tag
	p := int64(0)
	for _, name := range path {
		t = app.tagStore.GetNamed(p, name)
		if t == nil {
			return nil
		}		
		p = t.Id  // next in path
	}

	return t
}

// parseActions parses an action specification and returns a slide of actions.
func parseActions(spec string, tag string, user int64) []action {

	// items separated by whitespace
	ss := strings.Fields(spec)

	var as []action
	for _, s := range ss {

		// item is !user:tags, where ! is an action code
		cs := strings.Split(s, ":")

		a := action{code: cs[0][0:1]}

		if len(cs) == 1 {
			// system tag
			a.path = strings.Split(cs[0][1:], "/")

		} else if len(cs) > 1 {
			// user-specific tag
			u := cs[0][1:]
			switch u {
			case "#":
				a.forUser = user // parameter user

			case "*":
				a.forUser = -user // all users except this one

			default:
				a.forUser, _ = strconv.ParseInt(u, 10, 64)
			}
			a.path = strings.Split(cs[1], "/")
		}

		// parameterised tag in path
		if a.code != "$" {
			last := len(a.path) - 1
			if last >= 0 && a.path[last] == "#" {
				a.path[last] = tag
			}
		}

		as = append(as, a)
	}

	return as
}
