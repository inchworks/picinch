// Copyright © Rob Burke inchworks.com, 2024.

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

package cache

// Configurable site pages.

import (
	"net/url"
	"slices"
	"strings"

	"inchworks.com/picinch/internal/models"
)

type MenuItem struct {
	Name       string
	Path       string
	Sub        []*MenuItem
}

type item struct {
	path string
	sub  map[string]*item
}

type PageCache struct {
	MainMenu []*MenuItem // top menu sorted

	Diaries map[int64]*models.PageSlideshow
	Files   map[string]string // path -> filename
	Home    *models.PageSlideshow
	Pages   map[string]int64  // path -> page ID

	mainMenu map[string]*item // top menu indexed
}

// hold the same rules for pages by ID and pages by filename
var normaliser = strings.NewReplacer("/", "", "_", " ")

// AddFile adds a filename as a menu item.
// E.g prefix "menu-", suffix ".page.tmpl".
// It returns a list of warnings
func (pc *PageCache) AddFile(filename string, prefix string, suffix string) (warn []string) {

	if !strings.HasPrefix(filename, prefix) || !strings.HasSuffix(filename, suffix) {
		return nil // not a menu item template
	}
	name := filename[len(prefix) : len(filename)-len(suffix)] // remove prefix and suffix

	// add to menu
	path, w := pc.addPage("/info/", name)
	warn = w

	// add to files
	pc.Files[path] = filename
	return
}

// AddPage adds a page, optionally as a diary and/or a menu item.
// It returns a list of warnings.
func (pc *PageCache) AddPage(p *models.PageSlideshow) []string {

	var prefix string
	switch p.PageFormat {
	case models.PageHome:
		pc.Home = p
		return nil // no access by URL or menu

	case models.PageDiary:
		pc.Diaries[p.PageId] = p
		prefix = "/diary/"

	case models.PageInfo:
		prefix = "/info/"

	default:
		return nil // unknown
	}

	// encode path and add to menu
	path, warn := pc.addPage(prefix, p.Menu)

	// add to item map
	pc.Pages[path] = p.PageId

	return warn
}

// BuildMenus makes ordered lists of menu items.
func (pc *PageCache) BuildMenus() {
	pc.MainMenu = buildMenu(pc.mainMenu)
	pc.mainMenu = nil
}

// NewCache returns an empty page cache
func NewPages() *PageCache {
	return &PageCache{
		Diaries:  make(map[int64]*models.PageSlideshow, 2),
		Files:    make(map[string]string, 8),
		Pages:    make(map[string]int64, 8),
		mainMenu: make(map[string]*item, 8),
	}
}

// addMenu recusively adds page menu names to menu maps.
func addMenu(names []string, prefix string, path string, to map[string]*item, warn []string) []string {

	name := names[0]
	m, exists := to[name]

	if len(names) == 1 {
		// add leaf
		if exists {
			if m.path != "" {
				warn = append(warn, `Menu item "`+name+`" redefined.`)
			} else {
				warn = append(warn, `Menu dropdown "`+name+`" replaced.`)
			}
		}
		to[name] = &item{path: prefix + url.PathEscape(path)} // construct path

	} else {
		// parent item
		if exists {
			if m.path != "" {
				warn = append(warn, `Menu dropdown replaces "`+name+`".`)
			}
		} else {
			// add new parent
			m = &item{sub: make(map[string]*item, 3)}
			to[name] = m
		}
		// sub-menu
		warn = addMenu(names[1:], prefix, path, m.sub, warn)
	}
	return warn
}

// addpage adds an ID or file as a menu item.
func (pc *PageCache) addPage(prefix string, spec string) (path string, warn []string) {

	warn = make([]string, 0)

	// normalise names
	spec = normaliser.Replace(spec)

	// elements of path
	es := strings.Split(spec, ".")
	if len(es) > 2 {
		warn = append(warn, `Menu for "`+spec+`" too deep.`)
		return
	}

	toMenu := true

	for i, e := range es {
		es[i] = strings.TrimSpace(e)
		if len(es[i]) == 0 {
			if i == 0 && len(e) > 1 {
				toMenu = false // ".name" is a page without a menu item
			} else {
				warn = append(warn, `Blank menu item for "`+spec+`".`)
				return
			}
		}
		path += es[i]
	}

	// add to menu
	if toMenu {
		path = strings.ToLower(path) // non-menu items are capitalised as specified
		warn = addMenu(es, prefix, path, pc.mainMenu, warn)
	}

	return
}

// buildMenu recursively builds sorted menu lists from menu maps.
func buildMenu(from map[string]*item) (to []*MenuItem) {

	// menu to Item
	for name, it := range from {
		item := &MenuItem{Path: it.path, Name: strings.ToUpper(name)}
		to = append(to, item)

		// sub-menus
		if len(it.sub) > 0 {
			item.Sub = buildMenu(it.sub)
		}
	}

	// sort menu items
	slices.SortFunc(to, func(a, b *MenuItem) int {
		return strings.Compare(a.Name, b.Name)
	})
	return
}
