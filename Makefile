run: build
	@./bin/redis-go

build: fmt
	@go build -o bin/redis-go

fmt:
	@fieldalignment -fix ./...
	@go fmt ./...

test: lint
	@go test -v ./...

lint:
	@golangci-lint run
