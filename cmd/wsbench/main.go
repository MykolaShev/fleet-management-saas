// Command wsbench is C10's comparative-analysis evidence: it drives the
// same live-position feed through all three connectivity mechanisms for
// a fixed duration each, and prints request count / bytes transferred /
// updates observed / average detection latency per mechanism.
//
// Usage (server must already be running, APP_ENV=local for dev-token):
//
//	go run ./cmd/wsbench -duration 20s
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type vehicleDTO struct {
	ID             string     `json:"id"`
	Status         string     `json:"status"`
	LastLocationAt *time.Time `json:"last_location_at"`
}

type stats struct {
	mechanism string
	requests  int
	bytesIn   int64
	updates   int
	latencies []time.Duration
}

func (s *stats) avgLatency() time.Duration {
	if len(s.latencies) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range s.latencies {
		total += d
	}
	return total / time.Duration(len(s.latencies))
}

func main() {
	baseURL := flag.String("base-url", "http://localhost:8080", "API base URL")
	token := flag.String("token", "", "bearer token; auto-minted via /auth/dev-token if empty")
	duration := flag.Duration("duration", 20*time.Second, "how long to run EACH mechanism")
	flag.Parse()

	if *token == "" {
		t, err := mintDevToken(*baseURL)
		if err != nil {
			log.Fatalf("mint dev token: %v", err)
		}
		*token = t
	}

	if err := ensureMovingVehicle(*baseURL, *token); err != nil {
		log.Fatalf("ensure a moving vehicle exists: %v", err)
	}

	fmt.Printf("Running each mechanism for %s...\n\n", *duration)

	results := []*stats{
		runWebSocket(*baseURL, *token, *duration),
		runLongPoll(*baseURL, *token, *duration),
		runAdaptivePoll(*baseURL, *token, *duration),
	}

	fmt.Println("| Mechanism       | Requests | Bytes received | Updates observed | Avg detection latency |")
	fmt.Println("|-----------------|----------|----------------|------------------|-----------------------|")
	for _, s := range results {
		fmt.Printf("| %-15s | %8d | %15d | %17d | %22s |\n",
			s.mechanism, s.requests, s.bytesIn, s.updates, s.avgLatency().Round(time.Millisecond))
	}
}

// mintDevToken is the same /auth/dev-token shortcut used in the Postman
// demo, so wsbench needs no manual setup to run.
func mintDevToken(baseURL string) (string, error) {
	body, _ := json.Marshal(map[string]string{"role": "manager"})
	resp, err := http.Post(baseURL+"/api/v1/auth/dev-token", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var parsed struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	return parsed.Token, nil
}

// ensureMovingVehicle creates one en_route vehicle if the tenant has none
// yet — otherwise all three mechanisms would correctly report zero
// updates, which proves nothing about their comparative overhead.
func ensureMovingVehicle(baseURL, token string) error {
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/vehicles", strings.NewReader(`{"plate_number":"BENCH-001","model":"wsbench"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return err
	}

	patchReq, _ := http.NewRequest(http.MethodPatch, baseURL+"/api/v1/vehicles/"+created.ID, strings.NewReader(`{"status":"en_route"}`))
	patchReq.Header.Set("Authorization", "Bearer "+token)
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := http.DefaultClient.Do(patchReq)
	if err != nil {
		return err
	}
	defer patchResp.Body.Close()
	return nil
}

func runWebSocket(baseURL, token string, duration time.Duration) *stats {
	s := &stats{mechanism: "websocket"}
	wsURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/api/v1/vehicles/ws?token=" + token

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Printf("websocket dial failed: %v", err)
		return s
	}
	defer conn.Close()
	s.requests = 1 // one connection, not one request per update

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		s.bytesIn += int64(len(raw))

		var pos struct {
			At time.Time `json:"at"`
		}
		if json.Unmarshal(raw, &pos) == nil {
			s.updates++
			s.latencies = append(s.latencies, time.Since(pos.At))
		}
	}
	return s
}

func runLongPoll(baseURL, token string, duration time.Duration) *stats {
	s := &stats{mechanism: "long poll"}
	since := time.Now()
	deadline := time.Now().Add(duration)
	client := &http.Client{Timeout: 30 * time.Second}

	for time.Now().Before(deadline) {
		url := baseURL + "/api/v1/vehicles/poll?since=" + since.Format(time.RFC3339Nano)
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			break
		}
		s.requests++
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		s.bytesIn += int64(len(body))

		if resp.StatusCode == http.StatusOK {
			var parsed struct {
				UpdatedAt time.Time `json:"updated_at"`
			}
			if json.Unmarshal(body, &parsed) == nil {
				s.updates++
				s.latencies = append(s.latencies, time.Since(parsed.UpdatedAt))
				since = parsed.UpdatedAt
			}
		}
	}
	return s
}

const (
	adaptivePollMinInterval = 300 * time.Millisecond
	adaptivePollMaxInterval = 5 * time.Second
	adaptiveBackoffFactor   = 1.5
)

// runAdaptivePoll reuses the plain vehicles-list endpoint — no server
// change needed, the "adaptive" part is entirely client-side: poll fast,
// back off when nothing changes, reset to fast the moment something does.
func runAdaptivePoll(baseURL, token string, duration time.Duration) *stats {
	s := &stats{mechanism: "adaptive poll"}
	interval := adaptivePollMinInterval
	var lastSeen time.Time
	deadline := time.Now().Add(duration)

	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/vehicles?page_size=200", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			break
		}
		s.requests++
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		s.bytesIn += int64(len(body))

		var parsed struct {
			Items []vehicleDTO `json:"items"`
		}
		changed := false
		if json.Unmarshal(body, &parsed) == nil {
			for _, v := range parsed.Items {
				if v.LastLocationAt != nil && v.LastLocationAt.After(lastSeen) {
					lastSeen = *v.LastLocationAt
					changed = true
				}
			}
		}

		if changed {
			s.updates++
			s.latencies = append(s.latencies, time.Since(lastSeen))
			interval = adaptivePollMinInterval
		} else {
			interval = time.Duration(float64(interval) * adaptiveBackoffFactor)
			if interval > adaptivePollMaxInterval {
				interval = adaptivePollMaxInterval
			}
		}

		time.Sleep(interval)
	}
	return s
}