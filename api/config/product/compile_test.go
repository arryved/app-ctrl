package product

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/arryved/app-ctrl/api/config"
)

func TestCompile(t *testing.T) {
	assert := assert.New(t)

	env := "dev"
	clusterId := config.ClusterId{
		App:     "pay",
		Region:  "central",
		Variant: "default",
	}

	t.Run("Compile Successful", func(t *testing.T) {
		dir := "./testdata/valid"
		os.MkdirAll(dir+"/config/env", 0755)
		os.MkdirAll(dir+"/config/region", 0755)
		os.MkdirAll(dir+"/config/variant", 0755)
		defer os.RemoveAll(dir)

		ioutil.WriteFile(dir+"/config/defaults.yaml", []byte("default: config"), 0600)
		ioutil.WriteFile(dir+"/config/env/test.yaml", []byte("env: config"), 0600)
		ioutil.WriteFile(dir+"/config/region/central.yaml", []byte("region: config"), 0600)
		ioutil.WriteFile(dir+"/config/variant/default.yaml", []byte("variant: config"), 0600)

		path, err := Compile(dir, env, clusterId)
		assert.NoError(err)
		assert.Equal(dir+"/config.yaml", path)
	})

	t.Run("Compile Missing Defaults.yaml", func(t *testing.T) {
		dir := "./testdata/missing-defaults"
		os.MkdirAll(dir+"/config/env", 0755)
		os.MkdirAll(dir+"/config/region", 0755)
		os.MkdirAll(dir+"/config/variant", 0755)
		defer os.RemoveAll(dir)

		ioutil.WriteFile(dir+"/config/env/test.yaml", []byte("env: config"), 0600)
		ioutil.WriteFile(dir+"/config/region/central.yaml", []byte("region: config"), 0600)
		ioutil.WriteFile(dir+"/config/variant/default.yaml", []byte("variant: config"), 0600)

		path, err := Compile(dir, env, clusterId)
		assert.Error(err)
		assert.Empty(path)
	})

	t.Run("Compile Missing Env Config", func(t *testing.T) {
		dir := "./testdata/missing-env"
		os.MkdirAll(dir+"/config/env", 0755)
		os.MkdirAll(dir+"/config/region", 0755)
		os.MkdirAll(dir+"/config/variant", 0755)
		defer os.RemoveAll(dir)

		ioutil.WriteFile(dir+"/config/defaults.yaml", []byte("default: config"), 0600)
		ioutil.WriteFile(dir+"/config/region/central.yaml", []byte("region: config"), 0600)
		ioutil.WriteFile(dir+"/config/variant/default.yaml", []byte("variant: config"), 0600)

		path, err := Compile(dir, env, clusterId)
		assert.NoError(err)
		assert.Equal(dir+"/config.yaml", path)
	})

	t.Run("Compile File Write Error", func(t *testing.T) {
		dir := "./invalid/dir"
		os.MkdirAll(dir+"/config/env", 0555)
		defer os.RemoveAll(dir)

		ioutil.WriteFile(dir+"/config/defaults.yaml", []byte("default: config"), 0600)
		ioutil.WriteFile(dir+"/config/env/test.yaml", []byte("env: config"), 0600)
		ioutil.WriteFile(dir+"/config/region/central.yaml", []byte("region: config"), 0600)
		ioutil.WriteFile(dir+"/config/variant/default.yaml", []byte("variant: config"), 0600)

		path, err := Compile(dir, env, clusterId)
		assert.Error(err)
		assert.Empty(path)
	})

	t.Run("Compile Merge Error", func(t *testing.T) {
		dir := "./testdata/merge-error"
		os.MkdirAll(dir+"/config/env", 0755)
		os.MkdirAll(dir+"/config/region", 0755)
		os.MkdirAll(dir+"/config/variant", 0755)
		defer os.RemoveAll(dir)

		ioutil.WriteFile(dir+"/config/defaults.yaml", []byte("bad yaml"), 0600)
		ioutil.WriteFile(dir+"/config/env/test.yaml", []byte("env: config"), 0600)
		ioutil.WriteFile(dir+"/config/region/central.yaml", []byte("region: config"), 0600)
		ioutil.WriteFile(dir+"/config/variant/default.yaml", []byte("variant: config"), 0600)

		path, err := Compile(dir, env, clusterId)
		assert.Error(err)
		assert.Empty(path)
	})

	t.Run("Compile with Empty Config Files Error", func(t *testing.T) {
		dir := "./testdata/empty-config"
		os.MkdirAll(dir+"/config/env", 0755)
		os.MkdirAll(dir+"/config/region", 0755)
		os.MkdirAll(dir+"/config/variant", 0755)
		defer os.RemoveAll(dir)

		ioutil.WriteFile(dir+"/config/defaults.yaml", []byte(""), 0600)
		ioutil.WriteFile(dir+"/config/env/test.yaml", []byte(""), 0600)
		ioutil.WriteFile(dir+"/config/region/central.yaml", []byte(""), 0600)
		ioutil.WriteFile(dir+"/config/variant/default.yaml", []byte(""), 0600)

		path, err := Compile(dir, env, clusterId)
		assert.Error(err)
		assert.Equal("", path)
	})

	t.Run("Compile with Valid But Conflicting Configurations", func(t *testing.T) {
		dir := "./testdata/conflicting-config"
		os.MkdirAll(dir+"/config/env", 0755)
		os.MkdirAll(dir+"/config/region", 0755)
		os.MkdirAll(dir+"/config/variant", 0755)
		defer os.RemoveAll(dir)

		ioutil.WriteFile(dir+"/config/defaults.yaml", []byte("key: value1"), 0600)
		ioutil.WriteFile(dir+"/config/env/test.yaml", []byte("key: value2"), 0600)
		ioutil.WriteFile(dir+"/config/region/central.yaml", []byte("key: value3"), 0600)
		ioutil.WriteFile(dir+"/config/variant/default.yaml", []byte("key: value4"), 0600)

		path, err := Compile(dir, env, clusterId)
		assert.NoError(err)
		assert.Equal(dir+"/config.yaml", path)
	})
}
