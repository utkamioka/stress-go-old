package memory

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
)

// GenerateLoad は指定されたメモリサイズで負荷を生成します。
//
// 引数:
//
//	ctx  - 負荷生成の制御に使用するコンテキスト
//	size - 確保するメモリサイズ（バイト）。負の値の場合は空きメモリのパーセンテージとして解釈
func GenerateLoad(ctx context.Context, size int64) {
	if size < 0 {
		// Percentage specification - use dynamic adjustment
		percent := float64(-size)
		fmt.Printf("[Memory] Starting dynamic load generation with %.1f%% of free memory\n", percent)
		generateDynamicLoad(ctx, percent)
	} else {
		// Absolute value specification - use static allocation
		fmt.Printf("[Memory] Starting load generation with %d MB\n", size/(1024*1024))
		generateStaticLoad(ctx, size)
	}
}

// generateStaticLoad generates a fixed amount of memory load
func generateStaticLoad(ctx context.Context, size int64) {
	// Disable GC to ensure memory retention
	oldGCPercent := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGCPercent)

	// Allocate memory
	buffer := make([]byte, size)
	
	// Initialize memory content (to ensure actual memory usage)
	fmt.Printf("[Memory] Initializing memory...\n")
	for i := int64(0); i < size; i += 4096 { // Initialize in 4KB chunks
		if i+4096 > size {
			buffer[i] = byte(i % 256)
		} else {
			buffer[i] = byte(i % 256)
		}
	}

	fmt.Printf("[Memory] Allocated %d MB of memory\n", size/(1024*1024))
	
	// Periodically display memory usage
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[Memory] Stopping memory load generation\n")
			// Release buffer reference
			buffer = nil
			runtime.GC()
			return
		case <-ticker.C:
			showMemoryStats(size)
			// Lightly use buffer to prevent deallocation
			if len(buffer) > 0 {
				buffer[0] = byte(time.Now().Unix() % 256)
			}
		}
	}
}

// generateDynamicLoad generates memory load with dynamic adjustment based on percentage
func generateDynamicLoad(ctx context.Context, percent float64) {
	// Disable GC to ensure memory retention
	oldGCPercent := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGCPercent)

	var buffers [][]byte
	var totalAllocated int64
	
	// Check and adjust every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Initial allocation
	targetSize, err := calculatePercentageSize(percent)
	if err != nil {
		fmt.Printf("[Memory] Error: %v\n", err)
		return
	}
	
	if targetSize > 0 {
		buffer := make([]byte, targetSize)
		initializeBuffer(buffer)
		buffers = append(buffers, buffer)
		totalAllocated = targetSize
		fmt.Printf("[Memory] Initial allocation: %d MB\n", targetSize/(1024*1024))
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[Memory] Stopping dynamic memory load generation\n")
			// Release all buffers
			for i := range buffers {
				buffers[i] = nil
			}
			buffers = nil
			runtime.GC()
			return
			
		case <-ticker.C:
			// Recalculate target size based on current free memory
			newTargetSize, err := calculatePercentageSize(percent)
			if err != nil {
				fmt.Printf("[Memory] Error recalculating size: %v\n", err)
				continue
			}
			
			// Adjust allocation if needed
			if newTargetSize > totalAllocated {
				// Need to allocate more
				additionalSize := newTargetSize - totalAllocated
				if additionalSize > 0 {
					buffer := make([]byte, additionalSize)
					initializeBuffer(buffer)
					buffers = append(buffers, buffer)
					totalAllocated += additionalSize
					fmt.Printf("[Memory] Increased allocation by %d MB (total: %d MB)\n", 
						additionalSize/(1024*1024), totalAllocated/(1024*1024))
				}
			} else if newTargetSize < totalAllocated && len(buffers) > 1 {
				// Need to release some memory
				excessSize := totalAllocated - newTargetSize
				releasedSize := int64(0)
				
				// Release buffers from the end
				for i := len(buffers) - 1; i > 0 && releasedSize < excessSize; i-- {
					bufferSize := int64(len(buffers[i]))
					buffers[i] = nil
					buffers = buffers[:i]
					releasedSize += bufferSize
					totalAllocated -= bufferSize
				}
				
				if releasedSize > 0 {
					runtime.GC() // Force garbage collection
					fmt.Printf("[Memory] Decreased allocation by %d MB (total: %d MB)\n", 
						releasedSize/(1024*1024), totalAllocated/(1024*1024))
				}
			}
			
			showMemoryStats(totalAllocated)
			
			// Keep buffers active
			for _, buffer := range buffers {
				if len(buffer) > 0 {
					buffer[0] = byte(time.Now().Unix() % 256)
				}
			}
		}
	}
}

// initializeBuffer initializes buffer to ensure actual memory usage
func initializeBuffer(buffer []byte) {
	size := int64(len(buffer))
	for i := int64(0); i < size; i += 4096 { // Initialize in 4KB chunks
		if i < size {
			buffer[i] = byte(i % 256)
		}
	}
}

// calculatePercentageSize は空きメモリのパーセンテージから実際のサイズを計算します。
func calculatePercentageSize(percent float64) (int64, error) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// システムの総メモリを取得（簡易実装）
	// 実際のシステムメモリ情報はOSによって異なるため、
	// ここでは現在のヒープサイズを基準とした推定値を使用
	totalSystemMemory := int64(8 * 1024 * 1024 * 1024) // 8GB as default
	
	// より正確には /proc/meminfo (Linux) や Windows API を使用すべき
	usedMemory := int64(memStats.Sys)
	freeMemory := totalSystemMemory - usedMemory

	if freeMemory <= 0 {
		return 0, fmt.Errorf("insufficient free memory")
	}

	targetSize := int64(float64(freeMemory) * percent / 100.0)
	
	// Use 95% of calculated size for safety
	targetSize = int64(float64(targetSize) * 0.95)

	if targetSize <= 0 {
		return 0, fmt.Errorf("calculated memory size is invalid")
	}

	return targetSize, nil
}

// showMemoryStats displays memory usage statistics.
func showMemoryStats(allocatedSize int64) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	fmt.Printf("[Memory] Allocated: %d MB, System usage: %d MB, Heap size: %d MB\n",
		allocatedSize/(1024*1024),
		memStats.Sys/(1024*1024),
		memStats.HeapSys/(1024*1024))
}