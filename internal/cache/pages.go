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
	Sections []*Section
	SubPages []*SubPage
}

type Page struct {
	MetaTitle   string
	Description string
	NoIndex     bool
}

type Section struct {
	Div    template.HTML
	Format int
	Layout int
	Media  string
	Cards  []*Section // row of cards
}

type SubPage struct {
	Path        string        // from page
	Title       string        // from slideshow
	Description template.HTML // from 1st section caption
	Media       string        // from 1st section
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
func (pc *PageCache) AddPage(p *models.PageSlideshow, sections []*models.Slide, subPages []*models.SubPage) []string {

	var path string
	var warn []string
	switch p.PageFormat {
	case models.PageHome:
		path = "/" // no menu entry

	case models.PageDiary:
		// encode path and add to menu
		path, warn = pc.addPage("/diary/", p.Name)

	case models.PageInfo:
		// encode path and add to menu
		path, warn = pc.addPage("/info/", p.Name)

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
		pc.SetInformation(p, sections, subPages)
	}
	return warn
}

// BuildMenus makes ordered lists of menu items.
func (pc *PageCache) BuildMenus() {
	pc.MainMenu = buildMenu(pc.mainMenu)
	pc.mainMenu = nil
}

// NewPageCache returns an empty page cache
func NewPageCache() *PageCache {
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
func (pc *PageCache) SetInformation(page *models.PageSlideshow, sections []*models.Slide, subPages []*models.SubPage) {

	p := &Info{
		Page: Page{
			MetaTitle:   page.MetaTitle,
			Description: page.Description,
			NoIndex:     page.NoIndex,
		},
		Id:    page.Id,
		Title: page.Title,
	}

	setSections(sections, p)
	setSubPages(subPages, p)

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
	// check if it is an info page, and not a diary
	pg := pc.Infos[path]
	if pg != nil {
		setSections(sections, pc.Infos[path])
	}
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

// addCard adds a card to a row of cards, and updates the row in the page
func addCard(card *Section, row *Section, sections []*Section) (*Section, []*Section) {

	if row == nil {
		// start new row
		row = &Section{
			Layout: card.Layout,
			Cards:  make([]*Section, 0, 2),
		}
	}

	// add card to group
	row.Cards = append(row.Cards, card)

	// add new row
	if len(row.Cards) == 1 {
		sections = append(sections, row)
	}

	// current group and updated sections
	return row, sections
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

				// change parent to dropdown
				m.path = ""
				m.sub = make(map[string]*item, 3)
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
func (pc *PageCache) addPage(prefix string, name string) (path string, warn []string) {

	var isMenu bool
	var es []string

	// make path from elements of normalised page name
	path, es, isMenu, warn = toPathMenu(name)
	if len(warn) > 0 {
		return
	}

	// add to menu
	if isMenu {
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

// closeRow sets column width for the row.
func closeRow(r *Section) {
	switch len(r.Cards) {
	case 1:
		r.Format = 12
	case 2:
		r.Format = 6
	case 3:
		r.Format = 4
	default:
		r.Format = 3
	}
}

// sectionFormat returns a section's auto format.
func sectionFormat(fmt int) int {
	return fmt & (models.SlideDocument + models.SlideImage + models.SlideVideo)
}

// sectionLayout returns a section's manual format.
func sectionLayout(fmt int) int {
	l := fmt >> models.SlideFormatShift
	switch l {
	case models.SlideCard, models.SlideEvents, models.SlideSlideshows, models.SlideHighlights, models.SlideSubPages, models.SlidePageShows:
		return l

	default:
		if fmt&(models.SlideDocument+models.SlideImage+models.SlideVideo) == 0 {
			return 0 // default layout with no media
		}
	}
	return l
}

// setMetadata updates common metadata for diary and information pages.
func setMetadata(from *models.PageSlideshow, to *Page) {
	// ## why not cache whole records? Just because of markdown processing?
	to.MetaTitle = from.MetaTitle
	to.Description = from.Description
	to.NoIndex = from.NoIndex
}

// setSections sets the text+media sections for an information page.
func setSections(sections []*models.Slide, to *Info) {

	var row *Section // current row of cards
	toS := make([]*Section, 0, len(sections))

	for _, s := range sections {
		cs := &Section{
			Div:    toHTML(s.Caption),
			Format: sectionFormat(s.Format),
			Layout: sectionLayout(s.Format),
			Media:  s.Image,
		}

		if cs.Layout == models.SlideCard {
			row, toS = addCard(cs, row, toS)

		} else {
			// close row
			if row != nil {
				closeRow(row)
				row = nil
			}

			// single section
			toS = append(toS, cs)
		}
	}
	to.Sections = toS
}

// setSubPages sets the sub-pages for an information page.
func setSubPages(subs []*models.SubPage, to *Info) {

	to.SubPages = make([]*SubPage, 0, len(subs))

	for _, s := range subs {

		// elements of normalised page name
		path, _, _, _ := toPathMenu(s.Name)

		sp := &SubPage{
			Path:        "/info/" + path,
			Title:       s.Title,
			Description: toHTML(s.Caption),
			Media:       s.Image,
		}
		to.SubPages = append(to.SubPages, sp)
	}
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

// toPath makes an HTML path and menu elements from a page or file name.
func toPathMenu(name string) (path string, es []string, isMenu bool, warn []string) {

	warn = make([]string, 0)

	// normalise menu item names
	name = normaliser.Replace(name)

	// elements of path
	es = strings.Split(name, ".")

	isMenu = true

	for i, e := range es {

		// simplify whitespace
		ws := strings.Fields(e) // words
		e = strings.Join(ws, " ")

		// check for blank elements
		if len(e) == 0 {
			if i == 0 && len(es) > 1 {
				isMenu = false // ".name" is a page without a menu item
			} else {
				warn = append(warn, `Blank element in "`+name+`"`)
				return
			}
		}
		es[i] = e
	}
	if isMenu {
		if len(es) > 2 {
			warn = append(warn, name+`" has too many elements`)
			return
		}
		path = strings.Join(es, ".")
		
	} else {
		path = strings.Join(es[1:], ".")
	}

	// simplify path for page address
	path = simplify(path)

	return
}
