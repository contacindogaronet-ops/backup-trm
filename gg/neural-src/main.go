package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"neural-src/core"
)

var AI *core.BrainModel
const brainFile = "ENTERPRISE_BRAIN_50NODES.bin"

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

func main() {
	var err error
	AI, err = core.LoadBrain(brainFile)
	if err != nil {
		fmt.Println("[AI-SYS] ⚠️ Memulai dengan Otak Bayi 4-Nodes (Super Radar)...")
		AI = core.InitEnterpriseAI()
		AI.Save(brainFile)
	} else {
		fmt.Println("[AI-SYS] ✅ Otak Kognitif V2 Berhasil Dipulihkan!")
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C { AI.Save(brainFile) }
	}()

	// 💥 API BARU: JALUR EVAKUASI / RESET OTAK
	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("[AI-SYS] ☢️ NUKE DETONATED! Mereset seluruh sinapsis AI ke kondisi awal!")
		AI = core.InitEnterpriseAI() // Buat otak baru
		AI.Save(brainFile)           // Timpa file lama
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "brain_nuked_successfully"}`))
	})

	http.HandleFunc("/evaluate", func(w http.ResponseWriter, r *http.Request) {
		var req PredictRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil { return }

		f := core.ExtractFeatures(req.Target, req.Hits, req.LastActive, req.LastAnchor)
		inputs := []float64{f.IntervalFeature, f.HitFeature, f.AnchorFeature, f.RawIPFeature}
		score, _ := AI.Predict(inputs)
		verdict := "REGULAR"

		if f.AnchorFeature == 1.0 {
			verdict = "ALLOW_VVIP"
			go AI.Train(inputs, 1.0)
		} else if f.RawIPFeature == 1.0 && req.Hits > 10 && f.IntervalFeature < 0.5 {
			verdict = "DROP"
			go AI.Train(inputs, 0.0)
		} else if score > 0.85 {
			verdict = "ALLOW_VVIP"
		} else if score < 0.15 {
			verdict = "DROP"
			go AI.Train(inputs, 0.0)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PredictResponse{ Verdict: verdict, Probability: score * 100 })
	})

	fmt.Println("[AI-SYS] 🚀 NEURAL DEAMON V2 AKTIF! Port IPC 127.0.0.1:8877")
	http.ListenAndServe("127.0.0.1:8877", nil)
}
