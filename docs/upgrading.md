# Upgrading to v1.3
To use the new configuration features, some custom templates must be removed from `site.partial.html`:

1. Remove “banner” and set the website name with `Admin > Website`.

1. Remove “website” and if needed set a short title with `Admin > Website`.

1. Remove “homePage” from and set any home page text with `Curator > Information`.

1. Remove “homePageMeta” and set the home page description with `Admin Pages -> Metadata`.

The template definitions “copyrightNotice” and “dataPrivacyNotice” can be left unchanged. Or replace them by an information page with the menu path `.notices`.
