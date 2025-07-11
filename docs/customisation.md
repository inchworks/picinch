## Customise Your Website
This section is intended to help users with a basic knowledge of CSS, HTML, and Go templates. It is not needed to make a working website. The first sections below need less knowledge than the later ones.

Add files in `/srv/picinch/site/` to customise your installation. Restart PicInch for changes to take effect. (See Commands.)

### Graphics
Files in `images/` replace the default brand and favicon images for PicInch.

`brand.png` is the image shown on the site’s navbar. It should be 124px high. The width isn’t critical ; as a guide the default image is 558px wide.

[realfavicongenerator.net][1] was used to generate the default set of favicon files. If you want your own set, take care to generate all of these:
- apple-touch-icon.png
- favicon-96x96.png
- favicon.ico
- favicon.svg
- web-app-manifest-192x192.png
- web-app-manifest-512x512.png

site.webmanifest may be left unchanged, although realfavicongenerator.net will make it for you.

You may also add add additional images you wish to include in customised templates to `images/`. They will be served as `static/images/*`. These files are intended to be unchanging; dynamic content should go in
`/srv/picinch/misc`.

### Configuration Parameters
The essential items can be set in docker-compose.yml. For a more specific configuration add a `configuration.yml` file. See [configuration.yml]({{ site.baseurl }}{% link configuration.yml.md %}) for the full set of options.

### Appearance
Colours and fonts can be changed by adding CSS rules using Go templates. Add a template file `site.partial.tmpl` to `templates/`. There are two templates that can be redefined:

**siteStyle** Colours and fonts for site pages.

**siteSlideStyle** Background colour and font for slideshows.

Use [CSS Example]({{ site.baseurl }}{% link css-example.md %}) as a starting point and change colours and fonts as desired. Note that:

1. All the CSS rules in the examples must be specified, because each template replaces a full set of default rules.

1. The examples show a `<code>` tag containing CSS rules. It is also possible to include `<link>` tags. For example, to install Google fonts.

### Other Templates Changes
Most content can be set more easily by logging on as administrator [&#8658; Site Administrator]({{ site.baseurl }}{% link administrator.md %}). However Go templates can be defined to change page layouts and content. This feature is provided mainly for compatibility with previous versions.

Files in `templates/` define Go templates to specify static content for your site. Files with the names `*.partial.tmpl` override application templates. Typically a single `site.partial.tmpl` file is sufficient. See [Template Example]({{ site.baseurl }}{% link template-examples.md %}).

The following templates are intended to be redefined:

**copyrightNotice** Copyright statement for the Copyright and Privacy page.

**dataPrivacyNotice** Data privacy statement for the Copyright and Privacy page.

**favicons** Favicon links and meta tags. There is no need to redefine this if you use the same names as the default set.

**signupPage** Welcome text on the signup page.

**website** Website name shown on log-in page.

### Static Pages
Additional pages are added more easily by logging on as administrator [&#8658; Site Administrator]({{ site.baseurl }}{% link administrator.md %}). However static pages can be used to get full control over page layouts.

Add static pages with `templates/info-*.page.tmpl` files, and specify common page layouts with `*.layout.tmpl` files. A static page with template `templates/menu/name.page.tmpl` or `templates/menu/name.sub.page.tmpl` is added with a corresponding top-level or dropdown menu item. 
Use `info-notices.page.tmpl` and `gallery.layout.tmpl` as examples. Static pages are accessed by the same web addresses as editable information pages. Adding an information page overrides a static page with the same address.

Use `static/css` and `static/js` to hold any additional stylesheets and scripts needed by your static pages.
These are not included automatically; you will need to reference them as needed in your template files.

Note that additional templates and files should have different names to those used in the PicInch code, unless you intend to override the corresponding parts of PicInch.

[1]:	https://realfavicongenerator.net