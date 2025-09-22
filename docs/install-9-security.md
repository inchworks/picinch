## Security
Using the example docker-compose.yml, PicInch and the database run on the same server, and the MySQL port 3306 is not opened from the server. Even so, choose strong passwords for the MySQL root and server accounts.

Docker opens ports 80 and 443 on the host system’s firewall automatically, and it should not be necessary to make any further changes to the firewall. You may need to open these ports on any external firewall provided by your virtual host supplier.

Once the database has been initialised:
- You can remove the database root password `MYSQL_ROOT_PASSWORD` from docker-compose.yml.  You will need this password to backup the database.
- You can remove the admin username and password from docker-compose.yml or configuration.yml. This is essential if you reuse passwords across systems (which itself is not recommended!).
- The other environment settings for the database, including `MYSQL_PASSWORD` are not needed either. However there is little point in removing them, except tidiness, because the same password is still needed in `db-password`.