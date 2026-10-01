package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	commands["with-server"] = command{"-url <url> -start <cmd> [-dir <dir>] -run <cmd>...",
		"start a server, wait for <url>, run the commands, stop the server ({port} in any of them is a free port, {port2} another)", withServer}
	commands["migrate-local"] = command{"[-worker orpc-api] [-port <port>]",
		"apply migrations/*.sql to a running dev server's local D1, each once", migrateLocal}
	commands["migrate"] = command{"[-worker orpc-api]",
		"REMOTE: apply pending migrations/ to the Worker's D1 database (<worker>-db)", migrate}
	commands["size"] = command{"-max <bytes> <file>", "fail if <file>, gzipped, is larger than <bytes>", size}
}

type list []string

func (l *list) String() string     { return strings.Join(*l, "; ") }
func (l *list) Set(v string) error { *l = append(*l, v); return nil }

// server starts `start` (a shell command) in dir and waits until url answers. stop ends it.
func server(dir, start, url string) (stop func(), err error) {
	if _, err := http.Get(url); err == nil {
		return nil, fmt.Errorf("%s already answers: something else is running there", url)
	}
	log, err := os.CreateTemp("", "dev-server-*.log")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("bash", "-c", start)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // so its children stop with it
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }()
	stop = func() {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-exited
		}
		os.Remove(log.Name())
	}
	showLog := func() {
		if out, err := os.ReadFile(log.Name()); err == nil {
			os.Stderr.Write(out)
		}
	}
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		select {
		case <-exited:
			showLog()
			os.Remove(log.Name())
			return nil, fmt.Errorf("the server stopped before %s answered: %s", url, start)
		default:
		}
		if res, err := http.Get(url); err == nil {
			res.Body.Close()
			return stop, nil
		}
	}
	showLog()
	stop()
	return nil, fmt.Errorf("%s did not answer within 90 s: %s", url, start)
}

func withServer(args []string) error {
	var url, start, dir string
	var runs list
	flags("with-server", args, func(f *flag.FlagSet) {
		f.StringVar(&url, "url", "", "what must answer before the commands run")
		f.StringVar(&start, "start", "", "the server's command (a shell line)")
		f.StringVar(&dir, "dir", ".", "where to start the server and run the commands")
		f.Var(&runs, "run", "a command to run while the server is up (a shell line; repeat)")
	})
	if url == "" || start == "" || len(runs) == 0 {
		return errors.New("with-server needs -url, -start and at least one -run")
	}
	// {port} is a free port, the same one everywhere: several checks can then run at once (two
	// worktrees, two agents) without agreeing on port numbers. {port2} is a second one, for a test
	// that listens itself (a webhook receiver the server is told about).
	ports, err := freePorts(2)
	if err != nil {
		return err
	}
	fill := strings.NewReplacer("{port}", ports[0], "{port2}", ports[1])
	url, start = fill.Replace(url), fill.Replace(start)
	for i := range runs {
		runs[i] = fill.Replace(runs[i])
	}
	stop, err := server(dir, start, url)
	if err != nil {
		return err
	}
	defer stop()
	for _, run := range runs {
		if err := quiet(dir, nil, "bash", "-c", run); err != nil {
			stop()
			return fmt.Errorf("failed: %s", run)
		}
	}
	return nil
}

// freePort asks the system for a port nobody is using.
func freePort() (string, error) {
	ports, err := freePorts(1)
	if err != nil {
		return "", err
	}
	return ports[0], nil
}

// freePorts asks for n different ones: it holds them all before letting any go.
func freePorts(n int) ([]string, error) {
	var ports []string
	for range n {
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			return nil, err
		}
		defer listener.Close()
		ports = append(ports, strconv.Itoa(listener.Addr().(*net.TCPAddr).Port))
	}
	return ports, nil
}

// workerFlags are what both migrate commands take.
func workerFlags(name string, args []string, port *string) string {
	var worker string
	flags(name, args, func(f *flag.FlagSet) {
		f.StringVar(&worker, "worker", "orpc-api", "orpc-api, or orpc-api-go (the Go Worker)")
		if port != nil {
			f.StringVar(port, "port", "", "the dev server's port (default: PORT, or API_GO_PORT for orpc-api-go)")
		}
	})
	return worker
}

func migrations() ([]string, error) {
	files, err := filepath.Glob("migrations/*.sql")
	sort.Strings(files)
	if err == nil && len(files) == 0 {
		err = errors.New("no migrations/*.sql")
	}
	return files, err
}

// migrateLocal applies each migration once through the dev server's D1 explorer API (cf can't
// migrate the local database), remembering them in a table of its own.
func migrateLocal(args []string) error {
	var port string
	worker := workerFlags("migrate-local", args, &port)
	if port == "" {
		name := "API_GO_PORT" // the Go Worker's, whatever it is called in this project
		if worker == "orpc-api" {
			name = "PORT"
		}
		var err error
		if port, err = env(name); err != nil {
			return err
		}
	}
	base := "http://localhost:" + port + "/cdn-cgi/local/explorer/api/d1/database/DB-" + worker + "/raw"
	sql := func(statement string) (first any, err error) {
		body, _ := json.Marshal(map[string]string{"sql": statement})
		res, err := http.Post(base, "application/json", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("no %s dev server on port %s", worker, port)
		}
		defer res.Body.Close()
		var reply struct {
			Success bool
			Errors  []any
			Result  []struct{ Results struct{ Rows [][]any } }
		}
		if err := json.NewDecoder(res.Body).Decode(&reply); err != nil || !reply.Success {
			return nil, fmt.Errorf("local D1 DB-%s refused (HTTP %d %v): %s", worker, res.StatusCode, reply.Errors, statement)
		}
		if n := len(reply.Result); n > 0 && len(reply.Result[n-1].Results.Rows) > 0 && len(reply.Result[n-1].Results.Rows[0]) > 0 {
			first = reply.Result[n-1].Results.Rows[0][0]
		}
		return first, nil
	}
	if _, err := sql("CREATE TABLE IF NOT EXISTS _local_migrations (name TEXT PRIMARY KEY, applied_at TEXT DEFAULT CURRENT_TIMESTAMP)"); err != nil {
		return err
	}
	files, err := migrations()
	if err != nil {
		return err
	}
	applied := 0
	for _, file := range files {
		name := filepath.Base(file)
		count, err := sql("SELECT count(*) FROM _local_migrations WHERE name = '" + name + "'")
		if err != nil {
			return err
		}
		if n, _ := count.(float64); n > 0 {
			continue
		}
		statement, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := sql(string(statement)); err != nil {
			return err
		}
		if _, err := sql("INSERT INTO _local_migrations (name) VALUES ('" + name + "')"); err != nil {
			return err
		}
		fmt.Println("applied", name)
		applied++
	}
	fmt.Printf("local DB-%s: %d migration(s) applied\n", worker, applied)
	return nil
}

// migrate finds the database's UUID (cf d1 takes UUIDs only, and lists one page unless asked for
// more) and applies what is pending.
func migrate(args []string) error {
	database := workerFlags("migrate", args, nil) + "-db"
	out, err := output("api", cf, "d1", "list", "--name", database, "--per-page", "100")
	if err != nil {
		return err
	}
	var databases []struct{ Name, UUID string }
	if err := json.Unmarshal([]byte(out), &databases); err != nil {
		return fmt.Errorf("cf d1 list: %w", err)
	}
	for _, d := range databases {
		if d.Name == database {
			return sh("api", cf, "d1", "migrations", "apply", d.UUID, "--dir", "../migrations")
		}
	}
	return fmt.Errorf("no D1 database %s yet: deploy its Worker first", database)
}

func size(args []string) error {
	var max int
	rest := flags("size", args, func(f *flag.FlagSet) { f.IntVar(&max, "max", 0, "the limit in bytes, gzipped") })
	if len(rest) != 1 || max == 0 {
		return errors.New("size needs -max <bytes> <file>")
	}
	raw, err := os.ReadFile(rest[0])
	if err != nil {
		return err
	}
	gz, err := gzipped(raw)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d B, %d B gzipped (limit %d)\n", rest[0], len(raw), gz, max)
	if gz > max {
		return fmt.Errorf("%s is over the limit", rest[0])
	}
	return nil
}
