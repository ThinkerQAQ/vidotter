//go:build !windows

package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"os/user"
	"testing"
)

func TestServiceDataOwnerMatchesAccount(t *testing.T) {
	root := t.TempDir()
	if err := saveNative(root, "tunnel_SYNTHETIC", "synthetic-key"); err != nil {
		t.Fatal(err)
	}
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyServiceDataOwner(root, account.Username); err != nil {
		t.Fatalf("same account should own configuration: %v", err)
	}
	if err := verifyServiceDataOwner(root, "__nonexistent_service_account_for_test__"); err == nil {
		t.Fatal("unknown service account accepted")
	}
}
