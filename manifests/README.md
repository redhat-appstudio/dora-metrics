# DORA Metrics OpenShift Deployment

This directory contains the OpenShift manifests for deploying the DORA Metrics application.

## Prerequisites

- OpenShift cluster with appropriate permissions
- `oc` CLI tool installed and configured
- Access to the `quay.io/redhat-appstudio/dora-metrics` container image
- Access to the `redis:7.0-alpine` container image (used as sidecar)

## Deployment

### Option 1: Using Kustomize (Recommended)

```bash
# Deploy all resources
oc apply -k .

# Or using kustomize directly
kustomize build . | oc apply -f -
```

### Option 2: Individual Resource Deployment

```bash
# Create namespace first
oc apply -f namespace.yaml

# Apply RBAC
oc apply -f rbac.yaml

# Apply configuration
oc apply -f configmap.yaml

# Apply secrets (update with your values first)
oc apply -f secrets.yaml

# Deploy application (includes Redis sidecar)
oc apply -f deployment.yaml

# Create service
oc apply -f service.yaml

# Create route
oc apply -f route.yaml
```

## Configuration

### Required Secrets

Before deploying, you need to update the `secrets.yaml` file with your actual values:

1. **WebRCA Offline Token**: Get your offline token from the Red Hat API
2. **Redis Password**: If your Redis instance requires authentication
3. **DevLake Webhook Token**: Get your DevLake webhook token for data integration

```bash
# Encode your values
echo -n "your-offline-token" | base64
echo -n "your-redis-password" | base64
echo -n "your-devlake-webhook-token" | base64

# Update secrets.yaml with the encoded values
# Default Redis password is "dora-metrics-redis" (already encoded in secrets.yaml)
```

### Redis Configuration

The application uses Redis as a sidecar container within the same pod with Kubernetes service discovery:

- **REDIS_HOST**: Redis service name (default: "redis-service" from deployment)
- **REDIS_PORT**: Redis port (default: "6379" from deployment)  
- **REDIS_PASSWORD**: Redis password (from secrets)

**Service Discovery**: The DORA metrics container discovers Redis through the `redis-service` Kubernetes service, which provides automatic DNS resolution and load balancing within the cluster.

The application will automatically build the Redis address as `REDIS_HOST:REDIS_PORT` if both are provided, or use the YAML configuration as fallback.

### Configuration Updates

The application configuration is stored in the `configmap.yaml` file. You can modify:

- ArgoCD namespaces to monitor
- Components to monitor
- Known clusters
- DevLake integration settings
- Redis connection details

## Verification

After deployment, verify the application is running:

```bash
# Check pods
oc get pods -n konflux-dora-metrics

# Check services
oc get svc -n konflux-dora-metrics

# Check containers in the pod
oc describe pod -l app=dora-metrics -n konflux-dora-metrics

# Check routes
oc get route -n konflux-dora-metrics

# Check logs
oc logs -f deployment/dora-metrics -n konflux-dora-metrics

# Check Redis logs (sidecar container)
oc logs -f deployment/dora-metrics -c redis -n konflux-dora-metrics
```

## Accessing the Application

The application will be available at:
- **Route URL**: `https://dora-metrics.apps.rosa.kflux-c-prd-i01.7hyu.p3.openshiftapps.com`
- **Health Check**: `https://dora-metrics.apps.rosa.kflux-c-prd-i01.7hyu.p3.openshiftapps.com/health`

## Monitoring

The application includes:

- **Health Check Endpoint**: `/health`
- **Liveness Probe**: HTTP GET on port 8080
- **Readiness Probe**: HTTP GET on port 8080
- **Resource Limits**: CPU and memory limits configured

## RBAC Permissions

The application requires the following permissions:

- **ArgoCD Applications**: Read access to monitor application status
- **Deployments**: Read access across all namespaces
- **Pods**: Read access for monitoring
- **Events**: Read access for incident correlation
- **Services**: Read access for service monitoring
- **ConfigMaps/Secrets**: Read access for configuration

## Troubleshooting

### Common Issues

1. **Image Pull Errors**: Ensure you have access to the container registry
2. **RBAC Errors**: Check that the ServiceAccount has proper permissions
3. **Configuration Errors**: Verify the ConfigMap contains valid YAML
4. **Secret Errors**: Ensure secrets are properly base64 encoded

### Debug Commands

```bash
# Check pod status
oc describe pod -l app=dora-metrics -n konflux-dora-metrics

# Check events
oc get events -n konflux-dora-metrics

# Check logs
oc logs -f deployment/dora-metrics -n konflux-dora-metrics

# Check Redis logs (sidecar container)
oc logs -f deployment/dora-metrics -c redis -n konflux-dora-metrics

# Check configuration
oc get configmap dora-metrics-config -n konflux-dora-metrics -o yaml

# Check secrets
oc get secret dora-metrics-secrets -n konflux-dora-metrics -o yaml
```

## Scaling

To scale the application:

```bash
# Scale to 3 replicas
oc scale deployment dora-metrics --replicas=3 -n konflux-dora-metrics
```

## Updates

To update the application:

```bash
# Update image
oc set image deployment/dora-metrics dora-metrics=quay.io/redhat-appstudio/dora-metrics:new-tag -n konflux-dora-metrics

# Or update configuration
oc apply -f configmap.yaml
oc rollout restart deployment/dora-metrics -n konflux-dora-metrics
```

## Cleanup

To remove the application:

```bash
# Remove all resources
oc delete -k .

# Or remove individually
oc delete -f route.yaml
oc delete -f service.yaml
oc delete -f deployment.yaml
oc delete -f secrets.yaml
oc delete -f configmap.yaml
oc delete -f rbac.yaml
oc delete -f namespace.yaml
```
