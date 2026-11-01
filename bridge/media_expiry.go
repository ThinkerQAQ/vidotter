package vidotter

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

const mediaExpirySafety = 30 * time.Second

// mediaURLExpiry uses only explicitly named expiry fields. An unknown format
// never implies an expired (or valid) URL. No host-specific rules are used.
func mediaURLExpiry(raw string) (time.Time, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return time.Time{}, false
	}
	var earliest time.Time
	for key, values := range parsed.Query() {
		switch strings.ToLower(key) {
		case "expires", "expire", "expiry", "expiration", "exp":
		default:
			continue
		}
		for _, value := range values {
			if instant, ok := parseMediaExpiry(value); ok &&
				(earliest.IsZero() || instant.Before(earliest)) {
				earliest = instant
			}
		}
	}
	return earliest, !earliest.IsZero()
}

func parseMediaExpiry(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		if n >= 1_000_000_000_000 && n < 10_000_000_000_000 {
			n /= 1000
		}
		instant := time.Unix(n, 0)
		if instant.Year() >= 2000 && instant.Year() <= 2100 {
			return instant, true
		}
		return time.Time{}, false
	}
	if instant, err := time.Parse(time.RFC3339, value); err == nil &&
		instant.Year() >= 2000 && instant.Year() <= 2100 {
		return instant, true
	}
	return time.Time{}, false
}

func mediaURLExpiring(raw string, now time.Time) bool {
	expiry, known := mediaURLExpiry(raw)
	return known && !expiry.After(now.Add(mediaExpirySafety))
}
