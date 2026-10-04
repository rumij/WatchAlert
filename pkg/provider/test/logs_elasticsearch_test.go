package test

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/sirupsen/logrus"
	"os"
	"testing"
	"watchAlert/internal/models"
	"watchAlert/pkg/provider"
)

func TestNewElasticSearchClient(t *testing.T) {
	client, err := provider.NewElasticSearchClient(context.Background(), models.AlertDataSource{})
	if err != nil {
		logrus.Errorf("client -> %s", err.Error())
		return
	}

	client.Query(provider.LogQueryOptions{})
}

func TestElasticsearch_GetIndexName(t *testing.T) {
	var ess = []provider.Elasticsearch{
		{
			Index: "test.2000.10.23",
		},
		{
			Index: "test.YYYYMMdd",
		},
		{
			Index: "test.YYYY-MM-dd",
		},
		{
			Index: "test.YYYY_MM_dd",
		},
	}
	for _, es := range ess {
		/*
			test.2000.10.23
			test.20250223
			test.2025-02-23
			test.2025_02_23
		*/
		fmt.Println(es.GetIndexName())
	}
}

func TestElasticSearch_Query(t *testing.T) {
	if os.Getenv("WATCHALERT_ELASTICSEARCH_TEST_URL") == "" {
		t.Skip("set WATCHALERT_ELASTICSEARCH_TEST_URL for the integration test")
	}
	client, err := provider.NewElasticSearchClient(context.Background(), models.AlertDataSource{
		HTTP: models.HTTP{
			URL: os.Getenv("WATCHALERT_ELASTICSEARCH_TEST_URL"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	query, _, err := client.Query(provider.LogQueryOptions{ElasticSearch: provider.Elasticsearch{
		Index:     "test-2024-05.20",
		QueryType: "RawJson",
		RawJson:   `{"match_all":{}}`,
	}})
	if err != nil {
		t.Fatal(err)
	}
	json, _ := sonic.Marshal(query.Message)
	fmt.Println("query->", string(json))

}
