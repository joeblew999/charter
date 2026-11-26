//go:build js && wasm

package built

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/syumai/workers-go/cloudflare/fetch"
	"github.com/syumai/workers-go/cloudflare/sockets"
)

// fetch, and a TCP socket: a request the Worker makes to another site. Each waits on that site, so it is made when
// someone asks for it, never to show a page.

// elsewhere is IANA's example domain (Cloudflare refuses a request to the Worker's own address).
const elsewhere = "https://example.com/"

// Fetch asks another site over HTTP.
func Fetch(context.Context) Outside {
	start := time.Now()
	res, err := fetch.NewClient().HTTPClient(fetch.RedirectModeFollow).Get(elsewhere)
	if err != nil {
		return Outside{Asked: "GET " + elsewhere, Said: err.Error(), Took: time.Since(start), Failed: true}
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	return Outside{Asked: "GET " + elsewhere, Said: strconv.Itoa(res.StatusCode) + " " + res.Header.Get("Content-Type"), Took: time.Since(start)}
}

// socketHost is a site that is not on Cloudflare's own network, which a Worker's sockets may not reach.
const socketHost = "www.google.com"

// Socket writes HTTP by hand over a TCP socket, and gives the first line of the answer. workers-go's socket panics
// when a connection Cloudflare refused is closed, which would end this Go runtime: that is caught and shown.
func Socket(ctx context.Context) (out Outside) {
	start := time.Now()
	out.Asked = "TCP " + socketHost + ":80"
	defer func() {
		if p := recover(); p != nil {
			out.Said, out.Failed = fmt.Sprint(p), true
		}
		out.Took = time.Since(start)
	}()
	conn, err := sockets.Connect(ctx, socketHost+":80", nil)
	if err != nil {
		return Outside{Asked: out.Asked, Said: err.Error(), Failed: true}
	}
	defer conn.Close()
	fmt.Fprintf(conn, "HEAD / HTTP/1.0\r\nHost: %s\r\n\r\n", socketHost)
	first, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return Outside{Asked: out.Asked, Said: err.Error(), Failed: true}
	}
	out.Said = strings.TrimSpace(first)
	return out
}
