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

package v1alpha1

import (
	"knative.dev/pkg/apis"
)

var eventPolicyCondSet = apis.NewLivingConditionSet()

const (
	EventPolicyConditionReady = apis.ConditionReady
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*EventPolicy) GetConditionSet() apis.ConditionSet {
	return eventPolicyCondSet
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (et *EventPolicyStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return eventPolicyCondSet.Manage(et).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (et *EventPolicyStatus) IsReady() bool {
	return et.GetTopLevelCondition().IsTrue()
}

// GetTopLevelCondition returns the top level Condition.
func (et *EventPolicyStatus) GetTopLevelCondition() *apis.Condition {
	return eventPolicyCondSet.Manage(et).GetTopLevelCondition()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (et *EventPolicyStatus) InitializeConditions() {
	eventPolicyCondSet.Manage(et).InitializeConditions()
}
