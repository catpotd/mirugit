package update

import (
	"strconv"
	"strings"
)

type version struct {
	major int
	minor int
	patch int
}

func parseVersion(tag string) (version, bool) {
	if !strings.HasPrefix(tag, "v") {
		return version{}, false
	}
	parts := strings.Split(tag[1:], ".")
	if len(parts) != 3 {
		return version{}, false
	}
	values := [3]int{}
	for i, part := range parts {
		if part == "" || strings.HasPrefix(part, "-") || strings.HasPrefix(part, "+") {
			return version{}, false
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return version{}, false
		}
		values[i] = value
	}
	return version{major: values[0], minor: values[1], patch: values[2]}, true
}

func (v version) after(other version) bool {
	if v.major != other.major {
		return v.major > other.major
	}
	if v.minor != other.minor {
		return v.minor > other.minor
	}
	return v.patch > other.patch
}
