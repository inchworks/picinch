# PicInch Gallery
PicInch aims to provide a simple way for a group of website members, such as a photography club, to display their photographs. The emphasis is on uncluttered presentation of images, organised into slideshows.
## Features
Contributors create simple slideshows. Each slide has a selection of title, photo and caption. Slides adjust in layout to match the content. There is a basic facility to edit and reorder slides.
Slideshows may be organised into topics. Topics may be defined in advance, or applied to existing slideshows. A topic can be viewed as a single slideshow, or as a page listing the individual contributions.
Sign-up is restricted to a pre-set list of members, suitable for a club. 
Slideshows can be specified as publicly visible, or visible to website users, or hidden.
Individual photos can be added to a special topic “Highlights”, featured on the home page. It includes just the most recent highlights from each contributor.
The home page shows the most recent highlights and slideshows for each user. The numbers of each shown are configurable. A page for each contributor shows all their highlights and slideshows.
A set of recent highlight images are available for display on a parent website. See (images-for-parent-website) for details.
Two user roles are defined. An admin can add, suspend and remove users. A curator can manage topics and create slideshows on behalf of users. An admin is also a curator.
Basic usage statistics are recorded daily and aggregated by month.
## Appearance
Display layout is responsive to device size (PC, tablet or phone).
Full size images are resized for display automatically. JPGs, PNGs and TIFs are supported.
## Implementation
Written in Go and may be deployed to a virtual machine using Docker, for good performance, easy setup and maintenance. Suitable VMs are provided by Digital Ocean, Linode, or Amazon Lightsail, typically costing around $5 per month with 25GB of storage.
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
## Limitations
The following may be addressed in future updates:
- Photos can be viewed individually without logging in, if their names can be guessed.
 - There is no password change or reset facility. However a user’s status can be reset to allow a new password to be specified on sign-up. Slideshows and images are preserved.
- No option to restrict highlight visibility to signed-up users.
- No option for public viewing of all highlights and public slideshows for a user, only the most recent ones shown on the home page.
- There is no facility to comment on images or to rate them.
- There is no bulk submission of images.
- HEIC format images are not supported.