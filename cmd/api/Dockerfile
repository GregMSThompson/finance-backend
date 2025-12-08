# -------------------------------
# 1) BUILD STAGE (Go compiler)
# -------------------------------
FROM mirror.gcr.io/golang:1.24.0 AS builder

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY cmd/ ./cmd/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app ./cmd/api


# -------------------------------
# 2) RUNTIME STAGE (Cloud Run)
# -------------------------------
FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Copy only the compiled binary
COPY --from=builder /app/app /app/app

# Cloud Run requires listening on $PORT
ENV PORT=8080

EXPOSE 8080

# Run as non-root (Cloud Run best practice)
USER nonroot:nonroot

CMD ["/app/app"]
