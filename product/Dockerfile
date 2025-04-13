FROM golang:1.24.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY product/ ./product/
RUN go build -o productservice ./product/cmd/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/productservice .
EXPOSE 8080
CMD ["./productservice"]
