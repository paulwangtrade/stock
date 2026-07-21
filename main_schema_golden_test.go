package main

import (
	"encoding/json"
	"os"
	"testing"

	"go-stock/backend/db"

	"github.com/stretchr/testify/require"
)

type schemaRegistryGolden struct {
	Migrations   []db.MigrationDescriptor `json:"migrations"`
	Requirements []db.SchemaRequirement   `json:"requirements"`
}

func TestApplicationSchemaRegistry_Golden(t *testing.T) {
	raw, err := os.ReadFile("backend/db/testdata/schema_registry_golden.json")
	require.NoError(t, err)

	var expected schemaRegistryGolden
	require.NoError(t, json.Unmarshal(raw, &expected))

	registry, err := applicationMigrationRegistry()
	require.NoError(t, err)
	actual := schemaRegistryGolden{
		Migrations:   registry.Descriptors(),
		Requirements: applicationSchemaRequirements(),
	}
	require.Equal(t, expected, actual)
}
