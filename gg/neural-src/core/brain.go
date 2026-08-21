package core

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"time"
)

func sigmoid(x float64) float64 { return 1.0 / (1.0 + math.Exp(-x)) }
func dSigmoid(y float64) float64 { return y * (1.0 - y) }

type BrainModel struct {
	WIH [][]float64 `json:"wih"` // Weights Input(4) -> Hidden(50)
	WHO []float64   `json:"who"` // Weights Hidden(50) -> Output(1)
	LR  float64     `json:"lr"`
}

func InitEnterpriseAI() *BrainModel {
	rand.Seed(time.Now().UnixNano())
	b := &BrainModel{
		WIH: make([][]float64, 4), // 👁️ SEKARANG ADA 4 INPUT!
		WHO: make([]float64, 50),
		LR:  0.05,
	}
	for i := 0; i < 4; i++ {
		b.WIH[i] = make([]float64, 50)
		for j := 0; j < 50; j++ { b.WIH[i][j] = rand.Float64() - 0.5 }
	}
	for i := 0; i < 50; i++ { b.WHO[i] = rand.Float64() - 0.5 }
	return b
}

func (b *BrainModel) Predict(inputs []float64) (float64, []float64) {
	hidden := make([]float64, 50)
	for j := 0; j < 50; j++ {
		var sum float64
		for i := 0; i < 4; i++ { sum += inputs[i] * b.WIH[i][j] }
		hidden[j] = sigmoid(sum)
	}
	var outSum float64
	for j := 0; j < 50; j++ { outSum += hidden[j] * b.WHO[j] }
	return sigmoid(outSum), hidden
}

func (b *BrainModel) Train(inputs []float64, target float64) {
	out, hidden := b.Predict(inputs)
	outError := target - out
	outDelta := outError * dSigmoid(out)

	hiddenErrors := make([]float64, 50)
	for j := 0; j < 50; j++ { hiddenErrors[j] = outDelta * b.WHO[j] }

	for j := 0; j < 50; j++ { b.WHO[j] += b.LR * outDelta * hidden[j] }
	for i := 0; i < 4; i++ {
		for j := 0; j < 50; j++ { b.WIH[i][j] += b.LR * hiddenErrors[j] * dSigmoid(hidden[j]) * inputs[i] }
	}
}

func (b *BrainModel) Save(filename string) error {
	data, err := json.Marshal(b)
	if err != nil { return err }
	return os.WriteFile(filename, data, 0644)
}

func LoadBrain(filename string) (*BrainModel, error) {
	data, err := os.ReadFile(filename)
	if err != nil { return nil, err }
	var b BrainModel
	err = json.Unmarshal(data, &b)
	return &b, err
}
