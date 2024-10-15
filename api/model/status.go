package model

import (
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
