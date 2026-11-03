package vidotter

import "testing"

func TestDefaultDownloadConcurrency(t *testing.T) {
    config := defaultBridgeConfig()
    if config.Concurrent != 32 {
        t.Fatalf("default download concurrency = %d; want 32", config.Concurrent)
    }
    if _, err := normalizeBridgeConfig(config); err != nil {
        t.Fatalf("default configuration must remain valid: %v", err)
    }
}
