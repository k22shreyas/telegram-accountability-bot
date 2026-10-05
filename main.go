package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const stateFile = "state.json"

const defaultTopic = "a Kubernetes/infra project, system design practice (URL Shortener, Web Crawler, Twitter/Newsfeed, Dropbox/Drive), and behavioral question prep"

const promptTemplate = `You're messaging me the way a casual, friendly work lead would — low-key, dumping thoughts, not formal. I'm a software engineer prepping for new-grad job interviews. Right now I'm focused on: %s.

Send me ONE short message, casual tone, like one of these styles (pick a different one than you'd normally default to, vary it):
- A check-in referencing my current focus directly
- A nudge with no pressure
- A thought-dump, like you found something interesting related to my current focus (invent a plausible-sounding blog/resource if needed)
- A light accountability ping: "what are you stuck on right now, if anything?"

Keep it to 1-2 sentences max, lowercase/casual punctuation is fine, no corporate tone, no bullet points, no exclamation-point-heavy enthusiasm. Don't explain that you're an AI simulating a lead — just send the message like it's a real person texting. Output ONLY the message text, nothing else.`

type state struct {
	Topic        string `json:"topic"`
	LastUpdateID int64  `json:"last_update_id"`
}

func loadState() state {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return state{Topic: defaultTopic, LastUpdateID: 0}
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil || s.Topic == "" {
		return state{Topic: defaultTopic, LastUpdateID: 0}
	}
	return s
}

func saveState(s state) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0644)
}

// --- Telegram: check for incoming "new-topic:" messages ---

type tgUpdatesResponse struct {
	Result []struct {
		UpdateID int64 `json:"update_id"`
		Message  struct {
			Text string `json:"text"`
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	} `json:"result"`
}

func checkForNewTopic(botToken string, s *state) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d", botToken, s.LastUpdateID+1)
	resp, err := http.Get(apiURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var ur tgUpdatesResponse
	if err := json.Unmarshal(body, &ur); err != nil {
		return err
	}

	for _, u := range ur.Result {
		if u.UpdateID > s.LastUpdateID {
			s.LastUpdateID = u.UpdateID
		}
		text := strings.TrimSpace(u.Message.Text)
		lower := strings.ToLower(text)
		if strings.HasPrefix(lower, "new-topic:") {
			newTopic := strings.TrimSpace(text[len("new-topic:"):])
			if newTopic != "" {
				s.Topic = newTopic
				fmt.Println("topic updated to:", newTopic)
			}
		}
	}
	return nil
}

// --- Groq: generate the message ---

type groqRequest struct {
	Model    string        `json:"model"`
	Messages []groqMessage `json:"messages"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponse struct {
	Choices []struct {
		Message groqMessage `json:"message"`
	} `json:"choices"`
}

func generateMessage(apiKey, topic string) (string, error) {
	systemPrompt := fmt.Sprintf(promptTemplate, topic)
	reqBody := groqRequest{
		Model: "openai/gpt-oss-20b",
		Messages: []groqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "send the message now"},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("groq api error (%d): %s", resp.StatusCode, string(respBody))
	}

	var gr groqResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return "", err
	}
	if len(gr.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from groq")
	}
	return gr.Choices[0].Message.Content, nil
}

func sendTelegram(botToken, chatID, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	form := url.Values{}
	form.Set("chat_id", chatID)
	form.Set("text", text)

	resp, err := http.PostForm(apiURL, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram api error (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func main() {
	groqKey := os.Getenv("GROQ_API_KEY")
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if groqKey == "" || botToken == "" || chatID == "" {
		fmt.Println("missing one of GROQ_API_KEY, TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID")
		os.Exit(1)
	}

	s := loadState()

	if err := checkForNewTopic(botToken, &s); err != nil {
		fmt.Println("warning: couldn't check for topic updates:", err)
	}
	if err := saveState(s); err != nil {
		fmt.Println("warning: couldn't save state:", err)
	}

	// Fuzz the send time within a random window (0-15 min) so messages
	// don't land at the exact same minute every day, while still staying
	// close to the target time set in the cron schedule.
	rand.Seed(time.Now().UnixNano())
	delay := time.Duration(rand.Intn(15)) * time.Minute
	fmt.Printf("sleeping %v before sending (topic: %s)\n", delay, s.Topic)
	time.Sleep(delay)

	msg, err := generateMessage(groqKey, s.Topic)
	if err != nil {
		fmt.Println("error generating message:", err)
		os.Exit(1)
	}

	if err := sendTelegram(botToken, chatID, msg); err != nil {
		fmt.Println("error sending telegram message:", err)
		os.Exit(1)
	}

	fmt.Println("sent:", msg)
}
