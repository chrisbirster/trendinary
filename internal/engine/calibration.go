package engine

type LifecycleThresholds struct {
	Breaking int `json:"breaking"`
	Peaking  int `json:"peaking"`
	Rising   int `json:"rising"`
	Resurface int `json:"resurface"`
}

func DefaultLifecycleThresholds() LifecycleThresholds { return LifecycleThresholds{Breaking:85,Peaking:75,Rising:60,Resurface:55} }

// CalibratedThresholds raises (but never lowers) v0.1 thresholds when the
// observed score distribution demonstrates that the existing threshold is too
// permissive. A small history does not alter behavior.
func CalibratedThresholds(observations,scoreP75,scoreP90 int) LifecycleThresholds {
	thresholds:=DefaultLifecycleThresholds();if observations<50{return thresholds}
	if scoreP90>thresholds.Breaking{thresholds.Breaking=scoreP90}
	if scoreP75>thresholds.Rising{thresholds.Rising=scoreP75}
	if scoreP75>thresholds.Peaking{thresholds.Peaking=scoreP75}
	if thresholds.Peaking>=thresholds.Breaking{thresholds.Peaking=thresholds.Breaking-5}
	return thresholds
}

func LifecycleWithThresholds(input LifecycleInput,t LifecycleThresholds) string {
	delta:=input.Score-input.PreviousScore
	if input.PreviouslyCold&&input.Score>=t.Resurface&&input.Velocity>=.65{return "RESURFACING"}
	if input.Score>=t.Breaking&&input.Velocity>=.75&&input.SourceBreadth>=.6{return "BREAKING"}
	if input.Score>=t.Peaking&&delta>=-3&&delta<=3&&input.Velocity<.55{return "PEAKING"}
	if delta<=-10||(input.Score<t.Rising&&input.Velocity<.35){return "COOLING"}
	if input.Score>=t.Rising&&(delta>=5||input.Velocity>=.6){return "RISING"}
	return "EMERGING"
}
