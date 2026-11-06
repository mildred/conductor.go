package pidfile

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"
)

var (
	ErrPIDExists  = errors.New("PID file already exists with an active process")
	ErrLockFailed = errors.New("failed to acquire a lock on the PID file")
	ErrPIDStolen  = errors.New("PID file has been stolen by another process")
)

type PIDFile struct {
	filename string
	file     *os.File
	closed   bool
}

func (pidfile *PIDFile) Close() error {
	if pidfile.closed {
		return nil
	}

	defer pidfile.file.Close()

	pid, err := readPIDValue(pidfile.filename)
	if err != nil {
		return err
	}

	if pid != os.Getpid() {
		return ErrPIDStolen
	}

	err = os.Remove(pidfile.filename)
	if err != nil {
		return err
	}

	pidfile.closed = true

	return pidfile.file.Close()
}

func CreateUpdateOrRetryPIDFile(ctx context.Context, filename string, timeout time.Duration) (*PIDFile, error) {
	retrying := false
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		pidfile, err := CreateOrUpdatePIDFile(filename)
		if err == nil || err != ErrPIDExists {
			if err == nil && retrying {
				log.Printf("Acquired lock file %s", filename)
			}
			return pidfile, err
		}

		if !retrying {
			log.Printf("Failed to acquire lock file %s, waiting...", filename)
		}
		retrying = true

		func() {
			wait, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			<-wait.Done()
		}()
	}
}

// CreateOrUpdatePIDFile ensures that a PID file exists and contains the current process's PID.
// It attempts to create the PID file if it does not exist, and update it if the process is not active.
func CreateOrUpdatePIDFile(filename string) (*PIDFile, error) {
	pid, err := readPIDValue(filename)
	if err == nil {
		active, err := isProcessActive(pid)
		if err != nil {
			return nil, fmt.Errorf("error checking process activity: %w", err)
		}
		if active {
			return nil, ErrPIDExists
		}
	}
	return createPIDFile(filename)
}

// createPIDFile creates or updates the PID file with the current process's PID.
func createPIDFile(filename string) (*PIDFile, error) {
	err := os.MkdirAll(path.Dir(filename), 0755)
	if err != nil {
		return nil, fmt.Errorf("error creating dirs for %s: %w", filename, err)
	}

	pf, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening PID file %s: %w", filename, err)
	}

	if err := syscall.Flock(int(pf.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if err == syscall.EWOULDBLOCK {
			pf.Close()
			return nil, ErrLockFailed
		}
		pf.Close()
		return nil, fmt.Errorf("error locking PID file %s: %w", filename, err)
	}

	pid := os.Getpid()
	if _, err := pf.Write([]byte(strconv.Itoa(pid))); err != nil {
		pf.Close()
		return nil, fmt.Errorf("error writing pid to PID file %s: %w", filename, err)
	}

	return &PIDFile{filename, pf, false}, nil
}

// isProcessActive checks whether the process with the provided PID is running.
func isProcessActive(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil // invalid process ID
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		// On Unix systems, os.FindProcess always succeeds and returns a process with the given pid, irrespective of whether the process exists.
		return false, nil
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil {
		if err == syscall.ESRCH {
			// The process does not exist
			return false, nil
		}
		// Some other unexpected error
		return false, fmt.Errorf("error signaling process: %w", err)
	}

	// The process exists and is active
	return true, nil
}

// readPIDValue reads the PID value from the specified PID file.
func readPIDValue(filename string) (int, error) {
	value, err := os.ReadFile(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return 0, fmt.Errorf("error reading PID file: %w", err)
		}
		return 0, nil // PID file does not exist
	}
	pid, err := strconv.Atoi(string(value))
	if err != nil {
		return 0, fmt.Errorf("error parsing PID value: %w", err)
	}
	return pid, nil
}
