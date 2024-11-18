# Step 1: docker-compose.yml
Copy this example, and save it in `/srv/picinch` on your server.

```yml
version: '3'

services:

  db:
    image: mariadb:11.4
    container_name: picinch_db
    expose:
      - 3306
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: "<root-password>"
      MYSQL_DATABASE: picinch
      MYSQL_USER: server
      MYSQL_PASSWORD: "<server-password>"
      MARIADB_AUTO_UPGRADE: 1
      MARIADB_DISABLE_UPGRADE_BACKUP: 1
    volumes:
      - mysql:/var/lib/mysql
    logging:
      driver: "json-file"
      options:
        max-size: "2m"
        max-file: "5"

  geoipupdate:
    container_name: geoipupdate
    image: maxmindinc/geoipupdate
    restart: unless-stopped
    environment:
      GEOIPUPDATE_ACCOUNT_ID: "<MaxMind-account>"
      GEOIPUPDATE_LICENSE_KEY: "<MaxMind-licence>"
      GEOIPUPDATE_EDITION_IDS: GeoLite2-Country
      GEOIPUPDATE_FREQUENCY: 72
    networks:
      - geoipupdate
    volumes:
      - geodb:/usr/share/GeoIP

  gallery:
    image: inchworks/picinch:1.2
    ports:
      - 443:4000:
      - 80:8000
    restart: always
    environment:
      db-password: "<server-password>"
      domains: "example.com, www.example.com"
      certificate-email: "you@example.com"
      admin-name: "admin@example.com"
      admin-password: "<your-password>"
      geo-block: "<blocked-countries>"
    volumes:
      - certs:/certs
      - geodb:/geodb:ro
      - ./photos:/photos
      - ./site:/site:ro
      - ./misc:/misc:ro
    logging:
      driver: "json-file"
      options:
        max-size: "5m"
        max-file: "5"
    depends_on:
      - db

networks:
  geoipupdate:

volumes:
  certs:
  geodb:
  mysql:
```

Edit the example to change the following items. (Take care to keep indentation unchanged when editing. Do not use tabs.)
- `MY_SQL_ROOT_PASWORD`
- `MYSQL_PASSWORD` and `db-password` Make them the same.
- `domains` The domains or sub-domains for your server. They are needed here for certificate registration with Let's Encrypt.
- `certificate-email` An email address that Let’s Encrypt will use to notify you of any problems with certificates.
- `admin-name` The username you will use to log-in to PicInch as administrator. An email address is recommended.
- `admin-password` The administrator password. The admin account is exposed to the internet, so it is important to choose a strong password. A random sequence of at least 12 characters, or at least four random words is recommended.

If you intend to change other PicInch configuration settings, you may prefer to omit the environment settings here, and set them in a site/configuration.yml file instead.

Geo-blocking requires an account for [free geo-location data from MaxMind][1]. Change these items:
- `GEOIPUPDATE_ACCOUNT_ID` Supplied by registration with MaxMind.
- `GEOIPUPDATE_LICENSE_KEY` Issued by MaxMind after registration.
- `geo-block` The countries to be blocked.
If you do not want geo-blocking, remove all the lines for the service `geoipupdate`, and don't set `geo-block`.

[1]:  https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
