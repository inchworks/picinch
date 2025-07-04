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

## Additional Content
As administrator you can add additional information pages and menu items with `Admin > Pages`. Specify the menu path for a page as `name` for a top-level menu item or `name.sub` for a dropdown menu item. A leading "`.`", i.e. `.name` specifies a page without a menu item. Pages are referenced by `https://<your-domain>/info/name` or `/info/name.sub`.

Similarly, diaries can be added with `Admin > Diaries`. Typically just one diary is sufficient. By default the next upcoming event in each diary is shown automatically on the home page. Diaries are accessed by `https://<your-domain/diary/name` or `/diary/name.sub`.

A user with `curator` role, or an administrator, can edit the content for information pages and diaries with `Curator > Information`. 

It is also possible to add static pages using templates. These take more effort to understand and change, but give full control over page layouts. See [Customisation]({{ site.baseurl }}{% link customisation.md %}).

## Page Layout
Each page section has a block of text, or an image or video, or both. Markdown is supported for the text.

A section format specifies the layout of the section:

**above** The section's image (if specified) is shown above the text.

**below** The section's image is shown below the text.

**card** The section is shown as one of a grid of cards. I.e. in two or more columns, depending on the width of the browser window.

**left** The section's image is shown to the left of the text.

**right** The section's image is shown to the right of the text.

**events** This special section shows the next upcoming event for each diary.

**highlights** This special section shows a panel of thumbnails for the most recent hightlight images.

**slideshows** This special section shows a panel of thumbnails for the most recent slideshows.

**subpages** This special section shows summary cards for the sub-pages of this page that do not have menu items.
I.e. pages named `.name.sub`.

The special section formats `events`, `highlights` and `slideshows` are intended to be used just once each, and by default they are added to the home page. They can be rearranged in order on the home page, or moved to separate pages. The section text appears above the special content and typically is just a heading.

## Diary Layout
A diary has a introduction text-only section, and a sequence of dairy events. Each event has a title that appears on the home page, and detail text.

## Page Metadata
Web pages need a title, to be shown in browser tabs, and search engine results. By default the title of each web page is the same as its heading, but you can change the title with `Admin > Pages -> Metadata`. Typically this is done when a shorter title is needed.

Information and diary pages that are to appear in search engine results should have a description. Set the description with `Admin > Pages -> Metadata` and `Admin > Diaries -> Metadata`. Alternatively you can request that a page should not be indexed by search engines by setting the `Block search indexing` checkbox.

## Forgotten Passwords
If a user forgets their password, set their status back to `known` so that they can sign-up again. Their slideshows are preserved. (PicInch uses secure hashing of passwords, so they are not available to the adminstrator.)

If the admin password is lost, add a username and a password for a *new* administrator in `docker-compose.yml` and restart the server. You can then log in and reset the old administrator’s status to `known`, to keep their photos and allow them to sign-up with a new password. Or you can delete the old administrator.

Remember to remove the password from `docker-compose.yml`, especially if it might be reused elsewhere.
