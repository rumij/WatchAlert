package test

import (
	"context"
	"os"
	"testing"
	"watchAlert/internal/models"
	"watchAlert/pkg/provider"
)

func TestNewClickHouseClient(t *testing.T) {
	if os.Getenv("WATCHALERT_CLICKHOUSE_TEST_ADDR") == "" {
		t.Skip("set WATCHALERT_CLICKHOUSE_TEST_ADDR for the integration test")
	}
	cli, err := provider.NewClickHouseClient(context.Background(), models.AlertDataSource{
		Auth: models.Auth{
			User: os.Getenv("WATCHALERT_CLICKHOUSE_TEST_USER"),
			Pass: os.Getenv("WATCHALERT_CLICKHOUSE_TEST_PASSWORD"),
		},
		ClickHouseConfig: models.DsClickHouseConfig{
			Addr:    os.Getenv("WATCHALERT_CLICKHOUSE_TEST_ADDR"),
			Timeout: 10,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cli.Check(); err != nil {
		t.Fatal(err)
	}

	_, _, err = cli.Query(provider.LogQueryOptions{ClickHouse: provider.ClickHouse{
		Query: "SELECT * FROM zprod.express WHERE `_time_second_`='2025-05-29 00:00:00';",
	}})
	if err != nil {
		t.Fatal(err)
	}
}
