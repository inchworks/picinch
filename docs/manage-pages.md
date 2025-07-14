# Creating Pages
The administrator can add additional information pages and menu items with `Admin > Pages`. To keep site structure simple, Picinch supports just two levels of page: main pages and and sub-pages. Specify the menu path for a page as `name` for a top-level menu item or `name.sub` for a dropdown menu item.

Main pages can be accessed in two ways:

1. Usually a page with `name` has a top-level menu item. A leading "`.`", i.e. `.name` specifies a page without a menu item.

1. Pages can be referenced by `https://<your-domain>/info/name` from other websites or by `/info/name.sub` in links from local pages. Omit the leading `.` for a page without a menu item.

Sub-pages can be accessed in three ways:

1. Usually a page with `name.sub` has a dropdown menu item, under `name`. A leading "`.`", i.e. `.name.sub` specifies a sub-page without a menu item. Note that for simplicity ...

1. Sub-pages without menu items can be listed as summary cards on the parent main page. Use the special section format `subpages` (below) on the main page. 

1. Pages can be referenced by `https://<your-domain>/info/name.sub` from other websites or by `/info/name.sub` in links from local pages. Omit the leading `.` for a sub-page without a menu item.

Note that when a top level menu item has drop-down menu items it has no main page.

It is also possible to add static pages using Go templates. These take more effort to understand and change, but give full control over page layouts. See [Customisation]({{ site.baseurl }}{% link customisation.md %}).

## Diaries
Diaries for a club website can be added with `Admin > Diaries`. Typically just one diary is sufficient. By default the next upcoming event in each diary is shown automatically on the home page. Diaries are accessed by `https://<your-domain/diary/name` or `/diary/name.sub`.

## Page Metadata
Web pages need a title, to be shown in browser tabs, and search engine results. By default the title of each web page is the same as its heading, but you can change the title with `Admin > Pages -> Metadata`. Typically this is done when a shorter title is needed.

Information and diary pages that are to appear in search engine results should have a description. Set the description with `Admin > Pages -> Metadata` and `Admin > Diaries -> Metadata`. Alternatively you can request that a page should not be indexed by search engines by setting the `Block search indexing` checkbox.

