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

package mirror

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
)

func Test_deckhouseEdition(t *testing.T) {
	tests := []struct {
		name          string
		want          string
		editionInFile string
		createFile    bool
		wantErr       error
	}{
		{
			name:          "not EE edition",
			editionInFile: "ce",
			createFile:    true,
			wantErr:       ErrEditionNotEE,
		},
		{
			name:          "EE edition",
			editionInFile: "ee",
			createFile:    true,
			want:          "ee",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createFile {
				if err := os.WriteFile("/deckhouse/edition", []byte(tt.editionInFile), 0o755); err != nil {
					t.Error(err)
					return
				}
				// defer os.Remove("/deckhouse/edition")
			}

			got, err := deckhouseEdition()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("deckhouseEdition() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("deckhouseEdition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_deckhouseRegistry(t *testing.T) {
	type args struct {
		deckhouseRegistry string
		edtiton           string
		licenseToken      string
	}
	tests := []struct {
		name    string
		args    args
		want    *image.RegistryConfig
		wantErr error
	}{
		{
			name: "docker registry with license",
			args: args{
				deckhouseRegistry: "docker://registry.deckhouse.io/deckhouse",
				edtiton:           "ee",
				licenseToken:      "token",
			},
			want: image.MustNewRegistry("docker://registry.deckhouse.io/deckhouse/ee", &types.DockerAuthConfig{Username: "license-token", Password: "token"}),
		},
		{
			name: "docker registry without license",
			args: args{
				deckhouseRegistry: "docker://registry.deckhouse.io/deckhouse",
				edtiton:           "ee",
			},
			wantErr: ErrNoLicense,
		},
		{
			name: "file registry",
			args: args{
				deckhouseRegistry: "file:asafs.tar.gz",
				edtiton:           "ee",
			},
			want: image.MustNewRegistry("dir:asafs.tar.gz", nil),
		},
		{
			name: "bad transport registry",
			args: args{
				deckhouseRegistry: "docker-archive://tar.gz",
				edtiton:           "ee",
			},
			wantErr: image.ErrNoSuchRegistryTransport,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deckhouseRegistry(tt.args.deckhouseRegistry, tt.args.edtiton, tt.args.licenseToken)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("deckhouseRegistry() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("deckhouseRegistry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newRegistry(t *testing.T) {
	type args struct {
		registryWithTransport string
		auth                  *types.DockerAuthConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *image.RegistryConfig
		wantErr error
	}{
		{
			name: "docker registry with auth",
			args: args{
				registryWithTransport: "docker://registry.com",
				auth: &types.DockerAuthConfig{
					Username: "username",
					Password: "pass",
				},
			},
			want: image.MustNewRegistry("docker://registry.com", &types.DockerAuthConfig{Username: "username", Password: "pass"}),
		},
		{
			name: "archive file",
			args: args{
				registryWithTransport: "file:archive.tar.gz",
			},
			want: image.MustNewRegistry("file:archive.tar.gz", nil),
		},
		{
			name: "registry without transport",
			args: args{
				registryWithTransport: "docker-archive:archive",
				auth:                  &types.DockerAuthConfig{},
			},
			wantErr: image.ErrNoSuchRegistryTransport,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newRegistry(tt.args.registryWithTransport, tt.args.auth)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("newRegistry() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newRegistry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_registryAuth(t *testing.T) {
	type args struct {
		username string
		password string
	}
	tests := []struct {
		name string
		args args
		want *types.DockerAuthConfig
	}{
		{
			name: "username and password set",
			args: args{
				username: "user",
				password: "password",
			},
			want: &types.DockerAuthConfig{
				Username: "user",
				Password: "password",
			},
		},

		{
			name: "username set and password not set",
			args: args{
				username: "user",
			},
			want: nil,
		},

		{
			name: "username not set and password set",
			args: args{
				password: "password",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := registryAuth(tt.args.username, tt.args.password); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("registryAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}
