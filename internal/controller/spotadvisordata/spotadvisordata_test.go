/*
Copyright 2025 The Crossplane Authors.

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

package spotadvisordata

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// Unlike many Kubernetes projects Crossplane does not use third party testing
// libraries, per the common Go test review comments. Crossplane encourages the
// use of table driven unit tests. The tests of the crossplane-runtime project
// are representative of the testing style Crossplane encourages.
//
// https://github.com/golang/go/wiki/TestComments
// https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md#contributing-code

func TestIsInstanceTypeInFamilies(t *testing.T) {
	type args struct {
		instanceType string
		families     []string
	}

	type want struct {
		result bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"InstanceTypeInFamilies": {
			reason: "Should return true when instance type family is in the list",
			args: args{
				instanceType: "m7g.2xlarge",
				families:     []string{"m7g", "c5", "t3"},
			},
			want: want{
				result: true,
			},
		},
		"InstanceTypeNotInFamilies": {
			reason: "Should return false when instance type family is not in the list",
			args: args{
				instanceType: "m7g.2xlarge",
				families:     []string{"c5", "t3", "r5"},
			},
			want: want{
				result: false,
			},
		},
		"EmptyFamiliesList": {
			reason: "Should return false when families list is empty",
			args: args{
				instanceType: "m7g.2xlarge",
				families:     []string{},
			},
			want: want{
				result: false,
			},
		},
		"InvalidInstanceTypeFormat": {
			reason: "Should return false when instance type doesn't have a dot",
			args: args{
				instanceType: "m7g2xlarge",
				families:     []string{"m7g"},
			},
			want: want{
				result: false,
			},
		},
		"MultipleFamiliesMatch": {
			reason: "Should return true when instance type matches any family in the list",
			args: args{
				instanceType: "c5.large",
				families:     []string{"m7g", "c5", "t3"},
			},
			want: want{
				result: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{}
			got := e.isInstanceTypeInFamilies(tc.args.instanceType, tc.args.families)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nisInstanceTypeInFamilies(...): -want result, +got result:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestObserve(t *testing.T) {
	type fields struct {
		service interface{}
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		o   managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		// TODO: Add test cases.
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{service: tc.fields.service}
			got, err := e.Observe(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
