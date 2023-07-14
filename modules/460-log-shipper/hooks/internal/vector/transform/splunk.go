package transform

import (
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

func DateTime() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "splunk_datetime",
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.DateTimeRule.String(),
			"drop_on_abort": false,
		},
	}
}
