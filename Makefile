.PHONY: test
test: test.unit

.PHONY: test.unit
test.unit:
	go test -race -v ./pkg/...

.PHONY: test.integration
test.integration:
	@GOFLAGS="-tags=integration_tests" go test -race -v \
		-covermode=atomic -coverprofile=coverage.out ./...
