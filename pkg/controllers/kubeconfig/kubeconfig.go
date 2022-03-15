package kubeconfig

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/cluster-capi-operator/pkg/controllers"
	"github.com/openshift/cluster-capi-operator/pkg/operatorstatus"
)

const (
	serviceAccountName = "cluster-capi-operator"
)

// ClusterReconciler reconciles a ClusterOperator object
type KubeconfigReconciler struct {
	operatorstatus.ClusterOperatorStatusClient
	Scheme             *runtime.Scheme
	RestCfg            *rest.Config
	SupportedPlatforms map[string]bool
	clusterName        string
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeconfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		Complete(r)
}

func (r *KubeconfigReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	infra := &configv1.Infrastructure{}
	if err := r.Get(ctx, client.ObjectKey{Name: controllers.InfrastructureResourceName}, infra); err != nil {
		klog.Errorf("Unable to retrive Infrastructure object: %v", err)
		if err := r.SetStatusDegraded(ctx, err); err != nil {
			return ctrl.Result{}, fmt.Errorf("error syncing ClusterOperatorStatus: %v", err)
		}
		return ctrl.Result{}, err
	}

	if infra.Status.PlatformStatus == nil {
		klog.Infof("No platform status exists in infrastructure object. Skipping kubeconfig reconciliation...")
		if err := r.SetStatusAvailable(ctx); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	r.clusterName = infra.Status.InfrastructureName

	// If the platform type is not supported, we should skip cluster reconciliation.
	if _, ok := r.SupportedPlatforms[strings.ToLower(string(infra.Status.PlatformStatus.Type))]; !ok {
		klog.Infof("Platform type %v is not supported. Skipping kubeconfig reconciliation...", infra.Status.PlatformStatus.Type)
		if err := r.SetStatusAvailable(ctx); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	klog.Infof("Reconciling kubeconfig secret")
	if err := r.reconcileKubeconfig(ctx); err != nil {
		klog.Errorf("Error reconciling kubeconfig: %v", err)
		if err := r.SetStatusDegraded(ctx, err); err != nil {
			return ctrl.Result{}, fmt.Errorf("error syncing ClusterOperatorStatus: %v", err)
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.SetStatusAvailable(ctx)
}

func (r *KubeconfigReconciler) reconcileKubeconfig(ctx context.Context) error {
	// Get service account for cluster-capi-operator
	serviceAccount := &corev1.ServiceAccount{}
	saKey := client.ObjectKey{
		Name:      serviceAccountName,
		Namespace: controllers.DefaultManagedNamespace,
	}
	if err := r.Get(ctx, saKey, serviceAccount); err != nil {
		klog.Errorf("Unable to retrieve ServiceAccount: %v", err)
		return fmt.Errorf("error retrieving ServiceAccount %s: %v", serviceAccountName, err)
	}

	// Get secret that contains token and ca data
	var tokenSecretRef *corev1.ObjectReference
	prefix := fmt.Sprintf("%s-token", serviceAccountName)
	for i, secretRef := range serviceAccount.Secrets {
		if strings.HasPrefix(secretRef.Name, prefix) {
			tokenSecretRef = &serviceAccount.Secrets[i]
		}
	}

	if tokenSecretRef == nil {
		klog.Errorf("Unable to find token secret for service account %s", serviceAccountName)
		return fmt.Errorf("unable to find token secret for service account %s", serviceAccountName)
	}

	// Get the token secret
	tokenSecret := &corev1.Secret{}
	tokenSecretKey := client.ObjectKey{
		Name:      tokenSecretRef.Name,
		Namespace: controllers.DefaultManagedNamespace,
	}
	if err := r.Get(ctx, tokenSecretKey, tokenSecret); err != nil {
		klog.Errorf("Unable to retrieve Secret object: %v", err)
		return fmt.Errorf("unable to retrieve Secret object: %v", err)
	}

	// Generate kubeconfig
	kubeconfig, err := generateKubeconfig(kubeconfigOptions{
		token:            tokenSecret.Data["token"],
		caCert:           tokenSecret.Data["ca.crt"],
		apiServerEnpoint: r.RestCfg.Host,
		clusterName:      r.clusterName,
	})

	if err != nil {
		return fmt.Errorf("error generating kubeconfig: %v", err)
	}

	// Create a secret with generated kubeconfig
	out, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return fmt.Errorf("error writing kubeconfig: %v", err)
	}

	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kubeconfig", r.clusterName),
			Namespace: controllers.DefaultManagedNamespace,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: r.clusterName,
			},
		},
		Data: map[string][]byte{
			"value": out,
		},
		Type: clusterv1.ClusterSecretType,
	}

	kubeconfigSecretCopy := kubeconfigSecret.DeepCopy()
	if _, err := controllerutil.CreateOrPatch(ctx, r.Client, kubeconfigSecret, func() error {
		kubeconfigSecret.ObjectMeta = kubeconfigSecretCopy.ObjectMeta
		kubeconfigSecret.Data = kubeconfigSecretCopy.Data
		kubeconfigSecret.Type = kubeconfigSecretCopy.Type
		return nil
	}); err != nil {
		return fmt.Errorf("error reconciling kubeconfig secret: %v", err)
	}

	return nil
}
