package runners

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/compute/v1"

	"github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/gce"
)

type GCECache struct {
	sync.RWMutex
	data map[config.ClusterId][]*compute.Instance
}

func (c *GCECache) Get() map[config.ClusterId][]*compute.Instance {
	c.RLock()
	defer c.RUnlock()
	return c.data
}

func (c *GCECache) Set(newData map[config.ClusterId][]*compute.Instance) error {
	c.Lock()
	defer c.Unlock()
	for k, v := range newData {
		c.data[k] = v
	}
	return nil
}

type GCECacheRunner struct {
	cfg   *config.Config
	Cache *GCECache
}

func NewGCECacheRunner(cfg *config.Config) *GCECacheRunner {
	runner := &GCECacheRunner{
		Cache: &GCECache{
			data: make(map[config.ClusterId][]*compute.Instance),
		},
		cfg: cfg,
	}
	return runner
}

func (r *GCECacheRunner) Start() {
	go func() {
		log.Info("started GCECacheRunner")
		envRegions := r.regionsFromConfig()
		log.Infof("envRegions covered in config=%v", envRegions)
		for {
			for envRegion := range envRegions {
				client := gce.NewClient(envRegion.Env, envRegion.Region)
				instancesByEnvRegion, err := client.GetRegionAppControlInstances()
				if err != nil {
					log.Warnf("Could not get app-control instances, err=%s", err.Error())
				}
				r.Cache.Set(instancesByEnvRegion)
			}
			// update cache with instancesByEnvRegion
			log.Info("GCECacheRunner going to sleep for a bit")
			time.Sleep(time.Second * 60)
		}
	}()
}

type topoSet struct {
	Env    string
	Region string
}

func (r *GCECacheRunner) regionsFromConfig() map[topoSet]bool {
	envRegions := map[topoSet]bool{}
	for env, clusters := range r.cfg.Topology {
		for _, cluster := range clusters.Clusters {
			envRegions[topoSet{Env: env, Region: cluster.Id.Region}] = true
		}
	}
	return envRegions
}
