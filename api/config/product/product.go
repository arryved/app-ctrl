package product

import (
	"context"
	"fmt"
	"regexp"
	"runtime"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/config/storage"
)

//
// encapsulation of product configuration and operations

const (
	bucketName = "arryved-app-control-config"
)

type Config struct {
	ctx       context.Context
	storage   storage.StorageClient
	clusterId config.ClusterId
}

func (c *Config) Fetch(version string) ([]byte, error) {
	// Scan matching config bucket objects
	pattern := fmt.Sprintf("config-app=%s,hash=.*,version=%s.tar.gz", c.clusterId.App, version)
	log.Infof("looking for object with pattern=%s", pattern)
	objects, err := c.storage.ListObjects(bucketName)
	if err != nil {
		msg := fmt.Sprintf("Unexpected error listing objects in bucket=%s err=%s", bucketName, err.Error())
		log.Error(msg)
		return []byte{}, fmt.Errorf(msg)
	}

	for _, object := range objects {
		matched, err := regexp.MatchString(pattern, object.GetName())
		if err != nil {
			log.Debugf("Failed to match pattern: %v", err)
			continue
		}
		if matched {
			data, err := object.GetContents()
			if err != nil {
				log.Errorf("could not get configball object reader err=%s", err.Error())
				return []byte{}, err
			}
			return data, nil
		}
	}
	msg := fmt.Sprintf("No matching config for cluster=%v version=%s", c.clusterId, version)
	log.Warn(msg)
	return []byte{}, fmt.Errorf(msg)
}

func New(storageClient storage.StorageClient, clusterId config.ClusterId) *Config {
	ctx := context.Background()
	c := &Config{
		ctx:       ctx,
		storage:   storageClient,
		clusterId: clusterId,
	}
	runtime.SetFinalizer(c, func(c *Config) {
		log.Debugf("finalizer for config cluster id=%v", clusterId)
	})
	return c
}
