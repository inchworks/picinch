# Images for a Parent Website
One use of PicInch is to supplement a club’s main website. The gallery might then be accessed as e.g. https://gallery.example.com or https://picinch.example.com.
To enhance integration, the PicInch server can make a set of recent highlight images available for display on the main website. They are accessed as e.g.
- `https://gallery.example.com/highlight/T/0, 1, 2, …` for thumbnails.
- `https://gallery.example.com/highlight/P/0, 1, 2, …` for full size images.
- `https://gallery.example.com/highlights/6` for a plain page of e.g. 6 thumbnails that could be embedded as an iframe. (Making the resulting page responsive is left as an exercise for the parent website!)
The number of highlights available is specified by `parent-highlights` in the site configuration file.

