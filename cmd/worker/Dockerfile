# -------------------------------
# 1) BUILD STAGE (Go compiler)
# -------------------------------
FROM mirror.gcr.io/golang:1.24.0 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY cmd/ ./cmd/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o worker ./cmd/worker


# -------------------------------
# 2) RUNTIME STAGE (Cloud Run)
# -------------------------------
FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /app/worker /app/worker

ENV PORT=8080

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/worker"]
