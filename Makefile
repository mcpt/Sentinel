.PHONY: build run test clean docker

build:
	go build -o sentinel

run: build
	./sentinel

test:
	go test ./...

clean:
	rm -f sentinel

docker:
	docker build -t sentinel .