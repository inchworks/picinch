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
PicInch makes an image-oriented website. Optionally it provides a simple way for a group of website members, such as a photography club, to share their photographs. The emphasis is on uncluttered presentation of images, organised into slideshows.

Information pages and menu items can be added and edited, making PicInch a full-featured website. The aim is to make website editing as simple as possible, not to provide precise control of the website appearance.

Three options are provided:
- Club mode shows slideshows attributed to individual contributors.
- Solo mode makes an image-oriented website for images by a single owner.
- Competition mode makes a standalone host for a public photo competition.

PicInch is written in Go and ready to be deployed to a virtual machine using Docker, for good performance and easy setup. Support for HTTPS is automatic, using Let’s Encrypt certificates.

[![Project Status: Active – The project has reached a stable, usable state and is being actively developed.](https://www.repostatus.org/badges/latest/active.svg)](https://www.repostatus.org/#active)

_This is used at one photo club. I would like to work with another club, ideally in Scotland or the UK, to adapt it as needed for more general use. Contact support@picinch.com._

For more information, including setup and configuration, see https://picinch.com.

## Screenshots

| Public page | Club home page | User's gallery | Edit slideshow |
|:-------------------------:|:-------------------------:|:-------------------------:|:-------------------------:|
<img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-public.png" title="Public page" width="100%"> |<img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-club.png" title="Club home page" width="100%">|<img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-my-gallery.png" title="User's gallery" width="100%"> |<img src="https://raw.githubusercontent.com/inchworks/picinch/master/docs/images/ss-edit-slideshow.png" title="Edit slideshow" width="100%">|

## Acknowledgments

- [Let's Go! - Alex Edwards](https://lets-go.alexedwards.net) This was a big help to get started, and PicInch copies much of the application structure that he suggests.

Go Packages
- [alexedwards/scs](https://github.com/golangcollege/sessions) HTTP session management.
- [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) MySQL driver.
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