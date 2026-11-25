package intl

import (
	"cmp"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/joeblew999/charter/site/intl/workerd"
)

// daemon is charter intl daemon: workerd serving Intl on the files gen wrote beside the catalogs (intl-workerd/), in
// the foreground, at 127.0.0.1 on $PORT. It says when workerd answers.
func daemon(args []string) {
	c := app()
	bin, err := workerd.Path()
	must(err)
	addr := "127.0.0.1:" + cmp.Or(os.Getenv("PORT"), "8749")
	cmd := exec.Command(bin, "serve", filepath.Join(c.CatalogDir(), "intl-workerd", "intl-workerd.capnp"), "--socket-addr", "http="+addr)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	must(cmd.Start())
	go func() {
		for ; ; time.Sleep(50 * time.Millisecond) {
			if resp, err := http.Get("http://" + addr); err == nil {
				resp.Body.Close()
				fmt.Println("intl: workerd answers at http://" + addr)
				return
			}
		}
	}()
	if err := cmd.Wait(); err != nil {
		fail(fmt.Errorf("workerd: %w", err))
	}
}
