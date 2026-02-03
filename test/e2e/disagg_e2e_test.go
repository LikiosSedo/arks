/*
Copyright 2025.

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

package e2e

import (
	. "github.com/onsi/ginkgo/v2"

	"github.com/arks-ai/arks/test/e2e/framework"
	"github.com/arks-ai/arks/test/e2e/testcase"
)

var f *framework.Framework

var _ = Describe("ArksDisaggregatedApplication E2E Tests", Ordered, func() {

	BeforeAll(func() {
		f = framework.NewFramework()
		f.BeforeAll()
	})

	AfterAll(func() {
		if f != nil {
			f.AfterAll()
		}
	})

	AfterEach(func() {
		if f != nil {
			f.AfterEach()
		}
	})

	// Run all test cases
	Context("Basic CRUD Operations", func() {
		testcase.RunDisaggBasicTestCases(&f)
	})

	Context("Scaling Operations", func() {
		testcase.RunDisaggScalingTestCases(&f)
	})

	Context("Rolling Update Operations", func() {
		testcase.RunDisaggRollingUpdateTestCases(&f)
	})

	Context("Resource Update Operations", func() {
		testcase.RunDisaggResourcesTestCases(&f)
	})

	Context("Image Update Operations", func() {
		testcase.RunDisaggImageTestCases(&f)
	})

	Context("Command/Args Update Operations", func() {
		testcase.RunDisaggCommandTestCases(&f)
	})

	Context("Environment Variable Operations", func() {
		testcase.RunDisaggEnvTestCases(&f)
	})

	Context("Probe Configuration Operations", func() {
		testcase.RunDisaggProbeTestCases(&f)
	})

	Context("Volume Mount Operations", func() {
		testcase.RunDisaggVolumeTestCases(&f)
	})

	Context("Recovery Operations", func() {
		testcase.RunDisaggRecoveryTestCases(&f)
	})

	Context("Gang Scheduling Operations", func() {
		testcase.RunDisaggGangSchedulingTestCases(&f)
	})

	Context("LWS Gang + Coordination Scaling Operations", func() {
		testcase.RunDisaggLwsGangCoordinationTestCases(&f)
	})

	Context("GPU Resources Operations", func() {
		testcase.RunDisaggGPUResourcesTestCases(&f)
	})

	Context("Real GPU Cluster Operations", func() {
		testcase.RunDisaggRealGPUTestCases(&f)
	})
})
