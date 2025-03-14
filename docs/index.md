# PicInch Gallery
This web server application provides a simple way for a group of members, such as a photography club, to display their photographs. The emphasis is on uncluttered presentation of images, organised into slideshows.

## Features
Contributors create simple slideshows. Each slide has a selection of title, photo and caption. Slides adjust in layout to match the content. There is a basic facility to edit and reorder slides.

Slideshows can include long video clips, up to a configurable file size, with background file compression by the server.
Apple MOV files are converted to MP4 automatically.

Slideshows may be organised into topics, as defined by a curator. Topics may be defined in advance, or applied to existing slideshows. A topic can be viewed as a single slideshow, or as a page listing the individual contributions.

Sign-up is restricted to a pre-set list of members, suitable for a club. Two user roles are defined. A `curator` can manage topics and create slideshows on behalf of users. An `admin` can add, suspend and remove users, and is also a curator.

Slideshows can be specified as publicly visible, or visible to website users, or hidden.

Individual photos can be added to a special topic “Highlights”, featured on the home page. It includes just the most recent highlights from each contributor.

The home page shows the most recent highlights and slideshows for each user. The number shown of each is configurable. A page for each contributor shows all their highlights and slideshows.

A set of recent highlight images are available for display on a parent website. See [Images for a Parent Website]({{ site.baseurl }}{% link images-for-parent.md %}) for details.

Other features include: Static pages can be added using Go templates.
Basic usage statistics are recorded daily and aggregated by month.
A `misc` directory is provided to serve miscellaneous content such as club reports.
A topic can be shared with a URL containing an access code.

**New for v1.3**
Information pages, a diary of events, and menu items can all be added and edited, making PicInch a full-featured image-oriented website. Upcoming events are automatically listed on the home page. [&#8658; Upgrading]({{ site.baseurl }}{% link upgrading.md %})

## Appearance
Display layout is responsive to device size (PC, tablet or phone).

Full size images are resized for display automatically. JPGs, PNGs and TIFs are supported.

## Screenshots

| Public page | Club home page | User's gallery | Edit slideshow |
|:-------------------------:|:-------------------------:|:-------------------------:|:-------------------------:|
|<a href="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-public.png"><img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-public.png" title="Public page" width="100%"></a>|<a href="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-club.png"><img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-club.png" title="Club home page" width="100%"></a>|<a href="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-my-gallery.png"><img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-my-gallery.png" title="User's gallery" width="100%"></a>|<a href="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-edit-slideshow.png"><img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-edit-slideshow.png" title="Edit slideshow" width="100%"></a>|

## Website Variants
Coming soon: set up PicInch in solo mode as an image-oriented website for a single owner.

PicInch can be configured as a standalone host for a public photo competition.
It provides multiple classes, a competition entry form and email address verification.
Judging and entry management is supported by tagging entries and a simple workflow management system.

## Implementation
Written in Go and ready to be deployed to a virtual machine using Docker, for good performance and easy setup. Suitable VMs are provided by IONOS, Digital Ocean, Linode, or Amazon Lightsail, typically costing around $2 to $6 per month with 10GB to 25GB of storage.
Uses a MariaDB or MySQL database.

Automatic support for HTTPS using Let’s Encrypt certificates. Can be configured as a sub-domain of an existing domain name. E.g. gallery.our-website.com.

Data privacy features:
- Records no additional personal data, and uses only two “strictly necessary” cookies, so that no visitor consent popup is needed for GDPR compliance.
- Contributors can limit slideshow visibility to signed-up website members.
- Users without public images are not publicly identified.

Security features:
- Passwords are stored using bcrypt hashing.
- Encrypted session data.
- CSRF protection.
- Log-in attempts are rate limited to mitigate attempts to guess passwords.
- All database queries are parameterised, to block SQL injection attacks.
- Directory listing of photos and other directories is disabled. (But photos can be accessed by guessing names.)
- Isolation between host OS, Go application and database is implicit in Docker-based operation.
- Statistics record the number of attempts to probe the site with bad URLs and query parameters. Details are recorded in a log.
- Geo-blocking can be set for specified countries. (This adds little real security, but does reduce the number of log entries from countries where the majority of traffic is malicious.)

**New** Client Cache-Control support:
- Browser cache max-age is configurable, with public caching allowed for public slideshows.
- Deleted images and slideshows are kept accessible for the lifetime of cached pages that may reference them.
- Browser If-Modified-Since requests are supported, to reduce server load for popular slideshows.

## Limitations
The following may be addressed in future updates:
- Support for competitions and judging requires the ability to edit the SQL database, as no forms have been implemented yet to setup tags and edit topic specifications.
- There is no provision to integrate competition mode into the normal operation as a photo gallery.
- HEIC images and HEVC videos are not supported.
- There is no convenient way for a curator to review members latest changes.

The following are unlikely to change:
- Photos can be viewed individually without logging in, if their names can be guessed. Names are not easy to guess, however.
- There is no password change or reset facility. However a user’s status can be reset to allow a new password to be specified on sign-up. Slideshows and images are preserved.
- No option to restrict highlight visibility to signed-up users.
- There is no facility to comment on images or to rate them.
- There is no bulk submission of images.
- There is no facility to hide contributions until they have been vetted. Members must be trusted to obey club rules on acceptable images.