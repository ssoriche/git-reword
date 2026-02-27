.PHONY: all build test vet lint clean install

all: vet test build

build:
	go build -o git-reword ./cmd/git-reword

test:
	go test ./...

vet:
	go vet ./...

lint: vet
	staticcheck ./... 2>/dev/null || echo "staticcheck not installed, skipping"

clean:
	rm -f git-reword
	rm -rf dist/

install:
	go install ./cmd/git-reword
