/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"errors"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

func init() {
	f := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		hasIncompatibleCtrls := getter.Get(incompatibleVersionsKey).Bool()
		if hasIncompatibleCtrls {
			return false, errors.New("cluster has 2+ ingress controllers with the same ingress class but different versions")
		}

		desiredVersion, err := semver.NewVersion(requirementValue)
		if err != nil {
			return false, err
		}
		currentVersionStr := getter.Get(minVersionValuesKey).String()
		if currentVersionStr == "" {
			// no IngressNginxController CRs exist
			return true, nil
		}
		currentVersion, err := semver.NewVersion(currentVersionStr)
		if err != nil {
			return false, err
		}

		if currentVersion.LessThan(desiredVersion) {
			return false, errors.New("minimal IngressNginxController version is lower then required")
		}

		return true, nil
	}

	requirements.Register("ingressNginx", f)
}
