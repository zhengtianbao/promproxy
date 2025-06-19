.PHONY: build
build:
	go build -mod=vendor -o bin/promproxy cmd/main.go

.PHONY: run
run:
	go run cmd/main.go config.yaml
