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

package image

import (
	"strings"

	"github.com/containers/image/v5/types"
)

type RegistryConfig struct {
	registryPath string
	transport    string
	authConfig   *types.DockerAuthConfig
}

func NewRegistry(registryPath string, dockerCfg *types.DockerAuthConfig) *RegistryConfig {
	transportName, withinTransport, _ := strings.Cut(registryPath, ":")
	return &RegistryConfig{
		registryPath: withinTransport,
		transport:    transportName,
		authConfig:   dockerCfg,
	}
}

func (r *RegistryConfig) RegistryPath() string {
	return r.registryPath
}

func (r *RegistryConfig) RegistryTransport() string {
	return r.transport
}

func (r *RegistryConfig) AuthConfig() *types.DockerAuthConfig {
	return r.authConfig
}
