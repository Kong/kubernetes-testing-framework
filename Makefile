GOOS ?= "linux"
GOARCH ?= "amd64"

all: build

.PHONY: clean
clean:
	rm -f ktf*

.PHONY: build
build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o ktf.$(GOOS).$(GOARCH) cmd/ktf/main.go

.PHONY: test
test: test.unit

.PHONY: test.all
test.all: test.integration

.PHONY: test.unit
test.unit:
	go test -race -v ./pkg/...

.PHONY: test.integration
test.integration:
	@GOFLAGS="-tags=integration_tests" go test -race -v \
		-covermode=atomic -coverprofile=coverage.out ./...
