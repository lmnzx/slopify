proto/gen:
	protoc --go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative ${shell find . -name '*.proto'}

run/migrate:
	migrate -database "postgres://postgres:postgres@localhost:5432/slopify?sslmode=disable&x-migrations-table=account-schema" -path account/migrations up && \
	migrate -database "postgres://postgres:postgres@localhost:5432/slopify?sslmode=disable&x-migrations-table=product-schema" -path product/migrations up
