.PHONY: build test vet fmt checkfmt install analyze clean

build:
	go build ./...

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

clean:
	go clean ./...
