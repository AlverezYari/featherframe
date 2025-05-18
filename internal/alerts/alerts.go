package alerts

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/AlverezYari/featherframe/internal/config"
)

// SendBirdAlert sends an SMS about a bird detection using Twilio's REST API.
// The AlertConfig must contain all required fields.
func SendBirdAlert(bird string, cfg *config.AlertConfig) error {
	if cfg == nil {
		return fmt.Errorf("alert config is nil")
	}
	if cfg.ToNumber == "" || cfg.FromNumber == "" || cfg.AccountSID == "" || cfg.AuthToken == "" {
		return fmt.Errorf("alert configuration incomplete")
	}

	msgData := url.Values{}
	msgData.Set("To", cfg.ToNumber)
	msgData.Set("From", cfg.FromNumber)
	msgData.Set("Body", fmt.Sprintf("Bird detected: %s", bird))

	urlStr := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", cfg.AccountSID)
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(msgData.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(cfg.AccountSID, cfg.AuthToken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio error: %s", string(b))
	}
	return nil
}
