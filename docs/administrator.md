# Administrator
## Essentials on Setup

1. Set the website name: `admin > website`.

1. Set a description for the website: `admin > pages : Home Page -> Metadata`.

1. Add members who need to create slideshows: `admin > users`.

## Users
Add users with `Admin > Users`. Specify a name for each user, which will be visible on the website and can be changed by the user on sign-up. Also specify a unique identity that the user will know on sign-up, such as their email address.

Each user has a role. A `member` can add slideshows and view club slideshows as well as public ones. A `friend` can view club slideshows but not add slideshows. A `curator` can create topics, edit slideshows on behalf of members, edit information pages and edit diary events. An `admin` has full control of the website, including adding users and adding information pages.

Each user also has a current status. `known` shows a user that has been added, but who has not yet set a password to sign-up.
`active` indicates that a user has set a password and can login. `suspended` blocks a user from login but preserves their slideshows. Typically it is used for users who have left the club but might rejoin in the near term.

When a user forgets their password, set their status back to `known` so that they can sign-up again. Their slideshows are preserved. (PicInch uses secure hashing of passwords, so they are not available to the adminstrator.)

## Additional Content
As administrator you can add additional information pages and menu items with `Admin > Pages`. Specify the menu path for a page as `name` for a top-level menu item or `name.sub` for a dropdown menu item. A leading "`.`", i.e. `.name` specifies a page without a menu item. Pages are accessed by `https://<your-domain/info/name` or `/info/name.sub`.

Similarly, diaries can be added with `Admin > Diaries`. Typically just one diary is sufficient. By default the next upcoming event in each diary is shown automatically on the home page. Diaries are accessed by `https://<your-domain/diary/name` or `/diary/name.sub`.

An administrator or a user with `curator` role can edit the content for information pages and diaries with `Curator > Information`. Markdown is supported for the sections of an information page and the introduction section of a diary page. 

You can also add static pages using templates. These take more effort to understand and change, but give full control over page layouts. See [Customisation]({{ site.baseurl }}{% link install-5-customise.md %}).

## Page Metadata
Web pages need a title, to be shown in browser tabs, and search engine results. By default the title of each web page is the same as its heading, but you can change the title with `Admin > Pages -> Metadata`. Typically this is done when a shorter title is needed.

Information and diary pages that are to appear in search engine results should have a description. Set the description with `Admin > Pages -> Metadata` and `Admin > Diaries -> Metadata`. Alternatively you can request that a page should not be indexed by search engines by setting the `Block search indexing` checkbox.

## Change of Administrator
If the admin password is lost, add a username and a password for a *new* administrator in `docker-compose.yml` and restart the server. You can then log in and reset the old administrator’s status to `known`, to keep their photos and allow them to sign-up with a new password. Or you can delete the old administrator.

Remember to remove the password from `docker-compose.yml`, especially if it might be reused elsewhere.
