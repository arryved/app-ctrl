package gce

import (
	"context"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

type Client struct {
	Env    string
	Region string
	client *compute.Service
}

func NewClient(env, region string) *Client {
	ctx := context.Background()
	computeService, err := compute.NewService(ctx, option.WithScopes(compute.CloudPlatformScope))
	if err != nil {
		log.Fatalf("Failed to create compute service: %v", err)
	}
	return &Client{
		Env:    env,
		Region: region,
		client: computeService,
	}
}
