APP_NAME=gateway
CMD_PATH=./cmd/gateway

.PHONY: run build test race lint fmt tidy

run:
	go run $(CMD_PATH)

build:
	go build -o bin/$(APP_NAME) $(CMD_PATH)

test:
	go test ./...

race:
	go test -race ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		$$(go env GOPATH)/bin/golangci-lint run; \
	fi

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
	goimports -w $$(find . -name '*.go' -not -path './vendor/*')

tidy:
	go mod tidy
