## Step 10: Data Privacy
_The default copyright and privacy statements were not written by anyone with legal or data privacy expertise.
You should review the text and decide if it is suitable for your use of the website._

- You can replace the Copyright Notice that applies to website content by defining the template `copyrightNotice`.

- You can replace the Privacy Notice by defining the template `dataPrivacyNotice`.

A goal for PicInch is that it should comply with EU GDPR requirements without needing to request or record consents.
You should review this information to confirm that it meets your understanding of the regulations, and your users' expectations.

- For visitors to the website, anonymised IP addresses are stored for 24 hours to generate daily counts of the number of unique visitors.
IPv4 addresses are anonymised by clearing the least significant 8 bits.
IPv6 addresses are anonymised by clearing the least significant 80 bits. This is method 4 in
[ICANN RSSAC040 Recommendations on Anonymization Processes for Source IP Addresses][1].

- Access requests with invalid page addresses, invalid query parameters, or TLS handshake errors are treated a threat.
The IP address, time and invalid request are recorded in the system log.
Log retention is controlled by the host system, typically a Docker setting, not by PicInch.

- Repeated attempts to log-in with incorrect user names or passwords are treated as a threat.
The IP address is maintained in volatile memory so that sources can be recognised as blocked for a short period, reducing the rate at which password guesses can be made.
The IP address and time are recorded for blocked sources in the system log.

- For signed-up users, PicInch records the images and text that they enter to create slideshows.
It also records the creation and update times for images and slideshows so that recent contributions are show in order.
It does not record other user activity (although it might record popularity of viewing images and slideshows in future).

- By default, PicInch records a random anonymised ID for each user to generate daily counts of the number of visits by signed-up users.
The mapping from anonymised ID to real (database) ID is maintained in volatile memory and discarded every 24 hours.
This scheme has the disadvantage that restarting the server will cause later visits to be counted a second time.

- An alternative scheme records the real (database) ID for each user to count daily visits. The records are deleted every 24 hours.
Unavoidably they may be copied into database and server backups.
If you are satisfied with the privacy of this scheme and prefer its more dependable operation, set the configuration parameter `usage-anon` to 0.

[1]:    https://www.icann.org/en/system/files/files/rssac-040-07aug18-en.pdf