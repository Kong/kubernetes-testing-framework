GOOS ?= "linux"
GOARCH ?= "amd64"
NCPU ?= $(shell getconf _NPROCESSORS_ONLN)

all: build

.PHONY: clean
clean:
	rm -rf build/

.PHONY: build
build:
	mkdir -p build/
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o build/ktf.$(GOOS).$(GOARCH) cmd/ktf/main.go

.PHONY: install
install:
	go install ./cmd/ktf

.PHONY: lint
lint:
	@golangci-lint run -v ./...

.PHONY: test
test: test.unit

.PHONY: test.all
test.all: test.unit test.integration

.PHONY: test.unit
test.unit:
	go test \
		-race \
		-v \
		-covermode=atomic \
		-coverprofile=unit.coverage.out \
		-coverpkg=$(PKG_LIST) \
		./pkg/...

TEST_RUN ?= ""
PKG_LIST ?= ./pkg/...,./internal/...

.PHONY: test.integration
test.integration:
	@GOFLAGS="-tags=integration_tests" go test \
		-parallel $(NCPU) \
		-timeout 45m \
		-run $(TEST_RUN) \
		-race \
		-v \
		-covermode=atomic \
		-coverprofile=integration.coverage.out \
		-coverpkg=$(PKG_LIST) \
		./test/integration/...

.PHONY: test.e2e
test.e2e:
	@GOFLAGS="-tags=e2e_tests" go test -timeout 45m -race -v ./test/e2e/...
