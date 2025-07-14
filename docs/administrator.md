# Administrator
## Essentials on Setup

1. Set the website name: `Admin > Website`.

1. Set a description for the website: `Admin > Pages : Home Page -> Metadata`.

1. Add members who need to create slideshows: `Admin > Users`.

## Club Users
Add users with `Admin > Users`. Specify a name for each user, which will be visible on the website and can be changed by the user on sign-up. Also specify a unique identity that the user will know on sign-up, such as their email address.

Each user has a role. A `member` can add slideshows and view club slideshows as well as public ones. A `friend` can view club slideshows but not add slideshows. A `curator` can create topics, edit slideshows on behalf of members, edit information pages and edit diary events. An `admin` has full control of the website, including adding users and adding information pages.

Each user also has a current status. `known` shows a user that has been added, but who has not yet set a password to sign-up.
`active` indicates that a user has set a password and can login. `suspended` blocks a user from login but preserves their slideshows. Typically it is used for users who have left the club but might rejoin in the near term.

## Solo Website Users
Typically a solo website has just the administrator. However it is possible to add users with `Admin > Users`. Specify a name for each user, which will be visible on the website and can be changed by the user on sign-up. Also specify a unique identity that the user will know on sign-up, such as their email address.

Each user has a role. A `friend` can view slideshows limited to friends, but not add slideshows. A `curator` can edit slideshows, information pages and diary events. An `admin` has full control of the website, including adding or removing users, information pages, and diaries.

## Forgotten Passwords
If a user forgets their password, set their status back to `known` so that they can sign-up again. Their slideshows are preserved. (PicInch uses secure hashing of passwords, so they are not available to the adminstrator.)

If the admin password is lost, add a username and a password for a *new* administrator in `docker-compose.yml` and restart the server. You can then log in and reset the old administrator’s status to `known`, to keep their photos and allow them to sign-up with a new password. Or you can delete the old administrator.

Remember to remove the password from `docker-compose.yml`, especially if it might be reused elsewhere.
