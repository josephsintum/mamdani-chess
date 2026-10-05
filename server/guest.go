package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

const guestCookie = "guest"

// guestID returns the caller's guest ID: the SHA-256 of their guest cookie,
// which is set to a new random value first if there is none. Call it before
// writing the response. The cookie works as the guest's login, so only its
// hash leaves this file: seats, the database and logs never hold the cookie
// itself. Its 128 random bits need no salt.
func guestID(w http.ResponseWriter, r *http.Request) string {
	sum := sha256.Sum256([]byte(cookieValue(w, r)))
	return hex.EncodeToString(sum[:])
}

// cookieValue returns the caller's guest cookie, setting a new random one
// first if there is none.
func cookieValue(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(guestCookie); err == nil && len(c.Value) == 32 {
		return c.Value
	}
	b := make([]byte, 16)
	rand.Read(b)
	id := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     guestCookie,
		Value:    id,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
	return id
}
