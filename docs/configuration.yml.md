# configuration.yml
Configuration parameters may be specified in this file, or as environment variables. Settings here will be overridden by environment variables in `docker-compose.yml`.
This an example configuration file with just the essential settings.  

```yml
# Example configuration for PicInch Gallery server.
#  - Edit and rename to configuration.yml
#  - Take care to keep indentation unchanged when editing. Do not use tabs.

db-password: <server password>

# The following is needed for certificate registration with Let's Encrypt domains:
  - our-domain.com
  - www.our-domain.com

# Address to be notified of problems with certificates
certificate-email: you@example.com

# Administrator, to be added to the database
admin-name: admin@example.com
admin-password: <your-password>
```

Set the following items as needed. Default values are as shown.
## Database
A database connection is requested with DSN `db-user:db-password@db-source?parseTime=true `. A MariaDB or MySQL database is required.

**db-source** `tcp(picinch_db:3306)/picinch`

**db-user** `server`

**db-password** `<server-password>`

## Domains
**domains** List of domains for which Let’s Encrypt certificates will be requested on first access.
- The website must be reachable for each specified domain via a DNS entry. 
- The domains are listed one per line, each preceded by `" - "` as shown in the example above.
- The first domain listed will be identified as canonical in page headers.
- If no domains are specified, the website can be accessed as an insecure HTTP server.

This is intended for testing and is not recommended for production.

**certificate-email** Address given to Let’s Encrypt, for notification of problems with certificates.

## Administrator
Specifies the username and password for an PicInch administrator if the username does not exist in the database. These items may be removed after setup if desired.

**admin-name** E.g. me@mydomain.com.

**admin-password** `<your-password>`

## Maximum image sizes
Photos uploaded are resized to fit these dimensions.

**image-width**  `1600` stored image width

**image-height** `1200` stored image height

**max-audio-visual** `16` maximum unprocessed audio-visual file size, in megabytes

**thumbnail-width** `278` thumbnail width

**thumbnail-height** `208` thumbnail height

**max-upload** `64` maximum image or audio-visual upload, in megabytes

## Total limits
**highlights-page** `12` highlights shown on home page, and on user's page

**highlights-topic** `32` total slides highlights slideshow

**parent-highlights** `16` highlights available for parent website

**events-page** `1` mumber of upcoming events per diary shown on home page (0 for none)

**slideshows-page** `16` slideshows shown on home page

## Per-user limits
**highlights-user** `2` highlights shown on home page

**slides-show** `50` slides shown in a slideshow

**slides-topic** `8` slides shown in a topic contribution

**slideshows-club** `2` club slideshows on home page

**slideshows-public** `1` public slideshows on home page

## Operational settings
**allowed-queries** `fbclid` ignored query names in URL. Other queries trigger an IP ban.

**ban-bad-files** `false` apply IP ban to requests for missing media files.

**geo-block** ` ` blocked countries, specified by ISO 3166-1 alpha-2 codes. For example, `KP, RU`.

**max-cache-age** `1h` browser Cache-Control max-age

**max-unvalidated-age** `48h` maximum time for a competition entry to be validated.

**max-upload-age** `8h` time limit to save a slideshow update, after uploading images. Units m(inutes) or h(ours).

**thumbnail-refresh** `1h` refresh interval for topic thumbnails. Units m(inutes) or h(ours).

**timeout-download** `2m` maximum time for a file download

**timeout-upload** `5m` maximum time for a file upload

**timeout-web** `20s` maximum time for web request, same for response

**usage-anon** `1` anonymisation of user IDs: 0 = daily, 1 = immediate.

**video-snapshot** `3s` time within video for snapshot thumbnail. Units s(econds), -ve for no snapshots.

## Time zone
The local time zone **TZ** must be specified as an environment variable in `docker-compose.yml`, not in this file.

## Website variants
Options to change the operation of the website.

**date-format** `2 January` displayed date format, using the Go reference time `01/02 03:04:05PM '06`.

**home-switch** ` ` switches the home page to a specified template, for example, `disabled` to show `disabled.page.tmpl` when the website is offline.

**misc-name** `misc` path in URL for miscellaneous files, as in `example.com/misc/file`

**options** ` ` set to `solo` to configure PicInch as an image-oriented website for a single owner, and set to `main-comp` for a standalone host for a public photo competition

**video-types** ` ` acceptable video file types, such as `.mp4, .mov`.

## For testing
**http-addr** `:8000` site HTTP address

**https-addr** `:4000` site HTTPS address
