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
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
)

func TestCopyImage(t *testing.T) {
	deckhouseRegistry, err := image.NewRegistry("docker://registry.deckhouse.io/deckhouse/ce/", nil)
	if err != nil {
		t.Fatal(err)
	}

	localDir, err := image.NewRegistry("test/deckhouse/dir", nil)
	if err != nil {
		t.Fatal(err)
	}

	policyContext, err := image.NewPolicyContext()
	if err != nil {
		t.Fatal(err)
	}
	defer policyContext.Destroy()

	type args struct {
		ctx  context.Context
		src  *image.ImageConfig
		dest *image.ImageConfig
		opts []image.CopyOption
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "copy trivy-db from deckhouse registry to local dir by tag",
			args: args{
				ctx:  context.Background(),
				src:  image.NewImageConfig(deckhouseRegistry, "alpha", "", "release-channel"),
				dest: image.NewImageConfig(localDir, "alpha", "", "release-channel"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := image.CopyImage(tt.args.ctx, tt.args.src, tt.args.dest, policyContext, tt.args.opts...); (err != nil) != tt.wantErr {
				t.Errorf("CopyImage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
