/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"update-operator/api/v1alpha1"
	corev1alpha1 "update-operator/api/v1alpha1"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RealTimeAppReconciler reconciles a RealTimeApp object
type RealTimeAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var log = logf.Log.WithName("controller_podset")

func newDeploymentForRealTimeApp(rta *v1alpha1.RealTimeApp, number uint) *appv1.Deployment {

	labels := map[string]string{
		"app": rta.Name,
	}

	image := rta.Spec.Image
	generateName := fmt.Sprintf("%s-%s-%d-", rta.Name, "deployment", number)

	return &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
			Namespace:    rta.Namespace,
			Labels:       labels,
		},
		Spec: appv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:  rta.Name + "-container",
							Image: image,
						},
					},
				},
			},
		},
	}
}

//+kubebuilder:rbac:groups=core.isw.de,resources=realtimeapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.isw.de,resources=realtimeapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.isw.de,resources=realtimeapps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RealTimeApp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *RealTimeAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	// TODO(user): your logic here
	var app corev1alpha1.RealTimeApp
	err := r.Get(ctx, req.NamespacedName, &app)
	if err != nil {
		if errors.IsNotFound(err) {
			// Reqeuest object not found, could have been deleted?
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	lbls := labels.Set{
		"app": app.Name,
	}
	// Get deployment owned by this RealTimeApp
	existingDeployment := &appv1.DeploymentList{}
	err = r.Client.List(context.TODO(), existingDeployment, &client.ListOptions{
		Namespace:     app.Namespace,
		LabelSelector: labels.SelectorFromSet(lbls),
	})

	if err != nil {
		log.Error(err, "failed to fecht list of existing deployments in the application")
		return reconcile.Result{}, err
	}

	if len(existingDeployment.Items) == 1 {
		deploymentNamePrefix := fmt.Sprintf("%s-deployment-", app.Name)
		deployment := existingDeployment.Items[0]
		deploymentNameSuffix := deployment.Name[len(deploymentNamePrefix):]
		deploymentNumber, _ := strconv.ParseUint(strings.Split(deploymentNameSuffix, "-")[0], 10, 64)
		deploymentNumberUint := uint(deploymentNumber)
		deploymentNumberUintNew := uint(0)

		if deploymentNumberUint == 0 {
			deploymentNumberUintNew = 1
		}

		if deployment.Spec.Template.Spec.Containers[0].Image == app.Spec.Image {
			return reconcile.Result{}, nil
		} else {
			log.Info("Updating Image", "OldImage", deployment.Spec.Template.Spec.Containers[0].Image, "NewImage", app.Spec.Image)
			newDeployment := newDeploymentForRealTimeApp(&app, deploymentNumberUintNew)
			controllerutil.SetControllerReference(&app, newDeployment, r.Scheme)
			app.Status.State = "Creating"
			err := r.Status().Update(context.TODO(), &app)
			if err != nil {
				log.Error(err, "Failed to update Realtimeapp Status")
				return ctrl.Result{}, err
			}
			err = r.Create(context.TODO(), newDeployment)
			if err != nil {
				log.Error(err, "Creating the Update-Deployment failed.")
				return reconcile.Result{}, nil
			}
			app.Status.State = "Updating"
			err = r.Status().Update(context.TODO(), &app)
			if err != nil {
				log.Error(err, "Failed to update Realtimeapp Status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	} else if len(existingDeployment.Items) == 2 {
		idx := uint(0)
		idxNew := uint(1)
		if existingDeployment.Items[1].CreationTimestamp.Time.Before(existingDeployment.Items[0].CreationTimestamp.Time) {
			idx = uint(1)
			idxNew = uint(0)
		}
		log.Info("Deleting old Deployment", "Name", existingDeployment.Items[idx].Name)
		app.Status.State = "Deleting"
		err := r.Status().Update(context.TODO(), &app)
		if err != nil {
			log.Error(err, "Failed to update Realtimeapp Status")
			return ctrl.Result{}, err
		}
		err = r.Client.Delete(context.TODO(), &existingDeployment.Items[idx], client.GracePeriodSeconds(5))
		if err != nil {
			log.Error(err, "Failed to delete old deployment")
			return ctrl.Result{}, err
		}
		app.Status.State = "Running"
		app.Status.LastDeployment = existingDeployment.Items[idx].Name
		app.Status.Deployment = existingDeployment.Items[idxNew].Name
		err = r.Status().Update(context.TODO(), &app)
		if err != nil {
			log.Error(err, "Failed to update Realtimeapp Status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else {
		deployment := newDeploymentForRealTimeApp(&app, 0)
		controllerutil.SetControllerReference(&app, deployment, r.Scheme)
		err = r.Create(context.TODO(), deployment)
		if err != nil {
			log.Error(err, "Failed to create a deployment")
			return reconcile.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RealTimeAppReconciler) SetupWithManager(mgr ctrl.Manager) error {

	log.Info("The RT-App-Controller was registered")

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.RealTimeApp{}).
		Complete(r)
}
