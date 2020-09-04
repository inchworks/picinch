# Search Engine Settings
This is a complex topic and I am not sure the current configuration is optimal.

## Restrictions
The aim is to limit what is indexed.
- Google is asked not to index images, with a setting in robots.txt. There is currently no provision to 
customise robots.txt.
- Public slideshows may be indexed, including titles and captions, although images are excluded as above.
- The names of members with highlights and public slideshows appear on the home page, and may be indexed.
- Public topics are not indexed, with a `noindex` setting on the page.
- Non-public content cannot be indexed because it is not accessible to search engines.

# Search Optimisation
- If multiple domains are specified for the website, a canonical link is added to the headers of public pages.
- Site-specific metadata can be added to the home page, by defining the `homePageMeta` template.
- There is no site map.
