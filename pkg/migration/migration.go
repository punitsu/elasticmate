package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

const (
	migrationsIndex = ".elasticmate_migrations"
)

type Migration struct {
	Description string
	UpFunc      func(client *elasticsearch.Client) error
	version     string
}

func NewMigration(description string, upFunc func(client *elasticsearch.Client) error) Migration {
	m := Migration{
		Description: description,
		UpFunc:      upFunc,
	}
	m.version = m.computeVersion()
	return m
}

func (m Migration) Version() string {
	return m.version
}

func (m Migration) computeVersion() string {
	funcName := runtime.FuncForPC(reflect.ValueOf(m.UpFunc).Pointer()).Name()

	hasher := sha256.New()
	hasher.Write([]byte(funcName))
	hasher.Write([]byte(m.Description))

	hash := hex.EncodeToString(hasher.Sum(nil))
	return hash[:8]
}

type MigrationRecord struct {
	Version     string    `json:"version"`
	Description string    `json:"description"`
	AppliedAt   time.Time `json:"applied_at"`
	FuncName    string    `json:"func_name"`
}

// MigrationManager handles tracking and applying migrations
type MigrationManager struct {
	Client     *elasticsearch.Client
	Migrations []Migration
	FilePath   string // Optional path to text file for version management
}

func NewMigrationManager(client *elasticsearch.Client, filePath string) *MigrationManager {
	return &MigrationManager{
		Client:     client,
		Migrations: []Migration{},
		FilePath:   filePath,
	}
}

func (mm *MigrationManager) Register(migration Migration) {
	mm.Migrations = append(mm.Migrations, migration)
}

func (mm *MigrationManager) useTextFile() bool {
	return mm.FilePath != ""
}

// readVersionsFromFile reads applied migrations from a text file
func (mm *MigrationManager) readVersionsFromFile() (map[string]bool, error) {
	if !mm.useTextFile() {
		return nil, fmt.Errorf("text file path not provided")
	}

	// Check if file exists
	if _, err := os.Stat(mm.FilePath); os.IsNotExist(err) {
		// File doesn't exist, return empty map
		return make(map[string]bool), nil
	}

	file, err := os.Open(mm.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open version file: %w", err)
	}
	defer file.Close()

	var versions map[string]bool
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&versions); err != nil {
		// If file is empty or invalid JSON, return empty map
		if err.Error() == "EOF" || strings.Contains(err.Error(), "unexpected end of JSON input") {
			return make(map[string]bool), nil
		}
		return nil, fmt.Errorf("failed to decode version file: %w", err)
	}

	// If versions is nil, return empty map
	if versions == nil {
		return make(map[string]bool), nil
	}

	return versions, nil
}

// writeVersionsToFile writes applied migrations to a text file
func (mm *MigrationManager) writeVersionsToFile(versions map[string]bool) error {
	if !mm.useTextFile() {
		return fmt.Errorf("text file path not provided")
	}

	file, err := os.Create(mm.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create version file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(versions); err != nil {
		return fmt.Errorf("failed to encode version file: %w", err)
	}

	return nil
}

func (mm *MigrationManager) ensureMigrationsIndex() error {
	// Skip if using text file
	if mm.useTextFile() {
		return nil
	}

	res, err := mm.Client.Indices.Exists([]string{migrationsIndex})
	if err != nil {
		return fmt.Errorf("error checking migrations index: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		mapping := `{
			"mappings": {
				"properties": {
					"version": { "type": "keyword" },
					"description": { "type": "text" },
					"applied_at": { "type": "date" },
					"func_name": { "type": "keyword" }
				}
			}
		}`

		res, err := mm.Client.Indices.Create(
			migrationsIndex,
			mm.Client.Indices.Create.WithBody(strings.NewReader(mapping)),
		)
		if err != nil {
			return fmt.Errorf("error creating migrations index: %w", err)
		}
		defer res.Body.Close()
	}

	return nil
}

func (mm *MigrationManager) GetAppliedMigrations() (map[string]bool, error) {
	if mm.useTextFile() {
		return mm.readVersionsFromFile()
	}

	applied := make(map[string]bool)

	if err := mm.ensureMigrationsIndex(); err != nil {
		return nil, err
	}

	query := `{"query": {"match_all": {}}}`
	res, err := mm.Client.Search(
		mm.Client.Search.WithIndex(migrationsIndex),
		mm.Client.Search.WithBody(strings.NewReader(query)),
		mm.Client.Search.WithSize(1000),
	)
	if err != nil {
		return nil, fmt.Errorf("error querying migrations: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error querying migrations: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source MigrationRecord `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error parsing migrations: %w", err)
	}

	for _, hit := range result.Hits.Hits {
		applied[hit.Source.Version] = true
	}

	return applied, nil
}

func (mm *MigrationManager) RecordMigration(migration Migration) error {
	if mm.useTextFile() {
		applied, err := mm.readVersionsFromFile()
		if err != nil {
			return err
		}
		applied[migration.Version()] = true
		return mm.writeVersionsToFile(applied)
	}

	record := MigrationRecord{
		Version:     migration.Version(),
		Description: migration.Description,
		AppliedAt:   time.Now(),
		FuncName:    runtime.FuncForPC(reflect.ValueOf(migration.UpFunc).Pointer()).Name(),
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("error marshaling migration record: %w", err)
	}

	res, err := mm.Client.Index(
		migrationsIndex,
		strings.NewReader(string(data)),
		mm.Client.Index.WithRefresh("true"),
	)
	if err != nil {
		return fmt.Errorf("error recording migration: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error recording migration: %s", res.String())
	}

	return nil
}

func (mm *MigrationManager) RunMigrations() error {
	applied, err := mm.GetAppliedMigrations()
	if err != nil {
		return err
	}

	// Sort migrations by version
	sort.Slice(mm.Migrations, func(i, j int) bool {
		return mm.Migrations[i].Version() < mm.Migrations[j].Version()
	})

	// Apply pending migrations
	for _, migration := range mm.Migrations {
		if !applied[migration.Version()] {
			fmt.Printf("Applying migration %s: %s\n", migration.Version(), migration.Description)

			if err := migration.UpFunc(mm.Client); err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", migration.Version(), err)
			}

			if err := mm.RecordMigration(migration); err != nil {
				return err
			}

			fmt.Printf("Migration %s applied successfully\n", migration.Version())
		} else {
			fmt.Printf("Skipping migration %s: already applied\n", migration.Version())
		}
	}

	return nil
}
