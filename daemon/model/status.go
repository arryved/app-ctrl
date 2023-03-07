package model

import (
	"fmt"
	"strconv"
	"strings"
)

type Status struct {
	Versions Versions `json:"versions"`
	Healthy  *bool    `json:"healthy"`
}
type Versions struct {
	Config    int      `json:"config"`
	Installed *Version `json:"installed"`
	Running   *Version `json:"running"`
}

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
	Build int `json:"build"`
}

func ParseVersion(version string) (Version, error) {
	result := Version{
		Major: -1,
		Minor: -1,
		Patch: -1,
		Build: -1,
	}

	// first, split by '-' to look for a build suffix
	fields := strings.Split(version, "-")

	// give up if too many hyphens
	if len(fields) > 2 {
		return result, fmt.Errorf("version string %s has too many dashes", version)
	}

	// if build suffix there, set in result
	if len(fields) == 2 {
		value, err := strconv.Atoi(fields[1])
		if err != nil {
			return result, fmt.Errorf("version build suffix %s is not a number", fields[1])
		} else {
			result.Build = value
		}
	}

	// get major, minor, patch
	fields = strings.Split(fields[0], ".")
	if len(fields) > 3 {
		return result, fmt.Errorf("version %v has too many dots", fields)
	}

	for i := range fields {
		_, err := strconv.Atoi(fields[i])
		if err != nil {
			return result, fmt.Errorf("version field %s in %v is not a number", fields[i], fields)
		}
	}

	// set as many versions as there are, starting with major
	result.Major, _ = strconv.Atoi(fields[0])
	if len(fields) >= 2 {
		result.Minor, _ = strconv.Atoi(fields[1])
	}
	if len(fields) == 3 {
		result.Patch, _ = strconv.Atoi(fields[2])
	}

	return result, nil
}
