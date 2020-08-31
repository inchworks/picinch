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

package main

// Form handling for user sign-up and login

import (
	"errors"
	"net/http"
	"net/url"

	"inchworks.com/gallery/pkg/form"
	"inchworks.com/gallery/pkg/models"
)

// Check if username can sign up

func (s *GalleryState) CanSignup(username string) (*models.User, error) {

	// serialisation
	defer s.updatesNone()()

	user, err := s.app.UserStore.GetNamed(username)
	if err != nil {
		return nil, errors.New("Not recognised. Ask us for an invitation.")
	}

	switch user.Status {
	case models.UserKnown:
		// OK

	case models.UserActive, models.UserAdmin:
		return nil, errors.New("Already signed up. You can log in.")

	case models.UserSuspended:
		return nil, errors.New("Access suspended. Contact us.")

	default:
		panic("Unknown user status")
	}

	return user, nil
}

// Login user

func (app *Application) getFormLogin(w http.ResponseWriter, r *http.Request) {

	app.render(w, r, "user-login.page.tmpl", &simpleFormData{
		Form: form.New(nil),
	})
}

func (app *Application) postFormLogin(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// check username and password
	f := form.New(r.PostForm)
	username := f.Get("username")
	user, err := app.UserStore.GetNamed(username)
	if err == nil {
		err = user.Authenticate(f.Get("password"))
	}

	// take care not to reveal whether it is the username or password that is wrong
	// We shouldn't record the name or password, in case it is a mistake by a legitimate user.
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) || errors.Is(err, models.ErrInvalidCredentials) {
			app.threat("login error", r)
			f.Errors.Add("generic", "Username or password not known")
			app.render(w, r, "user-login.page.tmpl", &simpleFormData{
				Form: f,
			})

		} else {
			app.log(err)
			app.clientError(w, http.StatusInternalServerError)
		}
		return
	}

	// add the user ID to the session, so that they are now 'logged in'
	app.session.Put(r, "authenticatedUserID", user.Id)

	// get URL that the user accessed
	path := app.session.PopString(r, "redirectPathAfterLogin")
	if path != "" {
		http.Redirect(w, r, path, http.StatusSeeOther)

	} else {
		// redirect to club homepage (may have more now logged in)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// Signup new user

func (app *Application) getFormSignup(w http.ResponseWriter, r *http.Request) {

	app.render(w, r, "user-signup.page.tmpl", &simpleFormData{
		Form: form.New(nil),
	})
}

func (app *Application) postFormSignup(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	f := form.New(r.PostForm)
	f.Required("displayName", "username", "password")
	f.MaxLength("displayName", 60)
	f.MaxLength("username", 60)
	//   form.MatchesPattern("email", forms.EmailRX)
	f.MinLength("password", 10)
	f.MaxLength("password", 60)

	// check if username known here
	// We don't record the username, in case it is a mistake by a legitimate user.
	username := f.Get("username")
	user, err := app.galleryState.CanSignup(username)
	if err != nil {

		app.threat("signup error", r)
		f.Errors.Add("username", err.Error())
	}

	// If there are any errors, redisplay the signup form.
	if !f.Valid() {
		app.render(w, r, "user-signup.page.tmpl", &simpleFormData{Form: f})
		return
	}

	// add user
	err = app.galleryState.OnUserSignup(user, f.Get("displayName"), f.Get("password"))
	if err == nil {
		app.session.Put(r, "flash", "Your sign-up was successful. Please log in.")

		http.Redirect(w, r, "/user/login", http.StatusSeeOther)
	} else {
		app.clientError(w, http.StatusBadRequest)
	}
}

// Main form to setup users

func (app *Application) getFormUsers(w http.ResponseWriter, r *http.Request) {

	// allow access?
	if !app.isAdmin(r) {
		app.clientError(w, http.StatusUnauthorized)
		return
	}
	
	f := app.galleryState.ForEditUsers()

	app.galleryState.ForUsers()

	// display form
	app.render(w, r, "edit-users.page.tmpl", &usersFormData{
		Form: f,
	})
}

func (app *Application) postFormUsers(w http.ResponseWriter, r *http.Request) {

	// allow access?
	if !app.isAdmin(r) {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// process form data
	f := form.NewUsers(r.PostForm)
	users, err := f.GetUsers()
	if err != nil {
		app.errorLog.Print(err.Error())
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// redisplay form if data invalid
	if !f.Valid() {
		app.errorLog.Print(f.Errors)
		app.errorLog.Print(f.ChildErrors)

		app.render(w, r, "edit-users.page.tmpl", &usersFormData{Form: f})
		return
	}

	// save changes
	if app.galleryState.OnEditUsers(users) {
		app.session.Put(r, "flash", "User changes saved.")
		http.Redirect(w, r, "/", http.StatusSeeOther)

	} else {
		app.clientError(w, http.StatusBadRequest)
	}
}

// Get data to edit users

func (s *GalleryState) ForEditUsers() (f *form.UsersForm) {

	// serialisation
	defer s.updatesNone()()

	// users
	users := s.app.UserStore.ByName()

	// form
	var d = make(url.Values)
	f = form.NewUsers(d)

	// add template and users to form
	f.AddTemplate()
	for i, u := range users {
		f.Add(i, u)
	}

	return
}

// Processing when users are modified.
//
// Returns true if no client errors.

func (s *GalleryState) OnEditUsers(usSrc []*form.UserFormData) bool {

	// serialisation
	defer s.updatesGallery()()

	// compare modified users against current users, and update
	usDest := s.app.UserStore.ByName()

	nSrc := len(usSrc)
	nDest := len(usDest)

	// skip template
	iSrc := 1
	var iDest int

	for iSrc < nSrc || iDest < nDest {

		if iSrc == nSrc {
			// no more source users - delete from destination
			s.onRemoveUser(usDest[iDest])
			iDest++

		} else if iDest == nDest {
			// no more destination users - add new user
			u := models.User{
					Name: usSrc[iSrc].DisplayName,
					Username: usSrc[iSrc].Username,
					Status: usSrc[iSrc].Status,
					Password: []byte(""),
			}
			s.app.UserStore.Update(&u)
			iSrc++

		} else {
			ix := usSrc[iSrc].ChildIndex
			if ix > iDest {
				// source user removed - delete from destination
				s.onRemoveUser(usDest[iDest])
				iDest++

			} else if ix == iDest {
				// check if user's details changed
				uDest := usDest[iDest]
				if usSrc[iSrc].DisplayName != uDest.Name ||
					usSrc[iSrc].Username != uDest.Username ||
					usSrc[iSrc].Status != uDest.Status {
					uDest.Name = usSrc[iSrc].DisplayName
					uDest.Username = usSrc[iSrc].Username
					uDest.Status = usSrc[iSrc].Status
					s.app.UserStore.Update(uDest)
				}
				iSrc++
				iDest++

			} else {
				// out of sequence team index
				return false
			}
		}
	}

	return true
}
