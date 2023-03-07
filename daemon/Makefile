.PHONY: container test coverage clean

app-controld: app-controld.go config/config.go api/handlers.go api/api.go
	go build .

test:
	go test -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out

clean:
	rm app-controld coverage.out
