# Template Example
This example file reproduces the content of the default website.

```html
{% raw %}
{{define "copyrightNotice"}}
    <h2>Copyright Notice</h2>
    <p>The copyright for all images and and slideshows on this website belongs to the individual contributors. You may not use the content of the website for any purpose, except to view it on your web browser.</p>

    <p>Contributors to the web site are reminded that they must not upload any images or text unless they personally hold the copyright for the items submitted.</p>
{{end}}
	
{{define "dataPrivacyNotice"}}
    <h2>Data Privacy</h2>
    <p>This website is designed to comply with the UK Privacy and Electronic Communications Regulations (PECR),
    which implements the EU ePrivacy Directive.</p>
	
    <h3>Information Recorded</h3>
	<p>This website does not gather any personal information about visitors or users.</p>
    <p>The website logs usage data such as pages accessed, numbers of users and referring websites. This information is anonymised at the instant of collection and aggregated into statistical reports that cannot identify individuals.</p>
    <p>It records IP addresses only where needed to detect and block attempts at unauthorised access.</p>
	
    <h3>Cookies</h3>
    <p>The only cookies this website uses are those "strictly necessary" for website operation, for which consent is not required. There are two:
    <ul>
        <li><b>csrf_token:</b> This is for website security, and helps protect against unauthorised access using Cross-Site Request Forgery. It is removed when you close your browser.</li>
        <li><b>session_v2:</b> This enables per-user messages to be displayed, and identifies logged-in users. It expires after one day.</li>
    </ul>
    </p>
{{end}}
	
{{define "signupPage" .}}
    <p>For invited users of PicInch Gallery only. See your invitation email for your username, and choose your own password.</p>
{{end}}

{{define "website"}}PicInch Gallery{{end}}
{% endraw %}
```

If you wish to use a different set of favicon sizes, add and redefine this template.

```html
{% raw %}
{{define favicons}}        
    <link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png">
    <link rel="icon" type="image/png" sizes="32x32" href="/static/images/favicon-32x32.png">
    <link rel="icon" type="image/png" sizes="16x16" href="/static/images/favicon-16x16.png">
    <link rel="manifest" href="/static/images/site.webmanifest">
    <link rel="mask-icon" href="/static/images/safari-pinned-tab.svg" color="#5bbad5">
    <link rel="shortcut icon" href="/favicon.ico">
    <meta name="msapplication-TileColor" content="#2b5797">
    <meta name="msapplication-config" content="/static/images/browserconfig.xml">
    <meta name="theme-color" content="#ffffff">
{{end}}
{% endraw %}
```	

Take care when defining the templates, as syntax errors will prevent the service from starting.