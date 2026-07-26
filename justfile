default:
	just --list

fmt:
	go fmt ./...

lint:
	golangci-lint run

test:
	go test ./...

build:
	go build ./cmd/sqlkit

check: lint test build
