-- name: ListAllProducts :many
SELECT * FROM products
ORDER BY name;

-- name: ListProductsByCategory :many
SELECT * FROM products
WHERE category = $1
ORDER BY name;

-- name: CreateProduct :exec
INSERT INTO products (
  name, description, category, price, discount, quantity_in_stock
) VALUES (
  $1, $2, $3, $4, $5, $6
);
