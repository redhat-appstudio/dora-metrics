# Data Flow Diagrams

## 1. System Overview

```mermaid
graph TB
    subgraph "ArgoCD Cluster"
        AC[ArgoCD Controller]
        APP1[Application: konflux-ui]
        APP2[Application: konflux-api]
        APP3[Application: konflux-backend]
    end
    
    subgraph "DORA Metrics System"
        WATCH[ArgoCD Watcher<br/>api/watcher.go]
        PROC[Event Processor<br/>processor/event.go]
        IMG[Image Processor<br/>processor/images.go]
        COMM[Commit Processor<br/>processor/commits.go]
        FMT[DevLake Formatter<br/>parser/formatter.go]
    end
    
    subgraph "Storage & Cache"
        REDIS[(Redis Cache<br/>Deployment Records<br/>Processed Commits)]
    end
    
    subgraph "External APIs"
        GITHUB[GitHub API<br/>Commit Data<br/>Repository Info]
        DEVLAKE[DevLake API<br/>DORA Metrics<br/>Deployment Data]
    end
    
    AC -->|MODIFIED Events| WATCH
    WATCH -->|Application Events| PROC
    PROC -->|Image Processing| IMG
    PROC -->|Commit Processing| COMM
    COMM -->|Formatted Data| FMT
    FMT -->|DevLake Payload| DEVLAKE
    PROC -->|Cache Data| REDIS
    COMM -->|Fetch Commits| GITHUB
    IMG -->|Validate Images| GITHUB
```

## 2. Deployment Processing Flow

```mermaid
flowchart TD
    A[ArgoCD MODIFIED Event] --> B{Check Application Status}
    B -->|OutOfSync + Missing| C[Failed Deployment Path]
    B -->|Sync Revision Match| D[New Deployment Path]
    B -->|Other| E[Skip Processing]
    
    C --> C1[Get Commit History]
    C1 --> C2[Format as FAILED]
    C2 --> C3[Send to DevLake]
    C3 --> C4[Cache as Processed]
    
    D --> D1[Check if Already Processed]
    D1 -->|Yes| E
    D1 -->|No| D2[Mark as Processed]
    D2 --> D3[Extract Valid Images]
    D3 --> D4[Get Commit History]
    D4 --> D5[Format as SUCCESS]
    D5 --> D6[Send to DevLake]
    D6 --> D7[Store Deployment Record]
```

## 3. Commit Processing Detail

```mermaid
sequenceDiagram
    participant EP as Event Processor
    participant IP as Image Processor
    participant CP as Commit Processor
    participant GH as GitHub API
    participant R as Redis
    participant F as Formatter

    EP->>IP: ExtractValidImages(images)
    IP->>GH: Validate commit hashes
    GH-->>IP: Validation results
    IP-->>EP: Valid images list
    
    EP->>CP: GetCommitHistoryForDeployment(app, appInfo)
    CP->>R: Get previous deployment
    R-->>CP: Previous deployment data
    
    alt Previous deployment exists
        CP->>IP: FindChangedImages(current, previous)
        IP-->>CP: Changed images
        CP->>GH: Get commit history for each changed image
        GH-->>CP: Commit history
    else No previous deployment
        CP->>CP: createCommitsFromImages(validImages)
        CP->>GH: Get commit data for each image
        GH-->>CP: Commit data
    end
    
    CP->>CP: Validate commit dates
    CP-->>EP: Complete commit history
    
    EP->>F: FormatDeployment(app, appInfo, commits)
    F-->>EP: DevLake payload
```

## 4. Image Processing Detail

```mermaid
flowchart TD
    A[Raw Docker Images] --> B[Extract Image Tags]
    B --> C{Is Tag Valid Commit?}
    C -->|No| D[Skip Image]
    C -->|Yes| E[Find Repository]
    E --> F{Repository Found?}
    F -->|No| G[Try History Fallback]
    F -->|Yes| H[Get Commit Data]
    G --> I{History Found?}
    I -->|No| D
    I -->|Yes| H
    H --> J[Validate Commit Date]
    J --> K{Date Valid?}
    K -->|No| D
    K -->|Yes| L[Add to Valid Images]
    L --> M[Return Valid Images List]
```

## 5. DevLake Payload Structure

```mermaid
graph TD
    A[DevLake Payload] --> B[Deployment Info]
    A --> C[Commit Info]
    
    B --> B1[ID: commit-sha]
    B --> B2[Created Date]
    B --> B3[Started Date]
    B --> B4[Finished Date]
    B --> B5[Environment: PRODUCTION]
    B --> B6[Result: SUCCESS/FAILED]
    B --> B7[Display Title]
    B --> B8[Name]
    
    C --> C1[Repo URL]
    C --> C2[Ref Name]
    C --> C3[Started Date]
    C --> C4[Finished Date]
    C --> C5[Commit SHA]
    C --> C6[Commit Message]
    C --> C7[Result]
    C --> C8[Display Title]
    C --> C9[Name]
```

## 6. Error Handling Flow

```mermaid
flowchart TD
    A[Process Event] --> B{API Call Success?}
    B -->|Yes| C[Continue Processing]
    B -->|No| D[Log Error]
    D --> E{Retryable Error?}
    E -->|Yes| F[Retry with Backoff]
    E -->|No| G[Skip Processing]
    F --> H{Max Retries?}
    H -->|No| B
    H -->|Yes| G
    
    C --> I{Commit Date Valid?}
    I -->|Yes| J[Process Commit]
    I -->|No| K[Skip Commit]
    
    J --> L{DevLake Send Success?}
    L -->|Yes| M[Mark as Processed]
    L -->|No| N[Log Error, Continue]
```

## 7. Caching Strategy

```mermaid
graph TD
    A[Deployment Event] --> B{Check Redis Cache}
    B -->|Found| C[Skip Processing]
    B -->|Not Found| D[Process Deployment]
    
    D --> E[Mark Commit as Processed]
    E --> F[Store Deployment Record]
    F --> G[Cache DevLake Commit]
    
    H[Component Level] --> I[Prevent Duplicate DevLake Sends]
    J[Application Level] --> K[Prevent Duplicate Processing]
    L[Deployment Level] --> M[Store Complete Records]
```

## 8. Multi-Application Deployment

```mermaid
sequenceDiagram
    participant AC as ArgoCD
    participant W as Watcher
    participant EP as Event Processor
    participant R as Redis
    participant DL as DevLake

    Note over AC,DL: Multiple applications deploying simultaneously
    
    AC->>W: App1 MODIFIED (commit: abc123)
    AC->>W: App2 MODIFIED (commit: def456)
    AC->>W: App3 MODIFIED (commit: abc123)
    
    W->>EP: Process App1
    EP->>R: Check processed(abc123, app1, cluster1)
    R-->>EP: Not processed
    EP->>R: Mark processed(abc123, app1, cluster1)
    EP->>DL: Send deployment (component: ui)
    
    W->>EP: Process App2
    EP->>R: Check processed(def456, app2, cluster1)
    R-->>EP: Not processed
    EP->>R: Mark processed(def456, app2, cluster1)
    EP->>DL: Send deployment (component: api)
    
    W->>EP: Process App3
    EP->>R: Check processed(abc123, app3, cluster1)
    R-->>EP: Not processed
    EP->>R: Mark processed(abc123, app3, cluster1)
    EP->>R: Check devlake(abc123, ui)
    R-->>EP: Already sent
    Note over EP: Skip DevLake send (same component)
```

## 9. Failed Deployment Handling

```mermaid
flowchart TD
    A[ArgoCD Event] --> B{Status Check}
    B -->|OutOfSync + Missing| C[Failed Deployment]
    B -->|Other| D[Normal Processing]
    
    C --> E[Get Commit History]
    E --> F[Format as FAILED]
    F --> G[Update Commit Messages]
    G --> H[Send to DevLake]
    H --> I[Cache as Processed]
    I --> J[Wait for Recovery]
    
    K[Recovery Event] --> L[Health Status Change]
    L --> M[Process as New Deployment]
```

## 10. Performance Optimization

```mermaid
graph TD
    A[Incoming Events] --> B[Batch Processing]
    B --> C[Parallel Image Validation]
    C --> D[Parallel Commit Fetching]
    D --> E[Batch DevLake Sends]
    E --> F[Async Cache Updates]
    
    G[Memory Cache] --> H[Frequent Data]
    I[Redis Cache] --> J[Persistent Data]
    K[GitHub Cache] --> L[Commit Data]
```

This comprehensive data flow documentation shows how the DORA Metrics system processes ArgoCD deployments and sends them to DevLake, with detailed diagrams for each major component and process flow.

