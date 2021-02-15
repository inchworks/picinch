// Copyright © Rob Burke inchworks.com, 2020.

// This file is part of PicInch.
//
// PicInch is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// PicInch is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with PicInch.  If not, see <https://www.gnu.org/licenses/>.

package form

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"inchworks.com/picinch/pkg/images"
)

// Well, this much more complex than I like.
// o Variable items in the template are set directly.
// o Field values are set from a map in Form, because that is how they are returned on Post,
//   and we need to send the same values back to the client when there is an error.
// o But child form values are awkward to work with as arrays of values per field name, so
//   we always unpack them into structs when the form is received. We use the same structs
//   to contruct the template.
// o Errors (for parent and child) are mostly null, so held in maps within Form.
//   ## Keeps the child errors away from the Child struct, but they could have gone there instead,
//   and then Add and Get for them would have looked more like access to parent errors.
// o ## Tidier to put Child and its methods in another file?
// o Must rember to create a template item (index -1( when building the child structs,
//   and to skip it when processing the returned form.
// o ## Should some of the child processing be pushed down into formSlides.go?

// Email address patterm, as recommended by W3C and Web Hypertext Application Technology Working Group.
var EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// Create a custom Form struct, which anonymously embeds a url.Values object
// (to hold the form data) and an Errors field to hold any validation errors
// for the form data.
type Form struct {
	url.Values
	Errors      formErrors
	ChildErrors childErrors
}

type Child struct {
	parent     *Form
	ChildIndex int
}

// Define a New function to initialize a custom Form struct
// It takes the form data as the parameter.
func New(data url.Values) *Form {
	return &Form{
		Values:      data,
		Errors:      make(map[string][]string),
		ChildErrors: make(map[string]map[int][]string),
	}
}

// String from child form, may be empty
//
// url.Values is a map[string][]string. First item is the template.

func (f *Form) ChildGet(field string, i int) string {

	return f.Values[field][i]
}

// Index from child form

func (f *Form) ChildIndex(field string, i int) (int, error) {

	ix, err := strconv.Atoi(f.Values[field][i])

	if err != nil {
		return 0, err

	} else if ix < -1 {
		// not template or positive
		return 0, errors.New("Gallery: Bad child index in form")
	}

	return ix, nil
}

// Number of child items

func (f *Form) NChildItems() int {

	return len(f.Values["index"])
}

// ChildBool returns a checkbox value from child form.
// Unlike other fields, only checked fields are returned, and the value is the child index.
func (f *Form) ChildBool(field string, ix int) bool {

	// ignore template
	if ix == -1 {
		return false
	}

	// ## Better to convert the returned checkbox values to ints just once.
	ixStr := strconv.Itoa((ix))

	// a value returned means checked
	for _, v := range f.Values[field] {
		if v == ixStr {
			return true
		}
	}
	return false
}

// Image name from child form

func (f *Form) ChildImage(field string, i int, ix int) string {

	// don't validate template
	if i == 0 {
		return ""
	}

	value := f.Values[field][i]

	if value != "" && !images.ValidType(value) {
		f.ChildErrors.Add(field, ix, "File type not supported: ")
	}
	return value
}

// Minimum number from child form

func (f *Form) ChildMin(field string, i int, ix int, min int) int {

	// don't validate template
	if i == 0 {
		return 0
	}

	n, err := strconv.Atoi(f.Values[field][i])

	if err != nil {
		f.ChildErrors.Add(field, ix, "Must be a number")

	} else if n < min {
		f.ChildErrors.Add(field, ix, fmt.Sprintf("%d or more", min))
	}

	return n
}

// Positive number from child form

func (f *Form) ChildPositive(field string, i int, ix int) int {

	// don't validate template
	if i == 0 {
		return 0
	}

	n, err := strconv.Atoi(f.Values[field][i])

	if err != nil {
		f.ChildErrors.Add(field, ix, "Must be a number")

	} else if n < 0 {
		f.ChildErrors.Add(field, ix, "Cannot be negative")
	}

	return n
}

// Required text from child form, trimmed

func (f *Form) ChildRequired(field string, i int, ix int) string {

	// don't validate template
	if i == 0 {
		return ""
	}

	value := strings.TrimSpace(f.Values[field][i])
	if value == "" {
		f.ChildErrors.Add(field, ix, "Cannot be blank")
	}
	return value
}

// Value from select
// Assumes values are integers, 0 ... nOption-1

func (f *Form) ChildSelect(field string, i int, nOptions int) (int, error) {

	// don't validate template
	if i == 0 {
		return 0, nil
	}

	s := f.Values[field][i]

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}

	if n < 0 || n >= nOptions {
		return 0, errors.New("Gallery: Unexpected option in select")
	}

	return n, nil
}

// Text from child form, trimmed and may be empty

func (f *Form) ChildTrimmed(field string, i int) string {

	return strings.TrimSpace(f.Values[field][i])
}

// Check that field value is float within range
func (f *Form) Float(s string, field string, min float64, max float64) float64 {
	n, err := strconv.ParseFloat(s, 64)

	if err != nil {
		f.Errors.Add(field, "Must be a number")

	} else if n < min {
		f.Errors.Add(field, "Too small")

	} else if n >= max {
		f.Errors.Add(field, "Too large")
	}

	return n
}

// Check that field matches a regular expression.

func (f *Form) MatchesPattern(field string, pattern *regexp.Regexp) {
	value := f.Get(field)
	if value == "" {
		return
	}
	if !pattern.MatchString(value) {
		f.Errors.Add(field, "This field is invalid")
	}
}

// Check that field contains a maximum number of characters.

func (f *Form) MaxLength(field string, d int) {
	value := f.Get(field)
	if value == "" {
		return
	}
	if utf8.RuneCountInString(value) > d {
		f.Errors.Add(field, fmt.Sprintf("Too long (maximum %d characters)", d))
	}
}

// Check that field contains a minimum number of characters.

func (f *Form) MinLength(field string, d int) {
	value := f.Get(field)
	if value == "" {
		return
	}
	if utf8.RuneCountInString(value) < d {
		f.Errors.Add(field, fmt.Sprintf("Too short (minimum is %d characters)", d))
	}
}

// Check that field value is integer and >=0
func (f *Form) Positive(field string) int {
	s := f.Get(field)
	i, err := strconv.Atoi(s)

	if err != nil {
		f.Errors.Add(field, "Must be a number")

	} else if i < 0 {
		f.Errors.Add(field, "Cannot be negative")
	}

	return i
}

// Implement a Required method to check that specific fields in the form
// data are present and not blank. If any fields fail this check, add the
// appropriate message to the form errors.
func (f *Form) Required(fields ...string) {
	for _, field := range fields {
		value := f.Get(field)
		if strings.TrimSpace(value) == "" {
			f.Errors.Add(field, "Cannot be blank")
		}
	}
}

// Implement a PermittedValues method to check that a specific field in the form
// matches one of a set of specific permitted values. If the check fails
// then add the appropriate message to the form errors.
func (f *Form) PermittedValues(field string, opts ...string) {
	value := f.Get(field)
	if value == "" {
		return
	}
	for _, opt := range opts {
		if value == opt {
			return
		}
	}
	f.Errors.Add(field, "Value not permitted")
}

// Implement a Valid method which returns true if there are no errors.
func (f *Form) Valid() bool {
	return len(f.Errors)+len(f.ChildErrors) == 0
}

// ## Never used!
// Might be useful for complex fields. Specific to returned field names like "type[n]field",
// and repacks into another map, and still need to process that.
//
// From https://stackoverflow.com/questions/34839811/how-to-retrieve-form-data-as-array

func ParseFormCollection(r *http.Request, typeName string) []map[string]string {
	var result []map[string]string
	r.ParseForm()
	for key, values := range r.Form {
		re := regexp.MustCompile(typeName + "\\[([0-9]+)\\]\\[([a-zA-Z]+)\\]")
		matches := re.FindStringSubmatch(key)

		if len(matches) >= 3 {

			index, _ := strconv.Atoi(matches[1])

			for index >= len(result) {
				result = append(result, map[string]string{})
			}

			result[index][matches[2]] = values[0]
		}
	}
	return result
}

// ## Never used!
// Or this, uses field names like "type.field". Still creates key/value pairs.

func parseFormHandler(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()

	userParams := make(map[string]string)

	for key := range request.Form {
		if strings.HasPrefix(key, "contact.") {
			userParams[string(key[8:])] = request.Form.Get(key)
		}
	}

	fmt.Fprintf(writer, "%#v\n", userParams)
}

// Error for child field

func (c *Child) ChildError(field string) string {

	// save curren index for error report

	return c.parent.ChildErrors.Get(field, c.ChildIndex)
}

// Get display attribute for template child item v. normal itemes

func (c *Child) ChildStyle() template.HTMLAttr {

	var s string
	if c.ChildIndex == -1 {
		s = "style='display:none'"
	}

	return template.HTMLAttr(s)
}

// Class to indicate if field is valid, for Bootstrap
// ## Horrible!

func (c *Child) ChildValid(field string) string {

	// save curren index for error report
	es := c.parent.ChildErrors.Get(field, c.ChildIndex)
	if len(es) == 0 {
		return ""
	}
	return "is-invalid"
}
