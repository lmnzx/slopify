FROM cgr.dev/chainguard/go:latest AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY pkg/ ./pkg/
COPY auth/proto/ ./auth/proto/
COPY product/ ./product/
RUN CGO_ENABLED=0 go build -o /app/productservice ./product/cmd/

FROM cgr.dev/chainguard/static:latest
COPY --from=build /app/productservice /usr/local/bin/productservice
COPY product/config/config.yaml /product/config/
CMD ["/usr/local/bin/productservice"]
