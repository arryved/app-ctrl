//go:build !integration

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/arryved/app-ctrl/api/config"
)

func TestSubmitAndObtainDeployId(t *testing.T) {
	assert := assert.New(t)

	// set up a server config
	cfg := config.Load("../config/mock-config.yml")

	// set up interaction request and recorder for deploy handler
	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfiguredHandlerDeploy(cfg, nil, nil))
	requestBody := DeployRequest{
		Concurrency: "1",
		Version:     "0.1.0",
		Principal:   "example@arryved.com",
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("could not marshal body for test request err=%s", err.Error())
	}

	// simulate the API call
	uri := "/deploy/dev/arryved-api/central/default"
	fake_token, err := generateFakeIDToken()
	assert.NoError(err)
	req := httptest.NewRequest("POST", uri, bytes.NewBuffer(bodyBytes))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fake_token))
	handler.ServeHTTP(recorder, req)
	resp := recorder.Result()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("unable to read test request response body err=%s", err.Error())
	}

	// Body check basic submit/result flow
	response := DeployResponse{}
	err = json.Unmarshal(responseBody, &response)
	assert.Nil(err)
	assert.Equal("deploy job enqueued", response.Message)
	_, err = uuid.Parse(response.DeployId)
	assert.Nil(err)
}

// TODO - check to see that jobs submitted for an app already being acted on are rejected
