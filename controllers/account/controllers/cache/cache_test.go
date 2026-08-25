/*
Copyright 2026.

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

package cache

import (
	"reflect"
	"testing"

	userv1 "github.com/labring/sealos/controllers/user/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTransformUserKeepsOnlyMetadata(t *testing.T) {
	user := &userv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "user-a",
			ResourceVersion: "42",
			Annotations: map[string]string{
				"user.sealos.io/owner": "owner-a",
				"unused.example/key":   "large-value",
			},
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "test"}},
		},
		Spec: userv1.UserSpec{CSRExpirationSeconds: 600},
		Status: userv1.UserStatus{
			Phase:      userv1.UserActive,
			KubeConfig: "large-kubeconfig",
		},
	}

	transformed, err := transformUser(user)
	if err != nil {
		t.Fatalf("transform user: %v", err)
	}
	got, ok := transformed.(*userv1.User)
	if !ok {
		t.Fatalf("transformed type = %T, want *userv1.User", transformed)
	}
	if got.Name != user.Name || got.ResourceVersion != user.ResourceVersion {
		t.Fatalf("metadata was not retained: %#v", got.ObjectMeta)
	}
	wantAnnotations := map[string]string{"user.sealos.io/owner": "owner-a"}
	if !reflect.DeepEqual(got.Annotations, wantAnnotations) {
		t.Fatalf("annotations = %#v, want %#v", got.Annotations, wantAnnotations)
	}
	if len(got.ManagedFields) != 0 {
		t.Fatalf("managed fields were retained: %#v", got.ManagedFields)
	}
	if !reflect.DeepEqual(got.Spec, userv1.UserSpec{}) {
		t.Fatalf("spec was retained: %#v", got.Spec)
	}
	if !reflect.DeepEqual(got.Status, userv1.UserStatus{}) {
		t.Fatalf("status was retained: %#v", got.Status)
	}
	if len(user.ManagedFields) == 0 {
		t.Fatal("transform mutated the source object")
	}
}

func TestTransformPartialUserKeepsOnlyMetadata(t *testing.T) {
	user := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name: "user-a",
			Annotations: map[string]string{
				"user.sealos.io/owner": "owner-a",
				"unused.example/key":   "large-value",
			},
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "test"}},
		},
	}
	user.SetGroupVersionKind(userv1.GroupVersion.WithKind("User"))

	transformed, err := transformUser(user)
	if err != nil {
		t.Fatalf("transform partial user: %v", err)
	}
	got, ok := transformed.(*metav1.PartialObjectMetadata)
	if !ok {
		t.Fatalf("transformed type = %T, want *metav1.PartialObjectMetadata", transformed)
	}
	if got.Name != user.Name || got.Annotations[userv1.UserAnnotationOwnerKey] != "owner-a" {
		t.Fatalf("metadata was not retained: %#v", got.ObjectMeta)
	}
	if _, ok := got.Annotations["unused.example/key"]; ok || len(got.ManagedFields) != 0 {
		t.Fatalf("unused metadata was retained: %#v", got.ObjectMeta)
	}
}

func TestTransformNamespaceKeepsIndexedFields(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ns-a",
			Labels: map[string]string{"user.sealos.io/owner": "owner-a"},
			Annotations: map[string]string{
				"debt.sealos/status": "Normal",
				"unused.example/key": "large-value",
			},
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "test"}},
		},
		Spec: corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{"kubernetes"}},
		Status: corev1.NamespaceStatus{
			Phase:      corev1.NamespaceActive,
			Conditions: []corev1.NamespaceCondition{{Type: corev1.NamespaceDeletionContentFailure}},
		},
	}

	transformed, err := transformNamespace(ns)
	if err != nil {
		t.Fatalf("transform namespace: %v", err)
	}
	got, ok := transformed.(*corev1.Namespace)
	if !ok {
		t.Fatalf("transformed type = %T, want *corev1.Namespace", transformed)
	}
	if got.Labels["user.sealos.io/owner"] != "owner-a" ||
		got.Annotations["debt.sealos/status"] != "Normal" {
		t.Fatalf("indexed metadata was not retained: %#v", got.ObjectMeta)
	}
	if _, ok := got.Annotations["unused.example/key"]; ok {
		t.Fatalf("unused annotation was retained: %#v", got.Annotations)
	}
	if got.Status.Phase != corev1.NamespaceActive {
		t.Fatalf("phase = %q, want %q", got.Status.Phase, corev1.NamespaceActive)
	}
	if len(got.Spec.Finalizers) != len(ns.Spec.Finalizers) ||
		len(got.Status.Conditions) != 0 ||
		len(got.ManagedFields) != 0 {
		t.Fatalf("unused namespace fields were retained: %#v", got)
	}
}
