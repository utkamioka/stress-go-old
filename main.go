package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"stress-go/pkg/cpu"
	"stress-go/pkg/memory"
	"stress-go/pkg/storage"
)

type Config struct {
	Timeout time.Duration
	CPU     int
	Memory  string
	Storage string
}

func main() {
	var config Config
	var timeoutStr string

	flag.StringVar(&timeoutStr, "timeout", "", "Duration to apply load (e.g., 30s, 5m, 1h)")
	flag.IntVar(&config.CPU, "cpu", -1, "Number of CPU cores to use (0 = use all cores)")
	flag.StringVar(&config.Memory, "memory", "", "Memory load (e.g., 1GB, 512MB, 95%)")
	flag.StringVar(&config.Storage, "storage", "", "Storage load (e.g., 500MB, 80%)")
	flag.Parse()

	if timeoutStr == "" {
		fmt.Fprintf(os.Stderr, "Error: --timeout option is required\n")
		printUsage()
		os.Exit(1)
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid time format: %v\n", err)
		os.Exit(1)
	}
	config.Timeout = timeout

	// Check if at least one load type is specified
	if config.CPU < 0 && config.Memory == "" && config.Storage == "" {
		fmt.Fprintf(os.Stderr, "Error: At least one load type must be specified\n")
		printUsage()
		os.Exit(1)
	}

	fmt.Printf("Starting stress test...\n")
	fmt.Printf("Duration: %v\n", config.Timeout)
	if config.CPU >= 0 {
		if config.CPU == 0 {
			fmt.Printf("CPU load: all cores\n")
		} else {
			fmt.Printf("CPU load: %d cores\n", config.CPU)
		}
	}
	if config.Memory != "" {
		fmt.Printf("Memory load: %s\n", config.Memory)
	}
	if config.Storage != "" {
		fmt.Printf("Storage load: %s\n", config.Storage)
	}
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// シグナルハンドリング
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Start CPU load
	if config.CPU >= 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cpu.GenerateLoad(ctx, config.CPU)
		}()
	}

	// Start memory load
	if config.Memory != "" {
		memorySize, err := parseSize(config.Memory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to parse memory size: %v\n", err)
			os.Exit(1)
		}
		
		wg.Add(1)
		go func() {
			defer wg.Done()
			memory.GenerateLoad(ctx, memorySize)
		}()
	}

	// Start storage load
	if config.Storage != "" {
		storageSize, err := parseSize(config.Storage)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to parse storage size: %v\n", err)
			os.Exit(1)
		}
		
		wg.Add(1)
		go func() {
			defer wg.Done()
			storage.GenerateLoad(ctx, storageSize)
		}()
	}

	// Show progress
	go showProgress(ctx, config.Timeout)

	select {
	case <-sigChan:
		fmt.Println("\nInterrupt signal received. Stopping stress test...")
		cancel()
	case <-ctx.Done():
	}

	wg.Wait()
	fmt.Println("Stress test completed.")
}

func parseSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(sizeStr)
	
	// Percentage specification
	if strings.HasSuffix(sizeStr, "%") {
		percentStr := strings.TrimSuffix(sizeStr, "%")
		percent, err := strconv.ParseFloat(percentStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid percentage: %s", percentStr)
		}
		if percent < 0 || percent > 100 {
			return 0, fmt.Errorf("percentage must be in range 0-100: %f", percent)
		}
		// Return negative value to distinguish percentage
		return int64(-percent), nil
	}

	// Absolute value specification
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([KMGT]?B?)?$`)
	matches := re.FindStringSubmatch(strings.ToUpper(sizeStr))
	if matches == nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	unit := matches[2]
	multiplier := int64(1)

	switch unit {
	case "", "B":
		multiplier = 1
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported unit: %s", unit)
	}

	return int64(value * float64(multiplier)), nil
}

func showProgress(ctx context.Context, totalDuration time.Duration) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(startTime)
			remaining := totalDuration - elapsed
			
			if remaining <= 0 {
				return
			}

			progress := float64(elapsed) / float64(totalDuration) * 100
			fmt.Printf("\rProgress: %.1f%% (Remaining: %v)", progress, remaining.Truncate(time.Second))
		}
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `
Usage: stress-go --timeout <duration> [options]

Options:
  --timeout <duration>  Duration to apply load (e.g., 30s, 5m, 1h) [required]
  --cpu <cores>         Number of CPU cores to use (0 = use all cores)
  --memory <size>       Memory load (e.g., 1GB, 512MB, 95%%)
  --storage <size>      Storage load (e.g., 500MB, 80%%)
  --help                Show this help

Examples:
  stress-go --timeout 60s --cpu 2
  stress-go --timeout 30s --cpu 0          # Use all CPU cores
  stress-go --timeout 5m --memory 1GB
  stress-go --timeout 2m --storage 80%%
  stress-go --timeout 30s --cpu 1 --memory 512MB --storage 500MB

`)
}