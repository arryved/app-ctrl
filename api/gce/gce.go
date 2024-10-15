package gce

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"runtime"
	"strings"

	"github.com/arryved/app-ctrl/api/config"
)

// TODO use a canon API/library instead of hard-coding
var ProjectMap = map[string]string{
	"demo":      "arryved-demo1",
	"dev":       "arryved-177921",
	"dev-int":   "arryved-234222",
	"tools":     "arryved-tools",
	"stg":       "arryved-staging",
	"prod":      "arryved-prod",
	"cde":       "arryved-secure",
	"sandbox":   "arryved-177921",
	"simc-prod": "simc-prod",
}

type Client struct {
	cancel context.CancelFunc
	client *compute.Service
	ctx    context.Context
	Env    string
	Region string
}

type InstanceMetadata struct {
	Clusters []config.ClusterId `json:"clusters"`
}

func (c *Client) GetRegionAppControlInstances() (map[config.ClusterId][]*compute.Instance, error) {
	// A one-shot fetch of all app-control metadata in the region... for caching results
	appControlInstances := map[config.ClusterId][]*compute.Instance{}
	project := c.getGCPProjectId()
	region := c.getGCPRegion()

	// Collect all instances in region
	regionInfo, err := c.client.Regions.Get(project, region).Context(c.ctx).Do()
	if err != nil {
		log.Errorf("failed to get project=(%s) region=(%s) info: %s", project, region, err.Error())
		return appControlInstances, fmt.Errorf("could not get region info for current gce client")
	}
	for _, zone := range regionInfo.Zones {
		zoneName := zone[strings.LastIndex(zone, "/")+1:]
		zoneInstances, err := c.client.Instances.List(project, zoneName).Context(c.ctx).Do()
		if err != nil {
			msg := fmt.Sprintf("error getting instances project=%s region=%s zone=%s err=%s", project, region, zoneName, err.Error())
			log.Errorf(msg)
			return appControlInstances, fmt.Errorf(msg)
		}
		for _, instance := range zoneInstances.Items {
			// Look for app-control metadata
			var metadataJson *string
			for _, item := range instance.Metadata.Items {
				if item.Key == "app-control" && item.Value != nil {
					metadataJson = item.Value
					break
				}
			}
			if metadataJson == nil {
				log.Debugf("no app-control metadata instance=%s", instance.Name)
			} else {
				// Metadata found; parse and add if instance is in the target cluster
				metadata := &InstanceMetadata{}
				err := json.Unmarshal([]byte(*metadataJson), &metadata)
				if err != nil {
					log.Warnf("could not unmarshal metadata instance=%s json=%s", instance.Name, *metadataJson)
				} else {
					log.Debugf("unmarshaled metadata instance=%s metadata=%v", instance.Name, metadata)
					for _, id := range metadata.Clusters {
						list, ok := appControlInstances[id]
						if ok {
							list = append(list, instance)
						} else {
							appControlInstances[id] = []*compute.Instance{instance}
						}
						log.Infof("added instance=%s to id=%v", instance.Name, id)
					}
				}
			}
		}
	}
	return appControlInstances, nil
}

func (c *Client) getGCPProjectId() string {
	return ProjectMap[c.Env]
}

func (c *Client) getGCPRegion() string {
	// TODO use a canon API/library instead of hard-coding
	regionMap := map[string]map[string]string{
		"demo": {
			"central": "us-central1",
		},
		"dev": {
			"central": "us-central1",
		},
		"dev-int": {
			"central": "us-central1",
		},
		"tools": {
			"central": "us-central1",
		},
		"stg": {
			"central": "us-central1",
		},
		"prod": {
			"central": "us-central1",
			"east":    "us-east1",
		},
		"cde": {
			"central": "us-central1",
			"east":    "us-east1",
		},
		"sandbox": {
			"central": "us-central1",
		},
		"simc-prod": {
			"central": "us-central1",
			"west":    "us-west4",
		},
	}
	return regionMap[c.Env][c.Region]
}

func NewClient(env, region string) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	computeService, err := compute.NewService(ctx, option.WithScopes(compute.CloudPlatformScope))
	if err != nil {
		log.Fatalf("Failed to create compute service: %v", err)
	}
	client := &Client{
		cancel: cancel,
		client: computeService,
		ctx:    ctx,

		Env:    env,
		Region: region,
	}
	runtime.SetFinalizer(client, func(c *Client) {
		log.Debugf("finalizer called for GCE client env=%s region=%s", c.Env, c.Region)
		c.cancel()
	})

	return client
}
