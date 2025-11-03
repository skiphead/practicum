package entity

import (
	"encoding/json"
	"testing"
)

func TestShortenRequest_JSON(t *testing.T) {
	tests := []struct {
		name    string
		request ShortenRequest
		want    string
	}{
		{
			name:    "Regular URL",
			request: ShortenRequest{URL: "https://example.com"},
			want:    `{"url":"https://example.com"}`,
		},
		{
			name:    "Empty URL",
			request: ShortenRequest{URL: ""},
			want:    `{"url":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Тестируем маршалинг
			got, err := json.Marshal(tt.request)
			if err != nil {
				t.Errorf("Marshal() error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() got = %s, want %s", got, tt.want)
			}

			// Тестируем демаршалинг
			var unmarshalled ShortenRequest
			if err := json.Unmarshal(got, &unmarshalled); err != nil {
				t.Errorf("Unmarshal() error = %v", err)
				return
			}
			if unmarshalled != tt.request {
				t.Errorf("Unmarshal() got = %v, want %v", unmarshalled, tt.request)
			}
		})
	}
}

func TestShortenResponse_JSON(t *testing.T) {
	tests := []struct {
		name     string
		response ShortenResponse
		want     string
	}{
		{
			name:     "Regular result",
			response: ShortenResponse{Result: "abc123"},
			want:     `{"result":"abc123"}`,
		},
		{
			name:     "Empty result",
			response: ShortenResponse{Result: ""},
			want:     `{"result":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Тестируем маршалинг
			got, err := json.Marshal(tt.response)
			if err != nil {
				t.Errorf("Marshal() error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() got = %s, want %s", got, tt.want)
			}

			// Тестируем демаршалинг
			var unmarshalled ShortenResponse
			if err := json.Unmarshal(got, &unmarshalled); err != nil {
				t.Errorf("Unmarshal() error = %v", err)
				return
			}
			if unmarshalled != tt.response {
				t.Errorf("Unmarshal() got = %v, want %v", unmarshalled, tt.response)
			}
		})
	}
}
