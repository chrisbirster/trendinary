package scanner

var bootstrapDiscoverySources = map[string]struct{}{
	"google-trends-us": {},
	"wired-top":        {},
	"ars-all":          {},
	"abc-top":          {},
	"techcrunch":       {},
}

func bootstrapDiscoverySource(id string) bool {
	_, ok := bootstrapDiscoverySources[id]
	return ok
}
