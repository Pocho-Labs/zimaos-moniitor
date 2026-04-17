package collector

import (
	"fmt"

	"github.com/shirou/gopsutil/v3/mem"
)

type MemoryStats struct {
	UsedPct     float64 `json:"used_pct"`
	AvailableGB float64 `json:"available_gb"`
	TotalGB     float64 `json:"total_gb"`
}

func CollectMemory() (MemoryStats, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return MemoryStats{}, fmt.Errorf("virtual memory: %w", err)
	}
	return MemoryStats{
		UsedPct:     v.UsedPercent,
		AvailableGB: float64(v.Available) / 1e9,
		TotalGB:     float64(v.Total) / 1e9,
	}, nil
}
