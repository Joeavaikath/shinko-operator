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

package controller

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Joeavaikath/shinko-operator/api/v1alpha1"
	"github.com/pingcap/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// ShinkoDBReconciler reconciles a ShinkoDB object
type ShinkoDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=app.shinko.io,resources=shinkodbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.shinko.io,resources=shinkodbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.shinko.io,resources=shinkodbs/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ShinkoDB object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *ShinkoDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling ShinkoDb", "name", req.Name, "namespace", req.Namespace)

	// fetch db instance
	// 1️⃣ Fetch the ShinkoDb instance
	shinkoDB := &v1alpha1.ShinkoDB{}
	err := r.Get(ctx, req.NamespacedName, shinkoDB)
	if err != nil {
		if errors.IsNotFound(err) {
			// If deleted, nothing to do
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// ensure statefulset
	// 2️⃣ Ensure Deployment exists and matches the spec
	err = r.ensureStatefulSet(ctx, shinkoDB)
	if err != nil {
		log.Error(err, "Failed to create/update Statefulset")
		return ctrl.Result{}, err
	}

	// Ensure Service exists
	err = r.ensureService(ctx, shinkoDB)
	if err != nil {
		log.Error(err, "Failed to create/update Service")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// ensureStatefulSet ensures that the PostgreSQL StatefulSet exists
func (r *ShinkoDBReconciler) ensureStatefulSet(ctx context.Context, shinkoDB *v1alpha1.ShinkoDB) error {
	log := ctrl.Log.WithName("ShinkoDBReconciler")

	// Define the StatefulSet
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres",
			Namespace: shinkoDB.Namespace,
			Labels:    map[string]string{"app": shinkoDB.Name},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "postgres",
			Replicas:    &shinkoDB.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": shinkoDB.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": shinkoDB.Name},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "shinko-operator-controller-manager",
					Containers: []corev1.Container{
						{
							Name:  "postgres",
							Image: "postgres:15",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 5432},
							},
							Env: []corev1.EnvVar{
								{Name: "POSTGRES_USER", Value: "joeav"},
								{Name: "POSTGRES_PASSWORD", Value: "postgres"},
								{Name: "POSTGRES_DB", Value: "shinko"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "postgres-data", MountPath: "/var/lib/postgresql/data"},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "postgres-data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
		},
	}

	// Set owner reference to ensure garbage collection
	err := ctrl.SetControllerReference(shinkoDB, statefulSet, r.Scheme)
	if err != nil {
		return err
	}

	// Check if the StatefulSet already exists
	found := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: statefulSet.Name, Namespace: statefulSet.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// StatefulSet does not exist, create it
		log.Info("Creating new PostgreSQL StatefulSet", "Namespace", shinkoDB.Namespace)
		return r.Create(ctx, statefulSet)
	} else if err != nil {
		return err
	}

	// Update if needed
	if !reflect.DeepEqual(statefulSet.Spec, found.Spec) {
		log.Info("Updating existing PostgreSQL StatefulSet", "Namespace", shinkoDB.Namespace)
		found.Spec = statefulSet.Spec
		return r.Update(ctx, found)
	}

	return nil
}

func (r *ShinkoDBReconciler) ensureService(ctx context.Context, shinkoDB *v1alpha1.ShinkoDB) error {
	log := log.FromContext(ctx)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shinkoDB.Name + "-service",
			Namespace: shinkoDB.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": shinkoDB.Name},
			Ports: []corev1.ServicePort{
				{
					Name:       "postgres",
					Port:       5432,                 // External port
					TargetPort: intstr.FromInt(5432), // App container port
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	// Set the controller reference
	err := ctrl.SetControllerReference(shinkoDB, svc, r.Scheme)
	if err != nil {
		return err
	}

	// Check if the service exists
	found := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, svc)
	} else if err != nil {
		return err
	}

	// Ensure the service spec is up-to-date
	if !reflect.DeepEqual(svc.Spec, found.Spec) {
		found.Spec = svc.Spec
		log.Info("Service updated")
		return r.Update(ctx, found)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ShinkoDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ShinkoDB{}).
		Named("shinkodb").
		Complete(r)
}
