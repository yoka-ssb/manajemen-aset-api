# Build stage
FROM golang:1.22-alpine as build
WORKDIR /app
COPY . .
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server server.go

RUN ls -la

# Final stage
FROM alpine:latest
WORKDIR /app

COPY --from=build /app/server /app/server

COPY .env /app/.env

RUN ls -la

RUN chmod +x server

EXPOSE 8080

CMD ["./server"]