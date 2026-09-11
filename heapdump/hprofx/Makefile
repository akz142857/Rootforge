# hprofx — build a single static binary, for here and for the machines dumps
# actually land on.

BIN     := hprofx
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo 1.0.0)
LDFLAGS := -s -w -X main.version=$(VERSION)
DIST    := dist

# Heap dumps come off Linux servers far more often than off a laptop, so the
# release set covers both, on both architectures.
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

.PHONY: all build install dist clean test

all: build

## build — binary for this machine
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .
	@echo "built ./$(BIN) $(VERSION)"

## install — put it on your PATH
install: build
	install -m 0755 $(BIN) $(HOME)/.local/bin/$(BIN)
	@echo "installed to $(HOME)/.local/bin/$(BIN)"

## dist — cross-compiled binaries for every supported platform
dist: clean
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out=$(DIST)/$(BIN)-$$os-$$arch; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags "$(LDFLAGS)" -o $$out . || exit 1; \
		printf '%-34s %s\n' "$$out" "$$(du -h $$out | cut -f1)"; \
	done

test: build
	go vet ./...
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
	@echo "vet and gofmt clean"

clean:
	rm -rf $(DIST) $(BIN)
