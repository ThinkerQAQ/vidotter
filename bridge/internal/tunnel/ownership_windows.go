//go:build windows

package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import "os"

func preserveExistingOwner(temp string, existing os.FileInfo) error { return nil }
func assignServiceOwner(file, account string) error                 { return nil }
func verifyServiceDataOwner(root, account string) error             { return nil }
