package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/casdoor/casdoor/internal/portable"
	"github.com/gofrs/flock"
	"github.com/kardianos/service"
	"gopkg.in/natefinch/lumberjack.v2"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

//go:embed web/build
var assets embed.FS
var version = "dev"

func main() {
	if e := entry(); e != nil {
		log.Printf("MyAccounts: %v", e)
		os.Exit(1)
	}
}
func entry() error {
	if len(os.Args) > 1 && os.Args[1] == "__engine" {
		if os.Getenv("MYACCOUNTS_SHUTDOWN") == "" {
			return errors.New("the identity engine must be started by its launcher")
		}
		os.Args = append(os.Args[:1], os.Args[2:]...)
		lock := flock.New("engine.lock")
		ok, e := lock.TryLock()
		if e != nil {
			return e
		}
		if !ok {
			return errors.New("identity engine is already running")
		}
		defer lock.Unlock()
		runEngine()
		return nil
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	flags := flag.NewFlagSet("MyAccounts", flag.ContinueOnError)
	defaultDir := portable.DefaultDataDir(exe)
	var location struct {
		Path string `json:"path"`
	}
	if b, e := os.ReadFile(filepath.Join(filepath.Dir(exe), "data-location.json")); e == nil {
		if json.Unmarshal(b, &location) == nil && filepath.IsAbs(location.Path) {
			defaultDir = location.Path
		}
	}
	if d := os.Getenv("MYACCOUNTS_DATA"); d != "" {
		defaultDir = d
	}
	dir := flags.String("data", defaultDir, "protected data folder (NTFS recommended)")
	port := flags.Int("port", 8788, "local HTTP port")
	noBrowser := flags.Bool("no-browser", false, "do not open the browser")
	if e = flags.Parse(os.Args[1:]); e != nil {
		return e
	}
	absolute, e := filepath.Abs(*dir)
	if e != nil {
		return e
	}
	*dir = absolute
	args := flags.Args()
	if len(args) > 0 && args[0] == "update-check" {
		r, e := portable.CheckUpdate(context.Background(), os.Getenv("MYACCOUNTS_GITHUB_TOKEN"))
		if e != nil {
			return e
		}
		fmt.Printf("Current: %s\nAvailable: %s\n%s\n", version, r.Tag, r.URL)
		return nil
	}
	if len(args) > 0 && args[0] == "verify" {
		c, e := portable.Load(*dir)
		if e != nil {
			return e
		}
		if e = portable.VerifyIssuer(context.Background(), c.Issuer, nil); e != nil {
			return e
		}
		fmt.Println("Issuer HTTPS and discovery verified")
		return nil
	}
	if len(args) > 0 && args[0] == "version" {
		fmt.Println("MyAccounts", version)
		return nil
	}
	if len(args) > 0 && args[0] == "restore" {
		if len(args) != 3 {
			return errors.New("usage: -data NEW_DIRECTORY restore BACKUP PASSPHRASE_FILE")
		}
		p, e := os.ReadFile(args[2])
		if e != nil {
			return e
		}
		return portable.Restore(args[1], *dir, strings.TrimSpace(string(p)))
	}
	e = os.MkdirAll(*dir, 0700)
	if e == nil {
		e = portable.ProtectDir(*dir)
	}
	if e != nil {
		explicitData := os.Getenv("MYACCOUNTS_DATA") != ""
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "data" {
				explicitData = true
			}
		})
		if explicitData || *noBrowser || len(args) > 0 {
			return e
		}
		selected, chooseErr := portable.ChooseDataDir()
		if chooseErr != nil {
			return chooseErr
		}
		selected, chooseErr = filepath.Abs(selected)
		if chooseErr != nil {
			return chooseErr
		}
		if _, statErr := os.Stat(filepath.Join(selected, "config.json")); statErr == nil {
			if _, chooseErr = portable.Load(selected); chooseErr != nil {
				return chooseErr
			}
		} else if os.IsNotExist(statErr) {
			selected = filepath.Join(selected, "MyAccounts", "data")
		} else {
			return statErr
		}
		if chooseErr = os.MkdirAll(selected, 0700); chooseErr != nil {
			return chooseErr
		}
		if chooseErr = portable.ProtectDir(selected); chooseErr != nil {
			return chooseErr
		}
		*dir = selected
		b, _ := json.Marshal(struct {
			Path string `json:"path"`
		}{selected})
		_ = portable.AtomicWrite(filepath.Join(filepath.Dir(exe), "data-location.json"), b)
	}
	lock := flock.New(filepath.Join(*dir, "launcher.lock"))
	if len(args) > 0 && args[0] == "stop" {
		c, e := portable.Load(*dir)
		if e != nil {
			return e
		}
		b, e := os.ReadFile(filepath.Join(*dir, "secrets", "control.dpapi"))
		if e != nil {
			return errors.New("no running launcher control credential")
		}
		token, e := portable.Unprotect(b)
		if e != nil {
			return e
		}
		req, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/internal/shutdown", c.Port), nil)
		req.Header.Set("X-Shutdown-Token", string(token))
		r, e := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if e != nil {
			return e
		}
		r.Body.Close()
		if r.StatusCode != 204 {
			return errors.New("shutdown was rejected")
		}
		for n := 0; n < 100; n++ {
			ok, e := lock.TryLock()
			if e != nil {
				return e
			}
			if ok {
				lock.Unlock()
				fmt.Println("MyAccounts stopped cleanly")
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return errors.New("launcher shutdown deadline exceeded")
	}
	if len(args) > 0 && args[0] == "service" {
		return controlService(exe, *dir, args[1:])
	}
	ok, e := lock.TryLock()
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("another MyAccounts instance owns this data directory")
	}
	defer lock.Unlock()
	if len(args) > 0 && args[0] == "backup" {
		if len(args) != 3 {
			return errors.New("usage: backup OUTPUT.age PASSPHRASE_FILE")
		}
		engine := flock.New(filepath.Join(*dir, "engine.lock"))
		ok, e = engine.TryLock()
		if e != nil {
			return e
		}
		if !ok {
			return errors.New("stop MyAccounts before backup")
		}
		defer engine.Unlock()
		p, e := os.ReadFile(args[2])
		if e != nil {
			return e
		}
		return portable.Backup(*dir, args[1], strings.TrimSpace(string(p)))
	}
	for _, sub := range []string{"conf", "secrets", "files", "uploads", "logs", "backups", "tmp"} {
		if e = os.MkdirAll(filepath.Join(*dir, sub), 0700); e != nil {
			return e
		}
	}
	if len(args) > 0 && args[0] == "tunnel" {
		if len(args) == 2 && args[1] == "download" {
			path, e := portable.EnsureConnector(context.Background(), *dir)
			if e != nil {
				return e
			}
			fmt.Println("Verified connector:", path)
			return nil
		}
		if len(args) == 3 && args[1] == "token" {
			c, e := portable.Load(*dir)
			if e != nil {
				return e
			}
			b, e := os.ReadFile(args[2])
			if e != nil {
				return e
			}
			b = []byte(strings.TrimSpace(string(b)))
			if len(b) < 32 {
				return errors.New("invalid tunnel token")
			}
			protected, e := portable.Protect(b)
			if e != nil {
				return e
			}
			if e = portable.AtomicWrite(filepath.Join(*dir, "secrets", "tunnel.dpapi"), protected); e != nil {
				return e
			}
			c.TunnelMode = "token"
			return portable.Save(*dir, c)
		}
		return errors.New("usage: tunnel download | tunnel token PROTECTED_TOKEN_FILE (stop the application first)")
	}
	log.SetOutput(&lumberjack.Logger{Filename: filepath.Join(*dir, "logs", "launcher.log"), MaxSize: 10, MaxBackups: 5, MaxAge: 30})
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if len(args) > 0 && args[0] == "run-service" {
		cancel()
		return runService(exe, *dir)
	}
	explicitPort := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			explicitPort = true
		}
	})
	if !explicitPort && !*noBrowser {
		if _, loadError := portable.Load(*dir); os.IsNotExist(loadError) {
			chosen, e := portable.AvailablePort(*port)
			if e != nil {
				return e
			}
			*port = chosen
		}
	}
	return launch(ctx, exe, *dir, *port, !*noBrowser)
}
func openBrowser(url string) {
	_ = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url).Start()
}
func launch(ctx context.Context, exe, dir string, port int, browser bool) (result error) {
	c, e := portable.Load(dir)
	if os.IsNotExist(e) {
		if !browser {
			return errors.New("run interactive setup before service mode")
		}
		e = portable.Setup(ctx, dir, port, openBrowser)
		if e != nil {
			return e
		}
		c, e = portable.Load(dir)
	}
	if e != nil {
		return e
	}
	guard, e := portable.PrepareUpgrade(dir, exe, version)
	if e != nil {
		return fmt.Errorf("pre-upgrade recovery snapshot failed: %w", e)
	}
	accepted := false
	defer func() {
		if !accepted && guard.HasSnapshot() {
			if e := guard.Rollback(); e != nil {
				result = fmt.Errorf("%w; rollback unavailable: %v", result, e)
			} else {
				result = fmt.Errorf("%w; prior database restored; run the previous executable", result)
			}
		}
	}()
	if e = extractAssets(dir); e != nil {
		return e
	}
	if e = writeEngineConfig(dir, c); e != nil {
		return e
	}
	shutdownToken := portable.RandomToken()
	control, e := portable.Protect([]byte(shutdownToken))
	if e != nil {
		return e
	}
	controlPath := filepath.Join(dir, "secrets", "control.dpapi")
	if e = portable.AtomicWrite(controlPath, control); e != nil {
		return e
	}
	defer os.Remove(controlPath)
	cmd := exec.Command(exe, "__engine")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "MYACCOUNTS_SHUTDOWN="+shutdownToken, "MYACCOUNTS_VERSION="+version)
	out := &lumberjack.Logger{Filename: filepath.Join(dir, "logs", "identity.log"), MaxSize: 10, MaxBackups: 5, MaxAge: 30}
	defer out.Close()
	cmd.Stdout = out
	cmd.Stderr = out
	if e = cmd.Start(); e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	local := fmt.Sprintf("http://localhost:%d", c.Port)
	client := &http.Client{Timeout: time.Second}
	ready := false
	for n := 0; n < 120; n++ {
		r, e := client.Get(local + "/healthz")
		if e == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				ready = true
				break
			}
		}
		select {
		case e := <-done:
			return fmt.Errorf("identity process stopped during startup: %w", e)
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if !ready {
		_ = cmd.Process.Kill()
		<-done
		return errors.New("identity startup timed out; inspect logs/identity.log")
	}
	if e = guard.Commit(); e != nil {
		_ = cmd.Process.Kill()
		<-done
		return e
	}
	accepted = true
	if browser && c.InstallServiceRequested {
		c.InstallServiceRequested = false
		if e = portable.Save(dir, c); e != nil {
			log.Print("Service preference could not be saved; interactive mode remains available")
		} else if e = portable.RequestServiceInstallation(exe, dir); e != nil {
			log.Print("Automatic service installation was not approved; interactive mode remains available")
		}
	}
	if browser {
		openBrowser(local)
	}
	go portable.RunScheduledBackups(ctx, dir)
	tunnelCtx, stopTunnel := context.WithCancel(ctx)
	defer stopTunnel()
	if c.TunnelMode == "token" {
		go portable.RunTunnel(tunnelCtx, dir, out)
	}
	select {
	case e := <-done:
		return e
	case <-ctx.Done():
	}
	req, _ := http.NewRequest("POST", local+"/internal/shutdown", nil)
	req.Header.Set("X-Shutdown-Token", shutdownToken)
	r, e := client.Do(req)
	if e == nil {
		r.Body.Close()
	}
	select {
	case e = <-done:
		return e
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		return errors.New("identity engine exceeded shutdown deadline")
	}
}
func extractAssets(dir string) error {
	return fs.WalkDir(assets, "web/build", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		b, e := assets.ReadFile(path)
		if e != nil {
			return e
		}
		return portable.AtomicWrite(target, b)
	})
}
func writeEngineConfig(dir string, c portable.Config) error {
	// URLs and names have already been validated; no credentials go into INI.
	logging, _ := json.Marshal(map[string]any{"adapter": "file", "filename": "logs/casdoor.log", "maxsize": 10485760, "maxfiles": 5, "daily": true, "maxdays": 30, "perm": "0600"})
	ini := fmt.Sprintf(`appname = MyAccounts
httpport = %d
httpaddr = 127.0.0.1
runmode = prod
copyrequestbody = true
driverName = sqlite
dataSourceName = file:users.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=foreign_keys(ON)
dbName = users.db
showSql = false
origin = %s
originFrontend = %s
staticBaseUrl =
isDemoMode = false
verificationCodeTimeout = 10
defaultLanguage = en
defaultApplication = app-built-in
showGithubCorner = false
enableErrorMask = true
enableErrorMask2 = true
enableGzip = true
initDataFile =
initDataNewOnly = true
ldapServerPort = 0
ldapsServerPort = 0
radiusServerPort = 0
sessionCookieLifeTime = 86400
logConfig = %s
`, c.Port, c.Issuer, c.Issuer, string(logging))
	return portable.AtomicWrite(filepath.Join(dir, "conf", "app.conf"), []byte(ini))
}

type daemon struct {
	exe, dir string
	cancel   context.CancelFunc
	done     chan error
}

func (d *daemon) Start(service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.done = make(chan error, 1)
	go func() { d.done <- launch(ctx, d.exe, d.dir, 8788, false) }()
	return nil
}
func (d *daemon) Stop(service.Service) error {
	d.cancel()
	select {
	case e := <-d.done:
		return e
	case <-time.After(20 * time.Second):
		return errors.New("service shutdown timed out")
	}
}
func serviceConfig(exe, dir string) *service.Config {
	return &service.Config{Name: "MyAccounts", DisplayName: "MyAccounts Identity", Description: "Portable identity and OpenID Connect", Executable: exe, Arguments: []string{"-data", dir, "-no-browser", "run-service"}, WorkingDirectory: filepath.Dir(exe), Option: service.KeyValue{"DelayedAutoStart": true, "OnFailure": "restart", "OnFailureDelayDuration": "10s", "OnFailureResetPeriod": 60}}
}
func runService(exe, dir string) error {
	s, e := service.New(&daemon{exe: exe, dir: dir}, serviceConfig(exe, dir))
	if e != nil {
		return e
	}
	return s.Run()
}
func controlService(exe, dir string, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: service install|start|stop|uninstall")
	}
	action := args[0]
	if action != "install-start" && action != "install" && action != "start" && action != "stop" && action != "uninstall" {
		return errors.New("unknown service action")
	}
	if action == "install" || action == "install-start" {
		if _, e := portable.Load(dir); e != nil {
			return errors.New("complete setup before installing the service")
		}
	}
	s, e := service.New(&daemon{exe: exe, dir: dir}, serviceConfig(exe, dir))
	if e != nil {
		return e
	}
	if action == "install-start" {
		if e = s.Install(); e != nil {
			return e
		}
		if _, e = os.Stat(filepath.Join(dir, "secrets", "control.dpapi")); e == nil {
			stop := exec.Command(exe, "-data", dir, "stop")
			if e = stop.Run(); e != nil {
				return errors.New("service installed; stop the interactive instance before starting it")
			}
		}
		return s.Start()
	}
	return service.Control(s, action)
}
