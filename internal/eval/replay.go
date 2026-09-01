package eval

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

type LabeledSignal struct {
	Signal model.Signal `json:"signal"`
	Group  string       `json:"group"`
}

type Corpus struct {
	Name    string          `json:"name"`
	Signals []LabeledSignal `json:"signals"`
}

type Report struct {
	Corpus            string  `json:"corpus"`
	Signals           int     `json:"signals"`
	ExpectedPairs     int     `json:"expected_pairs"`
	PredictedPairs    int     `json:"predicted_pairs"`
	TruePositivePairs int     `json:"true_positive_pairs"`
	Precision         float64 `json:"precision"`
	Recall            float64 `json:"recall"`
}

func Load(reader io.Reader) (Corpus, error) {
	var corpus Corpus
	if err := json.NewDecoder(reader).Decode(&corpus); err != nil {
		return Corpus{}, err
	}
	if len(corpus.Signals) == 0 {
		return Corpus{}, fmt.Errorf("replay corpus has no signals")
	}
	return corpus, nil
}

// Evaluate measures pairwise cluster precision/recall against a small labeled
// corpus. Pairwise scoring is intentionally simple and deterministic so future
// clustering changes can be compared against the same benchmark without
// requiring a model or an online service.
func Evaluate(corpus Corpus, threshold float64) Report {
	signals := make([]model.Signal, 0, len(corpus.Signals))
	indexByID := make(map[string]int, len(corpus.Signals))
	for index, labeled := range corpus.Signals {
		signals = append(signals, labeled.Signal)
		indexByID[labeled.Signal.ID] = index
	}

	predicted := make(map[[2]int]struct{})
	for _, cluster := range engine.ClusterSignalsV2(signals, threshold) {
		for left := 0; left < len(cluster.Signals); left++ {
			for right := left + 1; right < len(cluster.Signals); right++ {
				leftIndex, leftOK := indexByID[cluster.Signals[left].ID]
				rightIndex, rightOK := indexByID[cluster.Signals[right].ID]
				if !leftOK || !rightOK || leftIndex == rightIndex {
					continue
				}
				if leftIndex > rightIndex {
					leftIndex, rightIndex = rightIndex, leftIndex
				}
				predicted[[2]int{leftIndex, rightIndex}] = struct{}{}
			}
		}
	}

	expected := make(map[[2]int]struct{})
	for left := 0; left < len(corpus.Signals); left++ {
		for right := left + 1; right < len(corpus.Signals); right++ {
			group := corpus.Signals[left].Group
			if group != "" && group == corpus.Signals[right].Group {
				expected[[2]int{left, right}] = struct{}{}
			}
		}
	}

	truePositives := 0
	for pair := range predicted {
		if _, ok := expected[pair]; ok {
			truePositives++
		}
	}

	precision := 1.0
	if len(predicted) > 0 {
		precision = float64(truePositives) / float64(len(predicted))
	}
	recall := 1.0
	if len(expected) > 0 {
		recall = float64(truePositives) / float64(len(expected))
	}

	return Report{
		Corpus:            corpus.Name,
		Signals:           len(signals),
		ExpectedPairs:     len(expected),
		PredictedPairs:    len(predicted),
		TruePositivePairs: truePositives,
		Precision:         precision,
		Recall:            recall,
	}
}
