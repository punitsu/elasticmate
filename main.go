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

func createUsersIndex(client *elasticsearch.Client) error {
	mapping := `{
		"mappings": {
			"properties": {
				"name": { "type": "text" },
				"email": { "type": "keyword" },
				"created_at": { "type": "date" }
			}
		}
	}`
	res, err := client.Indices.Create("users", client.Indices.Create.WithBody(strings.NewReader(mapping)))
	if err != nil {
		return fmt.Errorf("error creating users index: %w", err)
	}
	defer res.Body.Close()
	return nil
}

func main() {
	esURL := flag.String("url", "http://localhost:9200", "Elasticsearch URL")
	filePath := flag.String("file", "", "Optional path to text file for version management")
	flag.Parse()

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{*esURL},
	})
	if err != nil {
		log.Fatal(err)
	}

	mm := migration.NewMigrationManager(client, *filePath)
	mm.Register(migration.NewMigration(
		"Create users index",
		createUsersIndex,
	))

	if err := mm.RunMigrations(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
