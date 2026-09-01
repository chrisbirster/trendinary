package history

import (
	"context"
	"sort"
	"time"
)

type Calibration struct {
	Observations  int     `json:"observations"`
	AttentionP50 float64 `json:"attention_p50"`
	AttentionP75 float64 `json:"attention_p75"`
	AttentionP90 float64 `json:"attention_p90"`
	ScoreP50     int     `json:"score_p50"`
	ScoreP75     int     `json:"score_p75"`
	ScoreP90     int     `json:"score_p90"`
	WindowDays   int     `json:"window_days"`
}

func (s *Store) Calibration(ctx context.Context, since time.Time) (Calibration,error) {
	if since.IsZero(){since=time.Now().UTC().Add(-30*24*time.Hour)}
	rows,err:=s.db.QueryContext(ctx,`SELECT raw_attention,score FROM trend_snapshots WHERE observed_at>=?`,since.UTC().Format(time.RFC3339Nano));if err!=nil{return Calibration{},err};defer rows.Close()
	attention:=[]float64{};scores:=[]int{}
	for rows.Next(){var a float64;var score int;if err:=rows.Scan(&a,&score);err!=nil{return Calibration{},err};attention=append(attention,a);scores=append(scores,score)};if err:=rows.Err();err!=nil{return Calibration{},err}
	sort.Float64s(attention);sort.Ints(scores)
	return Calibration{Observations:len(scores),AttentionP50:qf(attention,.5),AttentionP75:qf(attention,.75),AttentionP90:qf(attention,.9),ScoreP50:qi(scores,.5),ScoreP75:qi(scores,.75),ScoreP90:qi(scores,.9),WindowDays:max(1,int(time.Since(since).Hours()/24))},nil
}

func qf(values []float64,q float64)float64{if len(values)==0{return 0};index:=int(float64(len(values)-1)*q+.5);if index<0{index=0};if index>=len(values){index=len(values)-1};return values[index]}
func qi(values []int,q float64)int{if len(values)==0{return 0};index:=int(float64(len(values)-1)*q+.5);if index<0{index=0};if index>=len(values){index=len(values)-1};return values[index]}
