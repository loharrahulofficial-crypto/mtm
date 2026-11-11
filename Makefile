BINARY := mtm
MODULE := github.com/loharrahulofficial-crypto/mtm
PREFIX ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X $(MODULE)/internal/cli.version=$(VERSION) -X $(MODULE)/internal/cli.commit=$(COMMIT) -X $(MODULE)/internal/cli.buildDate=$(DATE)

.PHONY: build install uninstall test vet fmt man demo smoke check clean scenes-gallery perf

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/mtm

man: build
	@mkdir -p man
	go run ./cmd/gendocs $(CURDIR)/man

# Launch the TUI against a throwaway copy of the demo dataset.
# Your real data is never touched; the sandbox is deleted on exit.
demo: build
	@sh contrib/demo.sh bin/$(BINARY)

# Measure idle CPU and paint cost under a pty in a throwaway sandbox.
# Runs the four shipping scenarios (120x40; override with PERF_ARGS) and
# prints user+sys CPU time, paints/s, bytes/paint, allocations/paint and
# the top 15 pprof functions. The idle-ambient-5m scenario runs 5 minutes;
# PERF_DURATION shortens it for a quick check.
# Example: make perf            make perf PERF_ARGS="--duration 10 home-ambient-off"
PERF_ARGS ?=
perf: build
	@python3 contrib/perf.py $(PERF_ARGS)

# End-to-end smoke run against a throwaway sandbox (temp HOME + XDG roots,
# MTM_SANDBOX=1). Always runs isolated; the sandbox is deleted on exit.
smoke: build
	@sh contrib/smoke.sh bin/$(BINARY)

install: build man
	install -Dm755 bin/$(BINARY) $(PREFIX)/bin/$(BINARY)
	mkdir -p $(PREFIX)/share/man/man1
	install -Dm644 man/*.1 $(PREFIX)/share/man/man1/
	mkdir -p $(PREFIX)/share/bash-completion/completions
	bin/$(BINARY) completion bash > $(PREFIX)/share/bash-completion/completions/mtm
	mkdir -p $(PREFIX)/share/zsh/site-functions
	bin/$(BINARY) completion zsh > $(PREFIX)/share/zsh/site-functions/_mtm
	mkdir -p $(PREFIX)/share/fish/vendor_completions.d
	bin/$(BINARY) completion fish > $(PREFIX)/share/fish/vendor_completions.d/mtm.fish

test:
	go test ./...

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

check: fmt vet test

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)
	rm -f $(PREFIX)/share/man/man1/mtm.1 $(PREFIX)/share/man/man1/mtm-*.1
	rm -f $(PREFIX)/share/bash-completion/completions/mtm
	rm -f $(PREFIX)/share/zsh/site-functions/_mtm
	rm -f $(PREFIX)/share/fish/vendor_completions.d/mtm.fish

clean:
	rm -rf bin man

# Write one PPM and one PNG per scene x size into $(OUT) (default: a
# sandbox temp dir). Sizes: 24x10 30x16 40x14 64x16 100x30 200x50.
# Never writes outside the sandbox; prints the paths it writes.
OUT := $(shell mktemp -d /tmp/mtm-scenes-XXXXXX)
scenes-gallery: build
	@sh contrib/scenes-gallery.sh $(CURDIR)/bin/mtm "$(OUT)"
