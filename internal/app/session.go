package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookieName = "image_host_session"
	sessionLifetime   = 24 * time.Hour
)

func passwordMatches(got, want string) bool {
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func (a *App) setSessionCookie(w http.ResponseWriter, now time.Time) error {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	payload := strconv.FormatInt(now.Add(sessionLifetime).Unix(), 10) + "." + base64.RawURLEncoding.EncodeToString(nonce)
	signedValue := payload + "." + a.sign(payload)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signedValue,
		Path:     "/",
		Expires:  now.Add(sessionLifetime),
		MaxAge:   int(sessionLifetime.Seconds()),
		HttpOnly: true,
		Secure:   a.config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func (a *App) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *App) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return a.validSessionToken(cookie.Value, a.now())
}

func (a *App) validSessionToken(token string, now time.Time) bool {
	lastDot := strings.LastIndexByte(token, '.')
	if lastDot <= 0 || lastDot == len(token)-1 {
		return false
	}
	payload, signature := token[:lastDot], token[lastDot+1:]
	if !hmac.Equal([]byte(signature), []byte(a.sign(payload))) {
		return false
	}
	firstDot := strings.IndexByte(payload, '.')
	if firstDot <= 0 || firstDot == len(payload)-1 {
		return false
	}
	expires, err := strconv.ParseInt(payload[:firstDot], 10, 64)
	if err != nil || now.Unix() >= expires {
		return false
	}
	if _, err := base64.RawURLEncoding.DecodeString(payload[firstDot+1:]); err != nil {
		return false
	}
	return true
}

func (a *App) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(a.config.SessionSecret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

var errUnauthenticated = errors.New("unauthenticated")
