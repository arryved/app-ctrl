package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/config"
)

type DeployBody struct {
	Host        string `json:"host"`
	Region      string `json:"region"`
	Variant     string `json:"variant"`
	Concurrency string `json:"concurrency"`
	Version     string `json:"version"`
	Principal   string `json:"principal"`
	Runtime     string `json:"runtime"`
}

func ConfiguredHandlerDeploy(cfg *config.Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		httpStatus := http.StatusOK
		log.Infof("%s %s %s %d", r.RemoteAddr, r.Method, r.URL, httpStatus)

		if r.Method != http.MethodPost {
			msg := fmt.Sprintf("%s not allowed for this endpoint", r.Method)
			handleMethodNotAllowed(w, msg)
			return
		}

		// parse the POST json request body (via r *http.Request) into a DeployBody
		var requestBody DeployBody
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			msg := fmt.Sprintf("invalid request body: %s", r.URL)
			log.Infof(msg)
			handleBadRequest(w, msg)
			return
		}

		urlElements := strings.Split(r.URL.String(), "/")
		if len(urlElements) != 6 {
			msg := fmt.Sprintf("invalid request path: %s", r.URL)
			log.Infof(msg)
			handleBadRequest(w, msg)
			return
		}

		env := urlElements[2]
		app := urlElements[3]
		region := urlElements[4]
		variant := urlElements[5]

		clusterStatus, err := GetClusterStatus(cfg, env, app, region, variant)
		if err != nil {
			log.Errorf("error fetching statuses: %v", err.Error())
			handleInternalServerError(w, err)
			return
		}

		if err == nil && clusterStatus == nil {
			msg := fmt.Sprintf("cluster not found: %s", r.URL)
			log.Infof(msg)
			handleNotFound(w, msg)
			return
		}

		responseBody, err := json.Marshal(clusterStatus)
		if err != nil {
			log.Errorf("error marshalling statuses: %v", err.Error())
			handleInternalServerError(w, err)
			return
		}

		log.Debugf("response body=%s", string(responseBody))
		w.WriteHeader(httpStatus)
		w.Write(responseBody)
	}
}
