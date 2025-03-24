# ElasticMate

A CLI tool for managing Elasticsearch schema migrations.

## Usage

```bash
elasticmate [flags]

Flags:
  -url string    Elasticsearch URL (default "http://localhost:9200")
```

## Features

- Automatic version generation based on migration content
- Tracks migrations in a dedicated Elasticsearch index (`.elasticmate_migrations`)
- Automatically runs migrations in version order
- Skips already applied migrations
- Stores migration history with timestamps and function names

## Example

1. Create your migration function:

```go
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

    res, err := client.Indices.Create(
        "users",
        client.Indices.Create.WithBody(strings.NewReader(mapping)),
    )
    if err != nil {
        return fmt.Errorf("error creating users index: %w", err)
    }
    defer res.Body.Close()

    return nil
}
```

2. Register the migration (version is computed automatically):

```go
mm.Register(migration.NewMigration(
    "Create users index",
    createUsersIndex,
))
```

3. Run the migrations:

```bash
$ elasticmate -url http://localhost:9200
```

The tool will:
- Connect to your Elasticsearch instance
- Create a `.elasticmate_migrations` index if it doesn't exist
- Generate unique versions based on migration content
- Run any pending migrations in order
- Store migration history with timestamps and function names

## How Versioning Works

The version for each migration is automatically computed using:
1. The function name of the migration
2. The description text
3. These are combined and hashed (SHA-256), with the first 8 characters used as the version

This means:
- No need to manually specify versions
- If you change the migration function or description, it gets a new version
- Consistent versions across different runs
- Easy to track in version control

## Development and Testing

### Prerequisites

- Docker and Docker Compose
- Go 1.21 or later

### Running Tests

1. Start the test Elasticsearch instance:
```bash
docker-compose up -d
```

2. Run the tests:
```bash
go test ./... -v
```

3. Clean up:
```bash
docker-compose down -v
```

The test suite includes:
- Migration registration and execution
- Idempotency checks (migrations run only once)
- Version generation verification
- Error handling

### Test Environment

The test suite uses:
- Elasticsearch 8.12.1 in Docker
- Automated test setup and teardown
- Isolated test indices
- Comprehensive test cases for all major features



