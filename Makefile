BINARY := mtm
MODULE := github.com/loharrahulofficial-crypto/mtm
PREFIX ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(MODULE)/internal/cli.version=$(VERSION)

.PHONY: build install uninstall test vet fmt check smoke demo clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/mtm

# Launch the TUI against a throwaway copy of the demo dataset (73 tasks).
# Your real data is never touched; the sandbox is deleted on exit.
demo: build
	@sh contrib/demo.sh bin/$(BINARY)

# End-to-end smoke run against a throwaway sandbox (temp HOME + XDG roots,
# MTM_SANDBOX=1). Always runs isolated; the sandbox is deleted on exit.
smoke: build
	@sh contrib/smoke.sh bin/$(BINARY)

install: build
	install -Dm755 bin/$(BINARY) $(PREFIX)/bin/$(BINARY)

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

check: fmt vet test

clean:
	rm -rf bin
