package portable

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/casdoor/casdoor/internal/mailrelay"
	"github.com/gofrs/flock"
)

// SetupEmail reconnects an existing installation without resetting its accounts,
// issuer or tunnel. A separate loopback wizard keeps its token out of public UI.
func SetupEmail(ctx context.Context, dir string, open func(string)) error {
	lock := flock.New(filepath.Join(dir, "email-setup.lock"))
	ok, e := lock.TryLock()
	if e != nil {
		return e
	}
	if !ok {
		return fmt.Errorf("email setup is already open on this computer")
	}
	defer lock.Unlock()
	config, e := Load(dir)
	if e != nil {
		return e
	}
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return e
	}
	origin := fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)
	token := RandomToken()
	done := make(chan struct{})
	connected := false
	var pending *mailrelay.Credential
	client, e := mailrelay.NewClient(mailrelay.DefaultURL)
	if e != nil {
		listener.Close()
		return e
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, reconnectHTML)
	})
	mux.HandleFunc("GET /info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"email": config.Email})
	})
	mux.HandleFunc("POST /connect", func(w http.ResponseWriter, r *http.Request) {
		if connected {
			http.Error(w, "email already connected", 409)
			return
		}
		var p struct {
			Code string `json:"code"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		if json.NewDecoder(r.Body).Decode(&p) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		if p.Code == "" {
			credential, e := client.Begin(r.Context(), config.Email, config.Issuer)
			if e != nil {
				http.Error(w, e.Error(), 400)
				return
			}
			pending = &credential
		} else {
			if pending == nil {
				http.Error(w, "send a code first", 400)
				return
			}
			if e := client.Confirm(r.Context(), *pending, p.Code); e != nil {
				http.Error(w, e.Error(), 400)
				return
			}
			if e := SaveSharedEmail(dir, *pending); e != nil {
				http.Error(w, "could not save protected email connection", 500)
				return
			}
			connected = true
			close(done)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
		if r.Host != listenerHost(origin) || (r.Method == "POST" && (r.Header.Get("Origin") != origin || r.Header.Get("X-Setup-Token") != token)) {
			http.Error(w, "local setup authorization required", 403)
			return
		}
		// serialize wizard state and writes; browser double-clicks cannot race.
		mux.ServeHTTP(w, r)
	})
	// A single request at a time guards pending credentials and completion.
	var gate = make(chan struct{}, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate <- struct{}{}
		defer func() { <-gate }()
		handler.ServeHTTP(w, r)
	}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 40 * time.Second}
	link, e := Protect([]byte(origin + "/#" + token))
	if e != nil {
		listener.Close()
		return e
	}
	linkPath := filepath.Join(dir, "secrets", "email-setup-url.dpapi")
	if e = AtomicWrite(linkPath, link); e != nil {
		listener.Close()
		return e
	}
	defer os.Remove(linkPath)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(listener) }()
	if open != nil {
		open(origin + "/#" + token)
	}
	select {
	case <-done:
	case <-ctx.Done():
		e = ctx.Err()
	case e = <-errCh:
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	return e
}
func listenerHost(origin string) string { return origin[len("http://"):] }

const reconnectHTML = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Connect MyAccounts email</title><style>body{font:16px system-ui;background:#f4f6fb;color:#17243a;margin:0}main{max-width:480px;margin:8vh auto;padding:32px;background:white;border-radius:20px}h1{font-size:26px}p{line-height:1.6;color:#63708a}input,button{font:inherit;padding:12px;border-radius:8px;margin:8px 0}input{border:1px solid #dce3ee;width:90%}button{background:#345ad8;color:white;border:0;cursor:pointer}[hidden]{display:none}label{display:block}#status{min-height:40px}@media(max-width:550px){main{margin:20px;padding:24px}}</style><main><h1>Connect MyAccounts email</h1><p id="address"></p><p>No email password or server settings needed. Verify your address to enable recovery on the default MyAccounts pages.</p><button id="send">Send verification code</button><div id="verify" hidden><label for="code">Code from your email</label><input id="code" inputmode="numeric" autocomplete="one-time-code" maxlength="6" placeholder="6-digit code"><button id="confirm">Verify and connect</button></div><p id="status" role="status"></p></main><script>const token=location.hash.slice(1);history.replaceState(null,'',location.pathname);const $=id=>document.getElementById(id);fetch('/info').then(r=>r.json()).then(p=>$('address').textContent='Connect '+p.email);async function post(code){const r=await fetch('/connect',{method:'POST',headers:{'Content-Type':'application/json','X-Setup-Token':token},body:JSON.stringify({code})});if(!r.ok)throw Error(await r.text())}$('send').onclick=async()=>{$('send').disabled=true;try{await post('');$('verify').hidden=false;$('status').textContent='Check your inbox. Enter the code within 10 minutes.';$('code').focus()}catch(e){$('status').textContent=e.message}finally{$('send').disabled=false}};$('confirm').onclick=async()=>{$('confirm').disabled=true;try{await post($('code').value);$('verify').hidden=true;$('send').hidden=true;$('status').textContent='Email connected. Restart MyAccounts to apply the connection.'}catch(e){$('status').textContent=e.message;$('confirm').disabled=false}};</script></html>`
