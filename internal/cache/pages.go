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
	"html/template"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"

	"github.com/microcosm-cc/bluemonday"

	"inchworks.com/picinch/internal/models"
)

type MenuItem struct {
	Name string
	Path string
	Sub  []*MenuItem
}

type Diary struct {
	Page
	Id      int64
	Title   string
	Caption template.HTML
}

type Info struct {
	Page
	Id       int64
	Title    string
	Caption  template.HTML
	Sections []*Section
}

type Page struct {
	MetaTitle   string
	Description string
	NoIndex     bool
}

type Section struct {
	Title template.HTML
	Div   template.HTML
	Media string
}

type item struct {
	name string // string case as specified
	path string
	sub  map[string]*item
}

type PageCache struct {
	MainMenu []*MenuItem // top menu sorted

	Diaries map[string]*Diary // path -> diary
	Files   map[string]string // path -> filename
	Infos   map[string]*Info  // path -> information page
	Paths   map[int64]string  // slideshow ID -> path (for editing)

	mainMenu map[string]*item // top menu indexed
}

// '.' and '/' separate menu names.
// '_' is a space (typically in a file name)
var normaliser = strings.NewReplacer("/", ".", "_", " ")

var mdRenderer = html.NewRenderer(html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank})

// HTML sanitizer, used for titles and captions
var sanitizer = bluemonday.UGCPolicy()

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
func (pc *PageCache) AddPage(p *models.PageSlideshow, sections []*models.Slide) []string {

	var path string
	var warn []string
	switch p.PageFormat {
	case models.PageHome:
		path = "/" // no menu entry

	case models.PageDiary:
		// encode path and add to menu
		path, warn = pc.addPage("/diary/", p.Menu)

	case models.PageInfo:
		// encode path and add to menu
		path, warn = pc.addPage("/info/", p.Menu)

	default:
		return nil // unknown
	}

	if path == "" {
		return warn
	}

	// add to item maps
	pc.Paths[p.Id] = path

	// cache contents
	switch p.PageFormat {

	case models.PageDiary:
		pc.SetDiary(p, sections)

	case models.PageHome, models.PageInfo:
		pc.SetInformation(p, sections)
	}
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
		Diaries:  make(map[string]*Diary, 2),
		Files:    make(map[string]string, 8),
		Infos:    make(map[string]*Info, 8),
		Paths:    make(map[int64]string, 8),
		mainMenu: make(map[string]*item, 8),
	}
}

// Sanitize makes user input safe to display as HTML
func (pc *PageCache) Sanitize(unsafe string) string {
	return sanitizer.Sanitize(unsafe)
}

// SetDiary updates a diary's content in the cache.
func (pc *PageCache) SetDiary(page *models.PageSlideshow, _ []*models.Slide) {

	d := &Diary{
		Page: Page{
			MetaTitle:   page.MetaTitle,
			Description: page.Description,
			NoIndex:     page.NoIndex,
		},
		Id:      page.Id,
		Title:   page.Title,
		Caption: toHTML(page.Caption),
	}
	// ## don't cache events because they change often?

	// default metadata title
	if d.MetaTitle == "" {
		d.MetaTitle = d.Title
	}

	path := pc.Paths[page.Id]
	if path == "" {
		panic("Lost ID for diary in cache.")
	}
	pc.Diaries[path] = d
}

// SetInformation updates an information page's content in the cache.
func (pc *PageCache) SetInformation(page *models.PageSlideshow, sections []*models.Slide) {

	p := &Info{
		Page: Page{
			MetaTitle:   page.MetaTitle,
			Description: page.Description,
			NoIndex:     page.NoIndex,
		},
		Id:      page.Id,
		Title:   page.Title,
		Caption: toHTML(page.Caption), // #### don't need a page caption
	}

	setSections(sections, p)

	// default metadata title (home page has a different default)
	if p.MetaTitle == "" && page.PageFormat == models.PageInfo {
		p.MetaTitle = p.Title
	}
	
	path := pc.Paths[page.Id]
	if path == "" {
		panic("Lost ID for page in cache.")
	}
	pc.Infos[path] = p
}

// SetSections updates an information page's sections in the cache.
func (pc *PageCache) SetSections(showId int64, sections []*models.Slide) {

	path := pc.Paths[showId]
	if path == "" {
		panic("Lost ID for page in cache.")
	}
	setSections(sections, pc.Infos[path])
}

// SetMetadata updates a diary or information pages' metadata in the cache.
func (pc *PageCache) SetMetadata(page *models.PageSlideshow) {

	path := pc.Paths[page.Slideshow.Id]
	if path == "" {
		panic("Lost ID for metadata in cache.")
	}

	// cache contents
	switch page.PageFormat {

	case models.PageDiary:
		setMetadata(page, &pc.Diaries[path].Page)

	case models.PageHome, models.PageInfo:
		setMetadata(page, &pc.Infos[path].Page)
	}
}

// addMenu recusively adds page menu names to menu maps.
func addMenu(names []string, prefix string, path string, to map[string]*item, warn []string) []string {

	name := names[0]
	ncb := strings.ToLower(name) // for case-blind index
	m, exists := to[ncb]

	if len(names) == 1 {
		// add leaf
		if exists {
			if m.path != "" {
				warn = append(warn, `Menu item "`+name+`" redefined`)
			} else {
				warn = append(warn, `Menu dropdown "`+name+`" replaced`)
			}
		}
		to[ncb] = &item{name: name, path: prefix + url.PathEscape(path)} // construct path

	} else {
		// parent item
		if exists {
			if m.path != "" {
				warn = append(warn, `Menu dropdown replaces "`+name+`"`)
			}
		} else {
			// add new parent
			m = &item{name: name, sub: make(map[string]*item, 3)}
			to[ncb] = m
		}
		// sub-menu
		warn = addMenu(names[1:], prefix, path, m.sub, warn)
	}
	return warn
}

// addPage adds an ID or file as a menu item.
func (pc *PageCache) addPage(prefix string, spec string) (path string, warn []string) {

	warn = make([]string, 0)

	// normalise menu item names
	spec = normaliser.Replace(spec)

	// elements of path
	es := strings.Split(spec, ".")
	if len(es) > 2 {
		warn = append(warn, `Menu for "`+spec+`" too deep`)
		return
	}

	toMenu := true

	for i, e := range es {

		// simplify whitespace
		ws := strings.Fields(e) // words
		e = strings.Join(ws, " ")

		// check for blank elements
		if len(e) == 0 {
			if i == 0 && len(es) > 1 {
				toMenu = false // ".name" is a page without a menu item
			} else {
				warn = append(warn, `Blank menu item for "`+spec+`"`)
				return
			}
		}
		es[i] = e
	}
	if toMenu {
		path = strings.Join(es, ".")
	} else {
		path = strings.Join(es[1:], ".")
	}

	// simplify path for page address
	path = simplify(path)

	// add to menu
	if toMenu {
		warn = addMenu(es, prefix, path, pc.mainMenu, warn)
	}

	return
}

// buildMenu recursively builds sorted menu lists from menu maps.
func buildMenu(from map[string]*item) (to []*MenuItem) {

	// menu to Item
	for _, it := range from {
		item := &MenuItem{Name: it.name, Path: it.path}
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

// setMetadata updates common metadata for diary and information pages
func setMetadata(from *models.PageSlideshow, to *Page) {
	// ## why not cache whole records? Just because of markdown processing?
	to.MetaTitle = from.MetaTitle
	to.Description = from.Description
	to.NoIndex = from.NoIndex
}

func setSections(sections []*models.Slide, to *Info) {

	toS := make([]*Section, 0, len(sections))

	for _, s := range sections {
		cs := &Section{
			Title: template.HTML(s.Title), // #### don't need a title
			Div:   toHTML(s.Caption),
			Media: s.Image,
		}
		toS = append(toS, cs)
	}
	to.Sections = toS
}

// simplify returns a lower-case path with spaces and '-' characters replaced by single '-' characters.
func simplify(path string) string {
	var b strings.Builder
	var last rune

	for _, r := range path {
		r = unicode.ToLower(r)
		if r == ' ' {
			r = '-'
		}

		if r != '-' || r != last {
			b.WriteRune(r)
			last = r
		}
	}

	return b.String()
}

// toHTML converts markdown to HTML and sanitises it.
func toHTML(md string) template.HTML {
	mdParser := parser.NewWithExtensions(parser.CommonExtensions | parser.NoEmptyLineBeforeBlock)

	doc := mdParser.Parse([]byte(md))

	unsafe := markdown.Render(doc, mdRenderer)
	html := sanitizer.SanitizeBytes(unsafe)
	return template.HTML(html)
}
