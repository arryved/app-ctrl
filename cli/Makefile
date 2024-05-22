DIST_FILES = $(wildcard src/dist/*.whl src/dist/*.tar.gz)

deps: Pipfile
	pipx install pipenv
	pipenv run pip install keyring keyrings.google-artifactregistry-auth

wheel: distclean deps $(shell find src -type f -name "*.py")
	(PIP_NO_INPUT=1 python3 -m build src)

distclean:
	\rm -f $(DIST_FILES)

release: wheel
	(gcloud config set project arryved-tools && gcloud config set artifacts/repository python && cd src; twine upload --skip-existing --repository-url https://oauth2accesstoken@us-central1-python.pkg.dev/arryved-tools/python/  dist/*)

test: $(shell find src -type f -name "*.py")
	flake8 src/
	pytest test/
