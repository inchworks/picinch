# CSS Example
This example file shows templates to customise site colours and fonts.
It reproduces the default site appearance.

```html
{% raw %}
{{define "defaultStyles"}}
<style>

  body {
    font-family: "HelveticaNeue-Light","Helvetica Neue Light","Helvetica Neue",Helvetica,Arial,"Lucida Grande",sans-serif;
    background-color: #ddd;
  }

  {{/* navigation */}}
  nav.navbar { background-color: #ccc; }
  ul.dropdown-menu { background-color: #ccc; }

  {{/* banner */}}
  header { background-color: #DDD; }
  h1.banner {font-family: Palatino, serif; font-size:40px;}

  {{/* section */}}
  section.intro {background-color: #DDD;}
  section.slideshows { background-color: #ccc; }

  {{/* scale down headers */}}
  h1 { font-size: 2rem; }
  h2 { font-size: 1.75rem; }
  h3 { font-size: 1.5rem; }
  h4 { font-size: 1.25rem; }
  h5 { font-size: 1rem; }

  {{/* highlight thumbnails */}}
  .highlight-thumbnails {
    color: #313437;
    background-color: rgb(204,204,204);
  }

  {{/* card on page */}}
  .page.card { background-color: rgb(204,204,204); color: rgb(33,37,41); }

  {{/* slideshow thumbnail */}}
  .slides-thumbnail { background-color: rgb(52,58,64); }
  .slides-thumbnail a.card-link { color: rgb(255,255,255); }
  .slides-thumbnail .card-link: hover { color: rgb(203,211,218); }

</style>
{{end}}

{{define "defaultSlideStyles"}}
<style>

body {
  font-family: "HelveticaNeue-Light","Helvetica Neue Light","Helvetica Neue",Helvetica,Arial,"Lucida Grande",sans-serif; }
  .bg-slideshow { background-color: rgb(102,102,102); }
  .text-slideshow { color: rgb(221,221,221); }
  
</style>
{{end}}
{% endraw %}
```	
