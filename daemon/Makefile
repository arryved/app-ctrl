PACKAGE_NAME              := app-controld
ARRYVED_BUILD_APP_VERSION ?= 0.0.0
VERSION                   := $(ARRYVED_BUILD_APP_VERSION)
PACKAGE                   := build/$(PACKAGE_NAME)_$(VERSION)_amd64.deb


.PHONY: $(PACKAGE_NAME)-dist build deps test coverage package clean release

$(PACKAGE_NAME): $(shell find . -type f -name "*.go") deps
	go build .

build: $(PACKAGE)

$(PACKAGE_NAME)-dist: $(shell find . -type f -name "*.go") deps
	GOOS=linux GOARCH=amd64 go build .

deps:
	go mod tidy

test: deps
	  go test -v -count=1 -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out

$(PACKAGE): $(PACKAGE_NAME)-dist
	nfpm pkg --packager deb --target build/

clean:
	\rm -f $(PACKAGE_NAME) coverage.out build/*

release: build
	gcloud artifacts apt upload arryved-apt --location=us-central1 --project=arryved-tools --source=$(PACKAGE)
