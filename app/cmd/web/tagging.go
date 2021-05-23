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
	tag     string
	forUser int64
}

// dropTagRef removes a tag, and adds any successor tags. Returns false if the user lacks permission.
func (app *Application) dropTagRef(parent int64, name string, forUser int64, slideshow int64) bool {

	// ## implement actions specified by parent

	t, err := app.tagStore.GetNamed(parent, name, forUser)
	if err != nil {
		app.log(err)
		return false
	}

	// add successor tag references
	ok := true
	as := parseActions(t.Action, name, forUser)
	for _, a := range as {
		switch a.code {

		case ">":
			if !app.addTagRef(parent, a.tag, a.forUser, slideshow, 0, "", true) {
				ok = false
			}
		}
	}

	// remove reference (OK if doesn't exist)
	if err := app.tagRefStore.DeleteIf(parent, name, forUser, slideshow); err != nil {
		// ## 
		app.log(err)
	}
	return ok
}

// setTagRef adds a tag to a slideshow. Errors are logged and ignored. Returns false if the user lacks permission.
func (app *Application) setTagRef(parent int64, name string, forUser int64, slideshow int64, byUser int64, detail string) bool {

	// actions specified by parent
	if parent != 0 {
		p := app.tagStore.GetIf(parent)
		if p == nil {
			return false
		}
		as := parseActions(p.Action, name, forUser)
		for _, a := range as {
			switch a.code {
			case "<":
				// remove predecessor(s)
				var err error
				if a.forUser == -1 {
					err = app.tagRefStore.DeleteAll(parent, a.tag, slideshow)
				} else {
					err = app.tagRefStore.DeleteIf(parent, a.tag, a.forUser, slideshow)
				}
				if err != nil {
					return false
				}

			case "!":
				// add notification
				if !app.addTagRef(parent, a.tag, a.forUser, slideshow, byUser, "", true) {
					return false
				}
			}
		}
	}

	// ## implement actions specified by tag (not useful unless we have a way to set up child tags)

	// add tag reference
	ok := app.addTagRef(parent, name, forUser, slideshow, byUser, detail, true)
	return ok
}

// INTERNAL FUNCTIONS

// addTagRef adds a tag to a slideshow. Errors are logged and ignored. Returns false if the user lacks permission.
func (app *Application) addTagRef(parent int64, name string, forUser int64, slideshow int64, byUser int64, detail string, create bool) bool {

	// lookup tag, creating it necessary
	t := app.getTag(parent, name, forUser, create)
	if t == nil {
		return false
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

// getTag returns a tag, creating it if necessary.
func (app *Application) getTag(parent int64, name string, forUser int64, create bool) *models.Tag {

	// lookup tag
	t, err := app.tagStore.GetNamed(parent, name, forUser)
	if err != nil {
		return nil
	}

	// ## auto-add missing tag, from system tag, if permitted?
	// #### Must walk tree from root tag. Better to create all tags on first reference to root tag.
	if t == nil && create {
		t = &models.Tag{
			Parent: parent,
			Name:   name,
			User:   forUser,
		}
		err = app.tagStore.Update(t)
		if err != nil {
			return nil // ## inconsistent logging
		}
	}

	return t
}

// parseActions parses an action specification and returns a slide of actions.
// #### Implement notify all except current user, and add for all with role tag.
func parseActions(spec string, tag string, user int64) []action {

	// items separated by whitespace
	ss := strings.Fields(spec)

	var as []action
	for _, s := range ss {

		// item is !tag:user, where ! is an action code
		cs := strings.Split(s, ":")

		a := action{
			code: cs[0][0:1],
			tag:  cs[0][1:],
		}

		// parameter tag
		if a.tag == "@" {
			a.tag = tag
		}

		// user-specific tag
		if len(cs) > 1 {
			switch cs[1] {
			case "@":
				a.forUser = user // parameter user

			case "*":
				a.forUser = -1 // all users

			default:
				a.forUser, _ = strconv.ParseInt(cs[1], 10, 64)
			}
		}
		as = append(as, a)
	}

	return as
}
