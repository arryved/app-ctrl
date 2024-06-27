PACKAGE_NAME := app-control-worker
VERSION      := $(ARRYVED_BUILD_APP_VERSION)
VERSION      ?= 0.0.0
PACKAGE      := build/$(PACKAGE_NAME)_$(VERSION)_amd64.deb


.PHONY: app-control-worker-dist build deps test coverage package clean release

app-control-worker: $(shell find . -type f -name "*.go") deps
	go build .

build: $(PACKAGE)

app-control-worker-dist: $(shell find . -type f -name "*.go") deps
	GOOS=linux GOARCH=amd64 go build .

deps:
	go mod tidy

test: deps
	go test -v -count=1 -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out

$(PACKAGE): app-control-worker-dist
	nfpm pkg --packager deb --target build/

clean:
	rm app-control-worker coverage.out build/*

release: build test
	gcloud artifacts apt upload arryved-apt --location=us-central1 --project=arryved-tools --source=$(PACKAGE)
