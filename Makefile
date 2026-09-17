.PHONY: build test vet fmt checkfmt install install-completions install-man analyze evolve clean

BIN := bin/navune
PREFIX ?= /usr/local
BASH_COMPLETION_DIR ?= $(PREFIX)/share/bash-completion/completions
MAN_DIR ?= $(PREFIX)/share/man/man1

build: $(BIN)

$(BIN): go.mod go.sum $(shell find . -name '*.go' -not -path './test/*')
	go build -o $(BIN) ./cmd/navune

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

checkfmt:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi

install:
	go install ./cmd/navune

# Install the bash completion (as an extensionless file, per bash-completion
# convention) and the man page. Override PREFIX/DESTDIR for staging.
install-completions: completions/navune.bash
	install -d "$(DESTDIR)$(BASH_COMPLETION_DIR)"
	install -m 644 completions/navune.bash "$(DESTDIR)$(BASH_COMPLETION_DIR)/navune"

install-man: man/navune.1
	install -d "$(DESTDIR)$(MAN_DIR)"
	install -m 644 man/navune.1 "$(DESTDIR)$(MAN_DIR)/navune.1"

# Analyze Navune's own source tree (self-analysis smoke test).
analyze:
	go run ./cmd/navune analyze .

# Fixture-evolution tests (opt-in, build tag evolution): staged mutations of
# tiny multi-language codebases assert metrics move in controlled directions.
evolve:
	go test -tags evolution -v ./internal/evolution/...

clean:
	go clean ./...
	rm -rf bin
