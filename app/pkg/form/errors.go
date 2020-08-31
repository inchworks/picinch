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

// Validation error messages for forms, keyed by name of the form field.

type formErrors map[string][]string
type childErrors map[string]map[int][]string

// Add error messages for a given field to the map.
func (e formErrors) Add(field, message string) {
	e[field] = append(e[field], message)
}

func (e childErrors) Add(field string, ix int, message string) {
	// allow the child maps to be nil
	if e[field] == nil {
		e[field] = make(map[int][]string)
	}

	e[field][ix] = append(e[field][ix], message)
}

// Retrieve the first error message for a given field from the map.
func (e formErrors) Get(field string) string {
	es := e[field]
	if len(es) == 0 {
		return ""
	}
	return es[0]
}

// Class to indicate if field is valid, for Bootstrap
// ## This is an utter pain - Bootstrap won't display invalidity message unless field is marked invalid.
// ## Cannot hard-code the message in the template because the error may vary.

func (e formErrors) Valid(field string) string {
	es := e[field]
	if len(es) == 0 {
		return ""
	}
	return "is-invalid"
}

// Add error for child by index number

func (e childErrors) Get(field string, ix int) string {
	es := e[field][ix]
	if len(es) == 0 {
		return ""
	}
	return es[0]
}
