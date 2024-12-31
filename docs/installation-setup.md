# Installation and setup
## Before installation
- Set up a host system with Docker and Docker Compose installed. For example, using a DigitalOcean [Docker Droplet][1].
- Acquire a domain name, or add a sub-domain to a domain you already own. Set the `A` record for the domain or subdomain to the IP address of your server. This should be done BEFORE starting the PicInch service.

## Installation steps
These instructions assume a Ubuntu Server host with Docker installed. Other Linux distributions may be similar (but CentOS/RHEL 8 provides a different technology to Docker). A basic installation requires the creation of just two files on the server.

1. Add `/srv/picinch/docker-compose.yml`. This Docker Compose file specifies the PicInch and MariaDB containers to be downloaded from Docker Hub, the settings to run them on the host system, and essential application parameters.
[&#8658; Docker Setup]({{ site.baseurl }}{% link install-1-docker-compose.md %})

1. In `/srv/picinch` run `docker compose up -d` When issued the first time, this fetches PicInch and MariaDB containers from Docker Hub, and starts PicInch. Then PicInch sets up the database, creates the directories to hold images and certificates (in`/srv/picinch/`). PicInch will be restarted automatically when the host system is rebooted.
[&#8658; Commands]({{ site.baseurl }}{% link install-3-commands.md %})

1. Connect to your server by domain name using a web browser and see that you can log in.
[&#8658; Site Administrator]({{ site.baseurl }}{% link administrator.md %})

## After installation

1. Customise the appearance of the website by adding optional files. Restart PicInch for changes to take effect. (See Commands.)
[&#8658; Customisation]({{ site.baseurl }}{% link install-5-customise.md %})

1. Log in to PicInch as administrator and set the website name and a description for the home page. You can also add information pages, a diary, and set page metadata as needed. [&#8658; Site Administrator]({{ site.baseurl }}{% link administrator.md %})

1. As administrator, add potential users with status set to `known`. Send invitations to the users, inviting them to sign-in.
[&#8658; Site Administrator]({{ site.baseurl }}{% link administrator.md %})

1. If desired, make one or more `active` (signed-up) users an `admin` or `curator`.
[&#8658; Site Administrator]({{ site.baseurl }}{% link administrator.md %})

1. Arrange for a regular backup of the database and photos. This may be an option from your hosting supplier, or you may need to do it regularly yourself.
[&#8658; Backups]({{ site.baseurl }}{% link install-8-backups.md %})

1. Review the security of your system.
[&#8658; Security]({{ site.baseurl }}{% link install-9-security.md %})

1. Review the data privacy of your system.
[&#8658; Privacy]({{ site.baseurl }}{% link install-10-privacy.md %})

## Public competition setup
PicInch has an alternative use as a website for a public competition.
See [Competition Setup]({{ site.baseurl }}{% link competition.md %})

[1]:	https://marketplace.digitalocean.com/apps/docker
