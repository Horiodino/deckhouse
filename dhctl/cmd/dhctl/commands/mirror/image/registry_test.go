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

package image_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
)

func TestRegistryConfig_ListTags(t *testing.T) {
	basePath := "test/registry"
	if err := createImageFiles(basePath); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(basePath)

	type fields struct {
		registryPath string
		authConfig   *types.DockerAuthConfig
	}
	type args struct {
		ctx  context.Context
		opts []image.ListOption
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        []string
		wantInitErr error
		wantErr     error
	}{
		{
			name: "list deckhouse skopeo",
			fields: fields{
				registryPath: "docker://registry.deckhouse.io/deckhouse/tools/skopeo/",
			},
			args: args{
				ctx: context.Background(),
			},
			want: []string{"v1.11.2"},
		},
		{
			name: "list directory",
			fields: fields{
				registryPath: fmt.Sprintf("dir:%s", basePath),
			},
			args: args{
				ctx: context.Background(),
			},
			want: []string{"image-1", "image-2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := image.NewRegistry(tt.fields.registryPath, tt.fields.authConfig)
			if !errors.Is(err, tt.wantInitErr) {
				t.Errorf("image.NewRegistry() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}

			got, err := r.ListTags(tt.args.ctx, tt.args.opts...)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("RegistryConfig.ListTags() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RegistryConfig.ListTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createImageFiles(basePath string) error {
	for i := 1; i < 3; i++ {
		if err := os.MkdirAll(fmt.Sprintf("%s/image-%d", basePath, i), 0o755); err != nil {
			return err
		}

		f, err := os.Create(fmt.Sprintf("%s/image-%d/manifest.json", basePath, i))
		if err != nil {
			return err
		}
		defer f.Close()
	}
	return nil
}
