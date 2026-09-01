package explain

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

const Mode = "grounded-deterministic-v1"

type Answer struct {
	Answer   string           `json:"answer"`
	Mode     string           `json:"mode"`
	Evidence []model.Evidence `json:"evidence"`
}

// Build produces Trendinary's first explanation layer from measured score,
// propagation, and concrete source signals. It does not invent facts outside
// the evidence bundle. A future LLM implementation can sit behind this same
// model.Explanation contract and remain constrained to the same citations.
func Build(trend model.Trend, signals []model.Signal, firstSeen time.Time) model.Explanation {
	evidence := evidenceBundle(signals, 8)
	summary := trend.Reason
	if strings.TrimSpace(summary) == "" {
		summary = fmt.Sprintf("%s has a Trendinary score of %d and is currently %s.", trend.Name, trend.Score, strings.ToLower(trend.Status))
	}
	if len(trend.Propagation) >= 2 {
		summary = fmt.Sprintf("%s Attention is now present across %d source networks; the earliest current source signal was %s.", summary, len(trend.Propagation), trend.Propagation[0].Source.Name)
	}

	whatChanged := fmt.Sprintf("Trendinary currently scores %s at %d (%s, %s).", trend.Name, trend.Score, trend.Change, trend.Status)
	if len(trend.Propagation) > 0 {
		path := make([]string, 0, len(trend.Propagation))
		for _, hop := range trend.Propagation {
			name := hop.Source.Name
			if name == "" {
				name = hop.Source.Domain
			}
			path = append(path, name)
		}
		whatChanged += " Observed propagation: " + strings.Join(path, " → ") + "."
	}

	lore := "Trendinary has not yet accumulated enough durable history to produce a deep backstory for this trend."
	if !firstSeen.IsZero() {
		lore = fmt.Sprintf("Trendinary first linked the current trend entity on %s. Its identity is preserved across wording changes so future spikes attach to the same history.", firstSeen.UTC().Format("January 2, 2006 at 15:04 UTC"))
	}
	if len(trend.Aliases) > 1 {
		aliases := append([]string(nil), trend.Aliases...)
		if len(aliases) > 5 {
			aliases = aliases[:5]
		}
		lore += " Known names/cluster aliases include: " + strings.Join(aliases, ", ") + "."
	}

	return model.Explanation{
		Summary:      summary,
		WhatChanged: whatChanged,
		Lore:         lore,
		Confidence:   confidence(len(signals), len(trend.Sources)),
		Mode:         Mode,
		Evidence:     evidence,
	}
}

// AnswerQuestion is intentionally conservative. It answers only the product's
// supported trend questions from the already-grounded trend object and returns
// the same source evidence instead of free-form speculation.
func AnswerQuestion(question string, trend model.Trend) Answer {
	question = strings.ToLower(strings.TrimSpace(question))
	explanation := trend.Explanation
	if explanation == nil {
		fallback := Build(trend, nil, time.Time{})
		explanation = &fallback
	}
	answer := explanation.Summary
	switch {
	case containsAny(question, "why", "trending", "matter", "important"):
		answer = explanation.Summary
	case containsAny(question, "change", "today", "now", "happen"):
		answer = explanation.WhatChanged
	case containsAny(question, "lore", "history", "background", "origin"):
		answer = explanation.Lore
	case containsAny(question, "source", "evidence", "receipt", "proof"):
		answer = evidenceSummary(explanation.Evidence)
	case containsAny(question, "who", "people", "voices", "accounts"):
		answer = voicesSummary(trend.TopVoices)
	case containsAny(question, "bias", "left", "right", "perspective"):
		answer = perspectiveSummary(trend.Perspective)
	default:
		answer = explanation.Summary + " I can answer grounded questions about why it is trending, what changed, its history, sources, top voices, or source-perspective mix."
	}
	return Answer{Answer: answer, Mode: explanation.Mode, Evidence: explanation.Evidence}
}

func evidenceBundle(signals []model.Signal, limit int) []model.Evidence {
	values := append([]model.Signal(nil), signals...)
	sort.SliceStable(values, func(i, j int) bool {
		wi := engagement(values[i])
		wj := engagement(values[j])
		if wi == wj {
			return values[i].ID < values[j].ID
		}
		return wi > wj
	})
	out := make([]model.Evidence, 0, min(limit, len(values)))
	seenURL := map[string]struct{}{}
	seenSource := map[string]int{}
	// First pass gives different source networks a chance to appear.
	for _, signal := range values {
		if len(out) >= limit {
			break
		}
		key := signal.Source.Domain
		if key == "" {
			key = signal.Source.Name
		}
		if seenSource[key] > 0 {
			continue
		}
		if appendEvidence(&out, seenURL, signal) {
			seenSource[key]++
		}
	}
	for _, signal := range values {
		if len(out) >= limit {
			break
		}
		appendEvidence(&out, seenURL, signal)
	}
	return out
}

func appendEvidence(out *[]model.Evidence, seen map[string]struct{}, signal model.Signal) bool {
	if signal.URL == "" {
		return false
	}
	if _, ok := seen[signal.URL]; ok {
		return false
	}
	seen[signal.URL] = struct{}{}
	title := strings.TrimSpace(signal.Title)
	if title == "" {
		title = strings.TrimSpace(signal.Text)
		if len(title) > 160 {
			title = title[:160] + "…"
		}
	}
	*out = append(*out, model.Evidence{
		Source:      signal.Source,
		Title:       title,
		URL:         signal.URL,
		Author:      signal.Author,
		PublishedAt: signal.PublishedAt,
	})
	return true
}

func engagement(signal model.Signal) int {
	return signal.Engagement.Score + signal.Engagement.Likes + signal.Engagement.Replies +
		2*signal.Engagement.Reposts + 2*signal.Engagement.Quotes
}

func confidence(signalCount, sourceCount int) string {
	switch {
	case sourceCount >= 3 && signalCount >= 8:
		return "high"
	case sourceCount >= 2 || signalCount >= 4:
		return "medium"
	default:
		return "low"
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func evidenceSummary(values []model.Evidence) string {
	if len(values) == 0 {
		return "Trendinary does not have a citable source bundle for this trend yet."
	}
	names := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		name := value.Source.Name
		if name == "" {
			name = value.Source.Domain
		}
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return fmt.Sprintf("The current explanation is grounded in %d citable items across: %s.", len(values), strings.Join(names, ", "))
}

func voicesSummary(values []model.ActorProfile) string {
	if len(values) == 0 {
		return "No source-native top voices have been resolved for this trend yet."
	}
	names := make([]string, 0, len(values))
	for _, value := range values {
		name := value.Handle
		if name == "" {
			name = value.DisplayName
		}
		if name == "" {
			name = value.DID
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return "Resolved high-signal voices include: " + strings.Join(names, ", ") + "."
}

func perspectiveSummary(mix model.PerspectiveMix) string {
	if mix.RatedSources == 0 {
		return "None of the current sources have an evidence-backed political-lean rating, so Trendinary does not infer one. " + mix.Note
	}
	return fmt.Sprintf("Rated source mix: left %d, lean-left %d, center %d, lean-right %d, right %d, mixed %d; %d sources are unrated. %s",
		mix.Left, mix.LeanLeft, mix.Center, mix.LeanRight, mix.Right, mix.Mixed, mix.Unrated, mix.Note)
}
