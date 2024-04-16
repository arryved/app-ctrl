package queue

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/pubsub"
	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/config"
)

type Queue struct {
	client *pubsub.Client
	cfg    config.QueueConfig
}

func (q *Queue) Enqueue(job *Job) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	topic := q.client.Topic(q.cfg.Topic)
	jsonData, err := json.Marshal(job)
	if err != nil {
		log.Errorf("failed to enqueue job=%v err=%s", job, err.Error())
		return "", err
	}
	result := topic.Publish(ctx, &pubsub.Message{Data: jsonData})
	id, err := result.Get(ctx)
	if err != nil {
		log.Errorf("failed to get publish result job=%v err=%s", job, err.Error())
		return "", err
	}
	log.Debugf("enqueued job jobid=%s pubid=%s", job.Id, id)
	return id, nil
}

func (q *Queue) Dequeue() (*Job, error) {
	job := Job{}
	sub := q.client.Subscription(q.cfg.Subscription)
	sub.ReceiveSettings.MaxOutstandingMessages = 1

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		log.Infof("received message pubid=%s", msg.ID)
		err := json.Unmarshal(msg.Data, &job)
		if err != nil {
			log.Errorf("failed to unmarshal msg.Data, err=%s", err.Error())
			return
		}
		log.Infof("dequeued job jobid=%s pubid=%s", job.Id, msg.ID)
		msg.Ack()
		cancel()
	})
	if err != nil {
		log.Errorf("failed to pull from sub=%v", sub)
		return nil, err
	}
	return &job, nil
}

func NewClient(cfg config.QueueConfig) (*pubsub.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := pubsub.NewClient(ctx, cfg.Project)
	if err != nil {
		log.Errorf("Failed to create client for cfg=%v, err=%s", cfg, err.Error())
		return nil, err
	}
	return client, nil
}

func NewQueue(cfg config.QueueConfig, client *pubsub.Client) *Queue {
	return &Queue{
		client: client,
		cfg:    cfg,
	}
}
