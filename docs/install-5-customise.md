## Step 5: Customise your website
Add files in `/srv/picinch/site/` to customise your installation. You must restart the service for changes to take effect.
### Templates
Files in `templates/` define Go templates to specify static content for your site. All files with the name `*.tmpl` are processed. Typically a single `site.tmpl` file is sufficient. See [Template Files]({{ site.baseurl }}{% link template-examples.md %})
.

Go template files in `/serv/picinch/site/templates` specify static content for your website. Typically a single file, e.g. `site.tmpl` will be sufficient.

The following templates are intended to be redefined, and you will probably want to change at least `banner`, `homePage` and `website`:

**banner** Banner text on each page.

**copyrightNotice** Copyright statement for the Copyright and Privacy page.

**dataPrivacyNotice** Data privacy statement for the Copyright and Privacy page.

**favicons** Favicon links and meta tags. There is no need to redefine this if you use the same names as the default set.

**homePage** Site description shown on the home page.

**homePageMeta** Metadata for the home page.

**signupPage** Welcome text on the signup page.

**website** Website name, added to page titles and shown on log-in page.
See (Template Examples) for details.

Note that the default copyright and privacy statements were not written by someone with legal or data privacy expertise. You must review the text and decide if it is suitable for your website.  
### Graphics
Files in `images/` replace the default brand and favicon images for PicInch.

`brand.png` is the image shown on the site’s navbar. It should be 124px high. The width isn’t critical ; as a guide the default image is 558px wide.

[realfavicongenerator.net][1] was used to generate the default set of favicon files. If you want your own set, take care to generate all of these:
- android-chrome-192.png
- android-chrome-512.png
- apple-touch-icon.png
- favicon.ico
- favicon-16.png
- favicon-32.png
- mstile-150.png
- safari-pinned-tab.svg

The following may be left unchanged (although realfavicongenerator.net will make them for you):
- browserconfig.xml
- site.webmanifest

You may also add any images you wish to include in customised templates. `site/images/filename` will be served as `static/images/filename`.

### Other Configuration
Essential items are shown in docker-compose.yml. See (configuration.yml)  for the full set of options.

[1]:	https://realfavicongenerator.net