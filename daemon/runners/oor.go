package runners

import (
	"fmt"
	"os"

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

func SetOOR(appDef config.AppDef) error {
	root := appDef.AppRoot
	path := fmt.Sprintf("%s/%s", root, oorFilename)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return nil
}

func UnsetOOR(appDef config.AppDef) error {
	root := appDef.AppRoot
	path := fmt.Sprintf("%s/%s", root, oorFilename)
	err := os.Remove(path)
	if err != nil {
		return err
	}

	return nil
}
