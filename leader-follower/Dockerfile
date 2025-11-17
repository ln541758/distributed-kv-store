FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -o server .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]