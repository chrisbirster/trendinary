package eval

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

type LabeledSignal struct { Signal model.Signal `json:"signal"`; Group string `json:"group"` }
type Corpus struct { Name string `json:"name"`; Signals []LabeledSignal `json:"signals"` }
type Report struct { Corpus string `json:"corpus"`; Signals int `json:"signals"`; ExpectedPairs int `json:"expected_pairs"`; PredictedPairs int `json:"predicted_pairs"`; TruePositivePairs int `json:"true_positive_pairs"`; Precision float64 `json:"precision"`; Recall float64 `json:"recall"` }

func Load(reader io.Reader)(Corpus,error){var corpus Corpus;if err:=json.NewDecoder(reader).Decode(&corpus);err!=nil{return Corpus{},err};if len(corpus.Signals)==0{return Corpus{},fmt.Errorf("replay corpus has no signals")};return corpus,nil}
func Evaluate(corpus Corpus,threshold float64)Report{signals:=make([]model.Signal,0,len(corpus.Signals));indexByID:=map[string]int{};for i,labeled:=range corpus.Signals{signals=append(signals,labeled.Signal);indexByID[labeled.Signal.ID]=i};clusters:=engine.ClusterSignalsV2(signals,threshold);predicted:=map[[2]int]struct{}{};for _,cluster:=range clusters{for i:=0;i<len(cluster.Signals);i++{for j:=i+1;j<len(cluster.Signals);j++{a,b:=indexByID[cluster.Signals[i].ID],indexByID[cluster.Signals[j].ID];if a>b{a,b=b,a};predicted[[2]int{a,b}]=struct{}{}}};expected:=map[[2]int]struct{}{};for i:=0;i<len(corpus.Signals);i++{for j:=i+1;j<len(corpus.Signals);j++{if corpus.Signals[i].Group!=""&&corpus.Signals[i].Group==corpus.Signals[j].Group{expected[[2]int{i,j}]=struct{}{}}}};tp:=0;for pair:=range predicted{if _,ok:=expected[pair];ok{tp++}};precision:=1.0;if len(predicted)>0{precision=float64(tp)/float64(len(predicted))};recall:=1.0;if len(expected)>0{recall=float64(tp)/float64(len(expected))};return Report{Corpus:corpus.Name,Signals:len(signals),ExpectedPairs:len(expected),PredictedPairs:len(predicted),TruePositivePairs:tp,Precision:precision,Recall:recall}}
