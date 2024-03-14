package api

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

// Handler for /healthz
func NewConfiguredHandlerHealthz(cfg *config.Config, cache *model.StatusCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Infof("Call to /healthz: addr=%s method=%s url=%s", r.RemoteAddr, r.Method, r.URL)
		w.Header().Set("content-type", "application/json")

		log.Debugf("Checking for app param")
		app := r.URL.Query().Get("app")
		if app == "" {
			errorBody := "{\"error\": \"No app query param provided\"}"
			log.Warnf(errorBody)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(errorBody))
			return
		}

		statuses := cache.GetStatuses()
		responseBody, err := json.Marshal(statuses)
		if err != nil {
			errorBody := "{\"error\": \"Error marshalling statuses\"}"
			log.Errorf("%s: %v", errorBody, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(errorBody))
			return
		}

		// get HealthResults for the specified app
		healthResults := statuses[app]
		log.Debugf("healthResults=%v", healthResults)

		upFlag := true
		if len(healthResults.Health) == 0 {
			upFlag = false
		}

		for _, result := range healthResults.Health {
			if !result.Healthy {
				upFlag = false
			}
		}

		var httpStatus int
		if upFlag {
			// if all ports are Healthy, return 200 + OK
			httpStatus = http.StatusOK
			responseBody = []byte("OK")
		} else {
			// if some ports are not Healthy, return 400 + null body
			httpStatus = http.StatusBadRequest
			responseBody = []byte("")
		}

		log.Debugf("Response body=%s", string(responseBody))
		w.WriteHeader(httpStatus)
		w.Write(responseBody)
	}
}
