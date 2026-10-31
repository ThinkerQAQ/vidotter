package vidotter

import (
	"bufio"
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRepeatedInitializeOnOneTunnelStdioChildCreatesFreshMCPToolsSession(t *testing.T) {
	client := newMCPBridgeClient()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	defer stdinWriter.Close()
	defer stdoutReader.Close()
	done := make(chan error, 1)
	go func() {
		done <- runReinitializableMCPStdio(ctx, func() *mcp.Server { return newVidOtterMCPServer(client) }, stdinReader, stdoutWriter)
		stdoutWriter.Close()
	}()
	reader := bufio.NewReader(stdoutReader)
	send := func(v string) {
		t.Helper()
		if _, e := io.WriteString(stdinWriter, v+"\n"); e != nil {
			t.Fatal(e)
		}
	}
	receive := func(id float64) map[string]any {
		t.Helper()
		type line struct {
			data []byte
			err  error
		}
		deadline := time.After(4 * time.Second)
		for {
			ch := make(chan line, 1)
			go func() { b, e := reader.ReadBytes('\n'); ch <- line{b, e} }()
			select {
			case r := <-ch:
				if r.err != nil {
					t.Fatal("read MCP reply:", r.err)
				}
				var response map[string]any
				if e := json.Unmarshal(r.data, &response); e != nil {
					t.Fatal(e)
				}
				// The SDK may issue a tools/list_changed notification during
				// reinitialization. It has no ID and must not replace the reply.
				if response["id"] == nil {
					continue
				}
				if response["id"] != id {
					t.Fatalf("wrong response id: got %v want %v", response["id"], id)
				}
				if errBody, exists := response["error"]; exists {
					t.Fatalf("MCP returned error: %v", errBody)
				}
				return response
			case <-deadline:
				t.Fatal("timeout waiting for MCP stdio response")
				return nil
			}
		}
	}
	initialize := func(id int) {
		send(`{"jsonrpc":"2.0","id":` + stringJSONID(id) + `,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"stdio-reconnect-test","version":"1"}}}`)
		response := receive(float64(id))
		data, _ := json.Marshal(response)
		if !strings.Contains(string(data), "protocolVersion") {
			t.Fatal("not an initialize response")
		}
		send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	}
	list := func(id int) {
		send(`{"jsonrpc":"2.0","id":` + stringJSONID(id) + `,"method":"tools/list","params":{}}`)
		response := receive(float64(id))
		result, ok := response["result"].(map[string]any)
		if !ok {
			t.Fatal("missing tools result")
		}
		tools, ok := result["tools"].([]any)
		if !ok || len(tools) != 6 {
			t.Fatalf("expected 6 tools, got %d", len(tools))
		}
	}
	initialize(1)
	list(2)
	// A new ChatGPT discovery session against the SAME long-lived tunnel child.
	// Previously this triggered the SDK's `duplicate initialize received`.
	initialize(3)
	list(4)
	// Verify more than one reconnect, not just a one-time reset.
	initialize(5)
	list(6)
	stdinWriter.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("MCP child failed to exit on EOF")
	}
}
func stringJSONID(id int) string { return string([]byte{'0' + byte(id)}) }
