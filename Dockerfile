# The base image is already multi-architecture, supporting both amd64 and arm64.
# GO_VERSION should match the toolchain version in go.mod
ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /workspace
COPY . .

# Ensure the build command targets the main package.
ARG LDFLAGS=""
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="$LDFLAGS" -o dd-csi-driver ./cmd/...

# Alpine's latest tag supports multiple architectures.
FROM alpine:latest

# Copy the binary from the builder to the appropriate location
COPY --from=builder /workspace/dd-csi-driver /bin/dd-csi-driver

# Ensure the binary is executable
RUN chmod +x /bin/dd-csi-driver

ENTRYPOINT ["/bin/dd-csi-driver"]
