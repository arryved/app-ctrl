package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

//
// Config 2.0 deploy for VMs; currently uses debs under the hood
//

type DeployResult struct {
	Code  int           `json:"code"`
	Err   string        `json:"err"`
	State *model.Deploy `json:"state"`
}

func handleError(w http.ResponseWriter, code int, errStr string) {
	// pick the right log level based on code
	logLevelFn := log.Info
	if code >= 400 {
		logLevelFn = log.Warn
	}
	if code >= 500 {
		logLevelFn = log.Error
	}

	// try to marshal and handle failure as 5xx if needed
	resultBody, err := json.Marshal(DeployResult{
		Code:  code,
		Err:   errStr,
		State: nil,
	})
	if err != nil {
		handleMarshalError(w, err)
		return
	}

	logLevelFn(errStr)
	w.WriteHeader(code)
	w.Write([]byte(resultBody))
}

func handleSuccess(w http.ResponseWriter, code int, result DeployResult, msg string) {
	resultBody, err := json.Marshal(result)
	if err != nil {
		handleMarshalError(w, err)
		return
	}
	log.Info(msg)
	w.WriteHeader(code)
	w.Write(resultBody)
}

func handleMarshalError(w http.ResponseWriter, err error) {
	errorBody := fmt.Sprintf("{\"error\": \"failed to marshal body err=%s\"}", err.Error())
	log.Errorf(errorBody)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(errorBody))
}

// Handler for /deploy?app=<APP>&version=<VERSION>
func NewConfiguredHandlerDeploy(cfg *config.Config, statusCache *model.StatusCache, deployCache *model.DeployCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Infof("Call to /deploy: addr=%s method=%s url=%s", r.RemoteAddr, r.Method, r.URL)
		w.Header().Set("content-type", "application/json")
		w.Header().Set("content-type", "keep-alive")

		// param validation
		log.Debugf("Checking for uri params")
		app := r.URL.Query().Get("app")
		version := r.URL.Query().Get("version")
		if app == "" || version == "" {
			handleError(w, http.StatusBadRequest, "Required query param missing, provide both app and version")
			return
		}

		// run deploy in bg
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(cfg.WriteTimeoutS)*time.Second)
		defer cancel()
		ch := make(chan DeployResult, 1)
		go func() {
			ch <- Deploy(cfg, statusCache, deployCache, app, version)
		}()

		// wait for deploy completion or timeout
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				handleError(w, http.StatusRequestTimeout, "Timeout exceeded waiting for deploy")
			} else {
				handleError(w, http.StatusInternalServerError, fmt.Sprintf("Context finished with unknown err=%v", ctx.Err()))
			}
		case result := <-ch:
			if result.Err != "" {
				handleError(w, result.Code, fmt.Sprintf("Deploy failed err=%s", result.Err))
			} else {
				logMsg := fmt.Sprintf("Deploy succeeded /deploy: addr=%s method=%s url=%s", r.RemoteAddr, r.Method, r.URL)
				handleSuccess(w, result.Code, result, logMsg)
			}
		}
		return
	}
}

func Deploy(cfg *config.Config, statusCache *model.StatusCache, deployCache *model.DeployCache, app, version string) DeployResult {
	// this doesn't call *directly* ; instead, it sets a desired version in a shared map, and then
	// waits a max amount of time for a bg runner to complete successfully & converge at the intended version.
	// if it does not complete, a failure is returned
	// if it does complete, a success is returned

	log.Debugf("Deploy() app=%s version=%s", app, version)
	deploy := model.Deploy{
		App:         app,
		Version:     version,
		RequestedAt: time.Now().Unix(),
	}

	// try to insert into DeployCache; if insert fails, then it's already present; return 429 in this case
	if !deployCache.AddDeploy(app, deploy) {
		return DeployResult{
			Code: http.StatusTooManyRequests,
			Err:  fmt.Sprintf("deploy already requested for %s", app),
		}
	}
	defer deployCache.DeleteDeploy(app)

	// wait for deploy action to complete
	latestState := deployCache.GetDeploys()[app]
	interval := time.Duration(float64(cfg.WriteTimeoutS) * 0.05 * float64(time.Second))
	for {
		log.Debugf("checking for completion %v", latestState)
		time.Sleep(time.Duration(interval))
		latestState = deployCache.GetDeploys()[app]
		if latestState.CompletedAt != 0 {
			log.Infof("deploy marked completed app=%s, state=%v", app, latestState)
			break
		}
		log.Debugf("deploy completion not seen yet app=%s", app)
	}

	// if any error attached to deploy, return now
	if latestState.Err != nil {
		return DeployResult{
			Code:  http.StatusInternalServerError,
			Err:   latestState.Err.Error(),
			State: &latestState,
		}
	}

	// error on time out based a configured converge interval (ideally less than deploy timeout)
	// confirm from statusCache that install+running statuses converged on requested version
	converged := waitForConverge(statusCache, app, version, time.Duration(cfg.ConvergeTimeoutSec)*time.Second)

	// if convergence failure after deploy, 408
	convergenceMsg := getConvergenceMsg(statusCache, app, version)
	if converged {
		log.Infof("deploy converged %s", convergenceMsg)
	} else {
		log.Errorf("deploy did not converge %s", convergenceMsg)
		return DeployResult{
			Code:  http.StatusRequestTimeout,
			Err:   fmt.Sprintf("deploy did not converge %s", convergenceMsg),
			State: &latestState,
		}
	}

	// otherwise things are probably fine, 200
	return DeployResult{
		Code:  http.StatusOK,
		State: &latestState,
	}
}

func getConvergenceMsg(statusCache *model.StatusCache, app, version string) string {
	latestStatuses := statusCache.GetStatuses()
	installedVersion := latestStatuses[app].Versions.Installed
	runningVersion := latestStatuses[app].Versions.Running
	return fmt.Sprintf("app=%s, desired=%s, installed=%v, running=%v", app, version, installedVersion, runningVersion)
}

func waitForConverge(statusCache *model.StatusCache, app, version string, duration time.Duration) bool {
	var installedVersion *model.Version
	var runningVersion *model.Version
	converged := false
	convergedInstall := false
	convergedRunning := false
	interval := time.Duration(duration * 5 / 100)

	requestedVersion, err := model.ParseVersion(version)
	if err != nil {
		log.Warnf("could not parse requested version string app=%s, version=%s", app, version)
	} else {
		// wait for deploy completion or timeout
		convergeCh := make(chan bool, 1)
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()
		go func() {
			for {
				log.Debugf("checking for convergence")
				time.Sleep(time.Duration(interval))
				latestStatuses := statusCache.GetStatuses()
				installedVersion = latestStatuses[app].Versions.Installed
				runningVersion = latestStatuses[app].Versions.Running

				convergedInstall = *installedVersion == requestedVersion
				convergedRunning = *runningVersion == requestedVersion
				converged = convergedInstall && convergedRunning
				if converged {
					convergeCh <- converged
				}
			}
		}()
		select {
		case <-ctx.Done():
			return false
		case converged = <-convergeCh:
			return converged
		}
	}
	return converged
}
