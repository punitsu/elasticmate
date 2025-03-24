package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/punitsu/elasticmate/pkg/migration"
)

func createProductsIndex(client *elasticsearch.Client) error {
	mapping := `{
		"mappings": {
			"properties": {
				"name": { "type": "text" },
				"price": { "type": "float" },
				"category": { "type": "keyword" },
				"created_at": { "type": "date" }
			}
		}
	}`
	res, err := client.Indices.Create("products", client.Indices.Create.WithBody(strings.NewReader(mapping)))
	if err != nil {
		return fmt.Errorf("error creating products index: %w", err)
	}
	defer res.Body.Close()
	return nil
}

func createOrdersIndex(client *elasticsearch.Client) error {
	mapping := `{
		"mappings": {
			"properties": {
				"order_id": { "type": "keyword" },
				"user_id": { "type": "keyword" },
				"products": {
					"type": "nested",
					"properties": {
						"product_id": { "type": "keyword" },
						"quantity": { "type": "integer" },
						"price": { "type": "float" }
					}
				},
				"total": { "type": "float" },
				"status": { "type": "keyword" },
				"created_at": { "type": "date" }
			}
		}
	}`
	res, err := client.Indices.Create("orders", client.Indices.Create.WithBody(strings.NewReader(mapping)))
	if err != nil {
		return fmt.Errorf("error creating orders index: %w", err)
	}
	defer res.Body.Close()
	return nil
}

func main() {
	esURL := flag.String("url", "http://localhost:9200", "Elasticsearch URL")
	flag.Parse()

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{*esURL},
	})
	if err != nil {
		log.Fatal(err)
	}

	mm := migration.NewMigrationManager(client)
	
	mm.Register(migration.NewMigration(
		"Create products index",
		createProductsIndex,
	))
	
	mm.Register(migration.NewMigration(
		"Create orders index with nested products",
		createOrdersIndex,
	))

	if err := mm.RunMigrations(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
