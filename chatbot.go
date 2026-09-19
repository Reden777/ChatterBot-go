// Package chatterbot provides a small, trainable conversational bot.
package chatterbot

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
)

const defaultSimilarityThreshold = 0.60

// Response is the answer selected by a Bot.
type Response struct {
	Text       string
	Confidence float64
}

// Option configures a Bot.
type Option func(*Bot) error

// Bot learns response relationships from training conversations and live chat.
// Its methods are safe for concurrent use.
type Bot struct {
	mu                sync.Mutex
	name              string
	store             *fileStore
	responses         map[string]map[string]*learnedResponse
	prompts           map[string]string
	lastInput         string
	lastResponse      string
	lastResponseKnown bool
	threshold         float64
	defaultResponses  []string
}

type learnedResponse struct {
	Text  string `json:"text"`
	Count uint64 `json:"count"`
}

// New constructs a chatbot. By default its knowledge is kept in memory.
func New(name string, options ...Option) (*Bot, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("chatterbot: name cannot be empty")
	}
	bot := &Bot{
		name:             name,
		responses:        make(map[string]map[string]*learnedResponse),
		prompts:          make(map[string]string),
		threshold:        defaultSimilarityThreshold,
		defaultResponses: []string{"I don't know how to respond to that yet."},
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(bot); err != nil {
			return nil, err
		}
	}
	if bot.store != nil {
		if err := bot.store.load(bot); err != nil {
			return nil, err
		}
		bot.removeFallbackKnowledge()
	}
	return bot, nil
}

// WithStorage persists learned conversations to a JSON file. The file is
// loaded when the bot is created and updated after every successful lesson.
func WithStorage(path string) Option {
	return func(bot *Bot) error {
		if strings.TrimSpace(path) == "" {
			return errors.New("chatterbot: storage path cannot be empty")
		}
		bot.store = &fileStore{path: path}
		return nil
	}
}

// WithSimilarityThreshold sets the minimum similarity required for a fuzzy
// prompt match. It must be between 0 and 1, inclusive.
func WithSimilarityThreshold(threshold float64) Option {
	return func(bot *Bot) error {
		if threshold < 0 || threshold > 1 {
			return fmt.Errorf("chatterbot: similarity threshold must be between 0 and 1: %v", threshold)
		}
		bot.threshold = threshold
		return nil
	}
}

// WithDefaultResponses sets the answers used when the bot has no match.
func WithDefaultResponses(responses ...string) Option {
	return func(bot *Bot) error {
		cleaned := make([]string, 0, len(responses))
		for _, response := range responses {
			if response = strings.TrimSpace(response); response != "" {
				cleaned = append(cleaned, response)
			}
		}
		if len(cleaned) == 0 {
			return errors.New("chatterbot: at least one non-empty default response is required")
		}
		bot.defaultResponses = cleaned
		return nil
	}
}

// Name returns the bot's display name.
func (b *Bot) Name() string { return b.name }

// Train learns each item in a conversation as a response to the item before
// it. A conversation must contain at least two non-empty statements.
func (b *Bot) Train(conversation []string) error {
	cleaned, err := cleanConversation(conversation)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := 1; i < len(cleaned); i++ {
		b.learnLocked(cleaned[i-1], cleaned[i])
	}
	return b.saveLocked()
}

// Learn explicitly records response as a valid answer to prompt.
func (b *Bot) Learn(prompt, response string) error {
	prompt = strings.TrimSpace(prompt)
	response = strings.TrimSpace(response)
	if prompt == "" || response == "" {
		return errors.New("chatterbot: prompt and response cannot be empty")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.learnLocked(prompt, response)
	return b.saveLocked()
}

// GetResponse selects an answer and learns from the live conversation.
// Consecutive user messages are learned as a prompt-response pair. When the
// preceding bot response came from existing knowledge, the user's reply is
// learned as a response to it as well. Fallback responses are never learned.
func (b *Bot) GetResponse(input string) (Response, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Response{}, errors.New("chatterbot: input cannot be empty")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.lastInput != "" {
		b.learnLocked(b.lastInput, input)
	}
	if b.lastResponseKnown && normalize(b.lastResponse) != normalize(b.lastInput) {
		b.learnLocked(b.lastResponse, input)
	}
	response, known := b.selectLocked(input)
	b.lastInput = input
	b.lastResponse = response.Text
	b.lastResponseKnown = known
	if err := b.saveLocked(); err != nil {
		return Response{}, err
	}
	return response, nil
}

// ResetConversation forgets only the current chat context. Trained and
// learned response relationships remain available.
func (b *Bot) ResetConversation() {
	b.mu.Lock()
	b.lastInput = ""
	b.lastResponse = ""
	b.lastResponseKnown = false
	b.mu.Unlock()
}

func (b *Bot) learnLocked(prompt, response string) {
	key := normalize(prompt)
	b.prompts[key] = strings.TrimSpace(prompt)
	if b.responses[key] == nil {
		b.responses[key] = make(map[string]*learnedResponse)
	}
	responseKey := normalize(response)
	known := b.responses[key][responseKey]
	if known == nil {
		known = &learnedResponse{Text: strings.TrimSpace(response)}
		b.responses[key][responseKey] = known
	}
	known.Count++
}

// removeFallbackKnowledge migrates databases created by early versions that
// accidentally saved fallback messages as learned answers.
func (b *Bot) removeFallbackKnowledge() {
	defaults := make(map[string]struct{}, len(b.defaultResponses))
	for _, response := range b.defaultResponses {
		defaults[normalize(response)] = struct{}{}
	}
	for prompt, responses := range b.responses {
		for key := range responses {
			if _, fallback := defaults[key]; fallback {
				delete(responses, key)
			}
		}
		if len(responses) == 0 {
			delete(b.responses, prompt)
			delete(b.prompts, prompt)
		}
	}
}

func (b *Bot) selectLocked(input string) (Response, bool) {
	inputKey := normalize(input)
	bestKey := ""
	bestScore := 0.0
	for candidate := range b.responses {
		score := similarity(inputKey, candidate)
		if score > bestScore || score == bestScore && candidate < bestKey {
			bestKey, bestScore = candidate, score
		}
	}
	if bestKey == "" || bestScore < b.threshold {
		return Response{Text: b.defaultResponses[rand.IntN(len(b.defaultResponses))]}, false
	}
	return Response{Text: chooseWeighted(b.responses[bestKey]), Confidence: bestScore}, true
}

func chooseWeighted(options map[string]*learnedResponse) string {
	var total uint64
	for _, option := range options {
		total += option.Count
	}
	choice := rand.Uint64N(total)
	for _, option := range options {
		if choice < option.Count {
			return option.Text
		}
		choice -= option.Count
	}
	panic("chatterbot: unreachable weighted response selection")
}

func cleanConversation(conversation []string) ([]string, error) {
	cleaned := make([]string, len(conversation))
	for i, statement := range conversation {
		cleaned[i] = strings.TrimSpace(statement)
		if cleaned[i] == "" {
			return nil, fmt.Errorf("chatterbot: statement %d cannot be empty", i)
		}
	}
	if len(cleaned) < 2 {
		return nil, errors.New("chatterbot: a training conversation needs at least two statements")
	}
	return cleaned, nil
}

func normalize(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

// similarity returns normalized Levenshtein similarity for UTF-8 text.
func similarity(left, right string) float64 {
	a, b := []rune(left), []rune(right)
	longest := len(a)
	if len(b) > longest {
		longest = len(b)
	}
	if longest == 0 {
		return 1
	}
	previous := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, leftRune := range a {
		current := make([]int, len(b)+1)
		current[0] = i + 1
		for j, rightRune := range b {
			cost := 1
			if leftRune == rightRune {
				cost = 0
			}
			current[j+1] = min(current[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous = current
	}
	return 1 - float64(previous[len(b)])/float64(longest)
}
