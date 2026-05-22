.PHONY: build run docker-build docker-run fmt

build:
	go build ./...

run:
	cd cmd/server && go run main.go

docker-build:
	docker build -t discord-bot-api:local .

docker-run:
	docker run --rm -p 8080:8080 -e PORT=8080 discord-bot-api:local

fmt:
	gofmt -w .
