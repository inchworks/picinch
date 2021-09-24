<h1 align="center">PicInch Gallery</h1>

<div align="center">
  <h3>
    <a href="https://picinch.com">Documentation</a>
    <span> | </span>
    <a href="https://sconephoto.club">Working Site</a>
    <span> | </span>
    <a href="https://hub.docker.com/r/inchworks/picinch">Docker Repository</a>
  </h3>
</div>

## Features
PicInch provides a simple way for a group of website members, such as a photography club, to share their photographs. The emphasis is on uncluttered presentation of images, organised into slideshows.

- Contributors create simple slideshows. Each slide has a selection of title, photo and caption. Slides adjust in layout to match the content. There is a basic facility to edit and reorder slides.

- Slideshows may be organised into topics, as defined by a curator. Topics may be defined in advance, or applied to existing slideshows. A topic can be viewed as a single slideshow, or as a page listing the individual contributions.

- Sign-up is restricted to a pre-set list of members, suitable for a club.

- Slideshows can be specified as publicly visible, or visible to website users, or hidden.

- Individual photos can be added to a special topic “Highlights”, featured on the home page. It includes just the most recent highlights from each contributor.

- The home page shows the most recent highlights and slideshows for each user. The number shown of each is configurable. A page for each contributor shows all their highlights and slideshows.

- Written in Go and ready to be deployed to a virtual machine using Docker, for good performance and easy setup. 

_This is used at one photo club. I would like to work with another club, ideally in Scotland or the UK, to adapt it as needed for more general use. Contact support@picinch.com._

For more information, including setup and configuration, see https://picinch.com.

## Screenshots

| Public page | Club home page | User's gallery | Edit slideshow |
|:-------------------------:|:-------------------------:|:-------------------------:|:-------------------------:|
<img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-public.png" title="Public page" width="100%"> |<img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-club.png" title="Club home page" width="100%">|<img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-my-gallery.png" title="User's gallery" width="100%"> |<img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-edit-slideshow.png" title="Edit slideshow" width="100%">|

## Acknowledgments

- [Let's Go! - Alex Edwards](https://lets-go.alexedwards.net) This was a big help to get started, and PicInch copies much of the application structure that he suggests.

Go Packages
- [disintegration/imaging](https://github.com/disintegration/imaging) Image processing.
- [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) MySQL driver.
- [golangcollege/sessions](https://github.com/golangcollege/sessions) HTTP session cookies.
- [ilyakaznacheev/cleanenv](https://github.com/ilyakaznacheev/cleanenv) Read configuration file and environment variables.
- [jmoiron/sqlx](https://github.com/jmoiron/sqlx) SQL library extensions.
- [julienschmidt/httprouter](https://github.com/julienschmidt/httprouter) HTTP request router.
- [justinas/alice](https://github.com/justinas/alice) HTTP middleware chaining.
- [justinas/nosurf](https://github.com/justinas/nosurf) CSRF protection.
- [microcosm-cc/bluemonday](https://github.com/microcosm-cc/bluemonday) HTML sanitizer for user input.

JavaScript Libraries
- [Bootstrap](https://getbootstrap.com) Toolkit for responsive web pages.
- [jQuery](https://jquery.com) For easier DOM processing and Ajax.
- [Lightbox](https://lokeshdhakar.com/projects/lightbox2/) Overlay images on the current page.
- [Popper](https://popper.js.org) Tooltip and popover positioning (used by Bootstrap).

Video processing uses [FFmpeg](https://ffmpeg.org).