FROM golang:1.26-bookworm AS builder

# Trust corporate CA for HTTPS during build (uncomment for Zscaler/local proxy)
# COPY certs/certificate-root-ca.crt /usr/local/share/ca-certificates/certificate-root-ca.crt
# RUN update-ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM gcr.io/distroless/static-debian12

COPY --from=builder /server /server

ENV PORT=8080
EXPOSE 8080

CMD ["/server"]
