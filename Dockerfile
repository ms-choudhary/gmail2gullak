FROM golang:1.24.0 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download 

COPY . .

RUN go build -o ./gmail2gullak .

FROM alpine:3.14

WORKDIR /app

COPY --from=builder /app/gmail2gullak .

COPY entrypoint.sh /entrypoint.sh

EXPOSE 8999

CMD ["/entrypoint.sh"]
