FROM golang:1.18 as builder

WORKDIR /workspace
# Copy the Go Modules manifests

COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o goose cmd/main.go

FROM alpine:3.16.0

WORKDIR /
COPY --from=builder /workspace/goose .

ENTRYPOINT ["/goose"]