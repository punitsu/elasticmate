package migration

import (
	"encoding/json"
	"os"
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
		mm := NewMigrationManager(client, "")

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

		for _, migration := range []Migration{migration1, migration2} {
			if !applied[migration.Version()] {
				t.Errorf("Expected migration %s to be applied", migration.Description)
			}
		}
	})

	t.Run("Test Migration Idempotence", func(t *testing.T) {
		mm := NewMigrationManager(client, "")

		migration := createTestMigration("test_idempotence", "Create test idempotence index")
		mm.Register(migration)

		// Run migrations twice
		if err := mm.RunMigrations(); err != nil {
			t.Fatalf("Failed to run migrations first time: %v", err)
		}

		if err := mm.RunMigrations(); err != nil {
			t.Fatalf("Failed to run migrations second time: %v", err)
		}

		// Check that the index exists
		exists, err := indexExists(client, "test_idempotence")
		if err != nil {
			t.Fatalf("Failed to check index existence: %v", err)
		}
		if !exists {
			t.Errorf("Expected index test_idempotence to exist")
		}
	})

	t.Run("Test Migration Versioning", func(t *testing.T) {
		// Test that migrations with the same function but different descriptions get different versions
		migration1 := NewMigration("Description 1", func(client *elasticsearch.Client) error {
			return nil
		})

		migration2 := NewMigration("Description 2", func(client *elasticsearch.Client) error {
			return nil
		})

		if migration1.Version() == migration2.Version() {
			t.Error("Expected different versions for different descriptions")
		}
	})

	t.Run("Test Migration Manager With Text File", func(t *testing.T) {
		filePath := "test_versions.json"
		defer os.Remove(filePath)

		mm := NewMigrationManager(nil, filePath)

		migration1 := NewMigration("Create test index 1", func(client *elasticsearch.Client) error {
			return nil
		})
		migration2 := NewMigration("Create test index 2", func(client *elasticsearch.Client) error {
			return nil
		})

		mm.Register(migration1)
		mm.Register(migration2)

		if err := mm.RunMigrations(); err != nil {
			t.Fatalf("Failed to run migrations: %v", err)
		}

		applied, err := mm.GetAppliedMigrations()
		if err != nil {
			t.Fatalf("Failed to get applied migrations: %v", err)
		}

		for _, migration := range []Migration{migration1, migration2} {
			if !applied[migration.Version()] {
				t.Errorf("Expected migration %s to be applied", migration.Description)
			}
		}

		// Verify the file exists and contains the correct data
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read version file: %v", err)
		}

		var versions map[string]bool
		if err := json.Unmarshal(fileData, &versions); err != nil {
			t.Fatalf("Failed to parse version file: %v", err)
		}

		for _, migration := range []Migration{migration1, migration2} {
			if !versions[migration.Version()] {
				t.Errorf("Expected migration %s to be in version file", migration.Description)
			}
		}
	})
}
