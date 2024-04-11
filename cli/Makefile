wheel: distclean $(shell find src -type f -name "*.py")
	(PIP_NO_INPUT=1 python3 -m build src)

distclean:
	\rm -f src/dist/*whl

release: wheel
	(gcloud config set project arryved-tools && gcloud config set artifacts/repository python && cd src; twine upload --repository-url https://oauth2accesstoken@us-central1-python.pkg.dev/arryved-tools/python/  dist/*)

test: $(shell find src -type f -name "*.py")
	flake8 src/
	pytest test/
