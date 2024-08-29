package runners

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/storage"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"gopkg.in/yaml.v3"

	apiconfig "github.com/arryved/app-ctrl/api/config"
	productconfig "github.com/arryved/app-ctrl/api/config/product"
	"github.com/arryved/app-ctrl/api/gce"
	secrets "github.com/arryved/app-ctrl/api/secrets"
	"github.com/arryved/app-ctrl/daemon/cli"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

func DeployRunner(cfg *config.Config, secretsClient secrets.SecretManagerClient, cache *model.DeployCache, executor *cli.Executor) {
	for {
		// insert pause to prevent hard busy-wait
		log.Debugf("Deploy runner going to sleep for %d seconds", cfg.DeployIntervalS)
		time.Sleep(time.Duration(cfg.DeployIntervalS) * time.Second)

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
			SetOOR(executor, cfg.AppDefs[app])
		}

		// Run install on combined list of desired packages + versions.
		// NOTE: because these can be batched, a failure in apt install or restart will cause the
		//       whole batch to fail. This is probably not a big deal since it's just one machine,
		//       and it generally can only happen on non-prod envs where there are multiple apps
		//       per machine. If batching is causing problems, reduce cfg.DeployIntervalS and/or
		//       add splay when kicking off multiple app deployments.
		log.Infof("Deploying the latest desired app=version set=%v", aptTargets)
		err := aptInstallAndRestart(cfg, secretsClient, aptTargets, executor)
		log.Infof("Deploy finished; err=%v", err)

		// Unset OOR for all targets (generally safe since the LB won't add the node back if the health check fails)
		for _, target := range aptTargets {
			app, _ := targetComponents(target)
			UnsetOOR(executor, cfg.AppDefs[app])
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

func aptInstallAndRestart(cfg *config.Config, secretsClient secrets.SecretManagerClient, aptTargets []string, executor *cli.Executor) error {
	log.Infof("Installing and restarting apt package for targets=%v", aptTargets)
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

	err = pullAndMergeConfigs(executor, cfg, secretsClient, aptTargets)
	if err != nil {
		msg := fmt.Sprintf("Pull or merge of one or more configs failed err=%v", err)
		log.Errorf(msg)
		return fmt.Errorf(msg)
	}

	err = cli.SystemdReload(executor, aptTargets)
	if err != nil {
		msg := fmt.Sprintf("Systemd reload failed err=%v", err)
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

func pullAndMergeConfigs(executor *cli.Executor, cfg *config.Config, secretsClient secrets.SecretManagerClient, targets []string) error {
	log.Infof("Pulling and merging configs for targets=%v", targets)
	for _, target := range targets {
		// get clusterId from VM metadata
		app, version := targetComponents(target)
		env, clusterId, err := GetAppControlMetadata(app)
		if err != nil {
			msg := fmt.Sprintf("could not get app-control metadata err=%s", err.Error())
			log.Error(msg)
			return fmt.Errorf(msg)
		}
		// get configBall
		configBall, err := getConfigBall(*clusterId, version)
		if err != nil {
			msg := fmt.Sprintf("could not get app-control config ball, err=%s", err.Error())
			log.Error(msg)
			return fmt.Errorf(msg)
		}
		// expand configBall in app root
		targetPath := cfg.AppDefs[app].AppRoot
		err = expandConfigBall(executor, configBall, app, targetPath)
		if err != nil {
			msg := fmt.Sprintf("could not expand app-control config ball err=%s", err.Error())
			log.Error(msg)
			return fmt.Errorf(msg)
		}
		// Fixup permissions (dir)
		err = cli.FixupDirectoryPermissions(executor, targetPath)
		if err != nil {
			msg := fmt.Sprintf("error fixing up directory permissions err=%s", err.Error())
			log.Error(msg)
			return fmt.Errorf(msg)
		}
		// Fixup permissions (files)
		err = cli.FixupFilePermissions(executor, targetPath)
		if err != nil {
			msg := fmt.Sprintf("error fixing up directory permissions err=%s", err.Error())
			log.Error(msg)
			return fmt.Errorf(msg)
		}
		// Fixup permissions (control)
		err = cli.FixupControlPermissions(executor, targetPath)
		if err != nil {
			msg := fmt.Sprintf("error fixing up control permissions err=%s", err.Error())
			log.Error(msg)
			return fmt.Errorf(msg)
		}
		// Fixup permissions (legacy control script)
		err = cli.FixupControlPermissionsLegacy(executor, targetPath)
		if err != nil {
			msg := fmt.Sprintf("error fixing up legacy control permissions err=%s", err.Error())
			log.Error(msg)
			return fmt.Errorf(msg)
		}
		// Fixup ownership
		err = cli.FixupOwnership(executor, targetPath)
		if err != nil {
			msg := fmt.Sprintf("error fixing up ownership err=%s", err.Error())
			log.Error(msg)
			return fmt.Errorf(msg)
		}
		// Merge config
		err = mergeConfig(executor, env, clusterId, targetPath)
		if err != nil {
			msg := fmt.Sprintf("error merging config err=%s", err.Error())
			return fmt.Errorf(msg)
		}
		// Extract files (anything not included in tarball has either inline contents or a secret urn
		err = extractFiles(executor, secretsClient, targetPath)
		if err != nil {
			msg := fmt.Sprintf("error extracting files from config err=%s", err.Error())
			return fmt.Errorf(msg)
		}
	}
	return nil
}

func extractFiles(executor *cli.Executor, secretsClient secrets.SecretManagerClient, targetPath string) error {
	// get projectId for secrets fetching
	projectIdBytes, err := getMetadata("project/project-id")
	if err != nil {
		return fmt.Errorf("metadata fetch failed for project/project-id err=%s", err.Error())
	}
	projectNumber, err := gce.GetProjectNumber(string(projectIdBytes))
	if err != nil {
		return fmt.Errorf("error getting a project number: err=%s", err.Error())
	}
	log.Debugf("project number=%s", projectNumber)

	// read in config yaml
	configFilePath := filepath.Join(targetPath, "config.yaml")
	yamlBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		log.Fatal(err)
	}
	var appConfig productconfig.AppConfig
	err = yaml.Unmarshal(yamlBytes, &appConfig)
	if err != nil {
		return fmt.Errorf("error unmarshaling config: err=%s", err.Error())
	}

	// extract files from config
	tmpDir, err := ioutil.TempDir("", "config_tmpdir_")
	// chmod the dir so arryved can see it
	if err := os.Chmod(tmpDir, 0750); err != nil {
		return fmt.Errorf("failed to set dir permissions: %s", err.Error())
	}
	defer os.RemoveAll(tmpDir)

	filesMap := map[string]productconfig.Schemaless{}
	files, ok := appConfig.Other["files"]
	if ok {
		filesMap = files.Value.(map[string]productconfig.Schemaless)
	}
	log.Debugf("filesMap=%v", filesMap)
	for relativePath, value := range filesMap {
		content := value.Value.(string)
		outPath := filepath.Join(tmpDir, relativePath)
		log.Infof("outPath=%s content=(%s)", outPath, content)
		secretMatcher := regexp.MustCompile(`\$\{urn:arryved:secret:(.+)\}`)
		match := secretMatcher.FindStringSubmatch(content)
		if len(match) > 0 {
			// in this case, try to fetch the secret at the urn and write it to the file
			secretId := match[1]
			secretBytes, err := secrets.SecretRead(context.Background(), secretsClient, projectNumber, secretId)
			if err != nil {
				log.Warnf("failed to fetch secretId=%s, files extract is incomplete, err=%s", secretId, err.Error())
				continue
			}
			err = writeToFile(executor, outPath, targetPath, secretBytes)
			if err != nil {
				log.Warnf("failed to write secretId=%s to file=%s, files extract is incomplete, err=%s", secretId, outPath, err.Error())
				continue
			}
			log.Infof("dumped secret=%s to path=%s", secretId, outPath)
		} else {
			// in this case, just write the inline content to the file
			err := writeToFile(executor, outPath, targetPath, []byte(content))
			if err != nil {
				log.Warnf("failed to write value to file=%s, files extract is incomplete, err=%s", outPath, err.Error())
				continue
			}
			log.Infof("dumped inline value to path=%s", outPath)
		}
	}

	// change group of the tmpDir and file to "arryved"
	chgrp(tmpDir, "arryved")

	// chmod the dir and file so arryved can read
	if err := os.Chmod(tmpDir, 0750); err != nil {
		return fmt.Errorf("failed to set dir permissions: %s", err.Error())
	}
	// have arryved user copy the file to the targetDir using recursive, incl dotfiles
	err = cli.CopyDirRecurse(executor, "arryved", tmpDir+"/.", targetPath)
	if err != nil {
		return fmt.Errorf("error copying tmpDir=%s to targetPath=%s err=%s", tmpDir, targetPath, err.Error())
	}
	log.Infof("extracted all files for targetPath=%s", targetPath)
	return nil
}

func writeToFile(executor cli.GenericExecutor, tmpPath, targetPath string, data []byte) error {
	// Create the directory and all necessary parents
	dir := filepath.Dir(tmpPath)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	// Write data to the file
	err = os.WriteFile(tmpPath, data, 0640)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
	// chgrp file
	chgrp(tmpPath, "arryved")
	return nil
}

func mergeConfig(executor *cli.Executor, env string, clusterId *apiconfig.ClusterId, targetPath string) error {
	region := clusterId.Region
	variant := clusterId.Variant

	defaultYamlPath := filepath.Join(targetPath, ".arryved", "config", "defaults.yaml")
	envYamlPath := filepath.Join(targetPath, ".arryved", "config", "env", env+".yaml")
	regionYamlPath := filepath.Join(targetPath, ".arryved", "config", "region", region+".yaml")
	variantYamlPath := filepath.Join(targetPath, ".arryved", "config", "variant", variant+".yaml")

	defaultYaml := readFileAsString(defaultYamlPath)
	envYaml := readFileAsString(envYamlPath)
	regionYaml := readFileAsString(regionYamlPath)
	variantYaml := readFileAsString(variantYamlPath)

	appConfig, err := productconfig.MultiMerge(defaultYaml, envYaml, regionYaml, variantYaml)
	if err != nil {
		return err
	}

	// Marshal appConfig object to targetPath/config.yaml as yaml
	yamlBytes, err := yaml.Marshal(appConfig)
	if err != nil {
		return err
	}

	// Write yamlBytes to tmpDir/config.yaml
	tmpDir, err := ioutil.TempDir("", "config_tmpdir_")
	defer os.RemoveAll(tmpDir)
	tmpConfigFilePath := filepath.Join(tmpDir, "config.yaml")
	err = ioutil.WriteFile(tmpConfigFilePath, yamlBytes, 0640)
	if err != nil {
		return err
	}

	// change group of the tmpDir and file to "arryved"
	chgrp(tmpDir, "arryved")
	chgrp(tmpConfigFilePath, "arryved")

	// chmod the dir and file
	if err := os.Chmod(tmpDir, 0750); err != nil {
		return fmt.Errorf("failed to set dir permissions: %s", err.Error())
	}
	if err := os.Chmod(tmpConfigFilePath, 0640); err != nil {
		return fmt.Errorf("failed to set file permissions: %s", err.Error())
	}

	// as arryved, copy tmpDir/config.yaml to targetPath/config.yaml
	configFilePath := filepath.Join(targetPath, "config.yaml")
	if err := cli.CopyFileAs(executor, "arryved", tmpConfigFilePath, configFilePath); err != nil {
		return fmt.Errorf("failed to copy config file to app dir: %s", err.Error())
	}
	return nil
}

// chgrp changes the group ownership of the file or directory at the given path to the new group specified by newGroup.
func chgrp(path string, newGroup string) error {
	// Lookup the group by name
	group, err := user.LookupGroup(newGroup)
	if err != nil {
		return fmt.Errorf("failed to lookup group %s: %w", newGroup, err)
	}

	// Convert the group GID from string to int
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		return fmt.Errorf("failed to convert group GID %s to integer: %w", group.Gid, err)
	}

	// Get the file info to retrieve the current owner UID
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	// Get the current UID
	uid := fileInfo.Sys().(*syscall.Stat_t).Uid

	// Change the group ownership of the file or directory
	if err := os.Chown(path, int(uid), int(gid)); err != nil {
		return fmt.Errorf("failed to change group ownership of %s to %s: %w", path, newGroup, err)
	}

	return nil
}

func readFileAsString(path string) string {
	log.Infof("trying to read file at %s as string", path)
	content, err := ioutil.ReadFile(path)
	if err != nil {
		log.Infof("got error reading the file at %s as string err=%s", path, err.Error())
		return ""
	}
	return string(content)
}

func getConfigBall(clusterId apiconfig.ClusterId, version string) ([]byte, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
		return []byte{}, err
	}
	bucketName := "arryved-app-control-config"
	pattern := fmt.Sprintf("config-app=%s,hash=.*,version=%s.tar.gz", clusterId.App, version)
	log.Infof("pattern=%s", pattern)
	defer client.Close()

	// scan matching config bucket objects
	iter := client.Bucket(bucketName).Objects(ctx, nil)
	mostRecent := ""
	var max time.Time
	for {
		attrs, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Errorf("Failed to list objects: %v", err)
			return []byte{}, err
		}

		matched, err := regexp.MatchString(pattern, attrs.Name)
		if err != nil {
			log.Errorf("Unexpected error matching pattern: %v", err)
			return []byte{}, nil
		}

		// tag mostRecent seen matching object by Created
		if matched {
			if mostRecent == "" {
				mostRecent = attrs.Name
				max = attrs.Created
			} else {
				if max.Before(attrs.Created) {
					mostRecent = attrs.Name
					max = attrs.Created
				}
			}
		}
	}

	if mostRecent == "" {
		msg := fmt.Sprintf("No match found for pattern: %s", pattern)
		log.Error(msg)
		return []byte{}, fmt.Errorf(msg)
	}

	// get the contents of the mostRecent matching object
	reader, err := client.Bucket(bucketName).Object(mostRecent).NewReader(ctx)
	if err != nil {
		log.Errorf("could not get configball object reader mostRecent=%s err=%s", mostRecent, err.Error())
		return []byte{}, err
	}
	defer reader.Close()
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Errorf("could not get configball object contents err=%s", err.Error())
		return []byte{}, err
	}
	log.Infof("got configball object contents name=%s %d bytes", mostRecent, len(data))
	return data, nil
}

func expandConfigBall(executor *cli.Executor, configBall []byte, app, targetPath string) error {
	// first, drop the configBall on disk in a tmp dir
	tmpDir, err := ioutil.TempDir("", "config_tmpdir_")
	if err != nil {
		return fmt.Errorf("failed to create temp directory err=%s", err.Error())
	}
	defer os.RemoveAll(tmpDir)

	filePath, err := dropConfigBall(configBall, tmpDir, app)
	if err != nil {
		return fmt.Errorf("failed to drop tarball err=%s", err.Error())
	}

	// get a list of the current config-app=*tar.gz files for removal
	var fileList, toDelete []os.DirEntry
	fileList, err = os.ReadDir(targetPath)
	if err != nil {
		log.Warnf("could not list files in targetPath=%s", targetPath)
		fileList = []os.DirEntry{}
	}
	for _, file := range fileList {
		if strings.Contains(file.Name(), "config-app=") && strings.HasSuffix(file.Name(), ".tar.gz") {
			toDelete = append(toDelete, file)
		}
	}

	// run tar extract as root to expand into the targetPath
	err = cli.ExpandConfigTarAsRoot(executor, filePath, targetPath)
	if err != nil {
		return fmt.Errorf("failed to expand tarball err=%s", err.Error())
	}

	// clean up old config-app= files
	for _, entry := range toDelete {
		path := filepath.Join(targetPath, entry.Name())
		err := cli.RemoveFile(executor, path)
		if err != nil {
			log.Warnf("could not remove directory entry=%s, err=%s", entry.Name(), err.Error())
		}
	}
	return nil
}

func dropConfigBall(configBall []byte, tmpDir, app string) (string, error) {
	// drop configBall in a tempDir for sudo-based expansion
	filePath := fmt.Sprintf("%s/%s-configBall.tar.gz", tmpDir, app)
	tgzFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)

	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %s", err.Error())
	}

	// Ensure the file is closed after writing
	defer tgzFile.Close()

	// Write the configBall bytes to the temporary file
	_, err = tgzFile.Write(configBall)
	if err != nil {
		return "", fmt.Errorf("failed to write to temp file: %s", err.Error())
	}

	// Ensure it's readable by the arryved user
	if err := os.Chmod(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to set file permissions: %s", err.Error())
	}
	if err := os.Chmod(filePath, 0644); err != nil {
		return "", fmt.Errorf("failed to set file permissions: %s", err.Error())
	}

	// Return the full path of the temporary file
	return filePath, nil
}
