package force_bind

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func PIDFromTID(tid int) (int, error) {
	if tid <= 0 {
		return 0, fmt.Errorf("invalid TID %d", tid)
	}

	f, err := os.Open(fmt.Sprintf("/proc/%d/status", tid))
	if err != nil {
		return 0, fmt.Errorf("open thread status: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), ":")
		if ok && key == "Tgid" {
			pid, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return 0, fmt.Errorf("parse Tgid: %w", err)
			}
			return pid, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read thread status: %w", err)
	}
	return 0, fmt.Errorf("Tgid not found for TID %d", tid)
}
