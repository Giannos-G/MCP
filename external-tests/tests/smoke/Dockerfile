FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod tidy
COPY main.go ./
RUN CGO_ENABLED=0 go build -o smoke-test .

FROM alpine:3.19
COPY --from=builder /app/smoke-test /smoke-test
ENTRYPOINT ["/smoke-test"]
