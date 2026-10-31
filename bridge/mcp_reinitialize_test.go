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
		if !ok || len(tools) != 13 {
			t.Fatalf("expected 13 tools, got %d", len(tools))
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

func TestModernMCPDiscoveryAndToolsOnSameTunnelChild(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	done := make(chan error, 1)
	go func() {
		defer stdoutWriter.Close()
		done <- runReinitializableMCPStdio(ctx, func() *mcp.Server {
			return newVidOtterMCPServer(newMCPBridgeClient())
		}, stdinReader, stdoutWriter)
	}()
	reader := bufio.NewReader(stdoutReader)
	modernMeta := map[string]any{
		"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
		"io.modelcontextprotocol/clientInfo":         map[string]string{"name": "regression", "version": "1"},
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}
	send := func(id int, method string, params map[string]any) map[string]any {
		t.Helper()
		if params == nil {
			params = map[string]any{}
		}
		params["_meta"] = modernMeta
		body, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": id, "method": method, "params": params,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stdinWriter.Write(append(body, '\n')); err != nil {
			t.Fatal(err)
		}
		type wireResult struct {
			body []byte
			err  error
		}
		ch := make(chan wireResult, 1)
		go func() {
			data, err := reader.ReadBytes('\n')
			ch <- wireResult{data, err}
		}()
		select {
		case answer := <-ch:
			if answer.err != nil {
				t.Fatal(answer.err)
			}
			var response map[string]any
			if err := json.Unmarshal(answer.body, &response); err != nil {
				t.Fatal(err)
			}
			if _, exists := response["error"]; exists {
				t.Fatalf("%s error: %v", method, response["error"])
			}
			if got, _ := response["id"].(float64); got != float64(id) {
				t.Fatalf("%s response ID = %v, want %d", method, response["id"], id)
			}
			result, ok := response["result"].(map[string]any)
			if !ok || result["resultType"] != "complete" {
				t.Fatalf("%s missing complete result: %#v", method, response)
			}
			return result
		case <-time.After(4 * time.Second):
			t.Fatalf("%s timed out", method)
			return nil
		}
	}
	first := send(1, "server/discover", nil)
	versions, ok := first["supportedVersions"].([]any)
	if !ok || len(versions) == 0 || versions[0] != "2026-07-28" {
		t.Fatalf("modern discovery versions: %v", first)
	}
	for _, id := range []int{2, 4} {
		list := send(id, "tools/list", nil)
		tools, ok := list["tools"].([]any)
		if !ok || len(tools) != 13 {
			t.Fatalf("modern tools/list returned %v", list)
		}
		if id == 2 {
			send(3, "server/discover", nil)
		}
	}
	result := send(5, "tools/call", map[string]any{
		"name": "vidotter_status", "arguments": map[string]any{},
	})
	if _, ok := result["structuredContent"]; !ok {
		t.Fatalf("modern tools/call missing content: %#v", result)
	}
	stdinWriter.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("modern MCP child failed to exit")
	}
}
