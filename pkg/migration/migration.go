package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
}

func NewMigrationManager(client *elasticsearch.Client) *MigrationManager {
	return &MigrationManager{
		Client:     client,
		Migrations: []Migration{},
	}
}

func (mm *MigrationManager) Register(migration Migration) {
	mm.Migrations = append(mm.Migrations, migration)
}

func (mm *MigrationManager) ensureMigrationsIndex() error {
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

		if res.IsError() {
			return fmt.Errorf("error creating migrations index: %s", res.String())
		}
	}

	return nil
}

func (mm *MigrationManager) GetAppliedMigrations() (map[string]bool, error) {
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
