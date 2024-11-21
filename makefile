
build:
	go build -o bin/main main.go

lint:
	gofmt -d **/*.go

run: build
	bin/main


.PHONY: build lint run
