FROM --platform=$BUILDPLATFORM golang:1.24 AS builder
LABEL maintainer="<support@picinch.com>"
WORKDIR /picinch
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build app, cross-platform
#  - rebuild all packages (-a)
ARG TARGETOS TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -a -installsuffix cgo -o picinch ./cmd/web

# New container for target platform, with certificates, ffmpeg and timezone data
FROM --platform=$TARGETPLATFORM alpine:3
RUN apk --no-cache add ca-certificates ffmpeg
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary from previous stage
COPY --from=builder /picinch/picinch /app/

CMD ["/app/picinch"]
