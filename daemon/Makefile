PACKAGE_NAME = app-controld
VERSION      = 2.0.3

.PHONY: app-control-dist deploy deps test coverage package clean

app-controld: $(shell find . -type f -name "*.go") deps
	go build .

app-controld-dist: $(shell find . -type f -name "*.go") deps
	GOOS=linux GOARCH=amd64 go build .

deps:
	go mod tidy

test:
	go test -count=1 -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out

package: app-controld-dist deps test
	nfpm pkg --packager deb --target build/

clean:
	rm app-controld coverage.out build/*

release: package
	gcloud artifacts apt upload arryved-apt --location=us-central1 --project=arryved-tools --source=build/$(PACKAGE_NAME)_$(VERSION)_amd64.deb
