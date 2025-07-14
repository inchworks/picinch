# Editing
The administrator, or a user with `curator` role, can edit the content for information pages and diaries with `Curator > Information` (for a club website) or `Edit > Pages` (for a solo website).

A page is defined by a sequence of sections. Each page section has a block of text, and optionally a media file.

## Page Sections
Markdown is supported for the text. Media files can be images, videos or PDF documents.

A section format specifies the layout of the section:

**above** The section's media image (if specified) is shown above the text.

**below** The section's media image is shown below the text.

**card** The section is shown as one of a grid of cards. I.e. in two or more columns, depending on the width of the browser window.

**left** The section's media image is shown to the left of the text.

**right** The section's media image is shown to the right of the text.

The special section formats `events`, `highlights` and `slideshows` are intended to be used just once each, and by default they are added to the home page. They can be rearranged in order on the home page, or moved to separate pages. The section text appears above the special content and typically is just a heading.

**events** This special section shows the next upcoming event for each diary.

**highlights** This special section shows a panel of thumbnails for the most recent hightlight images.

**slideshows** This special section shows a panel of thumbnails for the most recent slideshows and topics.

**subpages** This special section shows summary cards for the sub-pages of this page that do not have menu items.
I.e. pages named `.name.sub`.

## Diary Contents
A diary (for a club website) has a introduction text-only section, and a sequence of dairy events. Each event has a title that appears on the home page, and detail text.

## Slideshows
For a club website, slideshows are created and edited by club members. Slideshows are listed for each member under `Members` and the most recent slideshows are shown on one page, typically the home page. Slideshows can also be grouped into topics.

For a solo website, slideshows are created and edited by `Edit > Slideshows`. All slideshows are shown on one page, typically the home page.