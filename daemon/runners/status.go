package runners

import (
	"bufio"
	"os/exec"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/arryved/app-ctrl/daemon/clients/healthz"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
	"github.com/arryved/app-ctrl/daemon/varz"
)

func StatusRunner(cfg *config.Config, cache *model.StatusCache) {
	for {
		// get the latest values
		statuses, err := GetStatuses(cfg)
		if err != nil {
			log.Errorf("error getting statuses: %v", err)
			log.Debug("status runner sleeping")
			time.Sleep(time.Duration(cfg.PollIntervalS) * time.Second)
			continue
		}

		// update the cache
		log.Debugf("Updating the cache")
		cache.SetStatuses(statuses)

		// insert pause to prevent hard busy-wait
		log.Debug("status runner sleeping")
		time.Sleep(time.Duration(cfg.PollIntervalS) * time.Second)
	}
}

func GetStatuses(cfg *config.Config) (map[string]model.Status, error) {
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

	oor := isOOR(appDef)

	for i := range appDef.Healthz {
		result := healthz.Check(appDef.Healthz[i])
		// check for OOR file in app root; override to force unhealthy check if the OOR file is on disk
		if oor {
			result.OOR = true
			result.Healthy = false
		}
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
			log.Debugf("version %s could not be parsed: %v", fields[1], err)
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
		log.Debugf("could not parse version string %s", varzResult.ServerInfo.Version)
	} else {
		return parsedVersion
	}

	return version
}
