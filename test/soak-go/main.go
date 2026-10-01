// One notes.watch() call through the generated Go SDK, for the soak matrix (test/soak.mjs): prints
// each note as a JSON line and exits when the stream ends (0) or fails (1). The caller follows the
// client rule: run again with -after <last id>.
//
//	go run . -base <origin> [-after <id>] [-seconds 15]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	orpcapi "example.com/orpcapi"
	"example.com/orpcapi/client"
	"example.com/orpcapi/option"
)

func main() {
	base := flag.String("base", "", "API origin")
	after := flag.String("after", "", "resume after this note id")
	seconds := flag.Int("seconds", 15, "stream length")
	flag.Parse()

	c := client.NewClient(option.WithBaseURL(*base))
	req := &orpcapi.WatchNotesRequest{Seconds: seconds}
	if *after != "" {
		req.After = after
	}
	stream, err := c.Notes.Watch(context.Background(), req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "watch:", err)
		os.Exit(1)
	}
	defer stream.Close()
	for {
		note, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "recv:", err)
			os.Exit(1)
		}
		line, _ := json.Marshal(map[string]any{"id": note.ID, "body": note.Body, "created_at": note.CreatedAt})
		fmt.Println(string(line))
	}
}
