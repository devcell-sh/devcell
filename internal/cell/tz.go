package cell

import (
	"os"
	"strings"
)

// zoneinfoSuffixes are the prefixes stripped from an /etc/localtime symlink
// target to yield an IANA timezone name. macOS uses /var/db/timezone/zoneinfo/,
// most Linux distros use /usr/share/zoneinfo/.
var zoneinfoSuffixes = []string{
	"/usr/share/zoneinfo/",
	"/var/db/timezone/zoneinfo/",
	"/usr/share/zoneinfo/posix/",
	"/usr/share/lib/zoneinfo/",
}

// DetectTimezone returns the host's IANA timezone name by inspecting OS files.
// Resolution: /etc/localtime readlink > /etc/timezone > "UTC".
func DetectTimezone() string {
	if tz := detectFromLocaltime(); tz != "" {
		return tz
	}
	if tz := detectFromEtcTimezone(); tz != "" {
		return tz
	}
	return "UTC"
}

func detectFromLocaltime() string {
	target, err := os.Readlink("/etc/localtime")
	if err != nil {
		return ""
	}
	for _, prefix := range zoneinfoSuffixes {
		if strings.HasPrefix(target, prefix) {
			tz := strings.TrimPrefix(target, prefix)
			if tz != "" && strings.Contains(tz, "/") {
				return tz
			}
		}
	}
	return ""
}

func detectFromEtcTimezone() string {
	data, err := os.ReadFile("/etc/timezone")
	if err != nil {
		return ""
	}
	tz := strings.TrimSpace(string(data))
	if tz != "" && tz != "Local" {
		return tz
	}
	return ""
}
