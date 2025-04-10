FROM golang:1.24.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY account/ ./account/
RUN go build -o accountservice ./account/cmd/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/accountservice .
EXPOSE 8080
CMD ["./accountservice"]
