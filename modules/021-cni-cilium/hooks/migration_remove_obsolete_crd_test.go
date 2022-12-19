/*
Copyright 2022 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const ciliumEnvoyCRD = `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ciliumenvoyconfigs.cilium.io
spec: {}
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ciliumclusterwideenvoyconfigs.cilium.io
spec: {}
`

var _ = Describe("Modules :: cni-cilium :: hooks :: migration_remove_obsolete_crd ", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
	Context("CRDs exist", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ciliumEnvoyCRD))
			f.RunHook()
		})
		It("must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "ciliumenvoyconfigs.cilium.io").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "ciliumclusterwideenvoyconfigs.cilium.io").Exists()).To(BeFalse())
		})
	})
})
