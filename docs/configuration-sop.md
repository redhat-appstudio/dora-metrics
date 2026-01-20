# DORA Metrics Configuration Reference

## Table of Contents

1. [What Are DORA Metrics?](#what-are-dora-metrics)
2. [What You'll See in DevLake](#what-youll-see-in-devlake)
3. [Configuration Files](#configuration-files)
4. [ArgoCD Configuration](#argocd-configuration)
5. [DevLake Integration](#devlake-integration)
6. [WebRCA Configuration](#webrca-configuration)
7. [Current Teams Configuration](#current-teams-configuration)
8. [How to Add/Modify Configurations](#how-to-addmodify-configurations)
9. [Troubleshooting](#troubleshooting)

---

## What Are DORA Metrics?

DORA (DevOps Research and Assessment) metrics are four key performance indicators that measure software delivery performance. These metrics help teams understand and improve their DevOps practices.

### The Four DORA Metrics

| Metric | Description | What It Measures |
|--------|-------------|------------------|
| **Deployment Frequency** | How often you deploy to production | Frequency of successful deployments (e.g., daily, weekly) |
| **Lead Time for Changes** | Time from code commit to production deployment | How quickly changes reach users |
| **Change Failure Rate** | Percentage of deployments that fail | Reliability and stability of deployments |
| **Mean Time to Recovery (MTTR)** | Time to recover from failures | How quickly you can fix issues in production |

### Why DORA Metrics Matter

According to the [DORA research](https://www.devops-research.com/research.html), teams that excel in these four metrics are:

- **2x more likely to exceed goals** for profitability, productivity, and market share
- **Faster at delivering value** to customers
- **More reliable** with fewer production failures
- **Better at recovering** from incidents

---

## What You'll See in DevLake

[Apache DevLake](https://devlake.apache.org/docs/Overview/Introduction/) is a data engineering platform that ingests, analyzes, and visualizes DevOps data. When you configure DORA Metrics, you'll see deployment data and metrics in DevLake dashboards.

### Data Sent to DevLake

For each ArgoCD deployment, the system sends:

- **Deployment Information**:
  - Component name (e.g., `build-service`, `application-api`)
  - Deployment timestamp (when deployed to production)
  - Environment (PRODUCTION)
  - Deployment result (SUCCESS or FAILED)
  - Cluster name

- **Commit Information**:
  - Commit SHA, message, and creation date
  - Repository URL
  - Lead time calculation (commit date to deployment date)

- **Incident Information** (from WebRCA):
  - Incident details
  - Recovery time
  - Mean Time to Recovery (MTTR)

### What Metrics You'll See

Once data is in DevLake, you can view:

#### 1. Deployment Frequency

- **Dashboard**: Shows how often deployments occur
- **Visualization**: Deployment count per day/week/month
- **Insight**: Are you deploying frequently enough?

#### 2. Lead Time for Changes

- **Dashboard**: Time from commit to production
- **Visualization**: Average lead time, trends over time
- **Insight**: How quickly are changes reaching users?

#### 3. Change Failure Rate

- **Dashboard**: Percentage of failed deployments
- **Visualization**: Success vs. failure rates, trends
- **Insight**: How reliable are your deployments?

#### 4. Mean Time to Recovery (MTTR)

- **Dashboard**: Time to recover from production incidents
- **Visualization**: Recovery time trends, incident frequency
- **Insight**: How quickly can you fix production issues?

### Accessing DevLake Dashboards

1. **Navigate to DevLake UI**:
   - URL: `https://konflux-devlake-ui-konflux-devlake.apps.rosa.kflux-c-prd-i01.7hyu.p3.openshiftapps.com`

2. **Select Your Project**:
   - **Global Project** (Project ID: `1`): All deployments across all teams
   - **Team Project**: Your team's specific deployments (see [Current Teams Configuration](#current-teams-configuration))

3. **View Dashboards**:
   - Pre-built DORA metrics dashboards in Grafana
   - Custom dashboards using SQL queries
   - Metrics and visualizations for all four DORA metrics

### Global vs. Team Projects

- **Global Project** (Project ID: `1`):
  - Receives **ALL** deployment events from all components
  - Provides organization-wide visibility
  - Useful for executive dashboards and cross-team analysis

- **Team Projects** (Project IDs: `3`, `4`, `5`, etc.):
  - Receive deployments only for your team's components
  - Focused view of your team's performance
  - Enables team-specific dashboards and metrics

**Note**: Deployments are sent to **both** the global project and team projects (if configured), so you can see both views.

### Example: What a Team Sees

When the Konflux Build Team deploys `build-service`:

1. **In Global Project** (Project ID: `1`):
   - Deployment appears in organization-wide metrics
   - Contributes to overall DORA metrics

2. **In Build Team Project** (Project ID: `4`):
   - Deployment appears in team-specific dashboard
   - Shows only build-service deployments
   - Team can track their own DORA metrics

---

## Configuration Files

### File Locations

| File | Purpose | When to Edit |
|------|---------|--------------|
| `configs/config.yaml` | Staging/Local configuration | For staging environment changes |
| `manifests/production/configmap.yaml` | Production configuration | For production environment changes |

**Important**: Always update **both files** when making configuration changes.

### Configuration Structure

```yaml
argocd:          # ArgoCD monitoring settings
integration:     # DevLake integration (global + teams)
  devlake:
webrca:          # WebRCA incident monitoring
```

---

## ArgoCD Configuration

### Overview

ArgoCD configuration controls which applications are monitored and how components are identified.

### ArgoCD Application Naming Convention

**⚠️ Important**: ArgoCD applications in the cluster **must** follow this naming pattern:

```
{component-name}-{cluster-name}
```

**Examples**:

- `build-service-kflux-prd-rh02` → Component: `build-service`, Cluster: `kflux-prd-rh02`
- `application-api-kflux-ocp-p01` → Component: `application-api`, Cluster: `kflux-ocp-p01`
- `konflux-ui-stone-prod-p01` → Component: `konflux-ui`, Cluster: `stone-prod-p01`

**Why this matters**:

- The system extracts component names from application names using this pattern
- Component names are matched against team configurations
- Incorrect naming will prevent components from being monitored

### ArgoCD Configuration Section

```yaml
argocd:
  enabled: true
  namespaces:
    - "konflux-public-production"
    - "argocd"
  components_to_ignore:
    - "monitoring"
    - "openshift-gitops"
    - "kubearchive"
  known_clusters:
    - "kflux-ocp-p01"
    - "kflux-prd-rh02"
  repository_blacklist:
    - "https://github.com/user/repo"
```

### Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `enabled` | Enable/disable ArgoCD monitoring | `true` / `false` |
| `namespaces` | Kubernetes namespaces to watch | `["argocd", "konflux-public-production"]` |
| `components_to_ignore` | Components excluded from monitoring | `["monitoring", "kubearchive"]` |
| `known_clusters` | Cluster names for component parsing | `["kflux-ocp-p01", "stone-prod-p01"]` |
| `repository_blacklist` | Repository URLs to exclude from commits | `["https://github.com/user/repo"]` |

### Adding Components to Ignore

To exclude a component from monitoring:

```yaml
argocd:
  components_to_ignore:
    - "monitoring"
    - "openshift-gitops"
    - "your-component-name"  # Add here
```

### Adding New Clusters

When new clusters are added, update the `known_clusters` list:

```yaml
argocd:
  known_clusters:
    - "kflux-ocp-p01"
    - "new-cluster-name"  # Add here
```

**How to identify cluster names**: Extract from ArgoCD application names:

- Application: `build-service-kflux-prd-rh02` → Cluster: `kflux-prd-rh02`
- Application: `application-api-stone-prod-p01` → Cluster: `stone-prod-p01`

**Note**: Ensure ArgoCD applications follow the naming convention: `{component-name}-{cluster-name}`

### Adding Repository Blacklist

To exclude commits from specific repositories:

```yaml
argocd:
  repository_blacklist:
    - "https://github.com/DindaPutriFN/imao1"
    - "https://github.com/user/unwanted-repo"  # Add here
```

**Format**: Full repository URL (with or without `.git` suffix)

---

## DevLake Integration

### Overview

DevLake integration sends deployment events to [Apache DevLake](https://devlake.apache.org/docs/Overview/Introduction/) projects for DORA metrics tracking. There are two types of configuration:

1. **Global Configuration**: Sends ALL deployments to a global project. Useful when we want to see metrics at project level not only teams.
2. **Team Configuration**: Routes specific components to team projects

### Global Configuration

The global project receives **ALL** deployment events from all components.

```yaml
integration:
  devlake:
    enabled: true
    base_url: "https://konflux-devlake-ui-konflux-devlake.apps.rosa.kflux-c-prd-i01.7hyu.p3.openshiftapps.com"
    project_id: "1"  # Global project - DO NOT CHANGE without DevProd approval
    timeout_seconds: 30
```

**⚠️ Important**: Only Konflux DevProd team should modify global configuration.

### Team Configuration

Teams can route their component deployments to team-specific DevLake projects in addition to the global project.

#### Configuration Structure

```yaml
integration:
  devlake:
    teams:
      - name: "team-name"
        project_id: "X"
        argocd_components:
          - "component-1"
          - "component-2"
```

#### How It Works

1. **Every deployment** is sent to the global project (project_id: "1")
2. **Additionally**, if a component matches a team's `argocd_components` list, it's also sent to that team's project
3. **Result**: Teams get their specific metrics, while the global project maintains a complete view

#### Example: Deployment Routing

When `build-service` is deployed:

```
Deployment Event
    │
    ├─→ Global Project (project_id: "1")  ← Always sent
    │
    └─→ Konflux Build Team (project_id: "4")  ← Sent because build-service is in their list
```

---

## WebRCA Configuration

### Overview

WebRCA integration monitors OpenShift incidents and sends them to DevLake for Mean Time to Recovery (MTTR) calculations.

### WebRCA Configuration Section

```yaml
webrca:
  enabled: true
  api_url: "https://api.openshift.com/api/web-rca/v1/incidents"
  interval: "1h"
```

### Parameters

| Parameter | Description | Options |
|-----------|-------------|---------|
| `enabled` | Enable/disable WebRCA monitoring | `true` / `false` |
| `api_url` | WebRCA API endpoint | OpenShift API URL |
| `interval` | Polling interval | `"1h"`, `"30m"`, `"2h"`, `"15m"` |

### Authentication

WebRCA uses OpenShift offline tokens. **Never hardcode tokens in configuration files**.

Set via environment variable:
```bash
export OFFLINE_TOKEN="your-token-here"
```

Or in Kubernetes Secret:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dora-metrics-secrets
data:
  OFFLINE_TOKEN: <base64-encoded-token>
```

### Interval Options

- `"1h"` - Every hour (recommended)
- `"30m"` - Every 30 minutes
- `"2h"` - Every 2 hours
- `"15m"` - Every 15 minutes

### Example Configuration

```yaml
webrca:
  enabled: true
  api_url: "https://api.openshift.com/api/web-rca/v1/incidents"
  interval: "1h"
```

---

## Current Teams Configuration

### How to Obtain a Project ID

To get a DevLake project ID for your team:

1. **Contact Konflux DevProd**:

   - Open a request in **#forum-konflux-devprod** Slack channel
   - Include:
     - Team name
     - List of components to monitor
     - Brief description of team scope

2. **Wait for Assignment**:
   - DevProd will create a DevLake project
   - You'll receive a project ID (e.g., "2", "3", "4")

3. **Note**: This process will be automated in the future.

### Current Teams

#### 1. Konflux UI Team

- **Project ID**: `3`

#### 2. Konflux Build Team

- **Project ID**: `4`

#### 3. Konflux Release Team

- **Project ID**: `5`

#### 4. Konflux Integration Team

- **Project ID**: `6`

#### 5. Konflux Vanguard Team

- **Project ID**: `7`

#### 6. Konflux Infrastructure Team

- **Project ID**: `8`

#### 7. Konflux Mintmaker Team

- **Project ID**: `9`

#### 8. Konflux Pipelines Team

- **Project ID**: `10`

---

## How to Add/Modify Configurations

### Adding a New Team

1. **Get Project ID**: Contact DevProd in #forum-konflux-devprod

2. **Update `configs/config.yaml`**:
   ```yaml
   integration:
     devlake:
       teams:
         # ... existing teams ...
         
         - name: "konflux-new-team"
           project_id: "11"
           argocd_components:
             - "new-component-1"
             - "new-component-2"
   ```

3. **Update `manifests/production/configmap.yaml`**:
   - Apply the same changes in the `data.config.yaml` section

4. **Verify Component Names**:
   - Check ArgoCD application names: `kubectl get applications -n argocd`
   - **Ensure applications follow naming convention**: `{component-name}-{cluster-name}`
   - Component names must match exactly (case-sensitive)
   - Format: `{component-name}-{cluster-name}` → component: `{component-name}`

5. **Create Pull Request**:
   ```bash
   git checkout -b team/your-team-name/add-config
   git add configs/config.yaml manifests/production/configmap.yaml
   git commit -m "feat: add team configuration for konflux-new-team"
   git push origin team/your-team-name/add-config
   ```

### Adding Components to Existing Team

1. **Update `configs/config.yaml`**:
   ```yaml
   integration:
     devlake:
       teams:
         - name: "konflux-build-team"
           project_id: "4"
           argocd_components:
             - "build-service"
             - "image-controller"
             - "new-component"  # Add here
   ```

2. **Update `manifests/production/configmap.yaml`**:
   - Apply the same changes

3. **Verify and Commit**:
   - Validate YAML: `yamllint configs/config.yaml`
   - Commit changes

### Adding Components to Ignore

1. **Update both config files**:
   ```yaml
   argocd:
     components_to_ignore:
       - "monitoring"
       - "your-component"  # Add here
   ```

2. **Note**: Ignored components are excluded from ALL monitoring, even if they're in a team's component list.

### Adding New Clusters

1. **Identify cluster name** from ArgoCD application names

2. **Update both config files**:
   ```yaml
   argocd:
     known_clusters:
       - "kflux-ocp-p01"
       - "new-cluster-name"  # Add here
   ```

### Modifying WebRCA Configuration

1. **Update interval** (if needed):
   ```yaml
   webrca:
     interval: "30m"  # Change from "1h" to "30m"
   ```

2. **Enable/disable**:
   ```yaml
   webrca:
     enabled: false  # Disable WebRCA monitoring
   ```

**Note**: Authentication token is set via environment variable, not in config files.

---

## Troubleshooting

### Configuration Validation

**Validate YAML syntax**:
```bash
yamllint configs/config.yaml
yamllint manifests/production/configmap.yaml
```

### Common Issues

#### Issue: Component Not Being Monitored

**Check**:

1. Component name matches exactly (case-sensitive)
2. Component is NOT in `components_to_ignore`
3. Component is in team's `argocd_components` list (if using team routing)

#### Issue: Wrong Project ID

**Check**:

1. Project ID is correct in both config files
2. Staging and production project IDs may differ
3. Verify project ID with DevProd team

#### Issue: YAML Syntax Errors

**Check**:

1. Indentation (use 2 spaces, not tabs)
2. List format (dash + space: `- "item"`)
3. String quotes for values with special characters

#### Issue: Component Name Mismatch

**How to verify component names**:
```bash
# List ArgoCD applications
kubectl get applications -n argocd

# Extract component name
# Application: build-service-kflux-prd-rh02
# Component: build-service
```

**Component name extraction**:

- **Naming convention**: ArgoCD applications must be named `{component-name}-{cluster-name}`
- Pattern: `{component-name}-{cluster-name}`
- Must match exactly (case-sensitive)
- No wildcards supported

**If application doesn't follow naming convention**:

- The system cannot extract the component name correctly
- Component will not be monitored
- Update the ArgoCD application name to follow the convention

#### Issue: Not Seeing Data in DevLake

**Check**:

1. Verify deployment events are being sent (check logs)
2. Confirm project ID is correct
3. Check DevLake project exists and is accessible
4. Verify `DEVLAKE_WEBHOOK_TOKEN` is set correctly
5. Check DevLake dashboards are configured for your project

---

## Quick Reference

### Configuration Files

- `configs/config.yaml` - Staging/Local
- `manifests/production/configmap.yaml` - Production

### Key Sections

```yaml
# ArgoCD monitoring
argocd:
  components_to_ignore: [...]
  known_clusters: [...]
  repository_blacklist: [...]

# DevLake integration
integration:
  devlake:
    project_id: "1"  # Global
    teams:            # Team routing
      - name: "..."
        project_id: "..."
        argocd_components: [...]

# WebRCA incidents
webrca:
  enabled: true
  interval: "1h"
```

### DevLake Resources

- **DevLake Documentation**: [Apache DevLake Introduction](https://devlake.apache.org/docs/Overview/Introduction/)
- **DevLake Instance**: `https://konflux-devlake-ui-konflux-devlake.apps.rosa.kflux-c-prd-i01.7hyu.p3.openshiftapps.com`
- **Global Project ID**: `1`

### Contact Information

- **DevProd Team**: #forum-konflux-devprod (Slack)
- **Repository**: [dora-metrics](https://github.com/redhat-appstudio/dora-metrics)

---

**Last Updated**: 2025-11-19  
**Maintained By**: Konflux DevProd Team  
**Version**: 1.0
