package vidotter

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"vidotter/internal/tunnel"
)

func runMCPCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("MCP command required")
	}
	root := mcpRuntimeRoot()
	switch args[0] {
	case "--mcp-worker":
		return tunnel.RunWorker(root)
	case "--mcp-native-service":
		return tunnel.RunNativeService(root)
	case "--mcp-native":
		if len(args) < 2 {
			return errors.New("native service action required")
		}
		if args[1] == "uninstall" {
			return tunnel.UninstallService(root)
		}
		if args[1] != "install" {
			return errors.New("use --mcp-native install|uninstall")
		}
		flags := flag.NewFlagSet("mcp-native", flag.ContinueOnError)
		account := flags.String("account", "", "service account")
		scope := flags.String("scope", "user", "user or system")
		passwordStdin := flags.Bool("password-stdin", false, "read service-account password from stdin")
		if err := flags.Parse(args[2:]); err != nil {
			return err
		}
		password := ""
		if *passwordStdin {
			value, err := io.ReadAll(io.LimitReader(os.Stdin, 4097))
			if err != nil || len(value) > 4096 {
				return errors.New("invalid service password input")
			}
			password = strings.TrimRight(string(value), "\r\n")
			clear(value)
		}
		return tunnel.InstallService(root, *account, *scope, password)
	case "--mcp-service":
		if len(args) != 2 {
			return errors.New("use --mcp-service start|stop|restart|connect|disconnect|status")
		}
		if args[1] == "status" {
			return json.NewEncoder(os.Stdout).Encode(tunnel.Status(root))
		}
		return tunnel.Service(root, args[1])
	case "--mcp-configure":
		var input struct {
			ID      string `json:"id"`
			Key     string `json:"key"`
			Startup *bool  `json:"autostart_enabled"`
			Connect *bool  `json:"tunnel_auto_connect"`
		}
		decoder := json.NewDecoder(io.LimitReader(os.Stdin, 8192))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || !validTunnelID(input.ID) {
			return errors.New("provide connection settings as JSON on stdin")
		}
		if err := tunnel.RequireClient(root); err != nil {
			return err
		}
		return tunnel.Save(root, input.ID, input.Key, input.Startup, input.Connect)
	case "--mcp-lifecycle":
		if len(args) != 2 {
			return errors.New("use --mcp-lifecycle restore|uninstall")
		}
		if args[1] != "restore" && args[1] != "uninstall" {
			return errors.New("invalid lifecycle action")
		}
		return tunnel.Lifecycle(root, args[1])
	case "--mcp-startup":
		if len(args) != 2 || (args[1] != "enable" && args[1] != "disable") {
			return errors.New("use --mcp-startup enable|disable")
		}
		return tunnel.Startup(root, args[1] == "enable")
	}
	return errors.New("unknown MCP command")
}
