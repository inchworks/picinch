# Step 1: docker-compose.yml
Copy this example, and save it in `/srv/picinch` on your server.

```
version: '3'

services:

  db:
  image: mariadb:10.4
    container_name: gallery_db
    expose:
      - 3306
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: "<root-password>"
      MYSQL_DATABASE: picinch
      MYSQL_USER: server
      MYSQL_PASSWORD: "<server-password>"
    volumes:
      - ./mysql:/var/lib/mysql
    logging:
      driver: "json-file"
      options:
        max-size: "2m"
        max-file: "5"

  gallery:
    image: inchworks/picinch
    ports:
      - 443:4000
      - 80:8000
    restart: always
    environment:
      db-password: "<server-password>"
      domains: "example.com, www.example.com"
      certificate-email: "you@example.com"
      session-secret: Hk4TEiDgq8JaCNR?WaPeWBf4QQYNUjMR
      admin-name: "admin@example.com"
      admin-password: "<your-password>"
    volumes:
      - ./certs:/certs 
      - ./photos:/photos
      - ./site:/site
      - ./videos:/videos 
    logging:
      driver: "json-file"
      options:
        max-size: "5m"
        max-file: "5"
    depends_on:
      - db
```

Edit the example to change the following items. (Take care to keep indentation unchanged when editing. Do not use tabs.)
- `MY_SQL_ROOT_PASWORD`
- `MYSQL_PASSWORD` and `db-password` Make them the same.
- `domains` The domains or sub-domains for your server. They are needed here for certificate registration with Let's Encrypt.
- `certificate-email` An email address that Let’s Encrypt will use to notify you of any problems with certificates.
- `session-secret` A random 32 character key used to encrypt users session data. You could start with the one in the and change a lot of the individual characters.
- `admin-name` The username you will use to log-in to PicInch as administrator. An email address is recommended.
- `admin-password` The administrator password. The admin account is exposed to the internet, so it is important to choose a strong password. A random sequence of at least 12 characters, or at least four random words is recommended.

If you intend to change other PicInch configuration settings, you may prefer to omit the environment settings here, and set them in a site/configuration.yml file instead.

Run `docker-compose up` to fetch PicInch and MariahDB from Docker Hub and start them.
