FROM golang:1.16 AS builder
LABEL maintainer="support@picinch.com>"
WORKDIR /picinch
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build app
#  - rebuild all packages (-a)
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o picinch ./cmd/web

# New container with just the app
FROM scratch

# Certificates. Assume image is up to date. On Alpine could RUN apk --no-cache add ca-certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary and templates from previous stage
COPY --from=builder /picinch/picinch /app/
COPY --from=builder /picinch/ui /app/ui/

# Also copy some resources from modules
COPY --from=builder /go/pkg/mod/github.com/inchworks/webparts@v0.7.1/multiforms/web/static/ /app/ui/static/js/imported/
COPY --from=builder /go/pkg/mod/github.com/inchworks/webparts@v0.7.1/users/web/template/ /app/ui/html/pages/

CMD ["/app/picinch"]
