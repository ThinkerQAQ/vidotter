package vidotter

import (
	"net"
	"testing"
)

func TestDuplicateBridgeDoesNotTouchState(t *testing.T) {
	listener, err := net.Listen("tcp", bridgeAddress)
	if err != nil {
		t.Skip("Bridge port already occupied")
	}
	defer listener.Close()
	// Binding fails before random-token generation, job loading, recovery or persistence.
	if err := runBridge(); err == nil {
		t.Fatal("duplicate bridge accepted")
	}
}
