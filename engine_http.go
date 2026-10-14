package main

import (
	"context"
	"fmt"
	"github.com/beego/beego/v2/server/web"
	"github.com/casdoor/casdoor/conf"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Normalize only the opaque session cookie. UI configuration cookies must remain readable.
type sessionWriter struct {
	http.ResponseWriter
	secure      bool
	written     bool
	sessionName string
}

func (w *sessionWriter) WriteHeader(status int) {
	if !w.written {
		w.written = true
		headers := w.Header().Values("Set-Cookie")
		w.Header().Del("Set-Cookie")
		for _, value := range headers {
			resp := http.Response{Header: http.Header{"Set-Cookie": []string{value}}}
			cookies := resp.Cookies()
			name := w.sessionName
			if name == "" {
				name = "casdoor_session_id"
			}
			if len(cookies) == 1 && cookies[0].Name == name {
				c := cookies[0]
				c.HttpOnly = true
				c.Secure = w.secure
				c.SameSite = http.SameSiteLaxMode
				w.Header().Add("Set-Cookie", c.String())
			} else {
				w.Header().Add("Set-Cookie", value)
			}
		}
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *sessionWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}

func serveEngine(port int) {
	issuer := conf.GetConfigString("origin")
	u, e := url.Parse(issuer)
	if e != nil {
		panic(e)
	}
	local := fmt.Sprintf("localhost:%d", port)
	web.BConfig.WebConfig.Session.SessionCookieSameSite = http.SameSiteLaxMode
	web.BeeApp.Server.ReadHeaderTimeout = 5 * time.Second
	web.BeeApp.Server.ReadTimeout = 30 * time.Second
	web.BeeApp.Server.WriteTimeout = 30 * time.Second
	web.BeeApp.Server.IdleTimeout = 60 * time.Second
	var mu sync.Mutex
	var window time.Time
	attempts := 0
	web.RunWithMiddleWares(fmt.Sprintf("127.0.0.1:%d", port), func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host != local && r.Host != u.Host {
				http.Error(w, "invalid host", 403)
				return
			}
			if r.URL.Path == "/healthz" {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status":"ok"}`))
				return
			}
			if r.URL.Path == "/internal/shutdown" {
				if r.Host != local || r.Method != "POST" || os.Getenv("MYACCOUNTS_SHUTDOWN") == "" || r.Header.Get("X-Shutdown-Token") != os.Getenv("MYACCOUNTS_SHUTDOWN") {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(204)
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
					defer cancel()
					_ = web.BeeApp.Server.Shutdown(ctx)
				}()
				return
			}
			if r.URL.Path == "/api/login" {
				mu.Lock()
				if time.Since(window) > time.Minute {
					window = time.Now()
					attempts = 0
				}
				attempts++
				blocked := attempts > 30
				mu.Unlock()
				if blocked {
					w.Header().Set("Retry-After", "60")
					http.Error(w, "try again shortly", 429)
					return
				}
			}
			// Browser credential mutations must originate from this issuer or the local dashboard.
			// Standards token endpoints use upstream client authentication rather than browser sessions.
			mutation := r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH"
			protocol := strings.HasPrefix(r.URL.Path, "/api/login/oauth/") || strings.HasPrefix(r.URL.Path, "/api/saml/")
			if mutation && strings.HasPrefix(r.URL.Path, "/api/") && !protocol {
				origin := r.Header.Get("Origin")
				if origin != issuer && origin != "http://"+local {
					http.Error(w, "invalid request origin", 403)
					return
				}
			}
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "same-origin")
			w.Header().Set("X-Frame-Options", "DENY")
			next.ServeHTTP(&sessionWriter{ResponseWriter: w, secure: r.Host == u.Host && u.Scheme == "https", sessionName: web.BConfig.WebConfig.Session.SessionName}, r)
		})
	})
}
