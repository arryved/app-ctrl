package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/healthz"
	"github.com/arryved/app-ctrl/daemon/model"
	"github.com/arryved/app-ctrl/daemon/varz"
)

// Handler for /status
func ConfiguredHandlerStatus(
	cfg *config.Config,
	ch chan map[string]model.Status,
	cache map[string]model.Status) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		httpStatus := http.StatusOK

		log.Debugf("Handler channel=%v", ch)
		if len(ch) == 0 {
			// no result yet; use cache
			log.Debug("no result yet, rely on cache")
		} else {
			// pull latest result and cache it
			log.Debug("pull latest result and cache it")
			cache = <-ch
		}

		responseBody, err := json.Marshal(cache)
		if err != nil {
			httpStatus = http.StatusInternalServerError
			log.Errorf("error marshalling statuses: %v", err.Error())
			errorBody := fmt.Sprintf("{\"error\": \"%s\"}", err.Error())
			w.WriteHeader(httpStatus)
			w.Write([]byte(errorBody))
			return
		}

		log.Infof("%s %s %s %d", r.RemoteAddr, r.Method, r.URL, httpStatus)
		log.Debugf("response body=%s", string(responseBody))
		w.WriteHeader(httpStatus)
		w.Write(responseBody)
	}
}

func StatusRunner(cfg *config.Config, ch chan map[string]model.Status) {
	for {
		// get the latest values
		statuses, err := getStatuses(cfg)
		if err != nil {
			log.Errorf("error getting statuses: %v", err)
			continue
		}

		// if there's a value on the channel already, get rid of it in favor of the new one
		log.Debugf("Runner channel=%v", ch)
		if len(ch) > 0 {
			log.Debug("discarding stale value on channel")
			<-ch
		}
		log.Debugf("sending statuses = %v", statuses)
		ch <- statuses

		// insert pause to prevent hard busy-wait
		log.Debug("status runner sleeping")
		time.Sleep(5 * time.Second)
	}
}

func getStatuses(cfg *config.Config) (map[string]model.Status, error) {
	statuses := map[string]model.Status{}

	versionsByApp, err := getInstalledVersions(cfg)
	if err != nil {
		log.Errorf("could not get installed versions: %v", err)
		return statuses, err
	}

	appCount := len(versionsByApp)
	log.Debugf("found %d installed apps on this host", len(versionsByApp))
	if appCount == 0 {
		log.Warn("no installed apps found on this host")
	}

	for app, version := range versionsByApp {
		healthResults := runHealthChecks(cfg.AppDefs[app])
		runningVersion := getRunningVersion(cfg.AppDefs[app])

		status := model.Status{
			Versions: model.Versions{
				Installed: &model.Version{
					// copy values instead of using &version, otherwise flaky
					Major: version.Major,
					Minor: version.Minor,
					Patch: version.Patch,
					Build: version.Build,
				},
				Running: &model.Version{
					// copy values instead of using &version, otherwise flaky
					Major: runningVersion.Major,
					Minor: runningVersion.Minor,
					Patch: runningVersion.Patch,
					Build: runningVersion.Build,
				},
			},
			Health: healthResults,
		}
		statuses[app] = status
	}
	return statuses, nil
}

func runHealthChecks(appDef config.AppDef) []model.HealthResult {
	results := []model.HealthResult{}

	// only run for supported type(s)
	if appDef.Type != config.Online {
		return results
	}
	for i := range appDef.Healthz {
		result := healthz.Check(appDef.Healthz[i])
		results = append(results, result)
	}
	return results
}

func getInstalledVersions(cfg *config.Config) (map[string]model.Version, error) {
	statuses := map[string]model.Version{}

	args := "list --installed"
	cmd := exec.Command(cfg.AptPath, strings.Split(args, " ")...)

	stdout, _ := cmd.StdoutPipe()
	cmd.Start()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		// ignore non-content lines
		match, _ := regexp.MatchString(`\[.*installed.*\]`, line)
		if !match {
			log.Debugf("skipped line=%s", line)
			continue
		}

		// parse line, format: "arryved-api/unknown,now 2.14.2 amd64 [installed]"
		fields := strings.Split(line, "/")
		name := fields[0]

		// skip apps that aren't in the AppDefs
		_, ok := cfg.AppDefs[name]
		if !ok {
			continue
		}

		// parse out version
		fields = strings.Split(fields[1], " ")
		installedVersion, err := model.ParseVersion(fields[1])
		if err != nil {
			log.Warnf("version %s could not be parsed: %v", fields[1], err)
			return statuses, err
		}

		// select based on known apps
		statuses[name] = installedVersion
	}

	cmd.Wait()
	return statuses, nil
}

func getRunningVersion(appDef config.AppDef) model.Version {
	version := model.Version{
		Major: -1,
		Minor: -1,
		Patch: -1,
		Build: -1,
	}

	if appDef.Varz == nil {
		return version
	}
	varzResult := varz.Check(*appDef.Varz)
	parsedVersion, err := model.ParseVersion(varzResult.ServerInfo.Version)
	if err != nil {
		log.Warnf("could not parse version string %s", varzResult.ServerInfo.Version)
	} else {
		return parsedVersion
	}

	return version
}
