// Package contract checks what any client of the game server can see,
// through the public API only: status codes, error strings, JSON shapes and
// key order, cookies, SSE framing, static files and link previews. It runs
// against a real server, so it is off in a plain `go test ./...`:
//
//	pnpm --dir web build
//	CONTRACT=1 go test ./contract -count=1
//
// builds ./cmd/server, runs it on a free port with a new database and
// stops it at the end. BASE_URL=http://host:port tests a server that is
// already running instead. CONTRACT_SLOW=1 adds the check that needs a real
// minute (the first-move abort). The test binary doesn't import server or
// cmd/server, so a cached pass misses changes to the handlers: pass
// -count=1.
package contract

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// base is the server under test, "" when the checks skip.
var base string

// slow adds the minute-long checks.
var slow = os.Getenv("CONTRACT_SLOW") == "1"

const skipReason = "contract checks need a server: run `pnpm --dir web build`, then CONTRACT=1 go test ./contract (or set BASE_URL)"

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	if base = os.Getenv("BASE_URL"); base != "" || os.Getenv("CONTRACT") != "1" {
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "contract-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "contract:", err)
		return 1
	}
	defer os.RemoveAll(dir)
	srv, err := startServer(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "contract:", err)
		return 1
	}
	base = srv.url
	code := m.Run()
	if err := srv.stop(); err != nil {
		fmt.Fprintln(os.Stderr, "contract: the server didn't stop cleanly:", err)
		code = 1
	}
	if code != 0 {
		fmt.Fprintf(os.Stderr, "--- server log\n%s", srv.logs())
	}
	return code
}

// server is the game server binary running for the checks.
type server struct {
	url    string
	cmd    *exec.Cmd
	exited chan error
	log    string // the server's stderr, a file next to its database
}

// startServer builds ./cmd/server into dir and runs it from the repo root,
// so it serves web/build, on a free port with a new database in dir. It
// returns once /healthz answers. The binary runs itself, not under go run,
// which doesn't pass SIGTERM on to the server.
func startServer(dir string) (*server, error) {
	root, err := filepath.Abs("..")
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(root, "web", "build", "index.html")); err != nil {
		return nil, errors.New("web/build has no index.html: run `pnpm --dir web build` first")
	}
	bin := filepath.Join(dir, "server")
	build := exec.Command("go", "build", "-o", bin, "./cmd/server")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("go build: %v\n%s", err, out)
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	s := &server{url: "http://localhost:" + strconv.Itoa(port), exited: make(chan error, 1), log: filepath.Join(dir, "server.log")}
	// The log goes to a file, not a pipe: a pipe drained late can hold the
	// server up.
	logFile, err := os.Create(s.log)
	if err != nil {
		return nil, err
	}
	defer logFile.Close() // the server has its own copy
	s.cmd = exec.Command(bin)
	s.cmd.Dir = root
	s.cmd.Env = append(os.Environ(), "PORT="+strconv.Itoa(port), "DB_PATH="+filepath.Join(dir, "contract.db"))
	s.cmd.Stderr = logFile
	if err := s.cmd.Start(); err != nil {
		return nil, err
	}
	go func() { s.exited <- s.cmd.Wait() }()

	deadline := time.Now().Add(60 * time.Second)
	for {
		if s.healthy() {
			return s, nil
		}
		select {
		case err := <-s.exited:
			return nil, fmt.Errorf("the server exited (%v):\n%s", err, s.logs())
		case <-time.After(25 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			s.cmd.Process.Kill()
			return nil, fmt.Errorf("the server didn't answer /healthz within a minute:\n%s", s.logs())
		}
	}
}

func (s *server) healthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/healthz", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// stop sends SIGTERM, as Railway does on a deploy, and waits for the server
// to exit; after 15 s it kills it.
func (s *server) stop() error {
	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	select {
	case err := <-s.exited:
		return err
	case <-time.After(15 * time.Second):
		s.cmd.Process.Kill()
		<-s.exited
		return errors.New("still running 15 s after SIGTERM; killed")
	}
}

// logs is the server's stderr so far.
func (s *server) logs() string {
	b, _ := os.ReadFile(s.log)
	return string(b)
}

// freePort returns a TCP port nothing is listening on.
func freePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
