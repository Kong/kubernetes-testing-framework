all: build

.PHONY: clean
clean:
	rm -f ktf

.PHONY: build
build:
	go build -o ktf main.go

.PHONY: test
test: test.unit

.PHONY: test.unit
test.unit:
	go test -race -v ./pkg/...

.PHONY: test.integration
test.integration:
	@GOFLAGS="-tags=integration_tests" go test -race -v ./test/integration/...
