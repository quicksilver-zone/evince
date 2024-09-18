FROM golang:1.22-alpine3.20 AS build-env

# Set up dependencies
ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev python3  ca-certificates build-base

# Set working directory for the build
WORKDIR /go/src/github.com/ingenuity-build/evince

# Add source files
COPY .. .

# Install minimum necessary dependencies, build binary and remove packages
RUN apk add --no-cache $PACKAGES && make install

# Final image
FROM alpine:3.20

# Install ca-certificates
RUN apk add --update ca-certificates jq bash curl
WORKDIR /root

# Copy over binaries from the build-env
COPY --from=build-env /go/bin/evinced /usr/bin/evinced


