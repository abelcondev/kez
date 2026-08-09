package imageinput

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/abelcondev/kez/internal/zeroruntime"
)

// ReadClipboardImage returns the raw image bytes and media type from the OS
// clipboard, or (nil, "", nil) when the clipboard has no image. Called when
// text clipboard is empty (the user pasted a screenshot). The media type is
// sniffed from the bytes, not trusted from the clipboard.
func ReadClipboardImage() ([]byte, string, error) {
	data, err := readClipboardImageBytes()
	if err != nil {
		return nil, "", err
	}
	if data == nil {
		return nil, "", nil
	}
	// Sniff the media type from the bytes — don't trust the clipboard's claim.
	sniffLen := len(data)
	if sniffLen > 512 {
		sniffLen = 512
	}
	mediaType := zeroruntime.NormalizeImageMediaType(http.DetectContentType(data[:sniffLen]))
	if mediaType == "" {
		return nil, "", fmt.Errorf("clipboard image is not a supported type (allowed: png, jpeg, gif, webp)")
	}
	return data, mediaType, nil
}

// readClipboardImageBytes calls the platform-specific clipboard tool to extract
// image bytes. Returns (nil, nil) when no image is present.
func readClipboardImageBytes() ([]byte, error) {
	switch runtime.GOOS {
	case "windows":
		return readClipboardImageWindows()
	case "darwin":
		return readClipboardImageDarwin()
	case "linux":
		return readClipboardImageLinux()
	default:
		return nil, nil
	}
}

// readClipboardImageWindows uses PowerShell to check for and read a clipboard
// image. The image is saved as PNG to a temp file, read back, and the temp file
// deleted. Returns (nil, nil) when no image is on the clipboard.
func readClipboardImageWindows() ([]byte, error) {
	// Check if the clipboard contains an image.
	check := `Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing; [System.Windows.Forms.Clipboard]::ContainsImage()`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", check).Output()
	if err != nil {
		return nil, nil // clipboard not available, treat as no image
	}
	if strings.TrimSpace(string(out)) != "True" {
		return nil, nil
	}
	// Save the clipboard image as PNG to a temp file, then read the bytes.
	// PowerShell stdout can't reliably emit raw binary — $ms.ToArray() prints
	// a .NET byte array as space-separated text, not raw bytes. A temp file
	// is the correct binary-safe path.
	tmpFile, err := os.CreateTemp("", "zero-clipboard-*.png")
	if err != nil {
		return nil, nil
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Stay on -Command: -File is subject to script execution policy (default
	// Restricted on Windows clients) and Windows PowerShell decodes BOM-less
	// UTF-8 .ps1 files as ANSI. Doubling single quotes is all the escaping the
	// single-quoted PowerShell literal needs.
	escapedTmpPath := strings.ReplaceAll(tmpPath, "'", "''")
	script := `Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing; $img = [System.Windows.Forms.Clipboard]::GetImage(); if ($img -ne $null) { $img.Save('` + escapedTmpPath + `', [System.Drawing.Imaging.ImageFormat]::Png) }`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	if err := cmd.Run(); err != nil {
		return nil, nil
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil || len(data) == 0 {
		return nil, nil
	}
	return data, nil
}

// readClipboardImageDarwin reads a clipboard image on macOS. Returns (nil, nil)
// when no image is present.
func readClipboardImageDarwin() ([]byte, error) {
	// Check clipboard info for image classes first so we skip the extract work
	// (and any spurious errors) when the clipboard holds only text.
	out, err := exec.Command("osascript", "-e", "clipboard info").Output()
	if err != nil {
		return nil, nil
	}
	info := string(out)
	if !strings.Contains(info, "PNG") && !strings.Contains(info, "JPEG") && !strings.Contains(info, "TIFF") && !strings.Contains(info, "GIF") {
		return nil, nil
	}
	// Fast path: pngpaste emits the clipboard image as PNG on stdout when
	// installed. It usually isn't, so fall through to the dependency-free
	// osascript path below rather than requiring a Homebrew install.
	if data := runPngpaste(); len(data) > 0 {
		return data, nil
	}
	// Dependency-free path: osascript is always present on macOS. Coerce the
	// clipboard to PNG and write the raw bytes to a temp file, then read them
	// back — AppleScript's `write` is the only binary-safe way out of osascript
	// (stdout mangles binary). This works with no pngpaste / pyobjc install,
	// which is the common case (both are absent on a stock macOS).
	if data := runOsascriptClipboardPNG(); len(data) > 0 {
		return data, nil
	}
	return nil, nil
}

// runPngpaste returns the clipboard image via pngpaste, or nil when pngpaste is
// missing or the clipboard has no image.
func runPngpaste() []byte {
	cmd := exec.Command("pngpaste", "-")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil
	}
	return stdout.Bytes()
}

// runOsascriptClipboardPNG coerces the clipboard to PNG via AppleScript and
// writes it to a temp file (binary-safe), then reads the bytes back. Returns nil
// when the clipboard cannot be coerced to PNG (e.g. a TIFF-only image or no
// image). The temp path is generated by the OS, so it is never attacker-shaped.
func runOsascriptClipboardPNG() []byte {
	tmpFile, err := os.CreateTemp("", "zero-clipboard-*.png")
	if err != nil {
		return nil
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// «class PNGf» is the PNG pasteboard type. AppleScript raises if the
	// clipboard holds no PNG-coercible image; that surfaces as a command error,
	// treated as "no image" by the caller. The POSIX path is passed as an
	// AppleScript string literal, and CreateTemp paths never contain quotes.
	script := `set png to (the clipboard as «class PNGf»)
set fp to open for access POSIX file "` + tmpPath + `" with write permission
try
	write png to fp
	close access fp
on error errMsg
	close access fp
	error errMsg
end try`
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return nil
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil
	}
	return data
}

// readClipboardImageLinux tries wl-paste (Wayland) then xclip (X11) to read
// clipboard image bytes. Returns (nil, nil) when no image or no tool available.
//
// The MIME type comes from the clipboard (wl-paste --list-types / xclip
// TARGETS), so it is NEVER interpolated into a shell — every command runs via
// exec.Command(prog, args...) with the type passed as a discrete argument.
// (A hostile clipboard offerer could otherwise register a target like
// "image/png; rm -rf ~" that passes the "image/" prefix check.)
func readClipboardImageLinux() ([]byte, error) {
	// Try Wayland first.
	if types, err := runClipboardStdout("wl-paste", "--list-types"); err == nil {
		for _, t := range imageMIMETypes(types) {
			if data, err := runClipboardStdout("wl-paste", "--type", t); err == nil && len(data) > 0 {
				return data, nil
			}
		}
	}
	// Fall back to X11 xclip.
	if types, err := runClipboardStdout("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o"); err == nil {
		for _, t := range imageMIMETypes(types) {
			if data, err := runClipboardStdout("xclip", "-selection", "clipboard", "-t", t, "-o"); err == nil && len(data) > 0 {
				return data, nil
			}
		}
	}
	return nil, nil
}

// runClipboardStdout runs a clipboard helper and returns only its stdout.
// Stderr is discarded (the helpers are noisy when the clipboard is empty or the
// tool is missing); a missing tool surfaces as the command error, treated as
// "no image" by the callers.
func runClipboardStdout(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// imageMIMETypes extracts the "image/*" lines from a newline-separated type
// list, each safe to pass as a discrete argument (no shell).
func imageMIMETypes(list []byte) []string {
	var out []string
	for _, line := range strings.Split(string(list), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "image/") {
			out = append(out, line)
		}
	}
	return out
}
