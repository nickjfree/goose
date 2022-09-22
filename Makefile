VERSION ?= v0.1.2
REGISTRY ?= nickjfree
REPO ?= goose

all: linux windows

linux:
	GOOS=linux GOARCH=amd64 go build -o bin/goose cmd/main.go

windows:
	GOOS=windows GOARCH=amd64 go build -o bin/goose.exe cmd/main.go

arm32:
	GOOS=linux GOARCH=arm32 go build -o bin/goose cmd/main.go

docker-build:
	docker build -t $(REGISTRY)/$(REPO):$(VERSION) .

docker-push:
	docker push  $(REGISTRY)/$(REPO):$(VERSION)
