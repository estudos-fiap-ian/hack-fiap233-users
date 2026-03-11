FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /users .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /users /users
EXPOSE 8080
CMD ["/users"]
