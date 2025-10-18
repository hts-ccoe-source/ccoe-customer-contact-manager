# Transient Trigger Pattern - Architecture Diagrams

## Overview

This document provides visual representations of the Transient Trigger Pattern architecture, processing flows, and operational patterns.

## High-Level Architecture

### System Overview

```mermaid
graph TB
    subgraph "CCOE Team"
        A[User] --> B[Web Browser]
    end
    
    subgraph "AWS - Management Account"
        B --> C[CloudFront + Lambda@Edge]
        C --> D[S3 Static Website]
        D --> E[Enhanced Metadata Lambda]
        
        E --> F[S3 Metadata Bucket]
        
        subgraph "S3 Storage Structure"
            F --> G[archive/ - Single Source of Truth]
            F --> H[customers/ - Transient Triggers]
            F --> I[drafts/ - Working Copies]
        end
        
        H --> J[S3 Event Notifications]
    end
    
    subgraph "AWS - Production Governance Account"
        J --> K1[Customer HTS SQS Queue]
        J --> K2[Customer HTSNonProd SQS Queue]
        J --> K3[Customer N SQS Queue]
        
        K1 --> L1[ECS Task - HTS Processor]
        K2 --> L2[ECS Task - HTSNonProd Processor]
        K3 --> L3[ECS Task - Customer N Processor]
    end
    
    subgraph "Customer AWS Accounts"
        L1 --> M1[Assume HTS SES Role]
        L2 --> M2[Assume HTSNonProd SES Role]
        L3 --> M3[Assume Customer N SES Role]
        
        M1 --> N1[HTS SES Service]
        M2 --> N2[HTSNonProd SES Service]
        M3 --> N3[Customer N SES Service]
    end
    
    subgraph "Microsoft 365"
        L1 --> O[Microsoft Graph API]
        L2 --> O
        L3 --> O
        O --> P[Calendar Invites]
    end
    
    style G fill:#90EE90
    style H fill:#FFB6C1
    style I fill:#87CEEB
```

## Storage Architecture

### Transient Trigger Pattern

```mermaid
graph LR
    subgraph "Frontend Upload Sequence"
        A[User Submits Change] --> B[Upload to archive/changeId.json]
        B --> C[Upload to customers/hts/changeId.json]
        B --> D[Upload to customers/htsnonprod/changeId.json]
        B --> E[Upload to customers/customer-n/changeId.json]
    end
    
    subgraph "S3 Storage"
        F[archive/changeId.json<br/>PERMANENT<br/>Single Source of Truth]
        G[customers/hts/changeId.json<br/>TRANSIENT<br/>Deleted After Processing]
        H[customers/htsnonprod/changeId.json<br/>TRANSIENT<br/>Deleted After Processing]
        I[customers/customer-n/changeId.json<br/>TRANSIENT<br/>Deleted After Processing]
    end
    
    subgraph "Backend Processing"
        J[Load from archive/]
        K[Process Change]
        L[Update archive/]
        M[Delete customers/ trigger]
    end
    
    C --> G
    D --> H
    E --> I
    
    G --> J
    H --> J
    I --> J
    
    J --> K
    K --> L
    L --> M
    
    M --> G
    M --> H
    M --> I
    
    style F fill:#90EE90
    style G fill:#FFB6C1
    style H fill:#FFB6C1
    style I fill:#FFB6C1
```

### Storage Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Draft: User Creates Change
    Draft --> Archive: User Submits
    Archive --> Trigger: Frontend Creates Triggers
    Trigger --> Processing: S3 Event → SQS
    Processing --> ArchiveUpdate: Backend Updates Archive
    ArchiveUpdate --> TriggerDelete: Backend Deletes Trigger
    TriggerDelete --> [*]: Processing Complete
    
    note right of Draft
        drafts/changeId.json
        Optional working copy
    end note
    
    note right of Archive
        archive/changeId.json
        Single source of truth
        PERMANENT
    end note
    
    note right of Trigger
        customers/{code}/changeId.json
        Transient trigger
        TEMPORARY
    end note
    
    note right of ArchiveUpdate
        Add processing metadata
        Add meeting metadata
    end note
```

## Processing Flows

### Standard Email Processing Flow

```mermaid
sequenceDiagram
    participant F as Frontend
    participant S3 as S3 Bucket
    participant SQS as SQS Queue
    participant B as Backend (ECS)
    participant A as Archive
    participant SES as Customer SES
    
    F->>S3: 1. Upload archive/changeId.json
    F->>S3: 2. Upload customers/hts/changeId.json
    S3->>SQS: 3. S3 Event Notification
    SQS->>B: 4. SQS Message
    
    B->>S3: 5. Check trigger exists?
    alt Trigger exists
        B->>A: 6. Load from archive/changeId.json
        B->>B: 7. Process change
        B->>SES: 8. Send emails
        B->>A: 9. Update archive with results
        B->>S3: 10. Delete customers/hts/changeId.json
        B->>SQS: 11. Acknowledge message
    else Trigger already deleted
        B->>B: Skip (already processed)
        B->>SQS: Acknowledge message
    end
```

### Meeting Invite Processing Flow

```mermaid
sequenceDiagram
    participant F as Frontend
    participant S3 as S3 Bucket
    participant SQS as SQS Queue
    participant B as Backend (ECS)
    participant A as Archive
    participant SES1 as Customer 1 SES
    participant SES2 as Customer 2 SES
    participant Graph as Microsoft Graph API
    
    F->>S3: 1. Upload archive/changeId.json
    F->>S3: 2. Upload customers/customer1/changeId.json
    F->>S3: 3. Upload customers/customer2/changeId.json
    S3->>SQS: 4. S3 Events (2 messages)
    
    SQS->>B: 5. First customer processes
    B->>S3: 6. Check trigger exists
    B->>A: 7. Load from archive/
    B->>SES1: 8. Query aws-calendar topic
    B->>SES2: 9. Query aws-calendar topic
    B->>B: 10. Aggregate & deduplicate recipients
    B->>Graph: 11. Create meeting (idempotency key: changeId)
    Graph-->>B: 12. Meeting created
    B->>A: 13. Update archive with meeting metadata
    B->>S3: 14. Delete customers/customer1/changeId.json
    B->>SQS: 15. Acknowledge message
    
    SQS->>B: 16. Second customer processes
    B->>S3: 17. Check trigger exists
    B->>A: 18. Load from archive/
    B->>B: 19. Meeting already exists (idempotency)
    B->>S3: 20. Delete customers/customer2/changeId.json
    B->>SQS: 21. Acknowledge message
```

### Idempotency Flow

```mermaid
flowchart TD
    A[Receive SQS Event] --> B{Trigger Exists?}
    B -->|Yes| C[Load from archive/]
    B -->|No| D[Skip Processing]
    D --> E[Acknowledge SQS]
    
    C --> F[Process Change]
    F --> G{Processing Success?}
    
    G -->|Yes| H[Update archive/]
    G -->|No| I{Retryable Error?}
    
    I -->|Yes| J[Delete trigger]
    I -->|No| K[Delete trigger]
    
    J --> L[Do NOT acknowledge SQS]
    K --> M[Acknowledge SQS]
    
    H --> N{Archive Update Success?}
    N -->|Yes| O[Delete trigger]
    N -->|No| P[Delete trigger]
    
    O --> Q{Delete Success?}
    Q -->|Yes| R[Acknowledge SQS]
    Q -->|No| S[Log warning]
    S --> R
    
    P --> T[Do NOT acknowledge SQS]
    
    style B fill:#FFE4B5
    style N fill:#FFE4B5
    style Q fill:#FFE4B5
```

## Error Handling Patterns

### Archive Update Failure

```mermaid
flowchart TD
    A[Process Change] --> B[Update archive/]
    B --> C{Update Success?}
    
    C -->|Yes| D[Delete trigger]
    C -->|No| E[Log error]
    
    E --> F[Delete trigger anyway]
    F --> G[Do NOT acknowledge SQS]
    
    G --> H[Message returns to queue]
    H --> I[Next attempt]
    I --> J{Trigger exists?}
    
    J -->|No| K[Skip - already processed]
    J -->|Yes| L[Retry processing]
    
    D --> M[Acknowledge SQS]
    K --> M
    
    style C fill:#FFE4B5
    style J fill:#FFE4B5
```

### Trigger Delete Failure

```mermaid
flowchart TD
    A[Update archive/] --> B{Update Success?}
    B -->|Yes| C[Delete trigger]
    B -->|No| D[Handle update failure]
    
    C --> E{Delete Success?}
    E -->|Yes| F[Acknowledge SQS]
    E -->|No| G[Log warning]
    
    G --> H[Continue anyway]
    H --> F
    
    F --> I[Processing complete]
    
    style B fill:#FFE4B5
    style E fill:#90EE90
```

### Duplicate Event Handling

```mermaid
flowchart TD
    A[Event 1: Receive SQS] --> B[Check trigger exists]
    B --> C{Exists?}
    C -->|Yes| D[Process normally]
    D --> E[Update archive/]
    E --> F[Delete trigger]
    F --> G[Acknowledge SQS]
    
    H[Event 2: Receive SQS] --> I[Check trigger exists]
    I --> J{Exists?}
    J -->|No| K[Skip processing]
    K --> L[Acknowledge SQS]
    
    style C fill:#FFE4B5
    style J fill:#90EE90
```

## Data Flow Diagrams

### Archive-First Loading Pattern

```mermaid
graph TB
    subgraph "Frontend"
        A[Create Change] --> B[Generate changeId]
        B --> C[Upload to archive/]
        C --> D[Upload to customers/]
    end
    
    subgraph "S3 Storage"
        E[archive/changeId.json<br/>VERSION 1]
        F[customers/hts/changeId.json<br/>TRIGGER]
    end
    
    subgraph "Backend Processing"
        G[Receive S3 Event]
        H[Check Trigger Exists]
        I[Load from archive/]
        J[Process Change]
        K[Update archive/]
        L[Delete Trigger]
    end
    
    subgraph "Updated State"
        M[archive/changeId.json<br/>VERSION 2<br/>+ processing metadata]
        N[customers/hts/changeId.json<br/>DELETED]
    end
    
    C --> E
    D --> F
    F --> G
    G --> H
    H --> I
    I --> E
    E --> J
    J --> K
    K --> M
    K --> L
    L --> N
    
    style E fill:#90EE90
    style F fill:#FFB6C1
    style M fill:#90EE90
    style N fill:#D3D3D3
```

### Modification Array Evolution

```mermaid
graph LR
    subgraph "Initial State"
        A["archive/changeId.json<br/>{<br/>  changeId: '123',<br/>  modifications: []<br/>}"]
    end
    
    subgraph "After Processing"
        B["archive/changeId.json<br/>{<br/>  changeId: '123',<br/>  modifications: [<br/>    {type: 'processed',<br/>     customer: 'hts',<br/>     timestamp: '...'}]<br/>}"]
    end
    
    subgraph "After Meeting Creation"
        C["archive/changeId.json<br/>{<br/>  changeId: '123',<br/>  modifications: [<br/>    {type: 'processed', ...},<br/>    {type: 'meeting_scheduled', ...}],<br/>  meetingMetadata: {<br/>    meetingId: '...',<br/>    joinUrl: '...'}<br/>}"]
    end
    
    A --> B
    B --> C
    
    style A fill:#87CEEB
    style B fill:#90EE90
    style C fill:#FFD700
```

## Monitoring and Observability

### Key Metrics Dashboard

```mermaid
graph TB
    subgraph "Trigger Metrics"
        A[Trigger Creation Rate]
        B[Trigger Deletion Rate]
        C[Trigger Age Distribution]
        D[Orphaned Triggers Count]
    end
    
    subgraph "Processing Metrics"
        E[Processing Duration]
        F[Processing Success Rate]
        G[Archive Update Success Rate]
        H[Email Delivery Rate]
    end
    
    subgraph "Queue Metrics"
        I[SQS Queue Depth]
        J[SQS Message Age]
        K[DLQ Message Count]
        L[Message Processing Rate]
    end
    
    subgraph "System Health"
        M[ECS Task Count]
        N[ECS CPU Utilization]
        O[ECS Memory Utilization]
        P[Error Rate by Type]
    end
    
    style A fill:#87CEEB
    style B fill:#87CEEB
    style E fill:#90EE90
    style F fill:#90EE90
    style I fill:#FFB6C1
    style J fill:#FFB6C1
    style M fill:#FFD700
    style N fill:#FFD700
```

### Alert Thresholds

```mermaid
graph LR
    subgraph "Critical Alerts"
        A[Trigger Age > 15 min]
        B[Archive Update Failure > 5%]
        C[Queue Depth > 50]
        D[All ECS Tasks Down]
    end
    
    subgraph "Warning Alerts"
        E[Trigger Age > 10 min]
        F[Archive Update Failure > 2%]
        G[Queue Depth > 20]
        H[ECS CPU > 80%]
    end
    
    subgraph "Info Alerts"
        I[Trigger Delete Failure > 10%]
        J[Processing Duration > 5 min]
        K[Queue Depth > 10]
        L[ECS Memory > 70%]
    end
    
    style A fill:#FF6B6B
    style B fill:#FF6B6B
    style C fill:#FF6B6B
    style D fill:#FF6B6B
    style E fill:#FFA500
    style F fill:#FFA500
    style G fill:#FFA500
    style H fill:#FFA500
    style I fill:#87CEEB
    style J fill:#87CEEB
    style K fill:#87CEEB
    style L fill:#87CEEB
```

## Customer Isolation

### Multi-Customer Processing

```mermaid
graph TB
    subgraph "Single Change - Multiple Customers"
        A[archive/changeId.json<br/>Single Source of Truth]
    end
    
    subgraph "Customer-Specific Triggers"
        B[customers/hts/changeId.json]
        C[customers/htsnonprod/changeId.json]
        D[customers/customer-a/changeId.json]
    end
    
    subgraph "Customer-Specific Queues"
        E[HTS SQS Queue]
        F[HTSNonProd SQS Queue]
        G[Customer A SQS Queue]
    end
    
    subgraph "Customer-Specific Processing"
        H[HTS ECS Task]
        I[HTSNonProd ECS Task]
        J[Customer A ECS Task]
    end
    
    subgraph "Customer-Specific SES"
        K[HTS SES Service]
        L[HTSNonProd SES Service]
        M[Customer A SES Service]
    end
    
    A --> B
    A --> C
    A --> D
    
    B --> E
    C --> F
    D --> G
    
    E --> H
    F --> I
    G --> J
    
    H --> A
    I --> A
    J --> A
    
    H --> K
    I --> L
    J --> M
    
    style A fill:#90EE90
    style B fill:#FFB6C1
    style C fill:#FFB6C1
    style D fill:#FFB6C1
```

## Deployment Architecture

### Infrastructure Components

```mermaid
graph TB
    subgraph "Management Account"
        A[S3 Metadata Bucket]
        B[CloudFront Distribution]
        C[Lambda@Edge Auth]
        D[Enhanced Metadata Lambda]
    end
    
    subgraph "Production Governance Account"
        E[SQS Queues x N]
        F[ECS Cluster]
        G[CloudWatch Logs]
        H[CloudWatch Metrics]
    end
    
    subgraph "Customer Accounts x N"
        I[SES Service]
        J[IAM Role for Backend]
    end
    
    subgraph "External Services"
        K[AWS Identity Center]
        L[Microsoft Graph API]
    end
    
    B --> A
    C --> B
    D --> A
    A --> E
    E --> F
    F --> G
    F --> H
    F --> J
    J --> I
    C --> K
    F --> L
    
    style A fill:#90EE90
    style E fill:#FFB6C1
    style F fill:#87CEEB
    style I fill:#FFD700
```

## Comparison: Old vs New Pattern

### Old Pattern (Version-Based)

```mermaid
graph LR
    A[Frontend] --> B[customers/hts/changeId-v1.json]
    A --> C[archive/changeId-v1.json]
    
    D[Backend] --> E{Load from where?}
    E --> F[customers/ or archive/?]
    
    G[Update] --> H[customers/hts/changeId-v2.json]
    G --> I[archive/changeId-v2.json]
    
    J[Lifecycle Policy] --> K[Delete after 30 days]
    
    style B fill:#FFB6C1
    style C fill:#FFB6C1
    style F fill:#FF6B6B
    style H fill:#FFB6C1
    style I fill:#FFB6C1
```

### New Pattern (Transient Trigger)

```mermaid
graph LR
    A[Frontend] --> B[archive/changeId.json<br/>SINGLE SOURCE]
    A --> C[customers/hts/changeId.json<br/>TRIGGER ONLY]
    
    D[Backend] --> E{Load from where?}
    E --> F[ALWAYS archive/]
    
    G[Update] --> H[archive/changeId.json<br/>UPDATED]
    G --> I[customers/hts/changeId.json<br/>DELETED]
    
    J[No Lifecycle Policy] --> K[Backend handles cleanup]
    
    style B fill:#90EE90
    style C fill:#FFB6C1
    style F fill:#90EE90
    style H fill:#90EE90
    style I fill:#D3D3D3
```

## Related Documentation

- [Architecture Overview](./TRANSIENT_TRIGGER_PATTERN.md)
- [Operational Runbook](./TRANSIENT_TRIGGER_RUNBOOK.md)
- [Troubleshooting Guide](./TRANSIENT_TRIGGER_TROUBLESHOOTING.md)
- [FAQ](./TRANSIENT_TRIGGER_FAQ.md)
