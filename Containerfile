FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY . /app

RUN go build -o test-backend main.go

EXPOSE 8080

FROM registry.access.redhat.com/ubi9/ubi:latest

WORKDIR /app

COPY --from=builder /app/test-backend /app/test-backend

USER 1001

ENTRYPOINT ["./test-backend"]
