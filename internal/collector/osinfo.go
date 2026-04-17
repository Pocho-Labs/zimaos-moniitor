package collector

import (
	"os"
	"strings"
)

// ZimaOSVersion reads /etc/os-release and returns the VERSION_ID value
// (e.g. "1.5.4"). Falls back to VERSION if VERSION_ID is absent.
// Returns "" if the file cannot be read or has neither key.
func ZimaOSVersion() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}

	versionID := ""
	version := ""

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"`)

		if key == "VERSION_ID" && versionID == "" {
			versionID = val
		}
		if key == "VERSION" && version == "" {
			version = val
		}
	}

	if versionID != "" {
		return versionID
	}
	return version
}
