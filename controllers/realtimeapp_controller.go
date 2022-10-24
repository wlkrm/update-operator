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
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"update-operator/api/v1alpha1"
	corev1alpha1 "update-operator/api/v1alpha1"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RealTimeAppReconciler reconciles a RealTimeApp object
type RealTimeAppReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	mqttClient mqtt.Client
	done       chan bool
}

var log = logf.Log.WithName("controller_podset")

func newDeploymentForRealTimeApp(rta *v1alpha1.RealTimeApp, number uint) *appv1.Deployment {

	labels := map[string]string{
		"app":    rta.Name,
		"number": strconv.FormatUint(uint64(number), 10),
	}

	podspec := rta.Spec.PodSpec
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
				Spec: podspec,
			},
		},
	}
}

func newServiceForRealTimeApp(rta *v1alpha1.RealTimeApp, number uint) *corev1.Service {

	labels := map[string]string{
		"app":    rta.Name,
		"number": strconv.FormatUint(uint64(number), 10),
	}

	generateName := fmt.Sprintf("%s-%s-%d-", rta.Name, "service", number)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
			Namespace:    rta.Namespace,
			Labels:       labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

//+kubebuilder:rbac:groups=core,resources=deployments;services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments;services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;ingresses/status,verbs=get;list;watch;create;update;patch;delete
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

	exisitingServices := &corev1.ServiceList{}
	err = r.Client.List(context.TODO(), exisitingServices, &client.ListOptions{
		Namespace:     app.Namespace,
		LabelSelector: labels.SelectorFromSet(lbls),
	})
	if err != nil {
		log.Error(err, "failed to fetch list of existing services")
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

		if deployment.Spec.Template.Spec.Containers[0].Image == app.Spec.PodSpec.Containers[0].Image {
			return reconcile.Result{}, nil
		} else {
			log.Info("Updating Image", "OldImage", deployment.Spec.Template.Spec.Containers[0].Image, "NewImage", app.Spec.PodSpec.Containers[0].Image)
			newDeployment := newDeploymentForRealTimeApp(&app, deploymentNumberUintNew)
			newService := newServiceForRealTimeApp(&app, deploymentNumberUintNew)
			controllerutil.SetControllerReference(&app, newDeployment, r.Scheme)
			controllerutil.SetControllerReference(&app, newService, r.Scheme)
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
			err = r.Create(context.TODO(), newService)
			if err != nil {
				log.Error(err, "Creating the Update-Service failed.")
				return reconcile.Result{}, nil
			}
			app.Status.State = "Updating"
			app.Status.Deployment = newDeployment.Name
			app.Status.LastDeployment = deployment.Name
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
		log.Info("Updating new Deployment", "Name", existingDeployment.Items[idx].Name)
		app.Status.State = "Updating"
		err := r.Status().Update(context.TODO(), &app)
		if err != nil {
			log.Error(err, "Failed to update Realtimeapp Status")
			return ctrl.Result{}, err
		}

		oldName := fmt.Sprintf("%s-%d", app.Name, idx)
		newName := fmt.Sprintf("%s-%d", app.Name, idxNew)
		runUpdate(r, oldName, newName)

		log.Info("Deleting new Deployment", "Name", existingDeployment.Items[idx].Name)
		app.Status.State = "Deleting"
		err = r.Status().Update(context.TODO(), &app)
		if err != nil {
			log.Error(err, "Failed to update Realtimeapp Status")
			return ctrl.Result{}, err
		}

		err = r.Client.Delete(context.TODO(), &existingDeployment.Items[idx], client.GracePeriodSeconds(5))
		if err != nil {
			log.Error(err, "Failed to delete old deployment")
			return ctrl.Result{}, err
		}
		err = r.Client.Delete(context.TODO(), &exisitingServices.Items[idx], client.GracePeriodSeconds(5))
		if err != nil {
			log.Error(err, "Failed to delete old service")
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
		service := newServiceForRealTimeApp(&app, 0)
		controllerutil.SetControllerReference(&app, deployment, r.Scheme)
		controllerutil.SetControllerReference(&app, service, r.Scheme)

		err = r.Create(context.TODO(), deployment)
		if err != nil {
			log.Error(err, "Failed to create a deployment")
			return reconcile.Result{}, err
		}
		err = r.Create(context.TODO(), service)
		if err != nil {
			log.Error(err, "Failed to create a service")
			return reconcile.Result{}, err
		}
		app.Status.Deployment = deployment.Name
		app.Status.State = "Running"
		log.Info("Updating Status")
		err := r.Status().Update(context.TODO(), &app)
		if err != nil {
			log.Error(err, "Failed to update Realtimeapp Status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}
}

type RtAppState struct {
	State string `json:"state"`
}

type RtAppAction struct {
	Action    string `json:"action"`
	G_code    string `json:"g_code"`
	SyncBlock uint64 `json:"syncBlock"`
}

type RtAppUpdate struct {
	State     string `json:"state"`
	SyncBlock uint64 `json:"syncBlock"`
}

func runUpdate(r *RealTimeAppReconciler, oldAppName string, newAppName string) {
	a := make(chan bool)
	consumeUpdateReady := make(chan bool)
	providUpdateReady := make(chan bool)
	doneReady := make(chan bool)
	oldNotRunning := make(chan bool)

	// Wait for New App to Come Online
	var resp RtAppUpdate
	r.mqttClient.Subscribe("rtapps/"+oldAppName+"/update", 1, func(client mqtt.Client, msg mqtt.Message) {
		err := json.Unmarshal(msg.Payload(), &resp)
		if err != nil {
			log.Info("Error 1")
		}
		log.Info("Old App provided Update")
		select {
		case providUpdateReady <- true:
		default:
		}
	})

	r.mqttClient.Subscribe("rtapps/"+newAppName+"/state", 1, func(client mqtt.Client, msg mqtt.Message) {
		var state RtAppState
		err := json.Unmarshal(msg.Payload(), &state)
		if err == nil {
			if state.State == "CONSUMEUPDATE" {
				select {
				case consumeUpdateReady <- true:
				default:
				}

			}
		}
		select {
		case a <- true:
		default:
		}

	})

	y := func() func() bool {
		done := false

		return func() bool {
			ret := done
			done = true
			return ret
		}
	}

	isDone := y()
	r.mqttClient.Subscribe("rtapps/"+oldAppName+"/state", 1, func(client mqtt.Client, msg mqtt.Message) {

		var state RtAppState
		err := json.Unmarshal(msg.Payload(), &state)
		if err == nil {
			if state.State == "DONE" {
				if !isDone() {
					log.Info("Waiting on done")
					doneReady <- true
				}
			} else if state.State != "OP" {
				select {
				case oldNotRunning <- true:
				default:
				}
			} else {
				select {
				case oldNotRunning <- false:
				default:
				}
			}
		}
	})

	<-a
	x := <-oldNotRunning
	if x {
		r.mqttClient.Unsubscribe("rtapps/"+oldAppName+"/update", "rtapps/"+newAppName+"/state", "rtapps/"+oldAppName+"/state")
		return
	}
	// Request Update from Old App
	log.Info("Requesting Update from Old App")
	req := &RtAppAction{Action: "PROVIDEUPDATE", G_code: "", SyncBlock: 0}
	reqJson, err := json.Marshal(req)
	if err != nil {
		log.Info(err.Error())
	}
	if token := r.mqttClient.Publish("rtapps/"+oldAppName+"/action", 1, false, string(reqJson)); token.Wait() && token.Error() != nil {
		log.Info(token.Error().Error())
	}

	log.Info("Requested Update from Old App")
	<-providUpdateReady
	// Await Update from Old App

	// Request Consumeupdate
	log.Info("Requesting Consume Update to New App")
	req2 := RtAppAction{Action: "CONSUMEUPDATE", G_code: resp.State, SyncBlock: resp.SyncBlock}
	req2Json, err := json.Marshal(req2)
	if err != nil {
		log.Info(err.Error())
	}
	if token := r.mqttClient.Publish("rtapps/"+newAppName+"/action", 1, false, string(req2Json)); token.Wait() && token.Error() != nil {
		log.Info(token.Error().Error())
	}

	<-doneReady

	log.Info("Requesting Continue to New App")
	req3 := RtAppAction{Action: "MCM_PROCESS_ACTIVE"}
	req3Json, err := json.Marshal(req3)
	if err != nil {
		log.Info(err.Error())
	}
	if token := r.mqttClient.Publish("rtapps/"+newAppName+"/ncaction", 1, false, string(req3Json)); token.Wait() && token.Error() != nil {
		log.Info(token.Error().Error())
	}

	r.mqttClient.Unsubscribe("rtapps/"+oldAppName+"/update", "rtapps/"+newAppName+"/state", "rtapps/"+oldAppName+"/state")
}

func listen(r *RealTimeAppReconciler, topic string) {
	r.mqttClient.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("* [%s] %s\n", msg.Topic(), string(msg.Payload()))
		log.Info("The RT-App-Controller was registered")
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *RealTimeAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var broker = os.Getenv("MQTT_URL")
	var port = 1883
	log.Info("Connecting to MQTT_URL: " + broker)
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetReconnectingHandler(func(c mqtt.Client, options *mqtt.ClientOptions) {
		log.Info("...... mqtt reconnecting ......")
	})

	r.mqttClient = mqtt.NewClient(opts)

	token := r.mqttClient.Connect()
	for !token.WaitTimeout(1000 * time.Second) {
	}
	if err := token.Error(); err != nil {
		log.Info("Connection to Broker Failed")
	} else {
		log.Info("Connection to Broker succeeded")
	}

	log.Info("The RT-App-Controller was registered")

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.RealTimeApp{}).
		Complete(r)
}
