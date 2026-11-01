package vidotter

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
)

// No URL or response body may be emitted into job state or persistent errors.
// This is a display boundary, not a permission to send signed URLs elsewhere.
var diagnosticURL = regexp.MustCompile("(?i)https?://[^\\s\\\"'<>]+")

func safeJobError(message string) string {
	return diagnosticURL.ReplaceAllStringFunc(message, func(raw string) string {
		return logURLSummary(raw)
	})
}

type mediaAccessError struct {
	reason string // one of the fixed job-state reason codes
	status int
}

func (e *mediaAccessError) Error() string {
	if e.reason == "media_expired" {
		return "媒体链接已过期，请在浏览器重新播放以捕获新的媒体"
	}
	if e.status == 403 {
		return "HTTP 403：媒体服务器拒绝访问，请在浏览器检查播放权限并重新播放"
	}
	return fmt.Sprintf("HTTP %d：媒体服务器拒绝访问", e.status)
}

func mediaFailureReason(err error) string {
	var access *mediaAccessError
	if errors.As(err, &access) {
		return access.reason
	}
	return ""
}

// A signed or browser-origin media URL is not a durable task identity.
// Direct public URLs without potentially sensitive query fields retain the
// existing restart behavior. The full URL exists only in the live worker.
func nonDurableMediaURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err != nil || parsed.RawQuery != "" || parsed.User != nil
}
