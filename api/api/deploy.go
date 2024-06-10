package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/queue"
	"github.com/arryved/app-ctrl/api/rbac"
)

type DeployRequest struct {
	Concurrency string `json:"concurrency"`
	Principal   string `json:"principal"`
	Version     string `json:"version"`
}

type DeployResponse struct {
	DeployId string `json:"deployId"` // deployId (blank if not available)
	Message  string `json:"message"`  // message is either of success or failure
}

func ConfiguredHandlerDeploy(cfg *config.Config, jobQueue *queue.Queue) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		httpStatus := http.StatusOK
		log.Infof("%s %s %s %d", r.RemoteAddr, r.Method, r.URL, httpStatus)

		if r.Method != http.MethodPost {
			msg := fmt.Sprintf("%s not allowed for this endpoint", r.Method)
			handleMethodNotAllowed(w, msg)
			return
		}

		// user authenticated?
		if !authenticated(cfg, r) {
			msg := fmt.Sprintf("user not authenticated")
			handleUnauthorized(w, msg)
			return
		}
		claims := getClaims(r)
		log.Debugf("claims=%v", claims)

		// parse the POST json request body (via r *http.Request) into a DeployRequest
		var requestBody DeployRequest
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			msg := fmt.Sprintf("invalid request body: %s", r.URL)
			log.Infof(msg)
			handleBadRequest(w, msg)
			return
		}
		log.Debugf("body=%v", requestBody)

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
		clusterId := config.ClusterId{
			App:     app,
			Region:  region,
			Variant: variant,
		}

		// user authorized for action on target?
		// TODO replace w/ claims results
		principalUrn := config.PrincipalUrn(fmt.Sprintf("urn:arryved:user:%s", claims["email"]))
		appUrn := fmt.Sprintf("urn:arryved:app:%s", app)
		if !rbac.Authorized(cfg, principalUrn, config.Deploy, appUrn) {
			msg := fmt.Sprintf("user not authorized in for deploy action")
			handleForbidden(w, msg)
			return
		}
		log.Debugf("Authorization granted for principal=%v, action=Deploy, app=%v", principalUrn, appUrn)

		// if no such cluster, return 404
		cluster, err := findClusterById(cfg, env, clusterId)
		if err != nil {
			log.Errorf("error fetching cluster status, cannot submit deploy: %v", err.Error())
			handleInternalServerError(w, err)
			return
		}
		if cluster == nil {
			msg := fmt.Sprintf("no such cluster matching id=%v", clusterId)
			log.Infof(msg)
			handleNotFound(w, msg)
			return
		}

		// enqueue the job onto a job queue for worker pickup
		job, err := queue.NewJob(requestBody.Principal, queue.DeployJobRequest{
			Cluster:     *cluster,
			Concurrency: requestBody.Concurrency,
			Version:     requestBody.Version,
		})
		if err != nil {
			log.Errorf("error creating new job request, cannot submit deploy: %v", err.Error())
			handleInternalServerError(w, err)
			return
		}

		if jobQueue != nil {
			pubid, err := jobQueue.Enqueue(job)
			if err != nil {
				log.Errorf("error enqueing deploy job error=%s", err.Error())
				handleInternalServerError(w, err)
				return
			}
			log.Infof("enqueued job jobid=%s pubid=%s", job.Id, pubid)
		} else {
			log.Warnf("job *not* enqueued since no jobQueue available id=%s", job.Id)
		}

		// TODO get the id and set a reasonable message
		responseBody, err := json.Marshal(DeployResponse{
			DeployId: job.Id,
			Message:  "deploy job enqueued",
		})
		if err != nil {
			log.Errorf("error marshaling response body: %v", err.Error())
			handleInternalServerError(w, err)
			return
		}

		log.Debugf("response body=%v", responseBody)
		w.WriteHeader(httpStatus)
		w.Write(responseBody)
	}
}
