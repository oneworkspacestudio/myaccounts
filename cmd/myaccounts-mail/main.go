package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/casdoor/casdoor/internal/mailrelay"
	"github.com/casdoor/casdoor/internal/mailservice"
	"github.com/casdoor/casdoor/internal/portable"
	"github.com/gofrs/flock"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	if e := run(); e != nil {
		log.Print(e)
		os.Exit(1)
	}
}
func run() error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	defaultDir := filepath.Join(filepath.Dir(exe), "mail-data")
	var location struct {
		Path string `json:"path"`
	}
	if b, e := os.ReadFile(filepath.Join(filepath.Dir(exe), "mail-data-location.json")); e == nil && json.Unmarshal(b, &location) == nil && filepath.IsAbs(location.Path) {
		defaultDir = location.Path
	}
	dir := flag.String("data", defaultDir, "protected service data directory")
	port := flag.Int("port", 19890, "loopback service port")
	noBrowser := flag.Bool("no-browser", false, "do not open the operator wizard")
	revoke := flag.String("revoke", "", "revoke an installation ID and exit")
	flag.Parse()
	*dir, e = filepath.Abs(*dir)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Join(*dir, "secrets"), 0700); e != nil {
		return e
	}
	if e = portable.ProtectDir(*dir); e != nil {
		explicit := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "data" {
				explicit = true
			}
		})
		if explicit || *noBrowser || *revoke != "" {
			return e
		}
		selected, chooseError := portable.ChooseDataDir()
		if chooseError != nil {
			return chooseError
		}
		selected, chooseError = filepath.Abs(selected)
		if chooseError != nil {
			return chooseError
		}
		if chooseError = os.MkdirAll(filepath.Join(selected, "secrets"), 0700); chooseError != nil {
			return chooseError
		}
		if chooseError = portable.ProtectDir(selected); chooseError != nil {
			return chooseError
		}
		*dir = selected
		b, _ := json.Marshal(struct {
			Path string `json:"path"`
		}{selected})
		if chooseError = portable.AtomicWrite(filepath.Join(filepath.Dir(exe), "mail-data-location.json"), b); chooseError != nil {
			return chooseError
		}
	}
	lock := flock.New(filepath.Join(*dir, "service.lock"))
	ok, e := lock.TryLock()
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("shared email service already owns this data directory")
	}
	defer lock.Unlock()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	config, e := mailservice.Load(*dir)
	if os.IsNotExist(e) {
		open := func(link string) {
			if !*noBrowser {
				_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", link).Start()
			}
		}
		if e = mailservice.Wizard(ctx, *dir, *port, open); e != nil {
			if errors.Is(e, mailservice.ErrSkipped) {
				return nil
			}
			return e
		}
		config, e = mailservice.Load(*dir)
	}
	if e != nil {
		return e
	}
	server, e := mailrelay.Open(filepath.Join(*dir, "relay.db"), config.SMTP.Deliver)
	if e != nil {
		return e
	}
	defer server.Close()
	if *revoke != "" {
		return server.Revoke(*revoke)
	}
	server.TrustCloudflare = config.Tunnel
	u, _ := url.Parse(config.PublicURL)
	public := server.Handler()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != u.Host {
			http.NotFound(w, r)
			return
		}
		public.ServeHTTP(w, r)
	})
	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", config.Port), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 40 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	logFile := &lumberjack.Logger{Filename: filepath.Join(*dir, "service.log"), MaxSize: 5, MaxBackups: 3, MaxAge: 14}
	defer logFile.Close()
	if config.Tunnel {
		go portable.RunTunnel(ctx, *dir, logFile)
	}
	go func() {
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		_ = srv.Shutdown(shutdown)
	}()
	e = srv.ListenAndServe()
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
