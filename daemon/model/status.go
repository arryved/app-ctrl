package model

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type Status struct {
	Versions Versions       `json:"versions"`
	Health   []HealthResult `json:"health"`
}

type StatusCache struct {
	mutex    sync.RWMutex
	statuses map[string]Status
}

func NewStatusCache() *StatusCache {
	return &StatusCache{
		statuses: make(map[string]Status),
	}
}

func (sc *StatusCache) GetStatuses() map[string]Status {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()
	copy := make(map[string]Status)
	for key, value := range sc.statuses {
		copy[key] = value
	}
	return copy
}

func (sc *StatusCache) SetStatuses(newStatuses map[string]Status) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.statuses = newStatuses
}

type Versions struct {
	Config    int      `json:"config"`
	Installed *Version `json:"installed"`
	Running   *Version `json:"running"`
}

type HealthResult struct {
	Port    int  `json:"port"`
	Healthy bool `json:"healthy"`
	OOR     bool `json:"oor"`

	// service status is not known in this case;
	// the value of Healthy doesn't mean anything
	// when this is true
	Unknown bool `json:"unknown"`
}

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
	Build int `json:"build"`
}

func NewVersion() Version {
	return Version{
		Major: -1,
		Minor: -1,
		Patch: -1,
		Build: -1,
	}
}

func ParseVersion(version string) (Version, error) {
	result := NewVersion()

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
			// if this is not a number, just set to default -1; we only accept numbers for now
			result.Build = -1
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
