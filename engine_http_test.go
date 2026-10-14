package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionCookieProtection(t *testing.T) {
	for _, secure := range []bool{false, true} {
		r := httptest.NewRecorder()
		w := &sessionWriter{ResponseWriter: r, secure: secure}
		http.SetCookie(w, &http.Cookie{Name: "casdoor_session_id", Value: "opaque", Path: "/"})
		w.Write([]byte("ok"))
		cookies := r.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].Secure != secure || cookies[0].SameSite != http.SameSiteLaxMode {
			t.Fatalf("cookie protections: %+v", cookies)
		}
	}
}
