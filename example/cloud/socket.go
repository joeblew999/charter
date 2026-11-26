//go:build js && wasm

package cloud

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/syumai/workers-go/cloudflare/sockets"
)

// A TCP socket: HTTP written by hand to another site, and the first line of the answer.

// socketHost is a site that is not on Cloudflare's own network, which a Worker's sockets may not reach (nor may they
// reach the Worker's own address).
const socketHost = "www.google.com"

// firstLine gives the first line host's port 80 answers, or what went wrong. workers-go's socket panics when a
// connection Cloudflare refused is closed, which would end this Go runtime: that is caught here and shown instead.
func firstLine(ctx context.Context, host string) (line string) {
	defer func() {
		if p := recover(); p != nil {
			line = fmt.Sprint(host, ":80: ", p)
		}
	}()
	conn, err := sockets.Connect(ctx, host+":80", nil)
	if err != nil {
		return err.Error()
	}
	defer conn.Close()
	fmt.Fprintf(conn, "HEAD / HTTP/1.0\r\nHost: %s\r\n\r\n", host)
	first, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return host + ":80: " + err.Error()
	}
	return host + ":80 → " + strings.TrimSpace(first)
}
