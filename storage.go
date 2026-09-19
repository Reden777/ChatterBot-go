package chatterbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type fileStore struct{ path string }

type diskData struct {
	Version   int                                    `json:"version"`
	Responses map[string]map[string]*learnedResponse `json:"responses"`
	Prompts   map[string]string                      `json:"prompts"`
}

func (s *fileStore) load(bot *Bot) error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("chatterbot: read storage: %w", err)
	}
	var saved diskData
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("chatterbot: decode storage: %w", err)
	}
	if saved.Version != 1 {
		return fmt.Errorf("chatterbot: unsupported storage version %d", saved.Version)
	}
	for prompt, responses := range saved.Responses {
		if prompt == "" || len(responses) == 0 {
			return errors.New("chatterbot: storage contains an empty prompt or response set")
		}
		for key, response := range responses {
			if key == "" || response == nil || response.Text == "" || response.Count == 0 {
				return errors.New("chatterbot: storage contains an invalid response")
			}
		}
	}
	if saved.Responses != nil {
		bot.responses = saved.Responses
	}
	if saved.Prompts != nil {
		bot.prompts = saved.Prompts
	}
	return nil
}

func (s *fileStore) save(bot *Bot) error {
	data, err := json.MarshalIndent(diskData{Version: 1, Responses: bot.responses, Prompts: bot.prompts}, "", "  ")
	if err != nil {
		return fmt.Errorf("chatterbot: encode storage: %w", err)
	}
	data = append(data, '\n')
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("chatterbot: create storage directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".chatterbot-*")
	if err != nil {
		return fmt.Errorf("chatterbot: create temporary storage: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(data)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("chatterbot: write storage: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("chatterbot: close storage: %w", closeErr)
	}
	if err := os.Rename(temporaryName, s.path); err != nil {
		return fmt.Errorf("chatterbot: replace storage: %w", err)
	}
	return nil
}

func (b *Bot) saveLocked() error {
	if b.store == nil {
		return nil
	}
	return b.store.save(b)
}
