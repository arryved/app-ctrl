package cli

import (
	"fmt"
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
		log.Debugf("Apt update ouptut=%v", output)
		return err
	}
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
		log.Debugf("Apt install ouptut=%v", output)
		return err
	}
	return nil
}

func SystemdReload(executor GenericExecutor, aptTargets []string) error {
	services := []string{}
	for _, target := range aptTargets {
		packageName := strings.Split(target, "=")[0]
		service := packageNameToService(packageName)
		services = append(services, service)
	}
	executor.SetCommand("sudo")
	executor.SetArgs([]string{
		"/usr/bin/systemctl",
		"daemon-reload",
	})
	output, err := executor.Run(&map[string]string{})
	if err != nil {
		log.Errorf("Systemctl daemon-reload failed err=%v", err)
		log.Debugf("Systemctl daemon-reload ouptut=%v", output)
		return err
	}
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
		log.Debugf("Systemctl restart ouptut=%v", output)
		return err
	}
	return nil
}

func ExpandConfigTarAsRoot(executor GenericExecutor, filePath, targetPath string) error {
	log.Infof("expanding tar.gz filePath=%s to dir targetPath=%s", filePath, targetPath)
	executor.SetCommand("sudo")
	executor.SetArgs(append([]string{
		"-u",
		"arryved",
		"/usr/bin/tar",
		"-xvzf",
		filePath,
		"-C",
		targetPath,
	}))
	output, err := executor.Run(&map[string]string{})
	if err != nil {
		log.Errorf("Tar extract failed err=%s", err.Error())
		log.Debugf("Tar extract ouptut=%v", output)
		return err
	}
	log.Infof("Tar extract succeeded to targetPath=%s", targetPath)
	return nil
}

func FixupDirectoryPermissions(executor GenericExecutor, targetPath string) error {
	log.Infof("chmod 750 started for directories under targetPath=%s", targetPath)
	executor.SetCommand("sudo")
	executor.SetArgs(append([]string{
		"-u",
		"arryved",
		"/usr/bin/find",
		targetPath,
		"-type",
		"d",
		"-exec",
		"chmod",
		"750",
		"{}",
		";",
	}))
	output, err := executor.Run(&map[string]string{})
	if err != nil {
		log.Errorf("chmod 750 failed for directories under targetPath=%s err=%s", targetPath, err.Error())
		log.Debugf("chmod 750 output=%v", output)
		return err
	}
	log.Infof("chmod 750 succeeded for directories under targetPath=%s", targetPath)
	return nil
}

func FixupControlPermissions(executor GenericExecutor, targetPath string) error {
	log.Infof("chmod 750 started for control script under targetPath=%s", targetPath)
	executor.SetCommand("sudo")
	executor.SetArgs(append([]string{
		"-u",
		"arryved",
		"/usr/bin/chmod",
		"750",
		fmt.Sprintf("%s/.arryved/control", targetPath),
	}))
	output, err := executor.Run(&map[string]string{})
	if err != nil {
		log.Errorf("chmod 750 failed for control script under targetPath=%s err=%s", targetPath, err.Error())
		log.Debugf("chmod 750 output=%v", output)
		return err
	}
	log.Infof("chmod 750 succeeded for control script under targetPath=%s", targetPath)
	return nil
}

func FixupControlPermissionsLegacy(executor GenericExecutor, targetPath string) error {
	log.Infof("chmod 750 started for start script under targetPath=%s", targetPath)
	executor.SetCommand("sudo")
	executor.SetArgs(append([]string{
		"-u",
		"arryved",
		"/usr/bin/find",
		targetPath,
		"-type",
		"f",
		"-name",
		"*.sh",
		"-exec",
		"chmod",
		"750",
		"{}",
		";",
	}))
	output, err := executor.Run(&map[string]string{})
	if err != nil {
		log.Errorf("chmod 750 failed for start script under targetPath=%s err=%s", targetPath, err.Error())
		log.Debugf("chmod 750 output=%v", output)
		return err
	}
	log.Infof("chmod 750 succeeded for start script under targetPath=%s", targetPath)
	return nil
}

func SymlinkControlScript(executor GenericExecutor, targetPath string) error {
	log.Infof("symbolic link creation started for control script under targetPath=%s", targetPath)
	executor.SetCommand("sudo")
	executor.SetArgs(append([]string{
		"-u",
		"arryved",
		"ln",
		"-s",
		fmt.Sprintf("targetPath/%s", ".arryved/control"),
		targetPath,
	}))
	output, err := executor.Run(&map[string]string{})
	if err != nil {
		log.Errorf("chmod 750 failed for control script under targetPath=%s err=%s", targetPath, err.Error())
		log.Debugf("chmod 750 output=%v", output)
		return err
	}
	log.Infof("chmod 750 succeeded for control script under targetPath=%s", targetPath)
	return nil
}

func FixupFilePermissions(executor GenericExecutor, targetPath string) error {
	log.Infof("chmod 640 started for files under targetPath=%s", targetPath)
	executor.SetCommand("sudo")
	executor.SetArgs(append([]string{
		"-u",
		"arryved",
		"/usr/bin/find",
		targetPath,
		"-type",
		"f",
		"-exec",
		"chmod",
		"640",
		"{}",
		";",
	}))
	output, err := executor.Run(&map[string]string{})
	if err != nil {
		log.Errorf("chmod 640 failed for files under targetPath=%s err=%s", targetPath, err.Error())
		log.Debugf("chmod 640 output=%v", output)
		return err
	}
	log.Infof("chmod 640 succeeded for files under targetPath=%s", targetPath)
	return nil
}

func FixupOwnership(executor GenericExecutor, targetPath string) error {
	log.Infof("chown arryved.arryved started for targetPath=%s", targetPath)
	executor.SetCommand("sudo")
	executor.SetArgs(append([]string{
		"-u",
		"arryved",
		"/usr/bin/find",
		targetPath,
		"-exec",
		"chown",
		"arryved.arryved",
		"{}",
		";",
	}))
	output, err := executor.Run(&map[string]string{})
	if err != nil {
		log.Errorf("chown arryved.arryved failed for files under targetPath=%s err=%s", targetPath, err.Error())
		log.Debugf("chown arryved.arryved output=%v", output)
		return err
	}
	log.Infof("chown arryved.arryved succeeded for files under targetPath=%s", targetPath)
	return nil
}

func CopyFileAs(executor GenericExecutor, user, src, dst string) error {
	log.Infof("copy file src=%s dst=%s", src, dst)
	executor.SetCommand("sudo")
	executor.SetArgs(append([]string{
		"-u",
		user,
		"cp",
		src,
		dst,
	}))
	output, err := executor.Run(&map[string]string{})
	if err != nil {
		log.Errorf("copy file failed src=%s dst=%s err=%s", src, dst, err.Error())
		log.Debugf("copy file output==%v", output)
		return err
	}
	log.Infof("copy file succeeded src=%s dst=%s", src, dst)
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
