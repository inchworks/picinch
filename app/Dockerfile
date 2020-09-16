FROM golang:1.13 AS builder
LABEL maintainer="Inchworks <rob@inchworks.com>"
WORKDIR /gallery
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build app
#  - rebuild all packages (-a)
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o gallery ./cmd/web

# New container with just the app
FROM scratch

# Certificates. Assume image is up to date. On Alpine could RUN apk --no-cache add ca-certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary from previous stage
COPY --from=builder /gallery/gallery /app/

CMD ["/app/gallery"]
