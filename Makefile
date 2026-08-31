.PHONY: build test vet fmt checkfmt install analyze evolve clean

BIN := bin/navune

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
