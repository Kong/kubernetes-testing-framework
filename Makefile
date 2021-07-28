GOOS ?= "linux"
GOARCH ?= "amd64"

all: build

.PHONY: clean
clean:
	rm -rf build/

.PHONY: build
build:
	mkdir -p build/
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o build/ktf.$(GOOS).$(GOARCH) internal/cmd/main.go

.PHONY: lint
lint:
	@golangci-lint run ./...

.PHONY: test
test: test.unit

.PHONY: test.all
test.all: test.integration

.PHONY: test.unit
test.unit:
	go test -race -v ./pkg/...

.PHONY: test.e2e
test.e2e:
	@GOFLAGS="-tags=e2e_tests" go test -race -v -timeout 15m ./test/e2e/...

.PHONY: test.integration
test.integration:
	@GOFLAGS="-tags=integration_tests" go test -race -v \
		-covermode=atomic -coverprofile=coverage.out ./...
