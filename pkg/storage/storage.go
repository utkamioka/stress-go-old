package storage

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// GenerateLoad は指定されたストレージサイズで負荷を生成します。
//
// 引数:
//
//	ctx  - 負荷生成の制御に使用するコンテキスト
//	size - 書き込むデータサイズ（バイト）。負の値の場合は空きディスク容量のパーセンテージとして解釈
func GenerateLoad(ctx context.Context, size int64) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "stress-tool-storage-*")
	if err != nil {
		fmt.Printf("[Storage] Error: Failed to create temporary directory: %v\n", err)
		return
	}
	defer func() {
		os.RemoveAll(tempDir)
		fmt.Printf("[Storage] Cleaned up temporary files\n")
	}()

	fmt.Printf("[Storage] Temporary directory: %s\n", tempDir)

	if size < 0 {
		// Percentage specification - use dynamic adjustment
		percent := float64(-size)
		fmt.Printf("[Storage] Starting dynamic load generation with %.1f%% of free disk space\n", percent)
		if err := performDynamicStorageOperations(ctx, tempDir, percent); err != nil {
			fmt.Printf("[Storage] Error: %v\n", err)
		}
	} else {
		// Absolute value specification - use static allocation
		fmt.Printf("[Storage] Starting load generation with %d MB\n", size/(1024*1024))
		if err := performStorageOperations(ctx, tempDir, size); err != nil {
			fmt.Printf("[Storage] Error: %v\n", err)
		}
	}

	fmt.Printf("[Storage] Storage load generation completed\n")
}

// performStorageOperations はストレージの読み書き操作を実行します。
func performStorageOperations(ctx context.Context, tempDir string, totalSize int64) error {
	const chunkSize = 1024 * 1024 // 1MB chunks
	const numFiles = 10           // 複数ファイルに分散

	fileSize := totalSize / numFiles
	if fileSize < chunkSize {
		fileSize = chunkSize
	}

	// 複数ファイルを作成して書き込み
	filePaths := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		filePaths[i] = filepath.Join(tempDir, fmt.Sprintf("stress-file-%d.dat", i))
	}

	// 書き込みフェーズ
	fmt.Printf("[Storage] Writing data to %d files...\n", numFiles)
	for i, filePath := range filePaths {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err := writeFile(filePath, fileSize); err != nil {
			return fmt.Errorf("file write error: %v", err)
		}
		fmt.Printf("[Storage] File write %d/%d completed\n", i+1, numFiles)
	}

	// Continuous read/write operations
	fmt.Printf("[Storage] Starting continuous read/write operations\n")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	operationCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// ランダムにファイルを選択して読み書き
			fileIndex := operationCount % numFiles
			filePath := filePaths[fileIndex]

			// Read operation
			if err := readFile(filePath); err != nil {
				fmt.Printf("[Storage] Read error: %v\n", err)
			}

			// Update partial data (append write)
			if err := appendToFile(filePath, chunkSize/4); err != nil {
				fmt.Printf("[Storage] Append error: %v\n", err)
			}

			operationCount++
			fmt.Printf("[Storage] I/O operation %d completed\n", operationCount)
		}
	}
}

// performDynamicStorageOperations executes storage operations with dynamic size adjustment
func performDynamicStorageOperations(ctx context.Context, tempDir string, percent float64) error {
	var currentFiles []string
	var totalWritten int64
	fileCounter := 0
	
	// Check and adjust every 3 seconds
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Initial calculation and file creation
	targetSize, err := calculatePercentageSize(percent)
	if err != nil {
		return err
	}

	if targetSize > 0 {
		filePath := filepath.Join(tempDir, fmt.Sprintf("dynamic-stress-file-%d.dat", fileCounter))
		if err := writeFile(filePath, targetSize); err != nil {
			return fmt.Errorf("initial file write error: %v", err)
		}
		currentFiles = append(currentFiles, filePath)
		totalWritten = targetSize
		fileCounter++
		fmt.Printf("[Storage] Initial allocation: %d MB\n", targetSize/(1024*1024))
	}

	for {
		select {
		case <-ctx.Done():
			return nil
			
		case <-ticker.C:
			// Recalculate target size based on current free space
			newTargetSize, err := calculatePercentageSize(percent)
			if err != nil {
				fmt.Printf("[Storage] Error recalculating size: %v\n", err)
				continue
			}
			
			// Adjust disk usage if needed
			if newTargetSize > totalWritten {
				// Need to write more data
				additionalSize := newTargetSize - totalWritten
				if additionalSize > 0 {
					filePath := filepath.Join(tempDir, fmt.Sprintf("dynamic-stress-file-%d.dat", fileCounter))
					if err := writeFile(filePath, additionalSize); err != nil {
						fmt.Printf("[Storage] Error writing additional file: %v\n", err)
						continue
					}
					currentFiles = append(currentFiles, filePath)
					totalWritten += additionalSize
					fileCounter++
					fmt.Printf("[Storage] Increased disk usage by %d MB (total: %d MB)\n", 
						additionalSize/(1024*1024), totalWritten/(1024*1024))
				}
			} else if newTargetSize < totalWritten && len(currentFiles) > 1 {
				// Need to delete some files
				excessSize := totalWritten - newTargetSize
				deletedSize := int64(0)
				
				// Delete files from the end
				for i := len(currentFiles) - 1; i > 0 && deletedSize < excessSize; i-- {
					filePath := currentFiles[i]
					if info, err := os.Stat(filePath); err == nil {
						fileSize := info.Size()
						if err := os.Remove(filePath); err == nil {
							deletedSize += fileSize
							totalWritten -= fileSize
							currentFiles = currentFiles[:i]
						}
					}
				}
				
				if deletedSize > 0 {
					fmt.Printf("[Storage] Decreased disk usage by %d MB (total: %d MB)\n", 
						deletedSize/(1024*1024), totalWritten/(1024*1024))
				}
			}
			
			// Perform I/O operations on remaining files
			if len(currentFiles) > 0 {
				fileIndex := int(time.Now().Unix()) % len(currentFiles)
				filePath := currentFiles[fileIndex]
				
				// Read operation
				if err := readFile(filePath); err != nil {
					fmt.Printf("[Storage] Read error: %v\n", err)
				}
				
				// Light append operation to maintain activity
				if err := appendToFile(filePath, 1024); err != nil {
					fmt.Printf("[Storage] Append error: %v\n", err)
				}
				
				fmt.Printf("[Storage] Dynamic I/O operation completed (%d files active)\n", len(currentFiles))
			}
		}
	}
}

// writeFile は指定されたサイズのランダムデータを書き込みます。
func writeFile(filePath string, size int64) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	const bufferSize = 64 * 1024 // 64KB buffer
	buffer := make([]byte, bufferSize)

	written := int64(0)
	for written < size {
		// ランダムデータを生成
		if _, err := rand.Read(buffer); err != nil {
			return err
		}

		writeSize := bufferSize
		if written+int64(bufferSize) > size {
			writeSize = int(size - written)
		}

		n, err := file.Write(buffer[:writeSize])
		if err != nil {
			return err
		}

		written += int64(n)
	}

	return file.Sync() // ディスクに強制書き込み
}

// readFile はファイルを読み取ります。
func readFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, 64*1024)
	
	// ファイル全体を読み取り
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if n == 0 {
			break
		}
	}

	return nil
}

// appendToFile はファイルにデータを追記します。
func appendToFile(filePath string, size int) error {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return err
	}

	_, err = file.Write(buffer)
	return err
}

// calculatePercentageSize は空きディスク容量のパーセンテージから実際のサイズを計算します。
func calculatePercentageSize(percent float64) (int64, error) {
	// Get free space of current working directory
	wd, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("failed to get working directory: %v", err)
	}

	freeSpace, err := getDiskFreeSpace(wd)
	if err != nil {
		return 0, err
	}

	targetSize := int64(float64(freeSpace) * percent / 100.0)

	// Use 90% of calculated size for safety
	targetSize = int64(float64(targetSize) * 0.90)

	if targetSize <= 0 {
		return 0, fmt.Errorf("calculated storage size is invalid")
	}

	return targetSize, nil
}