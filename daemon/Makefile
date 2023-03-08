.PHONY: container test coverage clean

app-controld: $(shell find . -type f -name "*.go")
	go build .

test:
	go test -count=1 -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out

clean:
	rm app-controld coverage.out
