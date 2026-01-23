# build stage
FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o metrics .

# runtime stage
FROM gcr.io/distroless/static-debian12

WORKDIR /
COPY --from=builder /app/metrics /metrics

EXPOSE 9100
USER 65532:65532

ENTRYPOINT ["/metrics"]
