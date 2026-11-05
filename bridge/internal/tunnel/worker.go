package tunnel

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type workerRequest struct {
	action   string
	reply    chan workerReply
	finished chan struct{}
}
type workerReply struct {
	state workerStatus
	err   error
}

// The supervisor captures configuration and credentials once. Reconnects and
// transient child retries use that snapshot; only Restart adopts a saved revision.
func runWorker(root string) error {
	setup, key, profile, err := workerConfiguration(root)
	if err != nil {
		return err
	}
	snapshot := filepath.Join(root, "runtime", "active-"+sessionToken()+".yaml")
	if err = writePrivate(snapshot, profile); err != nil {
		return err
	}
	defer os.Remove(snapshot)
	token := sessionToken()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	requests := make(chan workerRequest)
	done := make(chan struct{})
	server := &http.Server{ReadHeaderTimeout: 2 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := strings.TrimPrefix(r.URL.Path, "/")
		allowed := r.Method == http.MethodGet && action == "status" || r.Method == http.MethodPost && (action == "connect" || action == "disconnect" || action == "stop")
		if !allowed || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		reply := make(chan workerReply, 1)
		finished := make(chan struct{})
		defer close(finished)
		select {
		case requests <- workerRequest{action, reply, finished}:
		case <-done:
			w.WriteHeader(503)
			return
		case <-r.Context().Done():
			return
		}
		select {
		case result := <-reply:
			if result.err != nil {
				w.WriteHeader(409)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(result.state)
		case <-done:
			w.WriteHeader(503)
		case <-r.Context().Done():
		}
	})}
	defer func() {
		close(done)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = listener.Close()
	}()
	control, _ := json.Marshal(controlData{Port: listener.Addr().(*net.TCPAddr).Port, Token: token})
	if err = writePrivate(filepath.Join(root, "runtime", "control.json"), control); err != nil {
		return err
	}
	go func() { _ = server.Serve(listener) }()
	connecting := setup.autoConnect()
	status := func() workerStatus {
		current := connecting
		return workerStatus{Label: "vidotter-mcp-" + strings.TrimPrefix(serviceName(root), "VidOtter-MCP-"), Revision: setup.Revision, Connecting: &current}
	}
	log, err := os.OpenFile(filepath.Join(root, "runtime", "tunnel.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	if err = protectPrivate(log.Name(), false); err != nil {
		return err
	}
	var child *exec.Cmd
	var terminate func() error
	var exited chan error
	var retry <-chan time.Time
	start := func() {
		if !connecting || child != nil {
			return
		}
		cmd := exec.Command(setup.Tunnel, append(append([]string{}, setup.TunnelArgs...), "run", "--config", snapshot)...)
		cmd.Dir = root
		cmd.Stdout = log
		cmd.Stderr = log
		for _, value := range os.Environ() {
			name := strings.ToUpper(strings.SplitN(value, "=", 2)[0])
			if name != "OPENAI_API_KEY" && name != "CONTROL_PLANE_API_KEY" {
				cmd.Env = append(cmd.Env, value)
			}
		}
		cmd.Env = append(cmd.Env, "OPENAI_API_KEY="+key)
		stopChild, err := launchChild(cmd)
		if err != nil {
			retry = time.After(5 * time.Second)
			return
		}
		terminate = stopChild
		child = cmd
		exited = make(chan error, 1)
		go func() { exited <- cmd.Wait() }()
	}
	stop := func() error {
		retry = nil
		if child == nil {
			return nil
		}
		select {
		case <-exited:
			_ = terminate()
			child = nil
			exited = nil
			return nil
		default:
		}
		if err := terminate(); err != nil {
			return errors.New("could not stop owned Tunnel process")
		}
		select {
		case <-exited:
			child = nil
			exited = nil
			return nil
		case <-time.After(4 * time.Second):
			return errors.New("Tunnel process did not stop")
		}
	}
	defer func() { _ = stop() }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	start()
	for {
		select {
		case <-signals:
			return stop()
		case <-exited:
			_ = terminate()
			child = nil
			exited = nil
			if connecting {
				retry = time.After(5 * time.Second)
			}
		case <-retry:
			retry = nil
			start()
		case request := <-requests:
			var err error
			switch request.action {
			case "connect":
				connecting = true
				start()
			case "disconnect":
				connecting = false
				err = stop()
			case "stop":
				connecting = false
				err = stop()
			}
			request.reply <- workerReply{status(), err}
			if request.action == "stop" && err == nil {
				<-request.finished
				return nil
			}
		}
	}
}

func (b startupBackend) remove(root string, setup persistedSetup) error {
	name := b.name(root, setup)
	if b.platform == "windows" {
		exists, _, _, err := b.windowsLoginTask(root, setup)
		if err != nil || !exists {
			return err
		}
		_, err = b.run("schtasks.exe", "/delete", "/tn", name, "/f")
		return err
	}
	file := b.file(root, setup)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil
	}
	if b.platform == "darwin" {
		if _, err := b.run("launchctl", "print", b.domain(root, setup)+"/"+name); err == nil {
			if _, err = b.run("launchctl", "bootout", b.domain(root, setup)+"/"+name); err != nil {
				return err
			}
		}
	}
	if err := os.Remove(file); err != nil {
		return err
	}
	if b.platform == "linux" {
		_, err := b.systemctl(root, setup, "daemon-reload")
		return err
	}
	return nil
}

// RootForExecutable keeps private MCP files inside this installation's Data/.
func RootForExecutable(executable string) string {
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	dir := filepath.Dir(executable)
	if strings.EqualFold(filepath.Base(dir), "bin") {
		dir = filepath.Dir(dir)
	}
	return filepath.Join(dir, "Data", "mcp")
}
func ClientInstalled(root string) bool {
	info, err := os.Stat(tunnel(root))
	return err == nil && info.Mode().IsRegular()
}
func RequireClient(root string) error {
	if !ClientInstalled(root) {
		return fmt.Errorf("install the official Tunnel Client first")
	}
	return nil
}
