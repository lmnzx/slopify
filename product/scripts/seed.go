package scrips

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lmnzx/slopify/product/repository"
	"github.com/meilisearch/meilisearch-go"
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

func Seed(queries *repository.Queries, index meilisearch.IndexManager) {
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

	for i, product := range response.Products {
		err := queries.CreateProduct(context.Background(), repository.CreateProductParams{
			ID:              int32(product.ID),
			Title:           product.Title,
			Description:     product.Description,
			Category:        product.Category,
			Price:           product.Price,
			Discount:        product.DiscountPercentage,
			QuantityInStock: int32(product.Stock),
		})
		if err != nil {
			fmt.Println(err)
			continue
		}
		_, err = index.AddDocuments(product)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		time.Sleep(time.Second * 1)
		fmt.Printf("seeding datatbase, entry no: %v\n", i)
	}
}
