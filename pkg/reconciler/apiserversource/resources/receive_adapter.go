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

package resources

import (
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/adapter/apiserver"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
)

// ReceiveAdapterArgs are the arguments needed to create a ApiServer Receive Adapter.
// Every field is required.
type ReceiveAdapterArgs struct {
	Image         string
	Source        *v1.ApiServerSource
	Labels        map[string]string
	Audience      *string
	SinkURI       string
	CACerts       *string
	Configs       reconcilersource.ConfigAccessor
	Namespaces    []string
	AllNamespaces bool
}

// MakeOIDCRole will return the role object config for generating the JWT token
func MakeOIDCRole(gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) (*rbacv1.Role, error) {
	roleName := "create-oidc-token"

	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: objectMeta.GetNamespace(),
			Annotations: map[string]string{
				"description": fmt.Sprintf("Role for OIDC Authentication for %s %q", gvk.GroupKind().Kind, objectMeta.Name),
			},
		},
		Rules: []rbacv1.PolicyRule{
			rbacv1.PolicyRule{
				APIGroups:     []string{""},
				ResourceNames: []string{auth.GetOIDCServiceAccountNameForResource(gvk, objectMeta)},
				Resources:     []string{"serviceaccounts/token"},
				Verbs:         []string{"create"},
			},
		},
	}, nil

}

// MakeOIDCRoleBinding will return the rolebinding object for generating the JWT token
// So that ApiServerSource's service account have access to create the JWT token for it's OIDC service account and the target audience
// Note:  it is in the source.Spec, NOT in source.Auth
func MakeOIDCRoleBinding(gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta, saName string) (*rbacv1.RoleBinding, error) {
	roleName := "create-oidc-token"
	roleBindingName := "create-oidc-token"

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: objectMeta.GetNamespace(),
			Annotations: map[string]string{
				"description": fmt.Sprintf("Role Binding for OIDC Authentication for %s %q", gvk.GroupKind().Kind, objectMeta.Name),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: objectMeta.GetNamespace(),
				//Note: apiServerSource service account name, it is in the source.Spec, NOT in source.Auth
				Name: saName,
			},
		},
	}, nil

}

// MakeReceiveAdapter generates (but does not insert into K8s) the Receive Adapter Deployment for
// ApiServer Sources.
func MakeReceiveAdapter(args *ReceiveAdapterArgs) (*appsv1.Deployment, error) {
	replicas := int32(1)

	env, err := makeEnv(args)
	if err != nil {
		return nil, fmt.Errorf("error generating env vars: %w", err)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Source.Namespace,
			Name:      kmeta.ChildName(fmt.Sprintf("apiserversource-%s-", args.Source.Name), string(args.Source.GetUID())),
			Labels:    args.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: args.Labels,
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: args.Labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.Source.Spec.ServiceAccountName,
					EnableServiceLinks: ptr.Bool(false),
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: args.Image,
							Env:   env,
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9090,
							}, {
								Name:          "health",
								ContainerPort: 8080,
							}},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Port: intstr.FromString("health"),
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.Bool(false),
								ReadOnlyRootFilesystem:   ptr.Bool(true),
								RunAsNonRoot:             ptr.Bool(true),
								Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
								SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
							},
						},
					},
				},
			},
		},
	}, nil
}

func makeEnv(args *ReceiveAdapterArgs) ([]corev1.EnvVar, error) {
	cfg := &apiserver.Config{
		Namespaces:    args.Namespaces,
		Resources:     make([]apiserver.ResourceWatch, 0, len(args.Source.Spec.Resources)),
		ResourceOwner: args.Source.Spec.ResourceOwner,
		EventMode:     args.Source.Spec.EventMode,
		AllNamespaces: args.AllNamespaces,
	}

	for _, r := range args.Source.Spec.Resources {
		gv, err := schema.ParseGroupVersion(r.APIVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse APIVersion: %w", err)
		}
		gvr, _ := meta.UnsafeGuessKindToResource(gv.WithKind(r.Kind))

		rw := apiserver.ResourceWatch{GVR: gvr}

		if r.LabelSelector != nil {
			selector, _ := metav1.LabelSelectorAsSelector(r.LabelSelector)
			rw.LabelSelector = selector.String()
		}

		cfg.Resources = append(cfg.Resources, rw)
	}

	config := "{}"
	if b, err := json.Marshal(cfg); err == nil {
		config = string(b)
	}

	envs := []corev1.EnvVar{{
		Name:  adapter.EnvConfigSink,
		Value: args.SinkURI,
	}, {
		Name:  "K_SOURCE_CONFIG",
		Value: config,
	}, {
		Name:  "SYSTEM_NAMESPACE",
		Value: system.Namespace(),
	}, {
		Name: adapter.EnvConfigNamespace,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}, {
		Name:  adapter.EnvConfigName,
		Value: args.Source.Name,
	}, {
		Name:  "METRICS_DOMAIN",
		Value: "knative.dev/eventing",
	},
	}

	if args.CACerts != nil {
		envs = append(envs, corev1.EnvVar{
			Name:  adapter.EnvConfigCACert,
			Value: *args.CACerts,
		})
	}

	if args.Audience != nil {
		envs = append(envs, corev1.EnvVar{
			Name:  "K_AUDIENCE",
			Value: *args.Audience,
		})
	}

	if args.Source.Status.Auth != nil {
		envs = append(envs, corev1.EnvVar{
			Name:  "K_OIDC_SERVICE_ACCOUNT",
			Value: *args.Source.Status.Auth.ServiceAccountName,
		})
	}

	envs = append(envs, args.Configs.ToEnvVars()...)

	if args.Source.Spec.CloudEventOverrides != nil {
		ceJson, err := json.Marshal(args.Source.Spec.CloudEventOverrides)
		if err != nil {
			return nil, fmt.Errorf("failure to marshal cloud event overrides %v: %v", args.Source.Spec.CloudEventOverrides, err)
		}
		envs = append(envs, corev1.EnvVar{Name: adapter.EnvConfigCEOverrides, Value: string(ceJson)})
	}
	return envs, nil
}
