# ElasticMate Examples

This directory contains example use cases for ElasticMate.

## Basic Example
[basic/main.go](basic/main.go)
- Simple single index creation
- Basic mapping with text, keyword, and date fields

## Multiple Indices Example
[multiple_indices/main.go](multiple_indices/main.go)
- Creating multiple indices in sequence
- Complex mappings with nested objects
- Demonstrates e-commerce data model (products and orders)

## Update Mapping Example
[update_mapping/main.go](update_mapping/main.go)
- Creating an initial index
- Adding new fields to existing mapping
- Multiple migrations for the same index
- Shows evolution of schema over time

## Running Examples

1. Start Elasticsearch:
```bash
docker-compose up -d
```

2. Run any example:
```bash
cd examples/basic
go run main.go

# Or run with custom ES URL:
go run main.go -url http://custom-es:9200
```
