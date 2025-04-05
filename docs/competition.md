## Competition Setup
PicInch can be re-configured as a standalone website for a public photography competition.

Currently, configuration as a competition requires more effort than needed for a normal gallery. For an example, see the site-competition folder with the application source on GitHub.

### Users

For a competition, PicInch gives users with the following roles access to entries:

**admin** or **curator** Can see all details of entries, including email addresses.

**member** Acts as competition manager. Can see entrants name but not email address.

**friend** Acts as judge. Can see entry titles, descriptions and media, but not entrants names and email addresses.

### Configuration Parameters

Two parameter settings are required in `configuration.yml`:

**options: main-comp** Enables web pages to select a competition class, get the competition entry form, and validate an email address. Also adds a menu item for authenticated users, allowing selection of competition entries.

**home-switch: classes** Replaces the default gallery home page with the classes that an entrant can select to get an entry form. Alternatively, `home-switch: info-competition` could be set to show a static page about the competition. This page should have a link to the `classes` page.

`home-switch` could also be set to other static pages: a holding page before the competition has started, or a results page when the competition has ended.

Other parameters are required to enable validation of entrants' email addresses (see below).

### Templates
The following templates should be defined:

**menu-public** Set this to replace the default menu items shown to public visitors. At a mimimum, set the name of the home entry to something appropriate instead of "GALLERY" and add an item for "CLASSES". Static pages for competition rules and other information can be added.
Omit the default "LOGIN" and "SIGN-UP" items.

**menu-friend** Defines the menu items shown to users registered as friends, typically judges.

**menu-member** Defines the menu items shown to users registered as members, typically competition managers.

(For sample menu definitions, see `menu.partial.tmpl`.)

Other templates allow customisation and will probably need to be defined:

**classesIntro** Text at the head of the `classes` page.

**emailValidatedPage** Content shown to an entrant when they click on the validation link sent to them by email.

**signupPage** Content shown to friends (judges) and members (managers) when they sign-up.

(See `site.partial.tmpl` for an example of competition customisation.)

These templates customise the competition entry form:

**compAgreeContent** Revised text to agree the media submitted.

**compAgreeRules** Revised text to agree the competition rules.

**compCaption** Explanatory text for the entry caption field.

**compEmail** Explanatory text for the entrant's email address field.

**compEnd** Text at the bottom of the entry form.

**compName** Explanatory text for the entrant's name field.

**compFooter** The footer of the entry form, for organisation information and formal notices.

**compIntro** Text at the head of the entry form.

**compLocation** Explanatory text for the entrant's location field.

**compPhoto** Explanatory text for the photo or video field.

**compTitle** Explanatory text for the entry title field.

(See `comp.partial.tmpl` for an example of form customisation.)

These templates customise the validation email:

**emailHtml** HTML body for the email.

**emailPlain** Plaintext body for the email.

**emailSubject** Subject for the email.

(See `validation-email.partial.tmpl` for an example of a validation email.)

### Email Address Validation

A website that accepts public entries may be given any email address, not necessarily owned by the person submitting a possibly bogus entry. So it is desirable to validate each entry by sending an email with an entry-specific link to be clicked by the entrant.

Entries are not tagged for viewing by judges until they have been validated. Entries not validated within a time limit are deleted.

Picinch can be configured to use an email transmission service:

**email-host** SMTP service address. If set to `mailgun`, an API to the [MailGun][1] service is used instead of SMTP. If blank, validation is not required.

**email-password** Service password.

**email-port** SMTP port. (Default 587.)

**email-user** Service username.

**max-unvalidated-age** Time allowed for an entry to be validated. (Default 48h.) 

**reply-to** Email reply address.

**sender** Sending email address.

### Tags
A simple workflow system is provided, using tags. It allows judges to mark the entries.
It can also be configured for other uses, such as to allow organisers to flag entries for publicity.

[1]:    https://mailgun.com