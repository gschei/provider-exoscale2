/*
Copyright 2022 The Crossplane Authors.

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

package exoscalesks

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/crossplane/provider-exoscale2/internal/controller/exoapi"
	exo "github.com/exoscale/egoscale/v2"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-exoscale2/apis/exoscale2/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-exoscale2/apis/v1alpha1"
	"github.com/crossplane/provider-exoscale2/internal/features"
)

const (
	errNotExoscaleSKS = "managed resource is not a ExoscaleSKS custom resource"
	errTrackPCUsage   = "cannot track ProviderConfig usage"
	errGetPC          = "cannot get ProviderConfig"
	errGetCreds       = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

const (
	exoApiKey    = "EXOSCALE_API_KEY"
	exoApiSecret = "EXOSCALE_API_SECRET"
)

type ExoscaleSKSController struct{}

var log logging.Logger

// Setup adds a controller that reconciles ExoscaleSKS managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	log = o.Logger
	log.Info("Setting up ExoscaleSKS Controller")

	name := managed.ControllerName(v1alpha1.ExoscaleSKSGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ExoscaleSKSGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithTimeout(10*time.Minute))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.ExoscaleSKS{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube  client.Client
	usage resource.Tracker
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	log.Info("entering connect")

	cr, ok := mg.(*v1alpha1.ExoscaleSKS)
	if !ok {
		return nil, errors.New(errNotExoscaleSKS)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	//fmt.Printf("  providerconfig found %v \n", pc.Spec.Credentials.APISecretRef.Name)

	secret := &corev1.Secret{}

	c.kube.Get(ctx, client.ObjectKey{
		Namespace: pc.Spec.Credentials.APISecretRef.Namespace,
		Name:      pc.Spec.Credentials.APISecretRef.Name,
	}, secret)

	exoKey, err := parseSecretKey(secret, exoApiKey)
	if err != nil {
		return nil, err
	}

	exoSecret, err := parseSecretKey(secret, exoApiSecret)
	if err != nil {
		return nil, err
	}

	return &external{
		exoApiKey:    exoKey,
		exoApiSecret: exoSecret,
	}, nil
}

func parseSecretKey(secret *corev1.Secret, key string) (string, error) {
	data := secret.Data[key]
	if data == nil {
		return "", errors.New(fmt.Sprintf("Key %v not found in secret %v", key, secret.Name))
	}

	value, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return "", errors.New(fmt.Sprintf("Cannot decode %v: %v", key, err))
	}
	return string(value), nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	exoApiKey    string
	exoApiSecret string
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	log.Info(fmt.Sprintf("Entering Observe for %v", mg.GetName()))

	cr, ok := mg.(*v1alpha1.ExoscaleSKS)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotExoscaleSKS)
	}

	// These fmt statements should be removed in the real implementation.
	//fmt.Printf("Observing: %+v", cr)

	clusters, err := exoapi.RetrieveClusters(c.exoApiKey, c.exoApiSecret, cr.Spec.ForProvider.Zone)
	if err != nil {
		return managed.ExternalObservation{}, errors.New(fmt.Sprintf("failed to retrieve sks clusters from exoscale: %v", err))
	}

	// todo: also check zone
	var found bool = false
	for _, c := range *&clusters.Clusters {
		if c.Name == cr.Name {
			found = true
		}
	}

	log.Info(fmt.Sprintf("Observe: result for %v: found %v", mg.GetName(), found))

	exoClient, err := exo.NewClient(c.exoApiKey, c.exoApiSecret)
	if err != nil {
		return managed.ExternalObservation{}, errors.New(fmt.Sprintf("Error creating Exoscale Client: %v", err))
	}

	found = true
	skscluster, err := exoClient.FindSKSCluster(ctx, cr.Spec.ForProvider.Zone, cr.Name)
	if err != nil {
		if err.Error() == "resource not found" {
			found = false
			log.Info(fmt.Sprintf("Observe: result for %v: found false", mg.GetName()))
		} else {
			return managed.ExternalObservation{}, errors.New(fmt.Sprintf("Error retrieving Exoscale SKS Cluster Info: %v", err))
		}
	} else {
		log.Info(fmt.Sprintf("Observe: result for %v: found %v", *skscluster.Name, *skscluster.ID))
	}

	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: found,

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: true,

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	log.Info(fmt.Sprintf("Entering Create for %v", mg.GetName()))

	cr, ok := mg.(*v1alpha1.ExoscaleSKS)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotExoscaleSKS)
	}

	//fmt.Printf("Creating: %+v", cr)

	exoClient, err := exo.NewClient(c.exoApiKey, c.exoApiSecret)
	if err != nil {
		return managed.ExternalCreation{}, errors.New(fmt.Sprintf("Error creating Exoscale Client: %v", err))
	}

	version := "1.28.4"
	skscluster := exo.SKSCluster{
		Name:         &cr.Spec.ForProvider.Name,
		ServiceLevel: &cr.Spec.ForProvider.ServiceLevel,
		Version:      &version,
		CNI:          &cr.Spec.ForProvider.Cni,
		Zone:         &cr.Spec.ForProvider.Zone,
	}

	dl, ok := ctx.Deadline()
	log.Info(fmt.Sprintf("deadline info: time: %v ok %v", dl, ok))

	log.Info(fmt.Sprintf("starting creation of SKS Cluster %v in zone %v", *skscluster.Name, *skscluster.Zone))

	sksclusteroutput, err := exoClient.CreateSKSCluster(context.Background(), cr.Spec.ForProvider.Zone, &skscluster)
	if err != nil {
		log.Info(fmt.Sprintf("Error creating SKS Cluster %v", err))
		return managed.ExternalCreation{}, errors.New(fmt.Sprintf("Error creating Exoscale SKS Cluster: %v", err))
	}

	log.Info("successfully created SKS Cluster %v", *sksclusteroutput.ID)

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	log.Info(fmt.Sprintf("Entering Update for %v", mg.GetName()))

	cr, ok := mg.(*v1alpha1.ExoscaleSKS)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotExoscaleSKS)
	}

	fmt.Printf("Updating: %+v", cr)

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	log.Info(fmt.Sprintf("Entering Delete for %v", mg.GetName()))

	cr, ok := mg.(*v1alpha1.ExoscaleSKS)
	if !ok {
		return errors.New(errNotExoscaleSKS)
	}

	fmt.Printf("Deleting: %+v", cr)

	return nil
}
