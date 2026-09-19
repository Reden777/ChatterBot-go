package chatterbot

// LogicAdapter produces a response for inputs it understands. The boolean is
// false when the adapter cannot process the input.
type LogicAdapter interface {
	Process(input string) (Response, bool)
}

// WithLogicAdapters adds deterministic logic adapters to the bot. Adapters are
// tried in the order supplied, before learned conversation responses.
func WithLogicAdapters(adapters ...LogicAdapter) Option {
	return func(bot *Bot) error {
		for _, adapter := range adapters {
			if adapter != nil {
				bot.adapters = append(bot.adapters, adapter)
			}
		}
		return nil
	}
}

// WithMathematicalEvaluation enables the mathematical evaluation adapter.
func WithMathematicalEvaluation() Option {
	return WithLogicAdapters(NewMathematicalEvaluation())
}
