package intl

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
)

// translate is mise-intl translate [locale…]: Claude Code translates what is missing or stale with the
// intl skill, one worker per language at once (intl.toml [translate] workers, default 6) on [translate]
// model. Each worker edits only its own catalog and checks it with mise-intl check <locale>, which writes nothing;
// then gen and the full check run once.
func translate(ids []string) {
	c := app()
	claude, err := exec.LookPath("claude")
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ mise-intl translate runs Claude Code (https://code.claude.com): install it, or translate by hand (mise-intl todo)")
		os.Exit(1)
	}
	p := read(c)
	var work []*catalog.Locale
	for _, l := range pick(p, ids) {
		if len(l.Todo(p)) > 0 {
			work = append(work, l)
		}
	}
	if len(work) == 0 {
		fmt.Println("nothing to translate")
		return
	}
	n := c.Translate.Workers
	if n <= 0 {
		n = 6
	}
	model := c.Translate.Model
	if model == "" {
		model = "Claude Code's default model"
	}
	fmt.Printf("translating %d languages, %d at a time, on %s\n", len(work), n, model)

	start := time.Now()
	sem := make(chan struct{}, n)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, l := range work {
		wg.Add(1)
		go func(l *catalog.Locale, todo int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rel, _ := filepath.Rel(c.Root, l.File)
			prompt := fmt.Sprintf("Translate this app's messages into %[1]s with the intl skill: the %[2]d missing and stale "+
				"messages of %[1]s, nothing else. Other languages are being translated at the same time, so edit only %[3]s, never run "+
				"mise-intl gen, and check your work with mise-intl check %[1]s until it passes. Then say in one line what you did.",
				l.ID, todo, rel)
			// Fenced to its own language: it may edit only its catalog (an Edit rule covers every file-editing tool,
			// Write too), and confirm only its own stale messages.
			args := []string{"-p", prompt, "--allowedTools",
				"Edit(" + rel + ")", "Read", "Glob", "Grep",
				"Bash(mise-intl todo -json " + l.ID + ")", "Bash(mise-intl check " + l.ID + ")", "Bash(mise-intl confirm " + l.ID + ":*)"}
			if c.Translate.Model != "" {
				args = append(args, "--model", c.Translate.Model)
			}
			cmd := exec.Command(claude, args...)
			cmd.Dir = c.Root
			out, err := cmd.CombinedOutput()
			mu.Lock()
			defer mu.Unlock()
			mark := "✓"
			if err != nil {
				mark = "✗"
			}
			fmt.Printf("%s %s, %d messages, %s: %s\n", mark, l.ID, todo, time.Since(start).Round(time.Second), strings.TrimSpace(string(out)))
		}(l, len(l.Todo(p)))
	}
	wg.Wait()
	must(read(c).Generate(os.Stderr))
	os.Exit(checkAll(c, false))
}

// locked runs f holding a lock file beside path, so concurrent mise-intl processes (translate's workers
// confirming) don't overwrite each other's intl.sum.
func locked(path string, f func() error) error {
	lock := path + ".lock"
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(50 * time.Millisecond) {
		fd, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			fd.Close()
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("intl: %s held for a minute: remove it if no mise-intl is running", lock)
		}
	}
	defer os.Remove(lock)
	return f()
}
