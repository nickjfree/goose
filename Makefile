VERSION ?= v0.2.2
REGISTRY ?= nickjfree
REPO ?= goose

all: linux windows

linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "-w -s" -o bin/goose cmd/main.go

windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "-w -s" -o bin/goose.exe cmd/main.go

arm32:
	GOOS=linux GOARCH=arm32 go build -ldflags "-w -s" -o bin/goose cmd/main.go

docker-build:
	docker build -t $(REGISTRY)/$(REPO):$(VERSION) .

docker-push:
	docker push  $(REGISTRY)/$(REPO):$(VERSION)
