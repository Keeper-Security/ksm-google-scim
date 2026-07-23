FROM golang:1.26-bookworm

# Trust corporate CA for HTTPS during build (uncomment for Zscaler/local proxy)
# COPY certs/certificate-root-ca.crt /usr/local/share/ca-certificates/certificate-root-ca.crt
# RUN update-ca-certificates

RUN apt-get update && apt-get install -y --no-install-recommends \
        git \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install honnef.co/go/tools/cmd/staticcheck@latest
