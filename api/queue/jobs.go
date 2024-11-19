package queue

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/config"
)

type JobRequest interface {
	Action() string
}

// JobRequest Type for Deploy
type DeployJobRequest struct {
	Cluster     config.Cluster
	Concurrency string
	Version     string
}

func (djr DeployJobRequest) Action() string {
	return "DEPLOY"
}

// JobRequest Type for Restart
type RestartJobRequest struct {
	Cluster     config.ClusterId
	Concurrency string
	Version     string
}

func (rjr RestartJobRequest) Action() string {
	return "RESTART"
}

type Job struct {
	Id        string     `json:"id"`
	Action    string     `json:"action"`
	Principal string     `json:"principal"`
	Request   JobRequest `json:"request"`
}

func (j *Job) UnmarshalJSON(data []byte) error {
	type JobAlias Job
	temp := &struct {
		Request json.RawMessage `json:"request"`
		*JobAlias
	}{
		JobAlias: (*JobAlias)(j),
	}

	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	switch j.Action {
	case "DEPLOY":
		var req DeployJobRequest
		err := json.Unmarshal(temp.Request, &req)
		if err != nil {
			return err
		}
		j.Request = &req
	case "RESTART":
		var req RestartJobRequest
		err := json.Unmarshal(temp.Request, &req)
		if err != nil {
			return err
		}
		j.Request = &req
	// case "ROLLBACK":
	// (add cases for other types as needed)
	//
	default:
		return fmt.Errorf("unknown action=%s", j.Action)
	}
	return nil
}

func NewJob(principal string, request JobRequest) (*Job, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		log.Errorf("Failed to generate job uuid: %v", err)
		return nil, err
	}
	job := Job{
		Id:        uuid.String(),
		Action:    request.Action(),
		Principal: principal,
		Request:   request,
	}
	return &job, nil
}
