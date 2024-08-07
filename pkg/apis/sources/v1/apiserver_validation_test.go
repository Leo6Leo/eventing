/*
Copyright 2020 The Knative Authors

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

package v1

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/apis"
)

func TestAPIServerValidation(t *testing.T) {
	tests := []struct {
		name string
		spec ApiServerSourceSpec
		want error
	}{{
		name: "valid spec",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Foo",
			}},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
		},
		want: nil,
	}, {
		name: "empty sink",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Foo",
			}},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			errs = errs.Also(apis.ErrGeneric("expected at least one, got none", "ref", "uri").ViaField("sink"))
			return errs
		}(),
	}, {
		name: "invalid mode",
		spec: ApiServerSourceSpec{
			EventMode: "Test",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Foo",
			}},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			errs = errs.Also(apis.ErrInvalidValue("Test", "mode"))
			return errs
		}(),
	}, {
		name: "invalid apiVersion",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1/v2/v3",
				Kind:       "Foo",
			}},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
		},
		want: errors.New("invalid value: v1/v2/v3: resources[0].apiVersion"),
	}, {
		name: "missing kind",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
			}},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
		},
		want: errors.New("missing field(s): resources[0].kind"),
	}, {
		name: "owner - invalid apiVersion",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Bar",
			}},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
			ResourceOwner: &APIVersionKind{
				APIVersion: "v1/v2/v3",
				Kind:       "Foo",
			},
		},
		want: errors.New("invalid value: v1/v2/v3: owner.apiVersion"),
	}, {
		name: "missing kind",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Bar",
			}},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
			ResourceOwner: &APIVersionKind{
				APIVersion: "v1",
			},
		},
		want: errors.New("missing field(s): owner.kind"),
	}, {
		name: "empty resources",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
		},
		want: errors.New("missing field(s): resources"),
	}, {
		name: "nil resources",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: nil,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
		},
		want: errors.New("missing field(s): resources"),
	}, {
		name: "invalid spec ceOverrides validation",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Foo",
			}},
			SourceSpec: duckv1.SourceSpec{
				CloudEventOverrides: &duckv1.CloudEventOverrides{
					Extensions: map[string]string{"Invalid_type": "any value"},
				},
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
		},
		want: apis.ErrInvalidKeyName(
			"Invalid_type",
			"ceOverrides.extensions",
			"keys are expected to be alphanumeric",
		),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.TODO())
			if test.want != nil {
				if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
					t.Errorf("APIServerSourceSpec.Validate (-want, +got) = %v", diff)
				}
			} else if got != nil {
				t.Errorf("APIServerSourceSpec.Validate wanted nil, got = %v", got.Error())
			}
		})
	}
}

func TestAPIServerValidationCallsSpecValidation(t *testing.T) {
	source := ApiServerSource{
		Spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: nil,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
		},
	}

	err := source.Validate(context.TODO())
	assert.EqualError(t, err, "missing field(s): spec.resources", "Spec is not validated!")
}

func TestAPIServerFiltersValidation(t *testing.T) {
	tests := []struct {
		name         string
		featureState feature.Flag
		want         error
		filters      []eventingv1.SubscriptionsAPIFilter
	}{{
		name:         "an error is raised if the feature is disabled but filters are specified",
		featureState: feature.Disabled,
		filters: []eventingv1.SubscriptionsAPIFilter{{
			Prefix: map[string]string{
				"invALID": "abc",
			},
		}},
		want: apis.ErrGeneric("Filters is not empty but the NewAPIServerFilters feature is disabled."),
	}, {
		name:         "filters are validated when the feature is enabled",
		featureState: feature.Enabled,
		filters: []eventingv1.SubscriptionsAPIFilter{{
			Prefix: map[string]string{
				"invALID": "abc",
			},
		}},
		want: apis.ErrInvalidKeyName("invALID", apis.CurrentField,
			"Attribute name must start with a letter and can only contain "+
				"lowercase alphanumeric").ViaFieldKey("prefix", "invALID").ViaFieldIndex("filters", 0),
	}, {
		name:         "validation works for valid filters",
		featureState: feature.Enabled,
		filters: []eventingv1.SubscriptionsAPIFilter{{
			Exact: map[string]string{"myattr": "myval"},
		}},
		want: nil,
	}, {
		name:         "validation works for empty filters",
		featureState: feature.Enabled,
		filters:      []eventingv1.SubscriptionsAPIFilter{},
		want:         nil,
	}, {
		name:         "validation does not work for empty filters",
		featureState: feature.Disabled,
		filters:      []eventingv1.SubscriptionsAPIFilter{},
		want:         nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			featureContext := feature.ToContext(context.TODO(), feature.Flags{
				feature.NewAPIServerFilters: test.featureState,
			})
			apiserversource := &ApiServerSourceSpec{
				Filters:   test.filters,
				EventMode: "Resource",
				Resources: []APIVersionKindSelector{{
					APIVersion: "v1",
					Kind:       "Foo",
				}},
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "broker",
							Name:       "default",
						},
					},
				},
			}
			got := apiserversource.Validate(featureContext)
			if test.want != nil {
				if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
					t.Errorf("APIServerSourceSpec.Validate (-want, +got) = %v", diff)
				}
			} else if got != nil {
				t.Errorf("APIServerSourceSpec.Validate wanted nil, got = %v", got.Error())
			}
		})
	}
}
