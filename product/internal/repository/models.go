// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0

package repository

import (
	"github.com/google/uuid"
)

type Product struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Category        string    `json:"category"`
	Price           float32   `json:"price"`
	Discount        float32   `json:"discount"`
	QuantityInStock int32     `json:"quantity_in_stock"`
}
