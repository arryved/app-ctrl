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

pipx install pipenv
pipenv run pip install keyring keyrings.google-artifactregistry-auth
```

### Development

```console
cd app-control # if not in the directory already

pipenv install # install dependencies from Pipfile; `app-control` is configured as an editable dependency
pipenv shell # set up & activate virtual environment
app-control [...] # run app-control from current (i.e. non-built) code
```

## Build & Deploy

Make sure you increment the version number as necessary in `setup.py`!

```console
make test # run tests & linter
make release # build the wheel and push it up to the Artifact Repository
```
