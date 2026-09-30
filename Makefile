BINARY  := pandoc-tui
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build run install test vet fmt check tidy clean preview

## build: compile the binary into bin/
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

## run: build and start the TUI
run:
	go run .

## install: put the binary on $GOBIN
install:
	go install -ldflags "$(LDFLAGS)" .

## test: run the test suite (pandoc-dependent tests skip when it is missing)
test:
	go test ./...

## vet: run the standard analysers
vet:
	go vet ./...

## fmt: rewrite files with gofmt
fmt:
	gofmt -w .

## check: everything CI runs
check:
	go vet ./...
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	go test ./...

## tidy: refresh go.mod and go.sum
tidy:
	go mod tidy

## preview: print every screen as plain text, handy without a terminal to hand
preview:
	PANDOC_TUI_PREVIEW=1 go test ./internal/ui -run TestPreview -v

## clean: remove build output
clean:
	rm -rf bin

## help: list the targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
