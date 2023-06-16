// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"strings"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deckhouse_cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(true),
			FilterFunc:                   applyDeckhouseConfigmapFilter,
		},
	},
}, migrationRemoveDeprecatedConfigmapDeckhouse)

func applyDeckhouseConfigmapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.ConfigMap
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}
	for labelName, _ := range cm.Labels {
		if strings.Contains(labelName, "argocd") {
			return true, nil
		}
	}
	return false, nil
}

func migrationRemoveDeprecatedConfigmapDeckhouse(input *go_hook.HookInput) error {
	deckhouseConfigSnap := input.Snapshots["deckhouse_cm"]
	if deckhouseConfigSnap[0].(bool) {
		input.MetricsCollector.Set(
			"d8_deprecated_configmap_managed_by_argocd",
			1,
			map[string]string{
				"namespace": "d8-system",
				"configmap": "deckhouse",
			},
			metrics.WithGroup("migration_remove_deprecated_deckhouse_cm"),
		)
	}
	if len(deckhouseConfigSnap) > 0 {
		input.PatchCollector.Delete("v1", "Configmap", "d8-system", "deckhouse")
	}
	return nil
}
