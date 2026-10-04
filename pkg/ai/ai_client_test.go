package ai

import (
	"context"
	"os"
	"testing"
	"watchAlert/internal/models"
)

func TestOpenAiChatCompletion(t *testing.T) {
	if os.Getenv("WATCHALERT_AI_TEST_URL") == "" || os.Getenv("WATCHALERT_AI_TEST_KEY") == "" {
		t.Skip("set WATCHALERT_AI_TEST_URL and WATCHALERT_AI_TEST_KEY for the integration test")
	}

	c := &models.AiConfig{
		//Type:      "openai",
		Url:       os.Getenv("WATCHALERT_AI_TEST_URL"),
		AppKey:    os.Getenv("WATCHALERT_AI_TEST_KEY"),
		Model:     "gpt-4o-mini",
		MaxTokens: 2048,
		Timeout:   3000,
	}
	client, err := NewAiClient(c)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.ChatCompletion(context.Background(), "你好，你是谁？")
	if err != nil {
		t.Fatal("completion err", err)
		return
	}

	t.Log("resp:", resp)
}

func TestOpenAiStreamCompletion(t *testing.T) {
	if os.Getenv("WATCHALERT_AI_TEST_URL") == "" || os.Getenv("WATCHALERT_AI_TEST_KEY") == "" {
		t.Skip("set WATCHALERT_AI_TEST_URL and WATCHALERT_AI_TEST_KEY for the integration test")
	}

	c := &models.AiConfig{
		//Type:      "openai",
		Url:       os.Getenv("WATCHALERT_AI_TEST_URL"),
		AppKey:    os.Getenv("WATCHALERT_AI_TEST_KEY"),
		Model:     "gpt-4o-mini",
		MaxTokens: 2048,
		Timeout:   3000,
	}
	client, err := NewAiClient(c)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.StreamCompletion(context.Background(), "你好，你是谁？")
	if err != nil {
		t.Fatal("streamCompletion err", err)
		return
	}

	var received []string
	for part := range resp {
		received = append(received, part)
	}
	t.Log("received:", received)
}
