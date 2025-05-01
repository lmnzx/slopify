FROM cgr.dev/chainguard/go:latest AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY pkg/ ./pkg/
COPY account/proto/ ./account/proto/
COPY auth/ ./auth/
RUN CGO_ENABLED=0 go build -o /app/authservice ./auth/cmd/

FROM cgr.dev/chainguard/static:latest
COPY --from=build /app/authservice /usr/local/bin/authservice
COPY auth/config/config.yaml /auth/config/
CMD ["/usr/local/bin/authservice"]
