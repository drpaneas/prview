VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X github.com/drpaneas/prview/cmd.version=$(VERSION)"

.PHONY: build test vet fmt lint clean

build:
	go build $(LDFLAGS) -o prview .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

lint:
	golangci-lint run

clean:
	rm -f prview
