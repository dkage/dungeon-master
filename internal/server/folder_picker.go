package server

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	errPickerUnavailable = errors.New("folder picker is not available")
	errPickerCancelled   = errors.New("folder selection cancelled")
)

// pickFolder opens the platform's native directory chooser. Each executable
// receives fixed argv; no shell interprets the selected path or script.
func pickFolder(ctx context.Context) (string, error) {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "osascript"
		if _, err := exec.LookPath(name); err != nil {
			return "", errPickerUnavailable
		}
		args = []string{"-e", `POSIX path of (choose folder with prompt "Choose a project folder")`}
	case "linux":
		if _, err := exec.LookPath("zenity"); err == nil {
			name, args = "zenity", []string{"--file-selection", "--directory", "--title=Choose a project folder"}
		} else if _, err := exec.LookPath("kdialog"); err == nil {
			name, args = "kdialog", []string{"--getexistingdirectory", "", "Choose a project folder"}
		} else {
			return "", errPickerUnavailable
		}
	case "windows":
		name = "powershell.exe"
		if _, err := exec.LookPath(name); err != nil {
			return "", errPickerUnavailable
		}
		script := `Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.FolderBrowserDialog; $d.Description = 'Choose a project folder'; if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::WriteLine($d.SelectedPath) }`
		args = []string{"-NoProfile", "-STA", "-Command", script}
	default:
		return "", errPickerUnavailable
	}

	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", errPickerCancelled
		}
		return "", fmt.Errorf("run folder picker: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", errPickerCancelled
	}
	return filepath.Clean(path), nil
}

func openFolder(_ context.Context, path string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{path}
	case "linux":
		name, args = "xdg-open", []string{path}
	case "windows":
		name, args = "explorer.exe", []string{path}
	default:
		return errPickerUnavailable
	}
	if _, err := exec.LookPath(name); err != nil {
		return errPickerUnavailable
	}
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
