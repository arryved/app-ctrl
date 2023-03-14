.PHONY: app-control-api-dist deps test coverage package clean

app-control-api: $(shell find . -type f -name "*.go") deps
	go build .

app-control-api-dist: $(shell find . -type f -name "*.go") deps
	GOOS=linux GOARCH=amd64 go build .

deps:
	go mod tidy

test:
	go test -count=1 -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out

package: app-control-api-dist
	nfpm pkg --packager deb --target build/

clean:
	rm app-control-api coverage.out build/*
