# PicInch Gallery
This web server application provides an image-oriented website. Optionally it provides a simple way for a group of members, such as a photography club, to share their photographs. The emphasis is on uncluttered presentation of images, organised into slideshows.

Information pages and menu items can be added and edited, making PicInch a full-featured website. The aim is to make website editing as simple as possible, not to provide precise control of the website appearance.

Three options are provided:
- Club mode shows slideshows attributed to individual contributors.
- Solo mode makes an image-oriented website for images by a single owner.
- Competition mode makes a standalone host for a public photo competition.

This application is written in Go and ready to be deployed to a virtual machine using Docker, for good performance and easy setup. Support for HTTPS is automatic, using Let’s Encrypt certificates. 

## Main Features
Contributors create simple slideshows. Each slide has a selection of title, image or video, and caption. Slides adjust in layout to match the content. There is a basic facility to edit and reorder slides.

Information pages are constructed in sections, each section having a selection of title, image or video, and text. The relative positioning of media and text in a section can be specified, and sections can be arranged as a rows of cards on a page. Markdown text format is supported.

Individual photos can be added to a special topic “Highlights”, featured on the home page. It includes just the most recent highlights from each contributor.

The home page is configurable. It can show information sections, recent highlights, and recent slideshows. The number shown of each is configurable.

Website layout is responsive to device size (PC, tablet or phone).

## Club Features
Sign-up is restricted to a pre-set list of members, suitable for a club. Two user roles are defined. A `curator` can manage topics and create slideshows on behalf of users. An `admin` can add, suspend and remove users, and is also a curator.

Members may create slideshows themselves or have them set up by a curator.
Slideshows can be specified as publicly visible, or visible to members, or hidden.

A page for each contributor shows all their highlights and slideshows.

Slideshows may be organised into topics, as defined by a curator. Topics may be defined in advance, or applied to existing slideshows. A topic can be viewed as a single slideshow, or as a page listing the individual contributions.

Diaries of events can be created, with upcoming events automatically listed on the home page.

The curator can review all contributions since a specified date and time. This is intended to support monitoring for online safety, but only as proportionate for a club where members are known and trusted.

## Solo Features
Typically a single admin user will manage the website. However a larger organisation can add additional admins, or curators who can add content but not re-configure the website.

Sign-up can be offered to a pre-set list of friends, who can view but not add or change slideshows.

Slideshows can be specified as publicly visible, or visible to friends, or hidden until ready.

Club limits on numbers and sizes of slideshows are disabled.

## Competition Features
PicInch can be configured as a standalone host for a public photo competition.

It provides:
- Multiple competion classes.
- A competition entry form
- Email address verification.
- Judging and entry management, supported by tagging entries and a simple workflow management system.

## Other Features
Full size images are converted and resized for display automatically. BMP, JPEG, PNG, TIFF and WEBP are supported, with conversion to JPEG and PNG.

There is a configurable file size limit for videos.
Video formats and codecs supported by FFmpeg, including Apple MOV files, are converted to MP4 and compressed automatically. This is done as a background activity, so may take a few minutes.

The site can be configured as a supplement to an organisation's parent website.
- The site may be addressed as a subdomain of the parent organisation (e.g. gallery.our-website.com). It can have its own domain address as well, if needed.
- A set of recent highlight images are available for display on a parent website. See [Images for a Parent Website]({{ site.baseurl }}{% link images-for-parent.md %}) for details.

A `misc` directory is provided to serve miscellaneous content such as club reports.

A topic can be shared with a URL containing an access code.

Basic usage statistics are recorded daily and aggregated by month.

For users with a knowledge of website construction:
- Static pages can be added using Go templates.
- Go templates can be redefined by site-specific files, to modify page layouts and contents.

**Upgrading to v1.3**
[&#8658; Upgrading]({{ site.baseurl }}{% link upgrading.md %})

## Screenshots

| Public page | Club home page | User's gallery | Edit slideshow |
|:-------------------------:|:-------------------------:|:-------------------------:|:-------------------------:|
|<a href="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-public.png"><img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-public.png" title="Public page" width="100%"></a>|<a href="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-club.png"><img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-club.png" title="Club home page" width="100%"></a>|<a href="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-my-gallery.png"><img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-my-gallery.png" title="User's gallery" width="100%"></a>|<a href="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-edit-slideshow.png"><img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-edit-slideshow.png" title="Edit slideshow" width="100%"></a>|

## Implementation
Suitable VMs are provided by IONOS, Digital Ocean, Linode, or Amazon Lightsail, typically costing around $2 to $6 per month with 10GB to 25GB of storage.

Uses a MariaDB or MySQL database.

Data privacy features:
- Records no additional personal data, and uses only two “strictly necessary” cookies, so that no visitor consent popup is needed for GDPR compliance.
- Users without any public images are not publicly identified.
- Identification of a contributor is limited to a display name chosen by the contributor.

Security features:
- Passwords are stored using bcrypt hashing.
- Encrypted session data.
- CSRF protection.
- Log-in attempts are rate limited to mitigate attempts to guess passwords.
- All database queries are parameterised, to block SQL injection attacks.
- Directory listing of photos, videos, and other content is disabled. Guessing media names is theoretically possible, but not easy.
- Isolation between host OS, Go application and database is implicit in Docker-based operation.
- Statistics record the number of attempts to probe the site with bad URLs and query parameters. Details are recorded in a log.
- Geo-blocking can be set for specified countries. (This adds little real security, but does reduce the number of log entries from countries where the majority of traffic is malicious.)

Client Cache-Control support:
- Browser cache max-age is configurable, with public caching allowed for public slideshows.
- Deleted images and slideshows are kept accessible for the lifetime of cached pages that may reference them.
- Browser If-Modified-Since requests are supported, to reduce server load for popular slideshows.

## Limitations
The following may be addressed in future updates:
- Only club topics, but not solo slideshows, can be shared with an access code.
- HEIC images and HEVC videos are not supported.
- There is no easy provision to change the website appearance by overriding CSS definitions.
- Support for competitions and judging requires the ability to edit the SQL database, as no forms have been implemented yet to setup tags and edit topic specifications.
- There is no provision to integrate competition mode into the normal operation as a photo gallery.

The following are unlikely to change:
- There is no password change or reset facility. However a user’s status can be reset to allow a new password to be specified on sign-up. Slideshows and images are preserved.
- No option to restrict highlight visibility to signed-up users.
- There is no facility to comment on images or to rate them.
- There is no bulk submission of images.
- There is no facility to hide contributions until they have been vetted. Members must be trusted to obey club rules on acceptable images.