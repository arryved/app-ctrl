PACKAGE_NAME = app-control-api
VERSION      = 2.1.0

.PHONY: app-control-api-dist deps test coverage package clean

app-control-api: $(shell find . -type f -name "*.go") deps
	go build .

app-control-api-dist: $(shell find . -type f -name "*.go") deps
	GOOS=linux GOARCH=amd64 go build .

deps:
	go mod tidy

test:
	go test -v -count=1 -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out

package: app-control-api-dist
	nfpm pkg --packager deb --target build/

clean:
	rm app-control-api coverage.out build/*

.PHONY: release
release: package
	gcloud artifacts apt upload arryved-apt --location=us-central1 --project=arryved-tools --source=build/$(PACKAGE_NAME)_$(VERSION)_amd64.deb
