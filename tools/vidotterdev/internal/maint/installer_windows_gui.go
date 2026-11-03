package maint

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
)

const windowsGUISubsystem = 2

func windowsPEInfo(data []byte) (uint16, pe.DataDirectory, error) {
	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return 0, pe.DataDirectory{}, fmt.Errorf("invalid Windows executable: %w", err)
	}
	defer f.Close()
	switch h := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		return h.Subsystem, h.DataDirectory[4], nil
	case *pe.OptionalHeader64:
		return h.Subsystem, h.DataDirectory[4], nil
	default:
		return 0, pe.DataDirectory{}, fmt.Errorf("Windows executable has no supported optional header")
	}
}

func requireWindowsGUI(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	subsystem, _, err := windowsPEInfo(data)
	if err != nil {
		return err
	}
	if subsystem != windowsGUISubsystem {
		return fmt.Errorf("Windows installer must use GUI subsystem (2), got %d", subsystem)
	}
	return nil
}

// Patch a private, unsigned SDK template copy before IFW appends its archives.
// Passing this to binarycreator -t also embeds the GUI maintenance executable.
func prepareWindowsGUITemplate(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read Qt installerbase: %w", err)
	}
	subsystem, certificate, err := windowsPEInfo(data)
	if err != nil {
		return err
	}
	if certificate.VirtualAddress != 0 || certificate.Size != 0 {
		return fmt.Errorf("refusing to modify a signed Qt installerbase")
	}
	if subsystem != 2 && subsystem != 3 {
		return fmt.Errorf("unsupported Qt installerbase subsystem %d", subsystem)
	}
	if len(data) < 64 || string(data[:2]) != "MZ" {
		return fmt.Errorf("invalid DOS header")
	}
	optional := uint64(binary.LittleEndian.Uint32(data[0x3c:])) + 24
	if optional+70 > uint64(len(data)) {
		return fmt.Errorf("truncated PE optional header")
	}
	binary.LittleEndian.PutUint16(data[optional+68:], windowsGUISubsystem)
	binary.LittleEndian.PutUint32(data[optional+64:], 0) // IFW appends data; checksum is unset until signing.
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
	if err != nil {
		return err
	} // Never overwrite the SDK, an existing file, or a linked template.
	_, writeErr := out.Write(data)
	closeErr := out.Close()
	if writeErr != nil {
		os.Remove(target)
		return writeErr
	}
	if closeErr != nil {
		os.Remove(target)
		return closeErr
	}
	return requireWindowsGUI(target)
}
