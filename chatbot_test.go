package chatterbot

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestTrainAndGetResponse(t *testing.T) {
	bot, err := New("test", WithSimilarityThreshold(0.5))
	if err != nil {
		t.Fatal(err)
	}
	if err := bot.Train([]string{"Hello", "Hi there!", "How are you?", "Doing well."}); err != nil {
		t.Fatal(err)
	}
	response, err := bot.GetResponse("hello!")
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "Hi there!" {
		t.Fatalf("response = %q, want %q", response.Text, "Hi there!")
	}
	if response.Confidence < 0.5 {
		t.Fatalf("confidence = %v, want at least 0.5", response.Confidence)
	}
}

func TestLiveConversationLearnsUserResponse(t *testing.T) {
	bot, err := New("test", WithDefaultResponses("Tell me more."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bot.GetResponse("What language do you like?"); err != nil {
		t.Fatal(err)
	}
	if _, err := bot.GetResponse("I like Go"); err != nil {
		t.Fatal(err)
	}
	bot.ResetConversation()
	response, err := bot.GetResponse("What language do you like?")
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "I like Go" {
		t.Fatalf("response = %q, want learned user response", response.Text)
	}
}

func TestRepeatedInputStopsUsingFallback(t *testing.T) {
	bot, err := New("test", WithDefaultResponses("I don't know yet."))
	if err != nil {
		t.Fatal(err)
	}
	first, err := bot.GetResponse("hi")
	if err != nil {
		t.Fatal(err)
	}
	if first.Text != "I don't know yet." {
		t.Fatalf("first response = %q", first.Text)
	}
	second, err := bot.GetResponse("hi")
	if err != nil {
		t.Fatal(err)
	}
	if second.Text != "hi" {
		t.Fatalf("second response = %q, want live-learned response", second.Text)
	}
}

func TestStorageRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge.json")
	first, err := New("test", WithStorage(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Train([]string{"Good morning", "Good morning to you"}); err != nil {
		t.Fatal(err)
	}
	second, err := New("test", WithStorage(path))
	if err != nil {
		t.Fatal(err)
	}
	response, err := second.GetResponse("Good morning")
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "Good morning to you" {
		t.Fatalf("response = %q after reload", response.Text)
	}
}

func TestInvalidInput(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("New accepted an empty name")
	}
	bot, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	if err := bot.Train([]string{"only one"}); err == nil {
		t.Fatal("Train accepted a one-statement conversation")
	}
	if _, err := bot.GetResponse("  "); err == nil {
		t.Fatal("GetResponse accepted empty input")
	}
}

func TestUnicodeSimilarity(t *testing.T) {
	if score := similarity("cómo estás", "como estas"); score < 0.7 {
		t.Fatalf("similarity = %v, want at least 0.7", score)
	}
}

func TestRejectsInvalidStorage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"responses":{"hello":{"hi":{"text":"hi","count":0}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New("test", WithStorage(path)); err == nil {
		t.Fatal("New accepted a response with a zero observation count")
	}
}

func TestRemovesPreviouslyLearnedFallbacks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.json")
	legacy := `{"version":1,"responses":{"hi":{"i don't know how to respond to that yet.":{"text":"I don't know how to respond to that yet.","count":8}}},"prompts":{"hi":"hi"}}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	bot, err := New("test", WithStorage(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := bot.responses["hi"]; exists {
		t.Fatal("legacy fallback response was not removed")
	}
	if _, err := bot.GetResponse("hi"); err != nil {
		t.Fatal(err)
	}
	response, err := bot.GetResponse("hi")
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "hi" {
		t.Fatalf("response = %q after legacy migration", response.Text)
	}
}

func TestConcurrentLearning(t *testing.T) {
	bot, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := bot.Learn("hello", "hi"); err != nil {
				t.Errorf("Learn: %v", err)
			}
		}()
	}
	wait.Wait()
	if count := bot.responses["hello"]["hi"].Count; count != 20 {
		t.Fatalf("learned count = %d, want 20", count)
	}
}
