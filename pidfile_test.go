package pidfile_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/makifdb/pidfile"
)

// tempPIDFile creates a temporary PID file and returns its path using os.TempDir.
func tempPIDFile(t *testing.T) string {
	t.Helper() // Marks the calling function as a test helper function.

	// Create the temporary PID file path
	pidFilePath := filepath.Join(os.TempDir(), "pidfile_test.pid")

	// Create the temporary file
	file, err := os.Create(pidFilePath)
	if err != nil {
		t.Fatalf("Unable to create temporary PID file: %v", err)
	}
	file.Close() // Close the file immediately as we only wanted to create it

	// Schedule the cleanup of the PID file at the end of the test
	t.Cleanup(func() {
		os.Remove(pidFilePath) // Ignore error as file may already be removed
	})

	return pidFilePath
}

// TestCreateOrUpdatePIDFile tests creating a PID file when one does not already exist.
func TestCreateOrUpdatePIDFile(t *testing.T) {
	pidFilePath := tempPIDFile(t)

	if err := pidfile.CreateOrUpdatePIDFile(pidFilePath); err != nil {
		t.Errorf("Failed to create PID file: %v", err)
	}

	// Ensure the PID file was created or updated
	_, err := os.Stat(pidFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("PID file '%s' does not exist.", pidFilePath)
		} else {
			t.Errorf("Error checking PID file: %v", err)
		}
	}

	// Verify content
	content, err := os.ReadFile(pidFilePath)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	pid := os.Getpid()
	if string(content) != strconv.Itoa(pid) {
		t.Errorf("PID file content '%s' does not match current process ID '%d'.", content, pid)
	}
}

// TestCreateOrUpdatePIDFileExistingActiveProcess tests the behavior when a PID file with an active process already exists.
func TestCreateOrUpdatePIDFileExistingActiveProcess(t *testing.T) {
	pidFilePath := tempPIDFile(t)
	// Create a temporary PID file with the current process's PID for testing purposes
	currentPID := os.Getpid()
	if err := os.WriteFile(pidFilePath, []byte(strconv.Itoa(currentPID)), 0644); err != nil {
		t.Fatalf("Failed to write current PID to temp file: %v", err)
	}

	// Call CreateOrUpdatePIDFile and expect an error indicating the PID already exists
	err := pidfile.CreateOrUpdatePIDFile(pidFilePath)
	if !errors.Is(err, pidfile.ErrPIDExists) {
		t.Errorf("Expected ErrPIDExists error, but got: %v", err)
	}
}
