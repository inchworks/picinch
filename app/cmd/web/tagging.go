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
//
// These functions may modify application state.

import (
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

// addUserTags creates any user-specific tags from the corresponding system tags.
func (app *Application) addUserTags(user int64) bool {

	var nErrs int // assume success

	// root tags for user
	uts := app.tagStore.ForUser(user)
	for _, ut := range uts {

		// corresponding system definition
		// ## must stop a user adding own root tags with same name as system tag
		st, err := app.tagStore.GetNamed(0, ut.Name, 0)
		if err != nil {
			nErrs++
		}

		if st != nil {
			// add child tags
			nErrs += app.addChildTags(st, ut, user)
		}
	}
	return nErrs == 0
}

// childSlideshowTags returns a user's editable tags for a slideshow.
func (app *Application) childSlideshowTags(parent int64, user int64, slideshow int64) []*slideshowTag {

	var fts []*slideshowTag

	ts := app.tagStore.ForParent(parent)
	for _, t := range ts {
		var isEdit, isSet bool
		var cts []*slideshowTag

		if isSet = app.tagRefStore.Exists(slideshow, t.Id); isSet {

			if user != 0 {
				ets := app.editableTags(t, user)
				if len(ets) > 0 {
					// editable tag (just one supported currently)
					t = ets[0]
					cts = app.childEditTags(t.Id, user, slideshow)
					isEdit = true
				}
			}
		}

		if !isEdit {
			// child tags for non-editable tag
			cts = app.childSlideshowTags(t.Id, user, slideshow)
		}

		// include tag that is referenced, or has referenced children
		if isSet || len(cts) > 0 {

			ft := &slideshowTag{
				id:       t.Id,
				parent:   t.Parent,
				name:     t.Name,
				format:   t.Format,
				children: cts,
				edit:     isEdit,
				set:      isSet,
			}
			fts = append(fts, ft)
		}
	}
	return fts
}

// dropTagRef removes a tag, and adds any successor tags. Returns false if the user lacks permission.
func (app *Application) dropTagRef(parent int64, name string, forUser int64, slideshow int64) bool {

	// only root tags are held by a user
	var keyUser int64
	if parent == 0 {
		keyUser = forUser
	}

	t, err := app.tagStore.GetNamed(parent, name, keyUser)
	if err != nil {
		app.log(err)
		return false
	}
	if t == nil {
		return false
	}

	// add successor tag references
	ok := true
	as := parseActions(t.Action, forUser, name)
	for _, a := range as {
		switch a.code {

		case ">":
			if !app.addTagRefAll(a.forUser, a.path, slideshow, "") {
				ok = false
			}
		}
	}

	// remove reference (OK if doesn't exist)
	if err := app.tagRefStore.DeleteIf(slideshow, t.Id); err != nil {
		app.log(err)
		return false
	}
	return ok
}

// formTags returns the tags to be changed on a form.
func (app *Application) formTags0(user int64, slideshow int64, refTag *models.Tag) []*slideshowTag {

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
			children: app.childEditTags(t.Id, user, slideshow),
			set:      app.tagRefStore.Exists(slideshow, t.Id),
		}
		fts = append(fts, ft)
	}

	return fts
}

// setTagRef adds a tag to a slideshow. Returns false if the user lacks permission. Errors are logged and ignored.
func (app *Application) setTagRef(parent int64, name string, forUser int64, slideshow int64, byUser int64, detail string) bool {

	// actions specified by parent
	if parent != 0 {
		p := app.tagStore.GetIf(parent)
		if p == nil {
			return false
		}
		as := parseActions(p.Action, forUser, name)
		for _, a := range as {
			switch a.code {
			case "<":
				// remove predecessor(s)
				if !app.deleteTagRefAll(a.forUser, slideshow, a.path) {
					return false
				}

			case "!":
				// add notification
				if !app.addTagRefAll(a.forUser, a.path, slideshow, "") {
					return false
				}
			}
		}
	}

	// ## implement actions specified by tag (if useful?)

	// add tag reference
	ok := app.addTagRef(parent, forUser, []string{name}, slideshow, byUser, detail, true)
	return ok
}

// INTERNAL FUNCTIONS

// addChildTags creates any user-specific child tags from the corresponding system tags
func (app *Application) addChildTags(sysTag *models.Tag, userTag *models.Tag, user int64) int {

	var nErrs int // assume success

	// system definitions of child tags
	sts := app.tagStore.ForParent(sysTag.Id)
	for _, st := range sts {

		// look for corresponding user-specific tag
		ut, err := app.tagStore.GetNamed(userTag.Id, st.Name, 0)
		if err != nil {
			nErrs++
			err = nil
		}
		if ut == nil {
			// add missing user-specific child
			ut = &models.Tag{
				Parent: userTag.Id,
				Name:   st.Name,
				Action: st.Action,
				Format: st.Format,
			}
			err = app.tagStore.Update(ut)

		} else if ut.Action != st.Action || ut.Format != st.Format {
			ut.Action = st.Action
			ut.Format = st.Format
			err = app.tagStore.Update(ut)
		}

		if err != nil {
			nErrs++
			err = nil
		}

		// add grandchildren
		nErrs += app.addChildTags(st, ut, user)
	}
	return nErrs
}

// addTagRef adds a tag to a slideshow. Errors are logged and ignored. Returns false if the user lacks permission.
// #### Use of byUser to assign changes to users needs a complete rethink.
func (app *Application) addTagRef(parent int64, forUser int64, path []string, slideshow int64, byUser int64, detail string, create bool) bool {

	// only root tags are held by a user
	if parent != 0 {
		forUser = 0
	}

	// lookup tag
	t := app.getTag(parent, forUser, path)
	if t == nil {
		return false
	}

	// is reference already set
	if app.tagRefStore.Exists(slideshow, t.Id) {
		return true
	}

	// link tag to slideshow
	r := &models.TagRef{
		Slideshow: slideshow,
		Tag:       t.Id,
		Added:     time.Now(),
		Detail:    detail,
	}

	// optional user
	if byUser != 0 {
		r.User.Int64 = byUser
		r.User.Valid = true
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
func (app *Application) addTagRefAll(forUser int64, path []string, slideshow int64, detail string) bool {

	ok := true
	if forUser < 0 {
		// all users holding the root tag, except this one
		us := app.userStore.ForTag(path[0])
		for _, u := range us {
			if u.Id != -forUser {
				if !app.addTagRef(0, u.Id, path, slideshow, 0, detail, false) {
					return false
				}
			}
		}

	} else {
		ok = app.addTagRef(0, forUser, path, slideshow, 0, detail, false)
	}
	return ok
}

// childSlideshowTags returns a user's referenced child tags for a slideshow.
func (app *Application) childSlideshowTags0(parent int64, user int64, slideshow int64) []*slideshowTag {

	var fts []*slideshowTag

	ts := app.tagStore.ForParent(parent)
	for _, t := range ts {

		cts := app.childSlideshowTags(t.Id, user, slideshow)

		// include tag that is referenced, or has referenced children
		if len(cts) > 0 || app.tagRefStore.Exists(slideshow, t.Id) {

			ft := &slideshowTag{
				id:       t.Id,
				parent:   t.Parent,
				name:     t.Name,
				format:   t.Format,
				children: cts,
				set:      true,
			}
			fts = append(fts, ft)
		}
	}
	return fts
}

// childFormTags returns the child tags for an edit tag.
func (app *Application) childEditTags(parent int64, user int64, slideshow int64) []*slideshowTag {

	var fts []*slideshowTag

	ts := app.tagStore.ForParent(parent)
	for _, t := range ts {

		ft := &slideshowTag{
			id:       t.Id,
			parent:   t.Parent,
			name:     t.Name,
			format:   t.Format,
			children: app.childEditTags(t.Id, user, slideshow),
			edit: 	  true,
			set:      app.tagRefStore.Exists(slideshow, t.Id),
		}
		fts = append(fts, ft)
	}
	return fts
}

// deleteTagRef finds and deletes the specified tag reference, if it exists.
func (app *Application) deleteTagRef(user int64, slideshow int64, path []string) error {

	// find tag
	t := app.getTag(0, user, path)
	if t == nil {
		return nil // ok
	}

	// delete reference
	return app.tagRefStore.DeleteIf(slideshow, t.Id)
}

// deleteTagRefAll finds and deletes any specified tag references. A negative user ID removes references for all users.
func (app *Application) deleteTagRefAll(user int64, slideshow int64, path []string) bool {

	if user < 0 {
		// all users with this root tag
		ts := app.tagStore.ForName(0, path[0])
		for _, t := range ts {
			if app.deleteTagRef(t.User, slideshow, path) != nil {
				return false
			}
		}
	} else {
		return app.deleteTagRef(user, slideshow, path) == nil
	}
	return true
}

// editableTag returns the editable tags corresponding to the specified tag.
func (app *Application) editableTags(tag *models.Tag, userId int64) []*models.Tag {

	var ets []*models.Tag

	// actions specify the editable tags
	as := parseActions(tag.Action, userId, tag.Name)
	for _, a := range as {

		if a.code == "$" {
			if a.path[0] == "#" {
				ets = append(ets, tag) // this tag

			} else if a.path[0] == "-" {
				ets = append(ets, app.tagStore.GetIf(tag.Parent)) // parent tag

			} else {
				ets = append(ets, app.getTag(0, userId, a.path)) // tag specified by path
			}
		}
	}
	return ets
}

// getTag returns a tag.
func (app *Application) getTag(parent int64, forUser int64, path []string) *models.Tag {

	// lookup path
	var t *models.Tag
	var err error
	p := parent
	for _, name := range path {
		t, err = app.tagStore.GetNamed(p, name, forUser)
		if err != nil {
			return nil
		}
		if t == nil {
			break
		}
		// next (only root has the user ID)
		p = t.Id
		forUser = 0
	}

	return t
}

// parseActions parses an action specification and returns a slide of actions.
// #### Implement notify all except current user.
func parseActions(spec string, user int64, tag string) []action {

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
