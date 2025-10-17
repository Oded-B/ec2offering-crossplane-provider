# provider-ec2offering

`provider-ec2offering` is a [Crossplane](https://crossplane.io/) Provider that provides
AWS EC2 instance type offerings and spot advisor data. It offers two main managed resource types:

## Managed Resources

### InstanceTypeOffering

The `InstanceTypeOffering` resource provides information about available EC2 instance types
in a specific AWS region. It queries the AWS EC2 `DescribeInstanceTypeOfferings` API to
retrieve real-time data about which instance types are available for launch.

**Specification:**
- `spec.forProvider.awsRegion` (required): The AWS region to query for instance type offerings
- `spec.forProvider.locationType` (optional): The location type filter (default: "region")

**Status:**
- `status.atProvider.instanceTypeOfferings`: Array of available instance types with their details
- `status.atProvider.location`: The location information
- `status.atProvider.locationType`: The location type used for the query

**Example:**
```yaml
apiVersion: ec2.ec2offering.crossplane.io/v1alpha1
kind: InstanceTypeOffering
metadata:
  name: us-east-1-offerings
spec:
  forProvider:
    awsRegion: us-east-1
```

### SpotAdvisorData

The `SpotAdvisorData` resource fetches and filters Amazon's spot instance advisor data
from their public S3 bucket. It provides information about spot instance interruption
rates and recommendations for different instance types and operating systems.

**Specification:**
- `spec.forProvider.awsRegion` (required): The AWS region to filter spot advisor data for
- `spec.forProvider.os` (optional): The operating system to filter for (default: "Linux", options: "Linux", "Windows")

**Status:**
- `status.atProvider.instanceTypes`: Map of instance type specifications (cores, RAM, EMR support)
- `status.atProvider.spotAdvisor`: Filtered spot advisor data for the specified region and OS
- `status.atProvider.globalRate`: Global spot interruption rate information
- `status.atProvider.ranges`: Rate ranges and their corresponding labels

**Example:**
```yaml
apiVersion: ec2.ec2offering.crossplane.io/v1alpha1
kind: SpotAdvisorData
metadata:
  name: us-east-1-spot-data
spec:
  forProvider:
    awsRegion: us-east-1
    os: Linux
```

## Features

- **Real-time Data**: Both resources fetch live data from AWS APIs and Amazon's public data sources
- **Region Filtering**: Automatically filters data based on the specified AWS region
- **OS Filtering**: SpotAdvisorData supports filtering by operating system (Linux/Windows)
- **Structured Output**: Provides well-structured, typed data in the resource status
- **Crossplane Integration**: Full integration with Crossplane's resource management and reconciliation

## Installation

1. Install the provider using Crossplane:
```bash
kubectl crossplane install provider provider-ec2offering
```

2. Configure AWS credentials by creating a ProviderConfig:
```yaml
apiVersion: ec2offering.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: aws-creds
      key: credentials
```

3. Create the AWS credentials secret:
```bash
kubectl create secret generic aws-creds -n crossplane-system --from-literal=credentials='[default]
aws_access_key_id = YOUR_ACCESS_KEY
aws_secret_access_key = YOUR_SECRET_KEY
region = us-east-1'
```

## Usage

### Querying Instance Type Offerings

Create an InstanceTypeOffering resource to get available instance types in a specific region:

```yaml
apiVersion: ec2.ec2offering.crossplane.io/v1alpha1
kind: InstanceTypeOffering
metadata:
  name: us-east-1-offerings
spec:
  forProvider:
    awsRegion: us-east-1
```

### Getting Spot Advisor Data

Create a SpotAdvisorData resource to get spot instance interruption rates and recommendations:

```yaml
apiVersion: ec2.ec2offering.crossplane.io/v1alpha1
kind: SpotAdvisorData
metadata:
  name: us-east-1-spot-data
spec:
  forProvider:
    awsRegion: us-east-1
    os: Linux
```

The provider will automatically fetch and filter the data, making it available in the resource's status.

> **Examples**: See the `examples/sample/` directory for complete example configurations of both resource types.

## Data Sources

### InstanceTypeOffering
- **Source**: AWS EC2 `DescribeInstanceTypeOfferings` API
- **Update Frequency**: Real-time (fetched on each reconciliation)
- **Authentication**: Requires AWS credentials with EC2 read permissions

### SpotAdvisorData
- **Source**: Amazon's public S3 bucket at `https://spot-bid-advisor.s3.amazonaws.com/spot-advisor-data.json`
- **Update Frequency**: Real-time (fetched on each reconciliation)
- **Authentication**: No authentication required (public data)
- **Data Includes**:
  - Instance type specifications (cores, RAM, EMR support)
  - Spot interruption rates by region and OS
  - Rate ranges and labels
  - Global spot interruption statistics

## Use Cases

This provider is useful for:
- **Cost Optimization**: Understanding which instance types are available in different regions
- **Spot Instance Planning**: Getting interruption rate data to make informed decisions about spot instances
- **Infrastructure Automation**: Programmatically querying AWS instance availability
- **Multi-Region Deployments**: Comparing instance availability across regions
- **Workload Planning**: Understanding instance specifications and spot interruption patterns

This POC function uses this data to generate [Karpenter](https://github.com/kubernetes-sigs/karpenter) nodepool objects: https://github.com/Oded-B/function-nodepools



## Developing

(notes regarding the usage of https://github.com/crossplane/provider-template)

1. Use this repository as a ec2offering to create a new one.
1. Run `make submodules` to initialize the "build" Make submodule we use for CI/CD.
1. Rename the provider by running the following command:
```shell
  export provider_name=MyProvider # Camel case, e.g. GitHub
  make provider.prepare provider=${provider_name}
```
4. Add your new type by running the following command:
```shell
  export group=sample # lower case e.g. core, cache, database, storage, etc.
  export type=MyType # Camel casee.g. Bucket, Database, CacheCluster, etc.
  make provider.addtype provider=${provider_name} group=${group} kind=${type}
```
5. Replace the *sample* group with your new group in apis/{provider}.go
5. Replace the *mytype* type with your new type in internal/controller/{provider}.go
5. Replace the default controller and ProviderConfig implementations with your own
5. Run `make reviewable` to run code generation, linters, and tests.
5. Run `make build` to build the provider.

Refer to Crossplane's [CONTRIBUTING.md] file for more information on how the
Crossplane community prefers to work. The [Provider Development][provider-dev]
guide may also be of use.

[CONTRIBUTING.md]: https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md
[provider-dev]: https://github.com/crossplane/crossplane/blob/master/contributing/guide-provider-development.md
