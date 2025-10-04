# Design Document

## Overview

This design outlines the systematic approach to complete the integration of functions from the original monolithic `aws-alternate-contact-manager-original.go.bak` file into the existing modular codebase. The design focuses on function analysis, categorization, and strategic placement while avoiding duplication and maintaining clean architecture.

## Architecture

### Current State Analysis

The codebase currently has the following modular structure:
- `main.go` - Entry point with two operation modes:
  - **CLI Mode:** Command-line interface for manual operations
  - **Lambda Mode:** AWS Lambda wrapper for automated processing
- `ses.go` - SES contact list management and some utility functions
- `list_management.go` - Microsoft Graph and email template functions (partially complete)
- `types.go` - Type definitions and structs
- `utils.go` - Miscellaneous utility functions (to be distributed to domain-specific modules)
- `aws-alternate-contact-manager-original.go.bak` - Original monolithic file (source of truth)

### Cleanup Strategy

**`utils.go` Elimination:** Functions currently in `utils.go` will be analyzed and moved to appropriate domain-specific modules based on their functionality and dependencies.

### Operation Modes

**CLI Mode:**
- Direct command-line execution with subcommands (`alt-contact`, `ses`, etc.)
- Interactive parameter handling via flags
- Immediate execution and output
- Used for manual administration and testing

**Lambda Mode:**
- Triggered by AWS Lambda runtime environment
- Processes S3 events containing change metadata payloads
- Maps S3 payload data to email message types using templates
- Automated workflow execution for change notifications and approvals
- Uses email templates for approval requests, change notifications, and meeting invites
- Used for production automation and event-driven processing

### Target Architecture

The integration will result in the following enhanced modular structure:

```
├── main.go                           # Entry point with dual-mode support
├── Makefile                          # Updated build configuration
├── go.mod                            # Go module dependencies
├── go.sum                            # Go module checksums
├── internal/                         # Internal application packages
│   ├── aws/
│   │   └── utils.go                  # Core AWS utility functions
│   ├── contacts/
│   │   └── alternate_contacts.go     # Alternate contact management
│   ├── ses/
│   │   ├── operations.go             # Enhanced SES operations
│   │   └── list_management.go        # Email templates and Microsoft Graph
│   ├── lambda/
│   │   └── handlers.go               # Lambda-specific handlers
│   ├── config/
│   │   └── config.go                 # Configuration management
│   └── types/
│       └── types.go                  # All type definitions
└── aws-alternate-contact-manager-original.go.bak  # Original file (to be removed)
```

### Project Organization Benefits

**Clean Root Directory:** Only essential files (`main.go`, `Makefile`, `go.mod`) remain in root
**Logical Grouping:** Related functionality is grouped in domain-specific packages
**Go Best Practices:** Follows Go project layout conventions with `internal/` directory
**Import Clarity:** Package imports will be more descriptive (e.g., `internal/aws`, `internal/contacts`)

### Function Distribution Strategy

**Eliminate `utils.go`:** All utility functions will be moved to domain-specific packages:
- AWS-related utilities → `internal/aws/utils.go`
- Contact management utilities → `internal/contacts/alternate_contacts.go`
- SES-related utilities → `internal/ses/operations.go`
- Email/template utilities → `internal/ses/list_management.go`
- Configuration utilities → `internal/config/config.go`

### Package Structure

Each internal package will have clear responsibilities:
- `internal/aws` - AWS service interactions and utilities
- `internal/contacts` - Alternate contact management operations
- `internal/ses` - SES operations and email management
- `internal/lambda` - Lambda-specific event handling
- `internal/config` - Configuration loading and management
- `internal/types` - Shared type definitions

### Mode-Specific Considerations

**CLI Mode Requirements:**
- Functions must support direct parameter passing
- Error handling should provide user-friendly messages
- Progress output should be visible to terminal users
- Configuration loading from files and environment variables

**Lambda Mode Requirements:**
- Functions must support S3 event-driven triggers
- S3 payload parsing and metadata extraction capabilities
- Email template processing and rendering functions
- Mapping from S3 change metadata to structured email messages
- Error handling should work with Lambda logging
- Integration with SES for email delivery
- Support for approval workflows and change notifications

## Components and Interfaces

### 1. AWS Utilities Package (`internal/aws/utils.go`)

**Purpose:** Centralize core AWS service interactions and credential management.

**Functions to Extract:**
- `CreateConnectionConfiguration(creds aws.Credentials) (aws.Config, error)`
- `GetManagementAccountIdByPrefix(prefix string, orgConfig []Organization) (string, error)`
- `GetCurrentAccountId(StsServiceConnection *sts.Client) string`
- `IsManagementAccount(OrganizationsServiceConnection *organizations.Client, AccountId string) bool`
- `AssumeRole(stsClient *sts.Client, roleArn string, sessionName string) (*ststypes.Credentials, error)`
- `GetAllAccountsInOrganization(OrganizationsServiceConnection *organizations.Client) ([]organizationsTypes.Account, error)`

**Conflict Resolution:**
- `GetConfigPath()` already exists in `ses.go` - consolidate into `internal/config/config.go`
- Ensure no duplicate implementations

### 2. Alternate Contacts Package (`internal/contacts/alternate_contacts.go`)

**Purpose:** Handle all alternate contact CRUD operations and workflow orchestration.

**Functions to Extract:**
- `GetAlternateContact(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType) (*accountTypes.AlternateContact, error)`
- `SetAlternateContact(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType, name, title, email, phone string) error`
- `DeleteAlternateContact(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType) error`
- `CheckIfContactExists(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType) (bool, error)`
- `SetAlternateContactIfNotExists(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType, name, title, email, phone string, overwrite bool) error`
- `SetContactsForSingleOrganization(contactConfigFile *string, orgPrefix *string, overwrite *bool)`
- `SetContactsForAllOrganizations(contactConfigFile *string, overwrite *bool)`
- `DeleteContactsFromOrganization(orgPrefix *string, contactTypes *string)`

### 3. Enhanced SES Package (`internal/ses/operations.go`)

**Purpose:** Consolidate all SES operations including advanced features from the original file.

**Functions to Enhance/Add:**
- Complete implementations of SES contact management functions
- Advanced topic management functions
- Identity Center integration functions
- Contact import/export functions

**Conflict Resolution:**
- Compare existing functions with original implementations
- Preserve the most complete version
- Ensure all advanced features are included

### 4. List Management Package (`internal/ses/list_management.go`)

**Purpose:** Handle email templates, Microsoft Graph integration, and S3 payload processing for Lambda mode.

**Functions to Complete:**
- S3 payload parsing and metadata extraction functions
- Email template processing and rendering functions
- S3 change metadata to email message mapping functions
- Complete Microsoft Graph meeting creation functions
- ICS calendar generation functions
- Template rendering and processing utilities

**Lambda Mode Integration:**
- Functions must support processing S3 event payloads
- Template system must handle dynamic content from S3 metadata
- Email message generation from structured change data

## Data Models

### Function Categorization Matrix

| Function Category | Target Package | Conflict Resolution Strategy |
|------------------|----------------|----------------------------|
| AWS Credentials & Config | `internal/aws/utils.go` | Extract from original, remove duplicates |
| Organization Management | `internal/aws/utils.go` | Extract from original |
| Alternate Contact CRUD | `internal/contacts/alternate_contacts.go` | Extract from original |
| Contact Workflows | `internal/contacts/alternate_contacts.go` | Extract from original |
| SES Core Operations | `internal/ses/operations.go` | Enhance existing with original |
| SES Advanced Features | `internal/ses/operations.go` | Add missing from original |
| Email Templates | `internal/ses/list_management.go` | Complete existing implementations |
| Microsoft Graph | `internal/ses/list_management.go` | Complete existing implementations |
| General Utilities (from utils.go) | Domain-specific packages | Distribute based on functionality |
| Configuration Management | `internal/config/config.go` | Consolidate configuration functions |
| Lambda Handlers | `internal/lambda/handlers.go` | Extract Lambda-specific code |

### Import Dependencies

Each package will have clearly defined import requirements:

```go
// main.go
import (
    "internal/aws"
    "internal/contacts"
    "internal/ses"
    "internal/lambda"
    "internal/config"
    "internal/types"
)

// internal/aws/utils.go
import (
    "context"
    "fmt"
    "os"
    "strings"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/organizations"
    "github.com/aws/aws-sdk-go-v2/service/sts"
    "internal/types"
)

// internal/contacts/alternate_contacts.go
import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "strings"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/account"
    "internal/aws"
    "internal/types"
)
```

### Makefile Updates

The Makefile will need updates to handle the new package structure:
- Build commands must account for internal packages
- Test commands must cover all internal packages
- Linting and formatting must include internal directory
- Binary output and deployment scripts may need adjustment

## Error Handling

### Duplicate Function Resolution Strategy

1. **Analysis Phase:** Compare function signatures and implementations
2. **Evaluation Phase:** Determine which version is more complete/correct
3. **Integration Phase:** Preserve the better implementation
4. **Cleanup Phase:** Remove duplicates and update references

### Missing Dependency Resolution

1. **Identification:** Scan for missing imports and undefined functions
2. **Resolution:** Add required imports and dependencies
3. **Validation:** Ensure all dependencies are available in go.mod
4. **Testing:** Verify compilation success

## Testing Strategy

### Compilation Validation

1. **Individual Module Testing:** Compile each module separately
2. **Integration Testing:** Compile entire codebase
3. **Dependency Verification:** Ensure all imports resolve correctly
4. **Function Signature Validation:** Verify no breaking changes

### Function Preservation Validation

1. **Function Inventory:** Create complete list of functions in original file
2. **Integration Tracking:** Track which functions have been integrated
3. **Completeness Check:** Verify all functions are accounted for
4. **Functionality Verification:** Ensure integrated functions work as expected

### Conflict Resolution Testing

1. **Duplicate Detection:** Identify all duplicate function definitions
2. **Implementation Comparison:** Compare duplicate implementations
3. **Resolution Validation:** Verify chosen implementation is correct
4. **Reference Updates:** Ensure all callers use correct implementation

## Implementation Phases

### Phase 1: Project Restructuring and Function Analysis
- Create `internal/` directory structure with domain-specific packages
- Systematically identify all functions in original file
- Analyze existing functions in current modular files for redistribution
- Categorize functions by domain and target package
- Update Makefile for new package structure
- Create integration plan with conflict resolution strategy
- Plan elimination of root-level modular files and function redistribution

### Phase 2: Core AWS Utilities Integration
- Extract and integrate AWS utility functions
- Resolve `GetConfigPath()` duplication
- Ensure proper import dependencies
- Validate compilation

### Phase 3: Alternate Contact Functions Integration
- Extract all alternate contact CRUD operations
- Extract workflow orchestration functions
- Integrate into new `alternate_contacts.go` module
- Validate functionality preservation

### Phase 4: SES Enhancement
- Compare existing SES functions with original implementations
- Integrate missing advanced SES features
- Consolidate duplicate implementations
- Enhance existing functions with original features

### Phase 5: List Management Completion
- Complete S3 payload parsing and metadata extraction functions
- Finish email template processing and rendering functions
- Complete S3 change metadata to email message mapping
- Complete Microsoft Graph integration functions
- Add missing helper functions for Lambda mode
- Validate email and calendar functionality for both CLI and Lambda modes

### Phase 6: Final Integration and Cleanup
- Resolve any remaining conflicts
- Update main.go to import from internal packages
- Update main.go CLI handlers to use new package structure
- Update Lambda handlers to use new package structure
- Ensure both operation modes work correctly with new structure
- Remove old root-level modular files and original backup file
- Update Makefile build, test, and deployment targets
- Perform final compilation and validation testing for both modes