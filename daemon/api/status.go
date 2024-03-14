package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

// Handler for /status
func NewConfiguredHandlerStatus(cfg *config.Config, cache *model.StatusCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		httpStatus := http.StatusOK

		statuses := cache.GetStatuses()

		responseBody, err := json.Marshal(statuses)
		if err != nil {
			httpStatus = http.StatusInternalServerError
			log.Errorf("Error marshalling statuses: %v", err.Error())
			log.Infof("Call to /status: addr=%s method=%s url=%s httpStatus=%d", r.RemoteAddr, r.Method, r.URL, httpStatus)
			errorBody := fmt.Sprintf("{\"error\": \"%s\"}", err.Error())
			w.WriteHeader(httpStatus)
			w.Write([]byte(errorBody))
			return
		}

		log.Infof("Call to /status: addr=%s method=%s url=%s httpStatus=%d", r.RemoteAddr, r.Method, r.URL, httpStatus)
		log.Debugf("Response body=%s", string(responseBody))
		w.WriteHeader(httpStatus)
		w.Write(responseBody)
	}
}
