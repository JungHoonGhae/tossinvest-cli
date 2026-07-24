BINARY := bin/tossctl
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X github.com/JungHoonGhae/tossinvest-cli/internal/version.Version=$(VERSION) \
	-X github.com/JungHoonGhae/tossinvest-cli/internal/version.Commit=$(COMMIT) \
	-X github.com/JungHoonGhae/tossinvest-cli/internal/version.Date=$(DATE)

.PHONY: build run test lint fmt tidy clean

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/tossctl

run:
	go run -ldflags "$(LDFLAGS)" ./cmd/tossctl

test:
	go test ./...

# lint is gofmt + vet only — no extra tooling to install. `gofmt -l` lists
# unformatted files without changing them, so the check fails loudly instead of
# silently reformatting; run `make fmt` to fix.
lint:
	@unformatted=$$(gofmt -l ./cmd ./internal); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt: 아래 파일이 포맷되지 않았습니다 — \`make fmt\` 를 실행하세요:"; \
		echo "$$unformatted" | sed 's/^/  /'; \
		exit 1; \
	fi
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal

tidy:
	go mod tidy

clean:
	rm -rf bin coverage.out
