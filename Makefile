
fmt:
	gofmt -s -w .

lint:
	golangci-lint run

build: fmt
	@go build -o bin/redis-go

test:
	go test -v -race
