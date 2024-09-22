VERSION ?= v0.2.5
REGISTRY ?= nickjfree
REPO ?= goose

all: linux windows

linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "-w -s" -o bin/goose cmd/main.go

windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "-w -s" -o bin/goose.exe cmd/main.go

arm32:
	GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-w -s" -o bin/goose cmd/main.go

mipsle:
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags "-w -s" -o bin/goose cmd/main.go

docker-build:
	docker build -t $(REGISTRY)/$(REPO):$(VERSION) .

docker-push:
	docker push  $(REGISTRY)/$(REPO):$(VERSION)
