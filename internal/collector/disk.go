package collector

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
	"zimaos-monitor/internal/config"
)

type DiskStats struct {
	Path    string  `json:"path"`
	UsedPct float64 `json:"used_pct"`
	UsedGB  float64 `json:"used_gb"`
	TotalGB float64 `json:"total_gb"`
	FreeGB  float64 `json:"free_gb"`
}

func CollectDisks(cfgDisks []config.DiskConfig) map[string]DiskStats {
	result := make(map[string]DiskStats, len(cfgDisks))
	for _, d := range cfgDisks {
		usage, err := disk.Usage(d.Path)
		if err != nil {
			log.Printf("warn: disk usage %s: %v (skipping)", d.Path, err)
			continue
		}
		if usage.Total == 0 {
			continue
		}
		result[d.Name] = DiskStats{
			Path:    d.Path,
			UsedPct: usage.UsedPercent,
			UsedGB:  float64(usage.Used) / 1e9,
			TotalGB: float64(usage.Total) / 1e9,
			FreeGB:  float64(usage.Free) / 1e9,
		}
	}
	return result
}

// DiscoverDisks returns the storage volumes that ZimaOS exposes to the user.
//
// ZimaOS mounts external/NVMe drives under /media/<name> using the same name shown in
// its UI. The internal eMMC data partition is the exception: it is mounted at /DATA but
// displayed as "ZimaOS-HD" in the ZimaOS web UI, so we apply that mapping explicitly.
//
// Skips read-only mounts and mounts with no usable space (empty/phantom bays).
func DiscoverDisks() []config.DiskConfig {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil
	}

	var result []config.DiskConfig
	for _, p := range partitions {
		var name string
		switch {
		case strings.HasPrefix(p.Mountpoint, "/media/"):
			name = filepath.Base(p.Mountpoint)
		case p.Mountpoint == "/DATA":
			name = "ZimaOS-HD"
		default:
			continue
		}

		isRO := false
		for _, opt := range p.Opts {
			if opt == "ro" {
				isRO = true
				break
			}
		}
		if isRO {
			continue
		}
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}
		result = append(result, config.DiskConfig{Path: p.Mountpoint, Name: name})
	}
	return result
}
