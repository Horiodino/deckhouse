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
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/directory"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image/transport"
)

var (
	DockerTransport            = docker.Transport.Name()
	fileTransport              = transport.Transport.Name()
	directoryTransport         = directory.Transport.Name()
	ErrNoSuchRegistryTransport = errors.New("no such transport for images. should be 'file:path', 'docker://docker-reference' or 'dir:path'")
)

type RegistryConfig struct {
	path       string
	transport  string
	authConfig *types.DockerAuthConfig
}

func MustNewRegistry(registryPath string, dockerCfg *types.DockerAuthConfig) *RegistryConfig {
	r, err := NewRegistry(registryPath, dockerCfg)
	if err != nil {
		panic(err)
	}
	return r
}

func NewRegistry(registryPath string, dockerCfg *types.DockerAuthConfig) (*RegistryConfig, error) {
	transportName, withinTransport, f := strings.Cut(transport.TrimExt(registryPath), ":")
	if !f {
		return nil, fmt.Errorf("can't find transport for '%s'", registryPath)
	}

	if transportName != DockerTransport && transportName != fileTransport && transportName != directoryTransport {
		return nil, ErrNoSuchRegistryTransport
	}

	return &RegistryConfig{
		path:       withinTransport,
		transport:  transportName,
		authConfig: dockerCfg,
	}, nil
}

func (r *RegistryConfig) copy() *RegistryConfig {
	n := new(RegistryConfig)
	*n = *r
	return n
}

func (r *RegistryConfig) Path() string {
	return r.path
}

func (r *RegistryConfig) Transport() string {
	return r.transport
}

func (r *RegistryConfig) AuthConfig() *types.DockerAuthConfig {
	return r.authConfig
}

func (r *RegistryConfig) WithAuthConfig(cfg *types.DockerAuthConfig) *RegistryConfig {
	n := r.copy()
	n.authConfig = cfg
	return n
}

func (r *RegistryConfig) ListTags(ctx context.Context, opts ...ListOption) ([]string, error) {
	imgRef, err := NewImageConfig(r, "", "").imageReference()
	if err != nil {
		return nil, err
	}

	switch r.Transport() {
	case DockerTransport:
		listOpts := &listOptions{}
		opts = append(opts, withAuth(r.AuthConfig()))
		for _, opt := range opts {
			opt(listOpts)
		}
		return docker.GetRepositoryTags(ctx, listOpts.sysCtx, imgRef)
	case directoryTransport:
		return listDirTags(r.Path())
	}
	return nil, ErrNoSuchRegistryTransport
}

func listDirTags(p string) ([]string, error) {
	tags := make([]string, 0)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return tags, nil
	} else if err != nil {
		return nil, err
	}

	err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
		// All copied to dir images have this file
		if err != nil || d.IsDir() || d.Name() != "manifest.json" {
			return err
		}

		tag, _, _ := strings.Cut(filepath.Base(filepath.Dir(path)), "@")
		tags = append(tags, tag)
		return filepath.SkipDir
	})
	return tags, err
}
