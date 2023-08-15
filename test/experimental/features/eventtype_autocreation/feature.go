package eventtype_autocreation

import (
	"context"
	"embed"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	eventingv1beta2 "knative.dev/eventing/pkg/apis/eventing/v1beta2"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed eventtype.yaml
var yaml embed.FS

type EventType struct {
	Name       string
	EventTypes func(etl eventingv1beta2.EventTypeList) (bool, error)
}

func WaitForEventType(eventtype EventType, timing ...time.Duration) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		interval, timeout := k8s.PollTimings(ctx, timing)
		var lastErr error
		var lastEtl *eventingv1beta2.EventTypeList
		err := wait.PollImmediate(interval, timeout, func() (done bool, err error) {
			etl, err := eventingclient.Get(ctx).
				EventingV1beta2().
				EventTypes(env.Namespace()).
				List(ctx, metav1.ListOptions{})
			if err != nil {
				lastErr = err
				return false, nil
			}
			lastEtl = etl
			return eventtype.EventTypes(*etl)
		})
		if err != nil {
			t.Fatalf("failed to verify eventtype %s %v: %v\n%+v\n", eventtype.Name, err, lastErr, lastEtl)
		}

	}
}

func AssertPresent(expectedCeTypes sets.String) EventType {
	return EventType{
		Name: "test eventtypes match or not",
		EventTypes: func(etl eventingv1beta2.EventTypeList) (bool, error) {
			// Clone the expectedCeTypes
			clonedExpectedCeTypes := expectedCeTypes.Clone()
			for _, et := range etl.Items {
				clonedExpectedCeTypes.Delete(et.Spec.Type) // remove from the cloned set
			}
			return clonedExpectedCeTypes.Len() == 0, nil
		},
	}

}

// The function will apply the config map eventtype.yaml file to enable auto creation of eventtype
// The yaml file is in /test/eventtype.yaml
func ApplyEventTypeConfigMap() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		manifest.InstallYamlFS(ctx, yaml, nil)
	}
}