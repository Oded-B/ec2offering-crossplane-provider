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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	errFetchSpotData      = "cannot fetch spot advisor data"
	errParseSpotData      = "cannot parse spot advisor data"

	errNewClient = "cannot create new Service"

	spotAdvisorDataURL = "https://spot-bid-advisor.s3.amazonaws.com/spot-advisor-data.json"
)

// SpotAdvisorResponse represents the structure of the spot advisor data from AWS
type SpotAdvisorResponse struct {
	InstanceTypes map[string]InstanceTypeInfo                 `json:"instance_types"`
	SpotAdvisor   map[string]map[string]map[string]RegionInfo `json:"spot_advisor"`
	GlobalRate    string                                      `json:"global_rate"`
	Ranges        []RangeInfo                                 `json:"ranges"`
}

// InstanceTypeInfo represents instance type information from the API
type InstanceTypeInfo struct {
	EMR   bool    `json:"emr"`
	Cores int     `json:"cores"`
	RAMGB float64 `json:"ram_gb"`
}

// RegionInfo represents spot advisor data for a specific region
type RegionInfo struct {
	S int `json:"s"` // Spot interruption frequency
	R int `json:"r"` // Spot interruption rate
}

// RangeInfo represents range information from the API
type RangeInfo struct {
	Index int    `json:"index"`
	Label string `json:"label"`
	Dots  int    `json:"dots"`
	Max   int    `json:"max"`
}

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

	// Time the operation
	startTime := time.Now()

	// Fetch spot advisor data from AWS S3
	spotData, err := c.fetchSpotAdvisorData(ctx)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errFetchSpotData)
	}

	// Filter data for the specific region and OS
	filteredData, err := c.filterDataByRegion(spotData, cr.Spec.ForProvider.AWSRegion, cr.Spec.ForProvider.OS)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot filter data by region")
	}

	// Update the status with the filtered data
	cr.Status.AtProvider = *filteredData

	duration := time.Since(startTime)

	// Log the timing information
	totalSpotAdvisorEntries := 0
	for _, osData := range filteredData.SpotAdvisor {
		totalSpotAdvisorEntries += len(osData)
	}
	// Get the OS being filtered
	os := "Linux"
	if cr.Spec.ForProvider.OS != nil && *cr.Spec.ForProvider.OS != "" {
		os = *cr.Spec.ForProvider.OS
	}
	fmt.Printf("SpotAdvisorData observation took %v for region %s (OS: %s), fetched %d instance types, %d spot advisor entries, %d ranges\n",
		duration, cr.Spec.ForProvider.AWSRegion, os, len(filteredData.InstanceTypes), totalSpotAdvisorEntries, len(filteredData.Ranges))

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

// fetchSpotAdvisorData fetches the spot advisor data from AWS S3
func (c *external) fetchSpotAdvisorData(ctx context.Context) (*SpotAdvisorResponse, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", spotAdvisorDataURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create HTTP request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch spot advisor data")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read response body")
	}

	var spotData SpotAdvisorResponse
	if err := json.Unmarshal(body, &spotData); err != nil {
		return nil, errors.Wrap(err, errParseSpotData)
	}

	return &spotData, nil
}

// filterDataByRegion filters the spot advisor data to only include data for the specified region and OS
func (c *external) filterDataByRegion(spotData *SpotAdvisorResponse, region string, osFilter *string) (*v1alpha1.SpotAdvisorDataObservation, error) {
	// Default to Linux if no OS filter is specified
	os := "Linux"
	if osFilter != nil && *osFilter != "" {
		os = *osFilter
	}
	// Convert instance types data
	instanceTypes := make(map[string]v1alpha1.InstanceTypeData)
	for instanceType, info := range spotData.InstanceTypes {
		instanceTypes[instanceType] = v1alpha1.InstanceTypeData{
			EMR:   info.EMR,
			Cores: info.Cores,
			RAMGB: fmt.Sprintf("%.1f", info.RAMGB),
		}
	}

	// Filter spot advisor data for the specific region and OS
	spotAdvisor := make(map[string]map[string]v1alpha1.RegionData)
	if regionData, exists := spotData.SpotAdvisor[region]; exists {
		// Only include the specified OS
		if osData, osExists := regionData[os]; osExists {
			spotAdvisor[os] = make(map[string]v1alpha1.RegionData)
			for instanceType, regionInfo := range osData {
				spotAdvisor[os][instanceType] = v1alpha1.RegionData{
					S: regionInfo.S,
					R: regionInfo.R,
				}
			}
		}
	}

	// Convert ranges data
	ranges := make([]v1alpha1.RangeData, len(spotData.Ranges))
	for i, rangeInfo := range spotData.Ranges {
		ranges[i] = v1alpha1.RangeData{
			Index: rangeInfo.Index,
			Label: rangeInfo.Label,
			Dots:  rangeInfo.Dots,
			Max:   rangeInfo.Max,
		}
	}

	return &v1alpha1.SpotAdvisorDataObservation{
		InstanceTypes: instanceTypes,
		SpotAdvisor:   spotAdvisor,
		GlobalRate:    spotData.GlobalRate,
		Ranges:        ranges,
	}, nil
}
