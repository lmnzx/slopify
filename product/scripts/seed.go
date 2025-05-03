package scrips

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lmnzx/slopify/pkg/middleware"
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
	log := middleware.GetLogger()

	ctx, cancle := context.WithTimeout(context.Background(), time.Second*5)
	defer cancle()

	isDataPresent, err := queries.ListAllProducts(ctx)
	if err != nil {
		panic(err)
	}

	if len(isDataPresent) > 0 {
		log.Info().Msg("some data is present in the database so skipping seeding...")
		return
	}

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

	for _, product := range response.Products {
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
		time.Sleep(time.Millisecond * 100)
	}
	log.Info().Msg("database is seeded and ready to use")
}
