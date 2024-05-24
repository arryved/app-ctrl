# app-control

A CLI for the app-control service, which lets SRE, developers & oncall control deployed apps at Arryved.

## Using `app-control`

See [internal docs](https://docs.arryved.com/posts/app-control-frmu3ako).

## Developing

### Setup

Needs double checking, but roughly:

```console
# assumes pip is aliased to pip3 and keyring + keyrings.google-artifactregistry-auth is already installed; those steps should/will be in Slab
pip config set --user global.extra-index-url https://oauth2accesstoken@us-central1-python.pkg.dev/arryved-tools/python/simple/
pip config set --user global.keyring-provider subprocess

# verify that `cat ~/.config/pip/pip.conf` looks like:
# [global]
# extra-index-url = https://oauth2accesstoken@us-central1-python.pkg.dev/arryved-tools/python/simple/
# keyring-provider = subprocess
```

### Local Dev

```console
cd app-control # if not in the directory already

make deps
source venv/bin/activate
```

### Test
```bash
make test
```

### Build
```bash
make build
```

### Release
Make sure you increment semver in pyproject.toml, commit + push, then:
```bash
make release
```
