package maint

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func fakeWindowsPE(gui bool, signed bool) []byte {
	data := make([]byte, 1024)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[0x3c:], 128)
	copy(data[128:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(data[132:], 0x8664)
	binary.LittleEndian.PutUint16(data[148:], 240)
	opt := 152
	binary.LittleEndian.PutUint16(data[opt:], 0x20b)
	binary.LittleEndian.PutUint32(data[opt+108:], 16)
	subsystem := uint16(3)
	if gui {
		subsystem = 2
	}
	binary.LittleEndian.PutUint16(data[opt+68:], subsystem)
	if signed {
		binary.LittleEndian.PutUint32(data[opt+112+32:], 512)
	}
	return data
}

func TestWindowsGUITemplatePreservesSDKAndRejectsConsoleOutputs(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "installerbase.exe")
	target := filepath.Join(root, "gui.exe")
	original := fakeWindowsPE(false, false)
	if err := os.WriteFile(source, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := requireWindowsGUI(source); err == nil {
		t.Fatal("console installer accepted")
	}
	if err := prepareWindowsGUITemplate(source, target); err != nil {
		t.Fatal(err)
	}
	if err := requireWindowsGUI(target); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(source)
	if !bytes.Equal(got, original) {
		t.Fatal("SDK template was modified")
	}
	if err := prepareWindowsGUITemplate(source, target); err == nil {
		t.Fatal("existing template overwritten")
	}
	if err := prepareWindowsGUITemplate(source, source); err == nil {
		t.Fatal("SDK overwrite accepted")
	}
}

func TestWindowsGUITemplateRejectsSignedOrInvalidPE(t *testing.T) {
	for _, data := range [][]byte{fakeWindowsPE(false, true), []byte("not PE"), make([]byte, 64)} {
		root := t.TempDir()
		source := filepath.Join(root, "base.exe")
		target := filepath.Join(root, "gui.exe")
		if err := os.WriteFile(source, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := prepareWindowsGUITemplate(source, target); err == nil {
			t.Fatal("signed/invalid template accepted")
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatal("invalid template produced output")
		}
	}
}
