package stdmeta

import (
	"context"

	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

var (
	patchPlugin = domain.PluginType("patch")
)

func init() {
	domain.RegisterPlugin(func() domain.Plugin { return &patch{} })
}

type patch struct{}

func (t *patch) PluginType() domain.PluginType { return patchPlugin }

func (t *patch) PropertyPatch(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
	body interface{},
) {
	// wow, how can it be this simple?
	// ... we need to add a way to add validation... so I guess it can't stay this simple for long.

	rrp.Value = body
}
