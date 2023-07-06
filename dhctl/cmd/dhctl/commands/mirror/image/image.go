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
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/directory"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
)

type ImageConfig struct {
	tag            string
	digest         string
	additionalPath string
	registry       *RegistryConfig
}

func NewImageConfig(registry *RegistryConfig, tag, digest string, additionalPaths ...string) *ImageConfig {
	return &ImageConfig{
		registry:       registry,
		tag:            tag,
		digest:         digest,
		additionalPath: filepath.Join(additionalPaths...),
	}
}

func (i *ImageConfig) copy() *ImageConfig {
	n := new(ImageConfig)
	*n = *i
	return n
}

func (i *ImageConfig) WithNewRegistry(r *RegistryConfig) *ImageConfig {
	n := i.copy()
	n.registry = r
	return n
}

func (i *ImageConfig) Digest() string {
	return i.digest
}

func (i *ImageConfig) WithDigest(d string) *ImageConfig {
	n := i.copy()
	n.digest = d
	return n
}

func (i *ImageConfig) Tag() string {
	return i.tag
}

func (i *ImageConfig) WithTag(t string) *ImageConfig {
	n := i.copy()
	n.tag = t
	return n
}

func (i *ImageConfig) ImageReference() (types.ImageReference, error) {
	imageBuilder := &strings.Builder{}
	imageBuilder.WriteString(i.RegistryTransport())
	imageBuilder.WriteByte(':')

	switch i.RegistryTransport() {
	case docker.Transport.Name():
		imageBuilder.WriteString(strings.TrimRight(i.RegistryPath(), "/"))
		if i.additionalPath != "" {
			imageBuilder.WriteByte('/')
			imageBuilder.WriteString(strings.Trim(i.additionalPath, "/"))
		}
		// https://github.com/containers/image/blob/v5.26.1/docker/docker_transport.go#L80
		if i.tag != "" && i.digest == "" {
			imageBuilder.WriteByte(':')
			imageBuilder.WriteString(i.tag)
		}

	case directory.Transport.Name():
		p := filepath.Join(i.RegistryPath(), i.additionalPath)
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, err
		}
		imageBuilder.WriteString(filepath.Join(p, i.tag))
	}

	if i.digest != "" {
		imageBuilder.WriteByte('@')
		imageBuilder.WriteString(i.digest)
	}

	return alltransports.ParseImageName(imageBuilder.String())
}

func (i *ImageConfig) RegistryPath() string {
	if i.registry == nil {
		return ""
	}
	return i.registry.RegistryPath()
}

func (i *ImageConfig) RegistryTransport() string {
	if i.registry == nil {
		return ""
	}
	return i.registry.RegistryTransport()
}

func (i *ImageConfig) AuthConfig() *types.DockerAuthConfig {
	if i.registry == nil {
		return nil
	}
	return i.registry.AuthConfig()
}
