.PHONY: all deps test build distclean release

VERSION := $(shell sed -n 's/^version = "\(.*\)"/\1/p' pyproject.toml)
WHEEL := dist/app_control-$(VERSION)-py3-none-any.whl
TGZ := dist/app_control-$(VERSION).tar.gz
VENV := venv


build: $(WHEEL) test

$(VENV): pyproject.toml
	python -m venv $(VENV)
	pip install keyring keyrings.google-artifactregistry-auth
	pip install -e .
	pip install ".[tests]"

deps: $(VENV)

$(WHEEL): $(shell find src -type f -name "*.py")
	PIP_NO_INPUT=1 python -m build

distclean:
	\rm -rf venv/
	\rm -rf dist/
	\mkdir dist/

release: build
	(gcloud config set project arryved-tools && gcloud config set artifacts/repository python && twine upload --skip-existing --repository-url https://oauth2accesstoken@us-central1-python.pkg.dev/arryved-tools/python/ $(WHEEL) $(TGZ))

test: deps
	flake8 src/
	pytest test/
