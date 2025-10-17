/*
Copyright 2025 The Crossplane Authors.

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

package spotadvisordata

import (
	"context"
	"fmt"
	"time"

	"github.com/Oded-B/ec2offering-crossplane-provider/apis/ec2/v1alpha1"
	"github.com/Oded-B/ec2offering-crossplane-provider/internal/features"

	"github.com/crossplane/crossplane-runtime/pkg/feature"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/statemetrics"

	apisv1alpha1 "github.com/Oded-B/ec2offering-crossplane-provider/apis/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	errNotSpotAdvisorData = "managed resource is not a SpotAdvisorData custom resource"
	errTrackPCUsage       = "cannot track ProviderConfig usage"
	errGetPC              = "cannot get ProviderConfig"
	errGetCreds           = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

// A NoOpService does nothing.
type NoOpService struct{}

var newNoOpService = func(_ []byte) (interface{}, error) { return &NoOpService{}, nil }

// Setup adds a controller that reconciles SpotAdvisorData managed resources.
// +kubebuilder:rbac:groups=ec2.ec2offering.crossplane.io,resources=spotadvisordatas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ec2.ec2offering.crossplane.io,resources=spotadvisordatas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ec2.ec2offering.crossplane.io,resources=spotadvisordatas/finalizers,verbs=update
// +kubebuilder:rbac:groups=ec2offering.crossplane.io,resources=providerconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=ec2offering.crossplane.io,resources=providerconfigs/status,verbs=get
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.SpotAdvisorDataGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: newNoOpService,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithManagementPolicies(),
	}

	if o.Features.Enabled(feature.EnableAlphaChangeLogs) {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
	}

	if o.MetricOptions != nil && o.MetricOptions.MRStateMetrics != nil {
		stateMetricsRecorder := statemetrics.NewMRStateRecorder(
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.SpotAdvisorDataList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.SpotAdvisorDataList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.SpotAdvisorDataGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.SpotAdvisorData{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(creds []byte) (interface{}, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.SpotAdvisorData)
	if !ok {
		return nil, errors.New(errNotSpotAdvisorData)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	// Get region from the managed resource spec
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cr.Spec.ForProvider.AWSRegion))
	if err != nil {
		return nil, errors.Wrap(err, "failed to load AWS config")
	}
	ec2Client := ec2.NewFromConfig(cfg)

	cd := pc.Spec.Credentials
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	svc, err := c.newServiceFn(data)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc, ec2Client: *ec2Client}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service   interface{}
	ec2Client ec2.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.SpotAdvisorData)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotSpotAdvisorData)
	}

	fmt.Printf("Observing SpotAdvisorData: %+v(NS:%+v)", cr.GetName(), cr.GetNamespace())

	// For now, just set some placeholder data
	// TODO: Implement actual spot advisor data fetching logic
	cr.Status.AtProvider.Data = fmt.Sprintf("Spot advisor data for region: %s", cr.Spec.ForProvider.AWSRegion)

	// Time the operation
	startTime := time.Now()
	// Simulate some processing time
	time.Sleep(100 * time.Millisecond)
	duration := time.Since(startTime)

	// Log the timing information
	fmt.Printf("SpotAdvisorData observation took %v for region %s\n", duration, cr.Spec.ForProvider.AWSRegion)

	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: true,

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
	cr, ok := mg.(*v1alpha1.SpotAdvisorData)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotSpotAdvisorData)
	}

	fmt.Printf("Creating SpotAdvisorData: %+v", cr)

	// For read-only resources like spot advisor data, Create is typically a no-op
	// The actual data fetching happens in the Observe function
	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.SpotAdvisorData)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotSpotAdvisorData)
	}

	fmt.Printf("Updating SpotAdvisorData: %+v", cr)

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.SpotAdvisorData)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotSpotAdvisorData)
	}

	fmt.Printf("Deleting SpotAdvisorData: %+v", cr)

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	return nil
}
