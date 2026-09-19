package chatterbot

import "testing"

func TestMathematicalEvaluationNaturalLanguage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"what is five plus five", "five plus five = 10"},
		{"what is 5 percent of 104.25", "5 percent of 104.25 = 5.2125"},
		{"How much is one thousand two hundred four divided by one hundred?", "one thousand two hundred four divided by one hundred = 12.04"},
		{"calculate twenty-one multiplied by three", "twenty-one multiplied by three = 63"},
		{"2 to the power of 10", "2 to the power of 10 = 1024"},
		{"(2 + 3) * 4", "(2 + 3) * 4 = 20"},
		{"what is negative five", "negative five = -5"},
		{"12 ÷ 3 + 2 × 4", "12 ÷ 3 + 2 × 4 = 12"},
	}

	adapter := NewMathematicalEvaluation()
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			response, ok := adapter.Process(test.input)
			if test.want == "" {
				if ok {
					t.Fatalf("Process(%q) unexpectedly returned %+v", test.input, response)
				}
				return
			}
			if !ok {
				t.Fatalf("Process(%q) could not evaluate the expression", test.input)
			}
			if response.Text != test.want {
				t.Fatalf("response = %q, want %q", response.Text, test.want)
			}
			if response.Confidence != 1 {
				t.Fatalf("confidence = %v, want 1", response.Confidence)
			}
		})
	}
}

func TestMathematicalEvaluationRejectsNonExpressions(t *testing.T) {
	adapter := NewMathematicalEvaluation()
	inputs := []string{
		"I have five cats",
		"what is your favorite song?",
		"five",
		"10 / 0",
		"2 plus",
		"2 + three elephants",
	}
	for _, input := range inputs {
		if response, ok := adapter.Process(input); ok {
			t.Errorf("Process(%q) unexpectedly returned %+v", input, response)
		}
	}
}

func TestBotUsesMathematicalEvaluationBeforeLearnedResponses(t *testing.T) {
	bot, err := New("test", WithMathematicalEvaluation())
	if err != nil {
		t.Fatal(err)
	}
	if err := bot.Learn("what is five plus five", "I am not sure"); err != nil {
		t.Fatal(err)
	}
	response, err := bot.GetResponse("what is five plus five")
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "five plus five = 10" {
		t.Fatalf("response = %q, want mathematical result", response.Text)
	}
}
