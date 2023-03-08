.PHONY: container test coverage clean

app-controld: $(shell find . -type f -name "*.go")
	go build .

app-controld-dist: $(shell find . -type f -name "*.go")
	GOOS=linux GOARCH=amd64 go build .

test:
	go test -count=1 -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out

package: app-controld-dist
	nfpm pkg --packager deb --target build/

clean:
	rm app-controld coverage.out build/*
