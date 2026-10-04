package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const guestCookie = "guest"

// guestID returns the caller's guest ID from its cookie, setting a new
// random one first if there is none. Call it before writing the response.
func guestID(w http.ResponseWriter, r *http.Request) string {
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
