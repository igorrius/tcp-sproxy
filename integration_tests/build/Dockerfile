# build stage
FROM golang:1.24-alpine as builder
WORKDIR /app
COPY . .
RUN go build -o /app/cmd/server/server /app/cmd/server/main.go
RUN go build -o /app/cmd/client/client /app/cmd/client/main.go

# final stage
FROM alpine:latest
RUN apk add --no-cache netcat-openbsd
WORKDIR /app
COPY --from=builder /app/cmd/server/server /app/server
COPY --from=builder /app/cmd/client/client /app/client
COPY . .
EXPOSE 8080
EXPOSE 8081
