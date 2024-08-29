package runners

import (
	"fmt"
	"os"

	"github.com/arryved/app-ctrl/daemon/cli"
	"github.com/arryved/app-ctrl/daemon/config"
)

const oorFilename = ".oor"

func isOOR(appDef config.AppDef) bool {
	root := appDef.AppRoot
	path := fmt.Sprintf("%s/%s", root, oorFilename)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func SetOOR(executor cli.GenericExecutor, appDef config.AppDef) error {
	root := appDef.AppRoot
	path := fmt.Sprintf("%s/%s", root, oorFilename)
	err := cli.TouchFile(executor, path)
	if err != nil {
		return err
	}
	return nil
}

func UnsetOOR(executor cli.GenericExecutor, appDef config.AppDef) error {
	root := appDef.AppRoot
	path := fmt.Sprintf("%s/%s", root, oorFilename)
	err := cli.RemoveFile(executor, path)
	if err != nil {
		return err
	}
	return nil
}
