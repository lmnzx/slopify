package scrips

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lmnzx/slopify/product/internal/repository"
)

type Response struct {
	Products []struct {
		ID                 int     `json:"id"`
		Title              string  `json:"title"`
		Price              float32 `json:"price"`
		Description        string  `json:"description"`
		Category           string  `json:"category"`
		DiscountPercentage float32 `json:"discountPercentage"`
		Stock              int     `json:"stock"`
	} `json:"products"`
	Total int `json:"total"`
	Skip  int `json:"skip"`
	Limit int `json:"limit"`
}

func Seed() {
	resp, err := http.Get("https://dummyjson.com/products?limit=100&select=title,price,description,category,discountPercentage,stock")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	var response Response

	err = json.Unmarshal(body, &response)
	if err != nil {
		panic(err)
	}

	conn, err := pgx.Connect(context.Background(), "postgres://postgres:password@my-postgres.orb.local:5432/slopify?sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	q := repository.New(conn)

	for i, product := range response.Products {
		err := q.CreateProduct(context.Background(), repository.CreateProductParams{
			Name:            product.Title,
			Description:     product.Description,
			Category:        product.Category,
			Price:           product.Price,
			Discount:        product.DiscountPercentage,
			QuantityInStock: int32(product.Stock),
		})
		time.Sleep(time.Second * 1)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("seeding datatbase, entry no: %v\n", i)
	}

}
