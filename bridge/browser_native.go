package vidotter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// The browser owns this process through chrome.runtime.connectNative.
// Remote commands flow exclusively through the already-authenticated loopback
// Bridge; the native host never exports browser or download credentials.
func runNativeBrowserCommands(state bridgeState) error {
	client := &http.Client{Timeout: 16 * time.Second}
	for {
		request, err := http.NewRequest(http.MethodGet, state.BaseURL+"/v1/browser/commands/next", nil)
		if err != nil {
			return err
		}
		request.Header.Set("X-VidOtter-Token", state.Token)
		response, err := client.Do(request)
		if err != nil {
			return errors.New("browser command connection to Bridge lost")
		}
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("Bridge command poll HTTP %d", response.StatusCode)
		}
		if readErr != nil {
			return errors.New("failed reading browser command")
		}
		var envelope struct {
			OK      bool
			Command *browserDownloadCommand
		}
		if json.Unmarshal(raw, &envelope) != nil || !envelope.OK {
			return errors.New("invalid Bridge command reply")
		}
		if envelope.Command == nil || (envelope.Command.Type != "browser_media_download" && envelope.Command.Type != "browser_media_refresh" && envelope.Command.Type != "browser_wait_notice") {
			continue
		}
		if err := writeNativeMessage(os.Stdout, envelope.Command); err != nil {
			return err
		}
	}
}
