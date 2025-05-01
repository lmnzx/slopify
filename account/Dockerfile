FROM cgr.dev/chainguard/go:latest AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY pkg/ ./pkg/
COPY auth/proto/ ./auth/proto/
COPY account/ ./account/
RUN CGO_ENABLED=0 go build -o /app/accountservice ./account/cmd/

FROM cgr.dev/chainguard/static:latest
COPY --from=build /app/accountservice /usr/local/bin/accountservice
COPY account/config/config.yaml /account/config/
CMD ["/usr/local/bin/accountservice"]
