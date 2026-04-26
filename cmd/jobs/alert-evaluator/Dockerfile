FROM mirror.gcr.io/golang:1.25.0 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY cmd/ ./cmd/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o alert-evaluator ./cmd/jobs/alert-evaluator


FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /app/alert-evaluator /app/alert-evaluator

USER nonroot:nonroot

ENTRYPOINT ["/app/alert-evaluator"]
