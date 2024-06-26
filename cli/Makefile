.PHONY: all deps test build distclean release

VERSION := $(shell sed -n 's/^version = "\(.*\)"/\1/p' pyproject.toml)
WHEEL := dist/app_control-$(VERSION)-py3-none-any.whl
TGZ := dist/app_control-$(VERSION).tar.gz
VENV := venv/pyvenv.cfg


build: $(WHEEL) deps

$(VENV): pyproject.toml
	python3 -m venv venv/
	. venv/bin/activate && pip install --upgrade pip
	. venv/bin/activate && pip install keyring keyrings.google-artifactregistry-auth
	. venv/bin/activate && pip install -e .
	. venv/bin/activate && pip install ".[tests]"

deps: $(VENV)

$(WHEEL): $(shell find src -type f -name "*.py") deps
	. venv/bin/activate && PIP_NO_INPUT=1 python3 -m build

distclean:
	\rm -rf venv/
	\rm -rf dist/
	\mkdir dist/

release: build
	(gcloud config set project arryved-tools && gcloud config set artifacts/repository python3 && . venv/bin/activate && twine upload --skip-existing --repository-url https://oauth2accesstoken@us-central1-python.pkg.dev/arryved-tools/python/ $(WHEEL) $(TGZ))

test: deps
	. venv/bin/activate && flake8 src/
	. venv/bin/activate && pytest test/
