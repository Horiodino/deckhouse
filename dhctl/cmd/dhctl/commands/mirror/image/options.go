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
	"io"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/types"
)

type CopyOptions func(*copy.Options)

func WithPreserveDigests() func(*copy.Options) {
	return func(o *copy.Options) {
		o.PreserveDigests = true
	}
}

func WithCopyAllImages() func(*copy.Options) {
	return func(o *copy.Options) {
		o.ImageListSelection = copy.CopyAllImages
	}
}

func WithOutput(w io.Writer) func(*copy.Options) {
	return func(o *copy.Options) {
		o.ReportWriter = w
	}
}

func WithDestInsecure() func(*copy.Options) {
	return func(o *copy.Options) {
		if o.DestinationCtx == nil {
			o.DestinationCtx = &types.SystemContext{}
		}
		o.DestinationCtx.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
	}
}

func withSourceAuth(cfg *types.DockerAuthConfig) func(*copy.Options) {
	return func(o *copy.Options) {
		if cfg == nil {
			return
		}
		if o.SourceCtx == nil {
			o.SourceCtx = &types.SystemContext{}
		}
		o.SourceCtx.DockerAuthConfig = cfg
	}
}

func withDestAuth(cfg *types.DockerAuthConfig) func(*copy.Options) {
	return func(o *copy.Options) {
		if cfg == nil {
			return
		}
		if o.DestinationCtx == nil {
			o.DestinationCtx = &types.SystemContext{}
		}
		o.DestinationCtx.DockerAuthConfig = cfg
	}
}
