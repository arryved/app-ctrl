PACKAGE_NAME = app-control-worker
VERSION      = 0.0.1

.PHONY: app-control-worker-dist deps test coverage package clean

app-control-worker: $(shell find . -type f -name "*.go") deps
	go build .

app-control-worker-dist: $(shell find . -type f -name "*.go") deps
	GOOS=linux GOARCH=amd64 go build .

deps:
	go mod tidy

test: deps
	go test -v -count=1 -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out

package: app-control-worker-dist
	nfpm pkg --packager deb --target build/

clean:
	rm app-control-worker coverage.out build/*

.PHONY: release
release: package
	gcloud artifacts apt upload arryved-apt --location=us-central1 --project=arryved-tools --source=build/$(PACKAGE_NAME)_$(VERSION)_amd64.deb
