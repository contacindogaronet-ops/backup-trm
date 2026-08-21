package logic

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"nganjuk-engine-reborn/config"
)

type PredictRequest struct {
	Target     string    `json:"target"`
	Hits       int       `json:"hits"`
	LastActive time.Time `json:"last_active"`
	LastAnchor time.Time `json:"last_anchor"`
}
type PredictResponse struct {
	Verdict     string  `json:"verdict"`
	Probability float64 `json:"probability_percent"`
}

var httpClient = &http.Client{Timeout: 300 * time.Millisecond}

func AskAI(target string, hits int, lastActive, lastAnchor time.Time) string {
	reqBody, err := json.Marshal(PredictRequest{Target: target, Hits: hits, LastActive: lastActive, LastAnchor: lastAnchor})
	if err != nil { return "REGULAR" }

	resp, err := httpClient.Post(config.AIPort, "application/json", bytes.NewBuffer(reqBody))
	if err == nil {
		defer resp.Body.Close()
		var reply PredictResponse
		if json.NewDecoder(resp.Body).Decode(&reply) == nil {
			if reply.Verdict == "DROP" { return "DROP" } else if reply.Verdict == "ALLOW_VVIP" { return "ALLOW_VVIP" }
		}
	}
	return "REGULAR"
}
