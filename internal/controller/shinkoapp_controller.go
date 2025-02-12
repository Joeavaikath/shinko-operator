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
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ShinkoAppReconciler reconciles a ShinkoApp object
type ShinkoAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=app.shinko.io,resources=shinkoapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.shinko.io,resources=shinkoapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.shinko.io,resources=shinkoapps/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ShinkoApp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *ShinkoAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling ShinkoApp", "name", req.Name, "namespace", req.Namespace)

	// 1️⃣ Fetch the ShinkoApp instance
	shinkoApp := &v1alpha1.ShinkoApp{}
	err := r.Get(ctx, req.NamespacedName, shinkoApp)
	if err != nil {
		if errors.IsNotFound(err) {
			// If deleted, nothing to do
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// 2️⃣ Ensure Deployment exists and matches the spec
	err = r.ensureDeployment(ctx, shinkoApp)
	if err != nil {
		log.Error(err, "Failed to create/update Deployment")
		return ctrl.Result{}, err
	}

	// Ensure Service exists
	err = r.ensureService(ctx, shinkoApp)
	if err != nil {
		log.Error(err, "Failed to create/update Service")
		return ctrl.Result{}, err
	}

	// 3️⃣ Check if image needs an update (auto-update enabled)
	if shinkoApp.Spec.AutoUpdate {
		err = r.checkAndUpdateImage(ctx, shinkoApp)
		if err != nil {
			log.Error(err, "Failed to update Deployment image")
			return ctrl.Result{}, err
		}
	}

	// 4️⃣ Update the status fields
	if shinkoApp.Status.CurrentAppImage != shinkoApp.Spec.AppImage {
		shinkoApp.Status.CurrentAppImage = shinkoApp.Spec.AppImage
		err = r.Status().Update(ctx, shinkoApp)
		if err != nil {
			log.Error(err, "Failed to update ShinkoApp status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ShinkoAppReconciler) ensureDeployment(ctx context.Context, shinkoApp *v1alpha1.ShinkoApp) error {
	log := log.FromContext(ctx)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shinkoApp.Name,
			Namespace: shinkoApp.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &shinkoApp.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": shinkoApp.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": shinkoApp.Name},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "shinko-operator-controller-manager",
					Containers: []corev1.Container{
						{
							Name:  "shinko-app",
							Image: shinkoApp.Spec.AppImage,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "DB_URL",
									Value: "postgres://joeav:postgres@shinko-db-service:5432/shinko?sslmode=disable",
								},
								{
									Name:  "JWT_SECRET",
									Value: "FxsUyXOKhQLNWT+RTRBvT2jlCxRBSzVHb/CiOet3KO2tKLqIwlPBQDwklT4K0KCB7KzXPaBiokOh8v86X6QZeQ==",
								},
								{
									Name:  "PLATFORM",
									Value: "dev",
								},
							},
						},
					},
				},
			},
		},
	}

	// Set controller ownership so the operator manages this resource
	err := ctrl.SetControllerReference(shinkoApp, deployment, r.Scheme)
	if err != nil {
		return err
	}

	// Check if Deployment exists
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: shinkoApp.Name, Namespace: shinkoApp.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Deployment does not exist, create it
		return r.Create(ctx, deployment)
	} else if err != nil {
		return err
	}

	// Update Deployment if spec has changed
	if !reflect.DeepEqual(deployment.Spec, found.Spec) {
		found.Spec = deployment.Spec
		log.Info("Deployment updated")
		return r.Update(ctx, found)
	}

	return nil
}

func (r *ShinkoAppReconciler) ensureService(ctx context.Context, shinkoApp *v1alpha1.ShinkoApp) error {
	log := log.FromContext(ctx)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shinkoApp.Name + "-service",
			Namespace: shinkoApp.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": shinkoApp.Name},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,                   // External port
					TargetPort: intstr.FromInt(8080), // App container port
				},
			},
			Type: corev1.ServiceTypeLoadBalancer, // Expose externally
		},
	}

	// Set the controller reference
	err := ctrl.SetControllerReference(shinkoApp, svc, r.Scheme)
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

func (r *ShinkoAppReconciler) checkAndUpdateImage(ctx context.Context, shinkoApp *v1alpha1.ShinkoApp) error {
	log := log.FromContext(ctx)
	latestImage, err := getLatestImageFromQuay("joeavaik/shinko-app") // todo: make this a spec field
	if err != nil {
		return err
	}

	// If the image is already up-to-date, return
	if latestImage == shinkoApp.Status.CurrentAppImage {
		return nil
	}

	// Fetch the existing deployment
	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: shinkoApp.Name, Namespace: shinkoApp.Namespace}, deployment)
	if err != nil {
		return err
	}

	// Update the deployment's container image
	// Update the current status in the CRD object
	log.Info(fmt.Sprintf("Old image: %s", shinkoApp.Status.CurrentAppImage))
	deployment.Spec.Template.Spec.Containers[0].Image = latestImage
	err = r.Update(ctx, deployment)
	log.Info(fmt.Sprintf("Image updated. New image: %s", latestImage))
	if err != nil {
		return err
	}

	// Update the CRD status with the new image
	shinkoApp.Status.CurrentAppImage = latestImage
	shinkoApp.Status.LastImageCheck = time.Now().Format(time.RFC3339)
	return r.Status().Update(ctx, shinkoApp)
}

func getLatestImageFromQuay(repo string) (string, error) {
	url := fmt.Sprintf("https://quay.io/api/v1/repository/%s/tag/?limit=1", repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data struct {
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if len(data.Tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}

	return fmt.Sprintf("quay.io/%s:%s", repo, data.Tags[0].Name), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ShinkoAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ShinkoApp{}).
		Named("shinkoapp").
		Complete(r)
}
