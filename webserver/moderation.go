package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const (
	sessionName = "mod_session"
	csrfName    = "mod_csrf"
	sessionTTL  = 8 * time.Hour
)

/*
current sessions are stored here. the mutex is required since
handlers run concurrently and a concurrent map write is fatal
*/
var (
	sessions   = map[string]modSession{}
	sessionsMu sync.Mutex
)

type modSession struct {
	user    string
	csrf    string
	expires time.Time
}

// generate a random 32 byte web safe string
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// reads the session cookie and validates it (exists + not expired).
// only called from requireAuth so the auth gate can't be skipped.
// keep here bc it touches the session[] map
func checkSession(r *http.Request) (modSession, bool) {
	cookie, err := r.Cookie(sessionName)
	if err != nil {
		return modSession{}, false
	}
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	// check that the time hasn't expired
	session, ok := sessions[cookie.Value]
	if !ok || time.Now().After(session.expires) {
		delete(sessions, cookie.Value)
		return modSession{}, false
	}
	return session, true
}

// let browsers hold the cookie over HTTPS only, while still
// working on plain http during local dev.
func secureFlag(r *http.Request) bool {
	return r.TLS != nil
}

// create a new session key, store session in it, then create cookie
func setSession(w http.ResponseWriter, r *http.Request, s modSession) error {
	sessionKey, err := newToken()
	if err != nil {
		return err
	}
	sessionsMu.Lock()
	sessions[sessionKey] = s
	sessionsMu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionName,
		Value:    sessionKey,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureFlag(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	return nil
}

// delete the current cookie session from our array
// also clear the cookie
func clearSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionName); err == nil {
		sessionsMu.Lock()
		delete(sessions, c.Value)
		sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secureFlag(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

/* validates a submitted csrf form field against the token minted into the session at login. */
func checkSessionCSRF(r *http.Request, session modSession) bool {
	token := r.FormValue("csrf")
	return subtle.ConstantTimeCompare([]byte(session.csrf), []byte(token)) == 1
}

// returns the form csrf token, minting and cookie-storing one if
// the browser has none yet. The cookie value and the hidden field must match.
func getOrCreateCSRF(w http.ResponseWriter, r *http.Request) string {
	// if already a cookie, just return it
	if c, err := r.Cookie(csrfName); err == nil && c.Value != "" {
		return c.Value
	}
	// if no cookie, make one
	csrf, err := newToken()
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfName,
		Value:    csrf,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureFlag(r),
		SameSite: http.SameSiteLaxMode,
	})
	return csrf
}
