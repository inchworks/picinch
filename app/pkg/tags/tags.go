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

package tags

// Processing for workflow management using tags.

// These functions may modify application state.
// Parameters are defined in the order: item, tag, user.

import (
	"database/sql"
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"inchworks.com/picinch/pkg/models"
	"inchworks.com/picinch/pkg/models/mysql"
)

type Tagger struct {
	ErrorLog  *log.Logger
	TagStore       *mysql.TagStore
	TagRefStore    *mysql.TagRefStore
	UserStore      *mysql.UserStore
}

type action struct {
	code    string
	forUser int64
	path    []string
}

type ItemTag struct {
	Id       int64
	Parent   int64
	Name     string
	Format   string
	Children []*ItemTag
	Edit     bool
	Set      bool
}

// ChildSlideshowTags returns a user's editable tags for a item.
func (tgr *Tagger) ChildSlideshowTags(item int64, parent int64, user int64, toEdit bool) []*ItemTag {

	var fts []*ItemTag

	ts := tgr.TagStore.ForParent(parent)
	for _, t := range ts {
		var isEdit, isSet bool
		var cts []*ItemTag

		if isSet = tgr.TagRefStore.Exists(item, t.Id, user); isSet {

			if toEdit {
				// get the tags that can be edited, instead of the one that is set
				ets := tgr.editableTags(t, user)
				if len(ets) > 0 {
					// editable tag (just one supported currently)
					t = ets[0]
					cts = tgr.childEditTags(item, t.Id, user)
					isEdit = true
					isSet = false
				}
			}
		}

		if !isEdit {
			// child tags for non-editable tag
			cts = tgr.ChildSlideshowTags(item, t.Id, user, toEdit)
		}

		if len(cts) == 1 && cts[0].Id == t.Id {

			// A child tag may have returned this tag as editable. We don't need it twice.
			fts = append(fts, cts[0])

		} else if len(cts) > 0 {

			// include the tag if has referenced children
			fts = append(fts, &ItemTag{
				Id:       t.Id,
				Parent:   t.Parent,
				Name:     t.Name + " : ",
				Format:   t.Format,
				Children: cts,
				Edit:     isEdit,
				Set:      isSet,
			})

		} else if isSet {

			// include the tag if it is referenced
			fts = append(fts, &ItemTag{
				Id:     t.Id,
				Parent: t.Parent,
				Name:   t.Name,
				Format: t.Format,
				Edit:   isEdit,
				Set:    true,
			})
		}
	}
	return fts
}

// DropTagRef removes a tag, and adds any successor tags. Returns false if the user lacks permission.
func (tgr *Tagger) DropTagRef(item int64, parent int64, name string, user int64) bool {

	t := tgr.TagStore.GetNamed(parent, name)
	if t == nil {
		return false
	}

	// add successor tag references
	ok := true
	as := parseActions(t.Action, name, user)
	for _, a := range as {
		switch a.code {

		case ">":
			if !tgr.addTagRefAll(item, a.path, a.forUser, "") {
				ok = false
			}
		}
	}

	// remove reference (OK if doesn't exist)
	if err := tgr.TagRefStore.DeleteIf(item, t.Id, user); err != nil {
		tgr.log(err)
		return false
	}
	return ok
}

// FormTags returns the tags to be changed on a form.
func (tgr *Tagger) FormTags(item int64, refTag *models.Tag, user int64) []*ItemTag {

	var fts []*ItemTag

	// actions specify the modifiable tags
	// ## only current user, so couldn't be called by a curator
	ets := tgr.editableTags(refTag, user)
	for _, t := range ets {

		ft := &ItemTag{
			Id:       t.Id,
			Parent:   t.Parent,
			Name:     t.Name,
			Format:   t.Format,
			Children: tgr.childEditTags(item, t.Id, user),
			Set:      tgr.TagRefStore.Exists(item, t.Id, user),
		}
		fts = append(fts, ft)
	}

	return fts
}

// HasPermission returns true if the user has a permission reference to the tag.
func (tgr *Tagger) HasPermission(rootId int64, userId int64) bool {
	return tgr.TagRefStore.HasPermission(rootId, userId)
}

// Names returns the parent name and the name, for a tag.
func (tgr *Tagger) Names(tagId int64) (parentName string, tagName string) {

	// tag name
	t := tgr.TagStore.GetIf(tagId)
	if t == nil {
		return
	}
	tagName = t.Name

	// parent tag name
	if t.Parent != 0 {
		p := tgr.TagStore.GetIf(t.Parent)
		if p != nil {
			parentName = p.Name + " : "
		}
	}
	return
}

// setTagRef adds a tag to a item. Returns false if the user lacks permission. Errors are logged and ignored.
func (tgr *Tagger) SetTagRef(item int64, parent int64, name string, user int64, detail string) bool {

	// actions specified by parent
	if parent != 0 {
		p := tgr.TagStore.GetIf(parent)
		if p == nil {
			return false
		}

		if !tgr.doSetActions(p.Action, item, name, user) {
			return false
		}
	}

	// actions specified by tag
	t := tgr.TagStore.GetNamed(parent, name)
	if t == nil {
		return false
	}
	if !tgr.doSetActions(t.Action, item, name, user) {
		return false
	}

	// add tag reference
	ok := tgr.addTagRef(item, t.Id, user, detail, true)
	return ok
}


// INTERNAL FUNCTIONS

// addTagRef adds a tag to a item. Errors are logged and ignored. Returns false if the user lacks permission.
// user is 0 for a system tag.
func (tgr *Tagger) addTagRef(item int64, tagId int64, user int64, detail string, create bool) bool {

	// is reference already set
	if tgr.TagRefStore.Exists(item, tagId, user) {
		return true
	}

	// link tag to item
	r := &models.TagRef{
		Item:      sql.NullInt64{Int64: item, Valid: true},
		Tag:       tagId,
		Added:     time.Now(),
		Detail:    detail,
	}

	if user != 0 {
		r.User = sql.NullInt64{Int64: user, Valid: true}
	}

	err := tgr.TagRefStore.Update(r)
	if err != nil {
		tgr.log(err)
		return false
	}
	return true
}

// addTagRefAll adds a tags to a item. A negative user ID of selects all users having the root tag, except the specified user.
// Errors are logged and ignored.
func (tgr *Tagger) addTagRefAll(item int64, path []string, user int64, detail string) bool {

	// lookup tag
	t := tgr.getTag(path)
	if t == nil {
		return false
	}

	ok := true
	if user < 0 {
		// all users holding the root tag, except this one
		us := tgr.UserStore.ForTagName(path[0])
		for _, u := range us {
			if u.Id != -user {
				if !tgr.addTagRef(item, t.Id, u.Id, detail, false) {
					return false
				}
			}
		}

	} else {
		ok = tgr.addTagRef(item, t.Id, user, detail, false)
	}
	return ok
}

// childFormTags returns the child tags for an edit tag.
func (tgr *Tagger) childEditTags(item int64, parent int64, user int64) []*ItemTag {

	var fts []*ItemTag

	ts := tgr.TagStore.ForParent(parent)
	for _, t := range ts {

		ft := &ItemTag{
			Id:       t.Id,
			Parent:   t.Parent,
			Name:     t.Name,
			Format:   t.Format,
			Children: tgr.childEditTags(item, t.Id, user),
			Edit:     true,
			Set:      tgr.TagRefStore.Exists(item, t.Id, user),
		}
		fts = append(fts, ft)
	}
	return fts
}

// deleteTagRef finds and deletes the specified tag reference, if it exists.
func (tgr *Tagger) deleteTagRef(item int64, path []string, user int64) error {

	// find tag
	t := tgr.getTag(path)
	if t == nil {
		return nil // ok
	}

	// delete reference
	return tgr.TagRefStore.DeleteIf(item, t.Id, user)
}

// deleteTagRefAll finds and deletes any specified tag references. A negative user ID removes references for all users.
func (tgr *Tagger) deleteTagRefAll(item int64, path []string, user int64) bool {

	if user < 0 {
		// all users with this root permission
		us := tgr.UserStore.ForTagName(path[0])
		for _, u := range us {
			if tgr.deleteTagRef(item, path, u.Id) != nil {
				return false
			}
		}
	} else {
		return tgr.deleteTagRef(item, path, user) == nil
	}
	return true
}

// doSetActions does the actions needed when a tag is set.
func (tgr *Tagger) doSetActions(spec string, slideshowId int64, tag string, userId int64) bool {

	as := parseActions(spec, tag, userId)
	for _, a := range as {
		switch a.code {
		case "<":
			// remove predecessor(s)
			if !tgr.deleteTagRefAll(slideshowId, a.path, a.forUser) {
				return false
			}

		case "!":
			// add notification
			if !tgr.addTagRefAll(slideshowId, a.path, a.forUser, "") {
				return false
			}
		}
	}
	return true
}

// editableTag returns the editable tags corresponding to the specified tag.
func (tgr *Tagger) editableTags(tag *models.Tag, userId int64) []*models.Tag {

	var ets []*models.Tag

	// actions specify the editable tags
	as := parseActions(tag.Action, tag.Name, userId)
	for _, a := range as {

		if a.code == "$" {
			if a.path[0] == "#" {
				ets = append(ets, tag) // this tag

			} else if a.path[0] == "-" {
				ets = append(ets, tgr.TagStore.GetIf(tag.Parent)) // parent tag

			} else {
				ets = append(ets, tgr.getTag(a.path)) // tag specified by path
			}
		}
	}
	return ets
}

// getTag returns a tag for a path.
func (tgr *Tagger) getTag(path []string) *models.Tag {

	// lookup path
	var t *models.Tag
	p := int64(0)
	for _, name := range path {
		t = tgr.TagStore.GetNamed(p, name)
		if t == nil {
			return nil
		}		
		p = t.Id  // next in path
	}

	return t
}

// log records an error.
func (tgr *Tagger) log(err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	tgr.ErrorLog.Output(2, trace)
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
