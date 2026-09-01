package scanner

import (
	"testing"

	"github.com/chrisbirster/trendinary/internal/engine"
	"github.com/chrisbirster/trendinary/internal/model"
)

func TestCandidateClusterRejectsNewsDataOnly(t *testing.T) {
	cluster := engine.Cluster{Signals: []model.Signal{
		{ID: "newsdata:a", Source: model.Source{Name: "One", Domain: "one.example"}, Title: "Same topic breaks"},
		{ID: "newsdata:b", Source: model.Source{Name: "Two", Domain: "two.example"}, Title: "Same topic continues"},
	}}
	if candidateClusterV2(cluster) {
		t.Fatal("NewsData-only cluster should be corroborative, not independently promotable")
	}
}

func TestCandidateClusterAllowsNewsDataToCorroborateFreshSource(t *testing.T) {
	cluster := engine.Cluster{Signals: []model.Signal{
		{ID: "gdelt:a", Source: model.Source{Name: "One", Domain: "one.example"}, Title: "Same topic breaks"},
		{ID: "newsdata:b", Source: model.Source{Name: "Two", Domain: "two.example"}, Title: "Same topic continues"},
	}}
	if !candidateClusterV2(cluster) {
		t.Fatal("NewsData should strengthen a cluster discovered by a fresher independent source")
	}
}
