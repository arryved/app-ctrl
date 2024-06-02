package storage

import (
	"cloud.google.com/go/storage"
	"context"
	"io/ioutil"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
)

type StorageClient interface {
	ListObjects(string) ([]StorageObject, error)
}

type StorageObject interface {
	GetContents() ([]byte, error)
	GetName() string
}

type GCPStorageClient struct {
	ctx     context.Context
	storage *storage.Client
}

func (c *GCPStorageClient) ListObjects(bucketName string) ([]StorageObject, error) {
	collector := []StorageObject{}
	iter := c.storage.Bucket(bucketName).Objects(c.ctx, nil)
	for {
		attrs, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Errorf("Failed to list objects: %v", err)
			return []StorageObject{}, err
		}
		collector = append(collector, &GCPStorageObject{
			bucketName: bucketName,
			storage:    c,

			Name:         attrs.Name,
			Size:         attrs.Size,
			ContentType:  attrs.ContentType,
			Updated:      attrs.Updated,
			Generation:   attrs.Generation,
			StorageClass: attrs.StorageClass,
			MD5:          attrs.MD5,
		})
	}
	return collector, nil
}

type GCPStorageObject struct {
	bucketName string
	storage    *GCPStorageClient

	Name         string
	Size         int64
	ContentType  string
	Updated      time.Time
	Generation   int64
	StorageClass string
	MD5          []byte
}

func (o *GCPStorageObject) GetContents() ([]byte, error) {
	// Get the contents of the mostRecent matching object
	reader, err := o.storage.storage.Bucket(o.bucketName).Object(o.GetName()).NewReader(o.storage.ctx)
	if err != nil {
		log.Errorf("could not get configball object reader err=%s", err.Error())
		return []byte{}, err
	}
	defer reader.Close()
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Errorf("could get configball object contents err=%s", err.Error())
		return []byte{}, err
	}
	log.Infof("matching config object found name=%v", o.Name)
	return data, nil
}

func (o *GCPStorageObject) GetName() string {
	return o.Name
}

func New(ctx context.Context) (*GCPStorageClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GCPStorageClient{
		ctx:     ctx,
		storage: client,
	}, nil
}
