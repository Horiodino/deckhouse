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
	"os"
	"strings"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func CopyImage(ctx context.Context, src, dest *ImageConfig, policyContext *signature.PolicyContext, opts ...CopyOptions) error {
	srcRef, err := src.ImageReference()
	if err != nil {
		return err
	}

	destRef, err := dest.ImageReference()
	if err != nil {
		return err
	}

	copyOptions := &copy.Options{ReportWriter: os.Stdout}

	opts = append(opts, withSourceAuth(src.AuthConfig()), withDestAuth(dest.AuthConfig()))
	for _, opt := range opts {
		opt(copyOptions)
	}

	log.InfoF("Copying %s image to %s...\n", trimRef(srcRef), trimRef(destRef))

	_, err = copy.Image(ctx, policyContext, destRef, srcRef, copyOptions)
	return err
}

func trimRef(ref types.ImageReference) string {
	return strings.TrimLeft(ref.StringWithinTransport(), "/")
}
