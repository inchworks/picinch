## Step 5: Customise your website
Add files in `/srv/picinch/site/` to customise your installation. You must restart the service for changes to take effect.

### Templates
Files in `templates/` define Go templates to specify static content for your site. Files with the names `*.partial.tmpl` override application templates. Typically a single `site.partial.tmpl` file is sufficient. See [Template Example]({{ site.baseurl }}{% link template-examples.md %})
.

The following templates are intended to be redefined:

**copyrightNotice** Copyright statement for the Copyright and Privacy page.

**dataPrivacyNotice** Data privacy statement for the Copyright and Privacy page.

**favicons** Favicon links and meta tags. There is no need to redefine this if you use the same names as the default set.

**homePageMeta** Metadata for the home page. See [Search Engine Settings]({{ site.baseurl }}{% link search-engine-settings.md %}).

**signupPage** Welcome text on the signup page.

**website** Website name shown on log-in page.

### Graphics
Files in `images/` replace the default brand and favicon images for PicInch.

`brand.png` is the image shown on the site’s navbar. It should be 124px high. The width isn’t critical ; as a guide the default image is 558px wide.

[realfavicongenerator.net][1] was used to generate the default set of favicon files. If you want your own set, take care to generate all of these:
- android-chrome-192x192.png
- android-chrome-512x512.png
- apple-touch-icon.png
- apple-touch-icon-152x152-precomposed.png
- favicon.ico
- favicon-16x16.png
- favicon-32x32.png
- mstile-150x150.png
- safari-pinned-tab.svg

The following may be left unchanged (although realfavicongenerator.net will make them for you):
- browserconfig.xml
- site.webmanifest

You may also add add additional images you wish to include in customised templates to `/images`. They will be served as `static/images/*`. These files are intended to be unchanging; dynamic content should go in
`/srv/picinch/misc`.

### Additional Content
Add additional information pages and menu items by logging on as a user with `admin` role. Specify the menu path for a page as `name` for a top-level menu item or `name.sub` for a dropdown menu item. A leading "`.`", i.e. `.name` or `.name.sub` specifies a page with no menu item. Pages are accessed by `https://<your-domain/info/name` or `info/name.sub`.

Similarly, diaries can be added by the administrator. Typically just one diary is needed. By default the next upcoming event in each diary is shown automatically on the home page. Diaries are accessed by `https://<your-domain/diary/name` or `diary/name.sub`.

Users with `curator` role can edit the content for information pages and diaries. Markdown is supported for the sections of an information page.

You may also add static pages with `templates/info-*.page.tmpl` files, and specify common page layouts with `*.layout.tmpl` files. A static page with template `templates/menu/name.page.tmpl` or `templates/menu/name.sub.page.tmpl` is added with a corresponding top-level or dropdown menu item. 
Use `info-notices.page.tmpl` and `gallery.layout.tmpl` as examples. Static pages are accessed by the same web addresses as editable information pages. Adding an information page overrides a static page with the same address.

Use `static/css` and `static/js` to hold any additional stylesheets and scripts needed by your static pages.
These are not included automatically; you will need to reference them as needed in your template files.

Note that additional templates and files should have different names to those used in the PicInch code, unless you intend to override the corresponding parts of PicInch.

### Configuration Parameters
The essential items are shown in docker-compose.yml. See [configuration.yml]({{ site.baseurl }}{% link configuration.yml.md %}) for the full set of options.

[1]:	https://realfavicongenerator.net