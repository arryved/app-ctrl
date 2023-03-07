//go:build !integration

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

func TestStatusHandler(t *testing.T) {
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")
	cfg.AptPath = "./mock_apt"

	responder := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfiguredHandlerStatus(cfg))
	req, err := http.NewRequest("GET", "/status", nil)

	handler.ServeHTTP(responder, req)

	assert.Nil(err)
	assert.Equal(200, responder.Code)

	// unmarshal and confirm result
	result := map[string]model.Status{}
	err = json.Unmarshal(responder.Body.Bytes(), &result)

	assert.Nil(err)
	assert.Equal(11, len(result))
	//assert.Equal(1, result["arryved-api"].Versions.Installed.Major)
}

// TODO finish these test cases
