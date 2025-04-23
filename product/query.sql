-- name: ListAllProducts :many
SELECT * FROM products;

-- name: ListProductsByCategory :many
SELECT * FROM products
WHERE category = $1;

-- name: CreateProduct :exec
INSERT INTO products (
  id, title, description, category, price, discount, quantity_in_stock
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
);
