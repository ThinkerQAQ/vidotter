package vidotter

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tunnel-client uses ONE shared stdio child process, even when ChatGPT
// rediscovers/reinitializes tools. The MCP SDK correctly rejects a second
// initialize on the same session. This adapter provides a new sequential MCP
// session for each initialize without restarting the tunnel or touching data.
// It does not provide simultaneous multi-client isolation; newer sessions
// cancel the previous one. HTTP MCP remains fully stateless and is preferred
// when a private loopback HTTP bridge is available.
func runReinitializableMCPStdio(ctx context.Context, server func() *mcp.Server, in io.Reader, out io.Writer) error {
	if server == nil || in == nil || out == nil {
		return errors.New("nil MCP stdio argument")
	}
	const maxFrame = 32 << 20
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), maxFrame)
	var outputMu sync.Mutex
	var generation atomic.Uint64
	type session struct {
		writer *io.PipeWriter
		cancel context.CancelFunc
		done   chan error
	}
	var current *session
	stop := func() {
		if current == nil {
			return
		}
		current.writer.Close()
		current.cancel()
		select {
		case <-current.done:
		case <-time.After(2 * time.Second):
		}
		current = nil
	}
	defer stop()
	for scanner.Scan() {
		frame := append([]byte(nil), scanner.Bytes()...)
		// Inspect method only, never log or copy raw order payloads to disk.
		var envelope struct {
			Method string `json:"method"`
			Params struct {
				Meta map[string]json.RawMessage `json:"_meta"`
			} `json:"params"`
		}
		if err := json.Unmarshal(frame, &envelope); err != nil {
			continue
		}
		// The 2026-07-28 MCP protocol uses self-contained requests and
		// server/discover instead of the legacy initialize handshake. Start
		// the same SDK session for either protocol before forwarding frames.
		var version string
		_ = json.Unmarshal(envelope.Params.Meta["io.modelcontextprotocol/protocolVersion"], &version)
		modern := version >= "2026-07-28"
		start := envelope.Method == "initialize" || current == nil &&
			(envelope.Method == "server/discover" || modern && envelope.Method != "")
		if start {
			generation.Add(1)
			stop()
			rdr, wtr := io.Pipe()
			sessionCtx, cancel := context.WithCancel(ctx)
			done := make(chan error, 1)
			gen := generation.Load()
			writer := &generationWriter{dst: out, mu: &outputMu, current: &generation, gen: gen}
			current = &session{writer: wtr, cancel: cancel, done: done}
			go func() {
				done <- server().Run(sessionCtx, &mcp.IOTransport{Reader: rdr, Writer: writer})
			}()
		}
		if current == nil {
			continue
		} // initialize is required before other MCP frames
		if _, err := current.writer.Write(append(frame, '\n')); err != nil {
			stop()
			return fmt.Errorf("MCP session pipe closed: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

type generationWriter struct {
	dst     io.Writer
	mu      *sync.Mutex
	current *atomic.Uint64
	gen     uint64
}

func (w *generationWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current.Load() != w.gen {
		return len(p), nil
	}
	return w.dst.Write(p)
}
func (w *generationWriter) Close() error { return nil }
