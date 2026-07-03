VERSION    ?= 0.0.0-dev
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w \
	-X github.com/vexarnetwork/vexpay/internal/version.Version=$(VERSION) \
	-X github.com/vexarnetwork/vexpay/internal/version.Commit=$(COMMIT) \
	-X github.com/vexarnetwork/vexpay/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: build run test vet tidy docker clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o vexpay ./cmd/vexpay

run:
	go run ./cmd/vexpay

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

docker:
	docker build -f deploy/Dockerfile \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t vexpay/gateway:$(VERSION) .

clean:
	rm -f vexpay vexpay.exe
	rm -rf dist
