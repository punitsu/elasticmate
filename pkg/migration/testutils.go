package migration

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

func setupTestES() (func(), error) {
	cmd := exec.Command("docker-compose", "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to start Elasticsearch: %w", err)
	}

	cleanup := func() {
		cmd := exec.Command("docker-compose", "down", "-v")
		cmd.Run()
	}

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to create ES client: %w", err)
	}

	// Retry connection for up to 30 seconds
	for i := 0; i < 30; i++ {
		_, err := client.Info()
		if err == nil {
			return cleanup, nil
		}
		time.Sleep(time.Second)
	}

	cleanup()
	return nil, fmt.Errorf("Elasticsearch failed to start within timeout")
}

func createTestMigration(indexName, description string) Migration {
	return NewMigration(
		description,
		func(client *elasticsearch.Client) error {
			mapping := `{
				"mappings": {
					"properties": {
						"test_field": { "type": "keyword" }
					}
				}
			}`

			res, err := client.Indices.Create(
				indexName,
				client.Indices.Create.WithBody(strings.NewReader(mapping)),
			)
			if err != nil {
				return err
			}
			defer res.Body.Close()

			if res.IsError() {
				return fmt.Errorf("error creating index: %s", res.String())
			}

			return nil
		},
	)
}

func indexExists(client *elasticsearch.Client, index string) (bool, error) {
	res, err := client.Indices.Exists([]string{index})
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}
