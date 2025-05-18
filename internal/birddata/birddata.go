package birddata

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Recording represents a single result from the xeno-canto API.
type Recording struct {
	ID      string `json:"id"`
	Genus   string `json:"gen"`
	Species string `json:"sp"`
	English string `json:"en"`
	File    string `json:"file"`
}

// Response mirrors the response from the xeno-canto API.
type Response struct {
	NumRecordings string      `json:"numRecordings"`
	Recordings    []Recording `json:"recordings"`
}

// Fetch retrieves recent bird recordings near the provided lat/lon using the
// xeno-canto API.
func Fetch(lat, lon float64) (Response, error) {
	url := fmt.Sprintf("https://xeno-canto.org/api/2/recordings?query=lat:%f+lon:%f", lat, lon)
	resp, err := http.Get(url)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("API status: %s", resp.Status)
	}

	var r Response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Response{}, err
	}
	return r, nil
}
