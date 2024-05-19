package runners

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/arryved/app-ctrl/daemon/cli"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

func DeployRunner(cfg *config.Config, cache *model.DeployCache, executor *cli.Executor) {
	for {
		// insert pause to prevent hard busy-wait
		log.Debugf("Deploy runner going to sleep for %d seconds", cfg.DeployIntervalSec)
		time.Sleep(time.Duration(cfg.DeployIntervalSec) * time.Second)

		// clean the cache of stale deploys
		log.Debug("Clean stale deploys")
		cache.CleanDeploys()

		// get the latest values
		log.Debug("Get latest deploys")
		deploys := cache.GetDeploys()

		// construct targets from deploys list
		log.Debug("Construct targets from deploys list")
		aptTargets := []string{}
		for _, deploy := range deploys {
			if deploy.CompletedAt == 0 {
				aptTargets = append(aptTargets, fmt.Sprintf("%s=%s", deploy.App, deploy.Version))
				cache.MarkDeployStart(deploy.App)
			}
		}

		// Restart loop/go back to sleep if no targets
		if len(aptTargets) == 0 {
			log.Debug("No deploy targets, so nothing to do")
			continue
		}
		log.Infof("Deploy runner targets=%v", aptTargets)

		// Set OOR for all targets
		for _, target := range aptTargets {
			app, _ := targetComponents(target)
			SetOOR(cfg.AppDefs[app])
		}

		// Run install on combined list of desired packages + versions.
		// NOTE: because these can be batched, a failure in apt install or restart will cause the
		//       whole batch to fail. This is probably not a big deal since it's just one machine,
		//       and it generally can only happen on non-prod envs where there are multiple apps
		//       per machine. If batching is causing problems, reduce cfg.DeployIntervalSec and/or
		//       add splay when kicking off multiple app deployments.
		log.Infof("Deploying the latest desired app=version set=%v", aptTargets)
		err := aptInstallAndRestart(aptTargets, executor)
		log.Infof("Deploy finished; err=%v", err)

		// Unset OOR for all targets (generally safe since the LB won't add the node back if the health check fails)
		for _, target := range aptTargets {
			app, _ := targetComponents(target)
			UnsetOOR(cfg.AppDefs[app])
		}

		// update deploys map with completion time and err for each app targeted in this loop
		for _, target := range aptTargets {
			app, _ := targetComponents(target)
			completed := cache.MarkDeployComplete(app, err)
			if !completed {
				log.Warnf("Unexpected failure to mark deploy as completed app=%s, cache=%v", app, cache.GetDeploys())
			}
		}
	}
}

func targetComponents(target string) (string, string) {
	list := strings.Split(target, "=")
	return list[0], list[1]
}

func aptInstallAndRestart(aptTargets []string, executor *cli.Executor) error {
	err := cli.AptUpdate(executor)
	if err != nil {
		msg := fmt.Sprintf("Apt update failed err=%v", err)
		log.Errorf(msg)
		return fmt.Errorf(msg)
	}

	err = cli.AptInstall(executor, aptTargets)
	if err != nil {
		msg := fmt.Sprintf("Apt install failed err=%v", err)
		log.Errorf(msg)
		return fmt.Errorf(msg)
	}

	err = cli.SystemdRestart(executor, aptTargets)
	if err != nil {
		msg := fmt.Sprintf("Systemd restart failed err=%v", err)
		log.Errorf(msg)
		return fmt.Errorf(msg)
	}
	return nil
}
