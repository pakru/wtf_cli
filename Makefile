.PHONY: build test clean run

build:
	go build -o wtf_cli cmd/wtf_cli/main.go

test:
	go test -v ./...

clean:
	rm -f wtf_cli

run: build
	./wtf_cli
