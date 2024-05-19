package cli

import (
	"strings"

	log "github.com/sirupsen/logrus"
)

func AptUpdate(executor GenericExecutor) error {
	executor.SetCommand("sudo")
	executor.SetArgs([]string{
		"apt",
		"update",
	})
	output, err := executor.Run(&map[string]string{
		"DEBIAN_FRONTEND": "noninteractive",
	})
	if err != nil {
		log.Errorf("Apt update failed err=%v", err)
		return err
	}
	log.Debugf("Apt update ouptut=%v", output)
	return nil
}

func AptInstall(executor GenericExecutor, aptTargets []string) error {
	executor.SetCommand("sudo")
	executor.SetArgs(append([]string{
		"apt",
		"install",
		"-o", "Dpkg::Options::=--force-confold",
		"-o", "Dpkg::Options::=--force-confdef",
		"-y",
		"--allow-downgrades",
		"--allow-remove-essential",
		"--allow-change-held-packages",
		"--reinstall",
	}, aptTargets...))
	output, err := executor.Run(&map[string]string{
		"DEBIAN_FRONTEND": "noninteractive",
	})
	if err != nil {
		log.Errorf("Apt install failed err=%v", err)
		return err
	}
	log.Debugf("Apt install ouptut=%v", output)
	return nil
}

func SystemdRestart(executor GenericExecutor, aptTargets []string) error {
	services := []string{}
	for _, target := range aptTargets {
		packageName := strings.Split(target, "=")[0]
		service := packageNameToService(packageName)
		services = append(services, service)
	}
	executor.SetCommand("sudo")
	executor.SetArgs(append([]string{
		"/usr/bin/systemctl",
		"restart",
	}, services...))
	output, err := executor.Run(&map[string]string{})
	if err != nil {
		log.Errorf("Systemctl restart failed err=%v", err)
		return err
	}
	log.Debugf("Systemctl restart ouptut=%v", output)
	return nil
}

// TODO temporary measure; remove when we sync app names with their systemd unit names
func packageNameToService(app string) string {
	switch app {
	case "arryved-api":
		return "arryved"
	case "arryved-merchant":
		return "merchant"
	case "arryved-customer":
		return "customer"
	case "arryved-portal":
		return "expo"
	case "arryved-onlineordering":
		return "nginx"
	case "arryved-gateway":
		return "gateway"
	case "arryved-hsm":
		return "hsm"
	case "arryved-insider":
		return "insider"
	case "arryved-integration":
		return "integration"
	default:
		return app
	}
}
