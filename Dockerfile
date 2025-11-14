FROM golang:1.24.0 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download 

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o ./gmail2gullak .

FROM alpine:3.14

WORKDIR /app

COPY --from=builder /app/gmail2gullak /usr/local/bin/gmail2gullak

EXPOSE 8999

CMD ["gmail2gullak"]
