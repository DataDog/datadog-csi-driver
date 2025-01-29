# The base image is already multi-architecture, supporting both amd64 and arm64.
FROM golang:1.23-alpine AS builder

WORKDIR /workspace
COPY . .

# The CGO_ENABLED=0 environment variable ensures a statically linked binary is produced,
# which is ideal for compatibility across different Linux distributions and architectures.
# Removed GOARCH specification here to allow build parameter control architecture.
RUN CGO_ENABLED=0 GOOS=linux go build -a -o dd-csi-driver cmd/main.go

# Alpine's latest tag supports multiple architectures.
FROM alpine:latest

COPY --from=builder /workspace/dd-csi-driver /dd-csi-driver

ENTRYPOINT ["/dd-csi-driver"]
