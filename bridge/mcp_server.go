package vidotter

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
)

func vidotterMCPTool(name, title, description string, readonly bool) *mcp.Tool {
	openWorld := name == "vidotter_download"
	destructive := false
	return &mcp.Tool{
		Name: name, Title: title, Description: description,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: readonly, OpenWorldHint: &openWorld, DestructiveHint: &destructive,
		},
	}
}

type mcpJobInput struct {
	ID string `json:"id" jsonschema:"VidOtter task ID returned when downloading"`
}
type mcpControlInput struct {
	ID     string `json:"id" jsonschema:"VidOtter task ID"`
	Action string `json:"action" jsonschema:"pause, resume, or retry"`
}
type mcpSubtitleInput struct {
	ID    string `json:"id" jsonschema:"Completed VidOtter task ID"`
	Index int    `json:"index,omitempty" jsonschema:"Zero-based subtitle file index, default 0"`
}
type mcpListResult struct {
	Jobs []mcpJobSummary `json:"jobs"`
}

func newVidOtterMCPServer(client *mcpBridgeClient) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "vidotter", Version: bridgeVersion}, &mcp.ServerOptions{
		Instructions: "Use VidOtter to download media that the user is authorized to download and manage local downloads. Downloads are asynchronous: start with vidotter_download, then poll vidotter_get_job. Only public HTTP(S) URLs are supported by AI tools; browser-authenticated media must be downloaded via the extension. Read subtitles only when the user asks for analysis or transcription. Never claim an enqueued task is complete or that local video bytes were uploaded. MCP responses omit URLs, local paths, cookies, and authorization tokens.",
	})
	mcp.AddTool(server, vidotterMCPTool("vidotter_status", "Check VidOtter", "Read-only Bridge and media dependency availability; does not start Bridge.", true), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, mcpStatus, error) {
		out, err := client.status(ctx)
		return nil, out, err
	})
	mcp.AddTool(server, vidotterMCPTool("vidotter_download", "Download media", "Enqueue a public media URL or supported public webpage for local download, optionally requesting subtitles. Returns a task ID immediately; downloading continues in the background.", false), func(ctx context.Context, req *mcp.CallToolRequest, in mcpDownloadInput) (*mcp.CallToolResult, mcpDownloadResult, error) {
		out, err := client.download(ctx, in)
		return nil, out, err
	})
	mcp.AddTool(server, vidotterMCPTool("vidotter_list_jobs", "List downloads", "List recent local download tasks and progress without source URLs or filesystem paths.", true), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, mcpListResult, error) {
		jobs, err := client.list(ctx)
		return nil, mcpListResult{Jobs: jobs}, err
	})
	mcp.AddTool(server, vidotterMCPTool("vidotter_get_job", "Get download", "Inspect one task's state, progress and subtitle count; never returns raw source URLs.", true), func(ctx context.Context, req *mcp.CallToolRequest, in mcpJobInput) (*mcp.CallToolResult, mcpJobSummary, error) {
		out, err := client.job(ctx, in.ID)
		return nil, out, err
	})
	mcp.AddTool(server, vidotterMCPTool("vidotter_control_job", "Control download", "Pause, resume or retry an existing download. Browser reauthorization is not permitted through MCP; destructive deletion is not exposed.", false), func(ctx context.Context, req *mcp.CallToolRequest, in mcpControlInput) (*mcp.CallToolResult, mcpJobSummary, error) {
		out, err := client.control(ctx, in.ID, in.Action)
		return nil, out, err
	})
	mcp.AddTool(server, vidotterMCPTool("vidotter_read_subtitle", "Read generated subtitles", "Read only a subtitle file already attached to a completed local VidOtter job (max 256 KiB); subtitle text is returned to the AI service.", true), func(ctx context.Context, req *mcp.CallToolRequest, in mcpSubtitleInput) (*mcp.CallToolResult, mcpSubtitleResult, error) {
		out, err := client.subtitle(ctx, in.ID, in.Index)
		return nil, out, err
	})
	return server
}

func runVidOtterMCP() error {
	client := newMCPBridgeClient()
	return runReinitializableMCPStdio(context.Background(), func() *mcp.Server {
		return newVidOtterMCPServer(client)
	}, os.Stdin, os.Stdout)
}
