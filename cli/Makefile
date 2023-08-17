wheel: $(shell find src -type f -name "*.py")
	(cd src; PIP_NO_INPUT=1 python -m build --wheel)

release: wheel
	(gcloud config set project arryved-tools && gcloud config set artifacts/repository python && cd src; twine upload --repository-url https://us-central1-python.pkg.dev/arryved-tools/python/  dist/*)
