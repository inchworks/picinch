# configuration.yml
Configuration parameters may be specified in this file, or as environment variables. Settings here will be overridden by environment variables in docker-compose.yml.
This an example configuration file with just the essential settings.  
	---
	# Example configuration for PicInch Gallery server.
	#  - Edit and rename to configuration.yml
	#  - Take care to keep indentation unchanged when editing. Do not use tabs.
	
	db-password: <server password>
	
	# The following is needed for certificate registration with Let's Encrypt
	domains:
	  - our-domain.com
	  - www.our-domain.com
	
	# Address to be notified of problems with certificates
	certificate-email: you@example.com
	
	# A random 32 character key used to encrypt users session data
	# For example, start with this one and change a lot of the individual characters.
	session-secret: Hk4TEiDgq8JaCNR?WaPeWBf4QQYNUjMR
	
	# Administrator, to be added to the database
	admin-name: admin@example.com
	admin-password: <your-password>
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
- If no domains are specified, the website can be accessed as an insecure HTTP server. This is intended for testing and is not recommended for production.
**certificate-email** Address given to Let’s Encrypt, for notification of problems with certificates.
## Session
**session-secret** A random 32 character key used to encrypt users session data.
## Administrator
Specifies the username and password for an PicInch administrator if the username does not exist in the database. These items may be removed after setup if desired.
**admin-name** E.g. me@mydomain.com.
**admin-password** `<your-password>`
## Maximum image sizes
Photos uploaded are resized to fit these dimensions.
**image-width**  `1600` stored image width
**image-height** `1200` stored image height
**thumbnail-width** `278` thumbnail width
**thumbnail-height** `208` thumbnail height
## Total limits
**highlights-page** `12` highlights shown on home page, and on user's page
**highlights-topic** `32` total slides in H format topic ??
## Per-user limits
Contributions on the home page are limited per-user.
**highlights-user** `2` highlights shown on home page
**slides-show** `10` not implemented
**slideshows-club** `2` club slideshows on home page, per user
**slideshows-public** `1` public slideshows on home page, per user
## Website
**parent-highlights** `16` highlights available for parent website
**thumbnail-refresh** `1h` refresh interval for topic thumbnails. Units m(inutes) or h(ours).
## For testing
**http-addr** `:8000` site HTTP address
**https-addr** `:4000` site HTTPS address
