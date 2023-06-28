package main

import (
	"context"
	"fmt"
	"time"

	argocd "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(argocd.AddToScheme(scheme))
}

type KubeClients struct {
	kubeClient *kubernetes.Clientset
	crClient   crclient.Client
}

func NewKubeClient() (*KubeClients, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	kclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	crClient, err := crclient.New(cfg, crclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	return &KubeClients{
		kubeClient: kclient,
		crClient:   crClient,
	}, nil
}

func (k *KubeClients) Clientset() *kubernetes.Clientset {
	return k.kubeClient
}

func (k *KubeClients) REST() crclient.Client {
	return k.crClient
}

func (k *KubeClients) ListArgoCDApps() (*argocd.ApplicationList, error) {
	//labels := map[string]string{"app.kubernetes.io/instance": "all-components-staging"}

	list := &argocd.ApplicationList{}
	err := k.REST().List(context.TODO(), list, &crclient.ListOptions{})
	if err != nil {
		return nil, err
	}

	return list, nil
}

func (k *KubeClients) ListArgoCDAppsByLabels(labelMap map[string]string) (*argocd.ApplicationList, error) {

	list := &argocd.ApplicationList{}
	err := k.crClient.List(context.TODO(), list, &crclient.ListOptions{LabelSelector: labels.SelectorFromSet(labelMap)})

	if err != nil {
		return nil, err
	}

	return list, nil
}

func (k *KubeClients) GetImagesFromArgoCDApp(app *argocd.Application) ([]string, error) {
	images := []string{}
	images = append(images, app.Status.Summary.Images...)
	return images, nil
}

func (k *KubeClients) GetImagesFromArgoCDAppList(list *argocd.ApplicationList) ([]string, error) {
	images := []string{}

	for _, app := range list.Items {
		images = append(images, app.Status.Summary.Images...)
	}

	return images, nil
}

func (k *KubeClients) ListDeploymentsByLabels(label string) (*appsv1.DeploymentList, error) {

	deploymentList, err := k.kubeClient.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{LabelSelector: label})

	if err != nil {
		return nil, err
	}

	return deploymentList, nil
}

func (k *KubeClients) GetConfigMap(name string, namespace string) (*v1.ConfigMap, error) {

	configMap, err := k.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	return configMap, nil
}

func (k *KubeClients) IsDeploymentActiveSince(deployment *appsv1.Deployment) (bool, metav1.Time) {
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == "Available" && condition.Status == "True" {
			return true, condition.LastUpdateTime
		}
	}
	return false, metav1.Time{}
}

func (k *KubeClients) GetDeploymentReplicaSetCreationTime(namespace string, owner string, image string) (metav1.Time, error) {

	rsList, err := k.kubeClient.AppsV1().ReplicaSets(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return metav1.Time{}, err
	}

	for _, rs := range rsList.Items {
		if rs.OwnerReferences[0].Name == owner && (rs.Status.AvailableReplicas == rs.Status.Replicas) {
			for _, cont := range rs.Spec.Template.Spec.Containers {
				if cont.Image == image {
					return rs.ObjectMeta.CreationTimestamp, nil
				}
			}
		}
	}
	return metav1.Time{}, fmt.Errorf("no replicaset found for %s or replicas are not available", image)
}

func (k *KubeClients) waitConfigMapAvailable(name string, namespace string) error {
	for {
		_, err := k.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err == nil {
			klog.V(1).Infof("ConfigMap found: %s \n", name)
			return nil
		}
		if !errors.IsNotFound(err) {
			klog.V(1).Infof("Error getting the configmap: %s \n", name)
			return nil
		}
		klog.V(1).Infof("ConfigMap not available: %s \n", name)
		time.Sleep(5 * time.Second)
	}
}

func (k *KubeClients) getGHTokenFromSecret(name string, namespace string, dataKey string) (string, error) {
	ctx := context.TODO()
	secret, err := k.kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secret.Data[dataKey]), nil
}
