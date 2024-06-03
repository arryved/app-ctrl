package product

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchema(t *testing.T) {
	assert := assert.New(t)

	exampleYaml := `---
name: my-cool-app
version: 0.0.*
kind: online
runtime: GKE
port: 8080
repo_type: gke
repo_name: arryved-product

app:
  foo: bar
  fiz: !DELETE

files:
  etc/stuff:

env:
  FOO: BAR
`
	appObject, err := ParseYaml([]byte(exampleYaml))
	assert.NoError(err)
	assert.NotNil(appObject)
	assert.Equal("my-cool-app", appObject.Name)
	assert.Equal("0.0.*", appObject.Version)
	assert.Equal(KindOnline, appObject.Kind)
	assert.Equal(RuntimeGKE, appObject.Runtime)
	assert.Equal(8080, *appObject.Port)
	assert.Equal(RepoContainer, appObject.RepoType)
	assert.Equal("arryved-product", appObject.RepoName)
	assert.Equal("bar", appObject.Other["app"].Value.(map[string]Schemaless)["foo"].Value)
	assert.Equal("!DELETE", appObject.Other["app"].Value.(map[string]Schemaless)["fiz"].Value)
}

func TestMerge(t *testing.T) {
	assert := assert.New(t)

	baseYaml := `---
app:
  foo: bar
  fiz: buzz

files:
  etc/stuff:

env:
  FOO: bar
`

	overrideYaml := `---
app:
  foo: baz
  fiz: !DELETE

files:
  etc/stuff:

  env: {}
`
	base, err := ParseYaml([]byte(baseYaml))
	assert.NoError(err)
	override, err := ParseYaml([]byte(overrideYaml))
	assert.NoError(err)

	err = Merge(base, *override)

	assert.NoError(err)
	// app/foo should have been overridden
	assert.Equal("baz", base.Other["app"].Value.(map[string]Schemaless)["foo"].Value)
	// fiz is deleted entirely
	assert.NotContains(base.Other["app"].Value.(map[string]Schemaless), "fiz")
	// empty env map merged into base map should yield original base map key/value
	assert.Equal("bar", base.Other["env"].Value.(map[string]Schemaless)["FOO"].Value)
}

func TestMultiMerge(t *testing.T) {
	assert := assert.New(t)
	defaultYaml := `---
name: my-cool-app
version: 0.0.*
kind: online
runtime: GKE
port: 8080
repo_type: gke
repo_name: arryved-product

app:
  d: default
  e: default
  r: default
  v: default
  deleteme: default

env:
  d: default
  e: default
  r: default
  v: default

files:
  config/d: default
  config/e: default
  config/r: default
  config/v: default
`
	envYaml := `---
app:
  e: env
  r: env
  v: env

env:
  e: env
  r: env
  v: env

files:
  config/e: env
  config/r: env
  config/v: env
`
	regionYaml := `---
app:
  r: region
  v: region

env:
  r: region
  v: region

files:
  config/r: region
  config/v: region
`
	variantYaml := `---
app:
  deleteme: !DELETE
  v: variant

env:
  v: variant

files:
  config/v: variant
`

	merged, err := MultiMerge(defaultYaml, envYaml, regionYaml, variantYaml)

	assert.NoError(err)
	assert.NotNil(merged)
	assert.Equal("default", merged.Other["app"].Value.(map[string]Schemaless)["d"].Value)
	assert.Equal("default", merged.Other["env"].Value.(map[string]Schemaless)["d"].Value)
	assert.Equal("default", merged.Other["files"].Value.(map[string]Schemaless)["config/d"].Value)
	assert.Equal("env", merged.Other["app"].Value.(map[string]Schemaless)["e"].Value)
	assert.Equal("env", merged.Other["env"].Value.(map[string]Schemaless)["e"].Value)
	assert.Equal("env", merged.Other["files"].Value.(map[string]Schemaless)["config/e"].Value)
	assert.Equal("region", merged.Other["app"].Value.(map[string]Schemaless)["r"].Value)
	assert.Equal("region", merged.Other["env"].Value.(map[string]Schemaless)["r"].Value)
	assert.Equal("region", merged.Other["files"].Value.(map[string]Schemaless)["config/r"].Value)
	assert.Equal("variant", merged.Other["app"].Value.(map[string]Schemaless)["v"].Value)
	assert.Equal("variant", merged.Other["env"].Value.(map[string]Schemaless)["v"].Value)
	assert.Equal("variant", merged.Other["files"].Value.(map[string]Schemaless)["config/v"].Value)
	assert.NotContains(merged.Other["app"].Value.(map[string]Schemaless), "deleteme")
}
