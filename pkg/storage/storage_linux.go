package storage

import (
	"fmt"
	"syscall"
)

// getDiskFreeSpace gets available disk space on Linux environment.
func getDiskFreeSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("failed to get disk space: %v", err)
	}

	// Calculate free space (in bytes)
	freeSpace := int64(stat.Bavail) * int64(stat.Bsize)
	return freeSpace, nil
}