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

func createArticlesIndex(client *elasticsearch.Client) error {
	mapping := `{
		"mappings": {
			"properties": {
				"title": { "type": "text" },
				"content": { "type": "text" },
				"author": { "type": "keyword" },
				"created_at": { "type": "date" }
			}
		}
	}`
	res, err := client.Indices.Create("articles", client.Indices.Create.WithBody(strings.NewReader(mapping)))
	if err != nil {
		return fmt.Errorf("error creating articles index: %w", err)
	}
	defer res.Body.Close()
	return nil
}

func addTagsField(client *elasticsearch.Client) error {
	mapping := `{
		"properties": {
			"tags": {
				"type": "keyword"
			}
		}
	}`
	res, err := client.Indices.PutMapping(
		[]string{"articles"},
		strings.NewReader(mapping),
	)
	if err != nil {
		return fmt.Errorf("error updating articles mapping: %w", err)
	}
	defer res.Body.Close()
	return nil
}

func addCategoryField(client *elasticsearch.Client) error {
	mapping := `{
		"properties": {
			"category": {
				"type": "keyword"
			},
			"subcategory": {
				"type": "keyword"
			}
		}
	}`
	res, err := client.Indices.PutMapping(
		[]string{"articles"},
		strings.NewReader(mapping),
	)
	if err != nil {
		return fmt.Errorf("error updating articles mapping: %w", err)
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

	mm := migration.NewMigrationManager(client, "")

	mm.Register(migration.NewMigration(
		"Create articles index",
		createArticlesIndex,
	))

	mm.Register(migration.NewMigration(
		"Add tags field to articles",
		addTagsField,
	))

	mm.Register(migration.NewMigration(
		"Add category fields to articles",
		addCategoryField,
	))

	if err := mm.RunMigrations(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
