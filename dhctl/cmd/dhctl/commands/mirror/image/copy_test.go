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
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
)

func TestCopyImage(t *testing.T) {
	deckhouseRegistry, err := image.NewRegistry("docker://registry.deckhouse.io/deckhouse/ce/", nil)
	if err != nil {
		t.Fatal(err)
	}

	localFile, err := image.NewRegistry("file:test/copy/file.tar.gz", nil)
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
		name     string
		args     args
		wantErr  error
		checkDir string
	}{
		{
			name: "copy release-channel:alpha from deckhouse registry to local dir by tag",
			args: args{
				ctx:  context.Background(),
				src:  image.NewImageConfig(deckhouseRegistry, "alpha", "", "release-channel"),
				dest: image.NewImageConfig(localFile, "alpha", "", "release-channel"),
				opts: []image.CopyOption{image.WithOutput(io.Discard)},
			},
			checkDir: filepath.Join(localFile.Path(), "release-channel", "alpha"),
		},

		{
			name: "dryRun copy from deckhouse registry to deckhouse registry by sha",
			args: args{
				ctx:  context.Background(),
				src:  image.NewImageConfig(deckhouseRegistry, "test-tag", "sha256:79ecc9578e5d18a524f5fecc9e5eb82231191d4deafd27e51bed212f9da336d4"),
				dest: image.NewImageConfig(deckhouseRegistry, "copy-test-tag", ""),
				opts: []image.CopyOption{image.WithOutput(io.Discard), image.WithDryRun()},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.RemoveAll(tt.checkDir)
			if err := image.CopyImage(tt.args.ctx, tt.args.src, tt.args.dest, policyContext, tt.args.opts...); !errors.Is(err, tt.wantErr) {
				t.Errorf("CopyImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkDir == "" {
				return
			}

			dirInfo, err := os.Stat(tt.checkDir)
			if err != nil {
				t.Errorf("CopyImage() error = path error for dir %s: %v", tt.checkDir, err)
				return
			}

			if dirInfo.IsDir() {
				t.Errorf("CopyImage() error = path is a dir: %v", tt.checkDir)
				return
			}

			for _, f := range []string{"version", "manifest.json"} {
				if _, err := os.Stat(filepath.Join(tt.checkDir, f)); err != nil {
					t.Errorf("CopyImage() error = %s path error for %s: %v", f, tt.checkDir, err)
				}
			}
		})
	}
}
