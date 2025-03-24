package migration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
)

func TestMigrationManager(t *testing.T) {
	cleanup, err := setupTestES()
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cleanup()

	// Create ES client
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})
	if err != nil {
		t.Fatalf("Failed to create ES client: %v", err)
	}

	t.Run("Test Migration Registration and Execution", func(t *testing.T) {
		mm := NewMigrationManager(client)

		migration1 := createTestMigration("test_index_1", "Create test index 1")
		migration2 := createTestMigration("test_index_2", "Create test index 2")

		mm.Register(migration1)
		mm.Register(migration2)

		if err := mm.RunMigrations(); err != nil {
			t.Fatalf("Failed to run migrations: %v", err)
		}

		for _, index := range []string{"test_index_1", "test_index_2"} {
			exists, err := indexExists(client, index)
			if err != nil {
				t.Fatalf("Failed to check index existence: %v", err)
			}
			if !exists {
				t.Errorf("Expected index %s to exist", index)
			}
		}

		applied, err := mm.GetAppliedMigrations()
		if err != nil {
			t.Fatalf("Failed to get applied migrations: %v", err)
		}

		if !applied[migration1.Version()] {
			t.Error("Expected migration1 to be recorded as applied")
		}
		if !applied[migration2.Version()] {
			t.Error("Expected migration2 to be recorded as applied")
		}
	})

	t.Run("Test Migration Idempotency", func(t *testing.T) {
		mm := NewMigrationManager(client)

		migration := createTestMigration("test_index_3", "Create test index 3")
		mm.Register(migration)

		if err := mm.RunMigrations(); err != nil {
			t.Fatalf("Failed to run migrations first time: %v", err)
		}
		if err := mm.RunMigrations(); err != nil {
			t.Fatalf("Failed to run migrations second time: %v", err)
		}

		exists, err := indexExists(client, "test_index_3")
		if err != nil {
			t.Fatalf("Failed to check index existence: %v", err)
		}
		if !exists {
			t.Error("Expected test_index_3 to exist")
		}

		// Verify only one migration record exists
		query := `{
			"query": {
				"match": {
					"version": "` + migration.Version() + `"
				}
			}
		}`
		res, err := client.Search(
			client.Search.WithIndex(migrationsIndex),
			client.Search.WithBody(strings.NewReader(query)),
		)
		if err != nil {
			t.Fatalf("Failed to search migration records: %v", err)
		}
		defer res.Body.Close()

		var result struct {
			Hits struct {
				Total struct {
					Value int `json:"value"`
				} `json:"total"`
			} `json:"hits"`
		}
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode search response: %v", err)
		}

		if result.Hits.Total.Value != 1 {
			t.Errorf("Expected 1 migration record, got %d", result.Hits.Total.Value)
		}
	})

	t.Run("Test Version Generation", func(t *testing.T) {
		// Create two migrations with same description but different functions
		migration1 := NewMigration(
			"Test migration",
			func(client *elasticsearch.Client) error { return nil },
		)
		migration2 := NewMigration(
			"Test migration",
			func(client *elasticsearch.Client) error { return nil },
		)

		// Versions should be different due to different function names
		if migration1.Version() == migration2.Version() {
			t.Error("Expected different versions for different functions")
		}

		// Create two migrations with same function but different descriptions
		f := func(client *elasticsearch.Client) error { return nil }
		migration3 := NewMigration("Description 1", f)
		migration4 := NewMigration("Description 2", f)

		// Versions should be different due to different descriptions
		if migration3.Version() == migration4.Version() {
			t.Error("Expected different versions for different descriptions")
		}
	})
}
