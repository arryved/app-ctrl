package model

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	StaleDeployCompletedS = 300
	StaleDeployRequestedS = 3600
)

type Deploy struct {
	App         string `json:"app"`
	Version     string `json:"version"`
	RequestedAt int64  `json:"requestedAt"`
	StartedAt   int64  `json:"startedAt"`
	CompletedAt int64  `json:"completedAt"`
	Err         error  `json:"err"`
}

type DeployCache struct {
	mutex   sync.RWMutex
	deploys map[string]Deploy
}

func NewDeployCache() *DeployCache {
	return &DeployCache{
		deploys: make(map[string]Deploy),
	}
}

func (dc *DeployCache) AddDeploy(app string, deploy Deploy) bool {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	_, exists := dc.deploys[app]
	if exists {
		return false
	}
	dc.deploys[app] = deploy
	return true
}

func (dc *DeployCache) MarkDeployComplete(app string, err error) bool {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	_, exists := dc.deploys[app]
	if !exists {
		return false
	}

	log.Debugf("mark complete app=%s", app)
	deploy := dc.deploys[app]
	deploy.CompletedAt = time.Now().Unix()
	deploy.Err = err
	dc.deploys[app] = deploy
	return true
}

func (dc *DeployCache) MarkDeployStart(app string) bool {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	_, exists := dc.deploys[app]
	if !exists {
		return false
	}

	deploy := dc.deploys[app]
	deploy.StartedAt = time.Now().Unix()
	dc.deploys[app] = deploy
	return true
}

func (dc *DeployCache) DeleteDeploy(app string) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	_, exists := dc.deploys[app]
	if !exists {
		// no-op
		return
	}
	delete(dc.deploys, app)
	return
}

func (dc *DeployCache) GetDeploys() map[string]Deploy {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()
	return dc.deploys
}

func (dc *DeployCache) CleanDeploys() {
	// Clean up probable stale deploys, where either:
	// - CompletedAt is too long ago
	// - RequestedAt is too long ago
	// WARNING: Assumes inside locked mutex; don't use outside of lock.
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	now := time.Now().Unix()
	completedAtStale := false
	requestedAtStale := false

	for app, deploy := range dc.deploys {
		if deploy.CompletedAt == 0 {
			completedAtStale = false
		} else {
			completedAtStale = (now - deploy.CompletedAt) > StaleDeployCompletedS
		}
		requestedAtStale = (now - deploy.RequestedAt) > StaleDeployRequestedS
		if completedAtStale || requestedAtStale {
			log.Debugf("clearing out stale entry %s=%v", app, deploy)
			delete(dc.deploys, app)
		}
	}
}
