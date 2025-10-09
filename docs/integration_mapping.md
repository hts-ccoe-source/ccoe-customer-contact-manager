# Integration Mapping Matrix

## Overview
This document provides a detailed mapping of each function from the original file to its target internal package, along with conflict resolution strategies for duplicate implementations. This mapping serves as the roadmap for systematic integration of all functions from `ccoe-customer-contact-manager-original.go.bak` into the new modular structure.

## Integration Summary
- **Total Functions Identified:** 37+ functions from original file
- **Functions with Conflicts:** 11 functions exist in both original and modular files
- **Missing Functions:** 26+ functions exist only in original file
- **Target Packages:** 6 internal packages for organized distribution
- **Priority Levels:** HIGH (critical functionality), MEDIUM (enhancements), LOW (utilities)

## Function Integration Matrix

| Function Name | Original File | Current Location | Target Package | Conflict Resolution | Priority |
|---------------|---------------|------------------|----------------|-------------------|----------|
| **Core AWS Utilities** |
| `CreateConnectionConfiguration` | ✅ Original | ❌ Missing | `internal/aws/utils.go` | Extract from original | HIGH |
| `GetManagementAccountIdByPrefix` | ✅ Original | ❌ Missing | `internal/aws/utils.go` | Extract from original | HIGH |
| `GetCurrentAccountId` | ✅ Original | ❌ Missing | `internal/aws/utils.go` | Extract from original | HIGH |
| `IsManagementAccount` | ✅ Original | ❌ Missing | `internal/aws/utils.go` | Extract from original | HIGH |
| `AssumeRole` | ✅ Original | ❌ Missing | `internal/aws/utils.go` | Extract from original | HIGH |
| `GetAllAccountsInOrganization` | ✅ Original | ❌ Missing | `internal/aws/utils.go` | Extract from original | HIGH |
| `GetConfigPath` | ✅ Original | ✅ ses.go | `internal/config/config.go` | Use original, remove from ses.go | MEDIUM |
| **Rate Limiting** |
| `NewRateLimiter` | ✅ Original | ✅ ses.go | `internal/aws/utils.go` | Use original, remove from ses.go | LOW |
| `RateLimiter.Wait` | ✅ Original | ✅ ses.go | `internal/aws/utils.go` | Use original, remove from ses.go | LOW |
| `RateLimiter.Stop` | ✅ Original | ✅ ses.go | `internal/aws/utils.go` | Use original, remove from ses.go | LOW |
| **Alternate Contact CRUD** |
| `GetAlternateContact` | ✅ Original | ❌ Missing | `internal/contacts/alternate_contacts.go` | Extract from original | HIGH |
| `SetAlternateContact` | ✅ Original | ❌ Missing | `internal/contacts/alternate_contacts.go` | Extract from original | HIGH |
| `DeleteAlternateContact` | ✅ Original | ❌ Missing | `internal/contacts/alternate_contacts.go` | Extract from original | HIGH |
| `CheckIfContactExists` | ✅ Original | ❌ Missing | `internal/contacts/alternate_contacts.go` | Extract from original | HIGH |
| `SetAlternateContactIfNotExists` | ✅ Original | ❌ Missing | `internal/contacts/alternate_contacts.go` | Extract from original | HIGH |
| `updateContact` | ❌ Missing | ✅ utils.go | `internal/contacts/alternate_contacts.go` | Replace with original functions | MEDIUM |
| `GetAlternateContacts` | ❌ Missing | ✅ utils.go | `internal/contacts/alternate_contacts.go` | Replace with original functions | MEDIUM |
| `getContact` | ❌ Missing | ✅ utils.go | `internal/contacts/alternate_contacts.go` | Replace with original functions | MEDIUM |
| **Workflow Orchestration** |
| `SetContactsForSingleOrganization` | ✅ Original | ❌ Missing | `internal/contacts/alternate_contacts.go` | Extract from original | HIGH |
| `SetContactsForAllOrganizations` | ✅ Original | ❌ Missing | `internal/contacts/alternate_contacts.go` | Extract from original | HIGH |
| `DeleteContactsFromOrganization` | ✅ Original | ❌ Missing | `internal/contacts/alternate_contacts.go` | Extract from original | HIGH |
| **SES Core Operations** |
| `CreateContactList` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Compare and use better version | MEDIUM |
| `AddContactToList` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Compare and use better version | MEDIUM |
| `RemoveContactFromList` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Compare and use better version | MEDIUM |
| `GetAccountContactList` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Compare and use better version | MEDIUM |
| `ListContactsInList` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Compare and use better version | MEDIUM |
| `DescribeContactList` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Compare and use better version | MEDIUM |
| `AddToSuppressionList` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Compare and use better version | MEDIUM |
| `RemoveFromSuppressionList` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Compare and use better version | MEDIUM |
| `DescribeTopic` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Enhance ses.go with original | MEDIUM |
| `DescribeAllTopics` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Enhance ses.go with original | MEDIUM |
| `DescribeContact` | ✅ Original | ✅ ses.go | `internal/ses/operations.go` | Enhance ses.go with original | MEDIUM |
| **SES Advanced Operations** |
| `validateContactListTopics` | ✅ Original | ❌ Missing | `internal/ses/operations.go` | Extract from original | MEDIUM |
| `AddOrUpdateContactToList` | ✅ Original | ❌ Missing | `internal/ses/operations.go` | Extract from original | MEDIUM |
| `areTopicListsEqual` | ✅ Original | ❌ Missing | `internal/ses/operations.go` | Extract from original | LOW |
| `AddContactTopics` | ✅ Original | ❌ Missing | `internal/ses/operations.go` | Extract from original | MEDIUM |
| `RemoveContactTopics` | ✅ Original | ❌ Missing | `internal/ses/operations.go` | Extract from original | MEDIUM |
| `CreateContactListBackup` | ✅ Original | ❌ Missing | `internal/ses/operations.go` | Extract from original | MEDIUM |
| `RemoveAllContactsFromList` | ✅ Original | ❌ Missing | `internal/ses/operations.go` | Extract from original | MEDIUM |
| `ManageTopics` | ✅ Original | ❌ Missing | `internal/ses/operations.go` | Extract from original | MEDIUM |
| `ExpandTopicsWithGroups` | ❌ Missing | ✅ ses.go | `internal/ses/operations.go` | Keep ses.go version | LOW |
| `ManageSESLists` | ❌ Missing | ✅ ses.go | `internal/ses/operations.go` | Keep ses.go version | LOW |
| **Email Templates & Microsoft Graph** |
| `CreateMeetingInvite` | ✅ Original | ✅ list_management.go | `internal/ses/list_management.go` | Enhance with original | HIGH |
| `SendApprovalRequest` | ✅ Original | ✅ list_management.go | `internal/ses/list_management.go` | Enhance with original | HIGH |
| `SendChangeNotificationWithTemplate` | ✅ Original | ✅ list_management.go | `internal/ses/list_management.go` | Enhance with original | HIGH |
| `generateGraphMeetingPayload` | ✅ Original | ✅ list_management.go | `internal/ses/list_management.go` | Compare and use better | MEDIUM |
| `createGraphMeeting` | ✅ Original | ⚠️ Placeholder | `internal/ses/list_management.go` | Extract from original | HIGH |
| `getGraphAccessToken` | ✅ Original | ✅ list_management.go | `internal/ses/list_management.go` | Compare and use better | MEDIUM |
| **Utility Functions** |
| `UpdateAlternateContacts` | ❌ Missing | ✅ utils.go | `internal/contacts/alternate_contacts.go` | Keep utils.go version | MEDIUM |
| `ValidateCustomerCode` | ❌ Missing | ✅ utils.go | `internal/config/config.go` | Keep utils.go version | LOW |
| `ValidateEmail` | ❌ Missing | ✅ utils.go | `internal/config/config.go` | Keep utils.go version | LOW |
| `SetupLogging` | ❌ Missing | ✅ utils.go | `internal/config/config.go` | Keep utils.go version | LOW |
| `Contains` | ❌ Missing | ✅ utils.go | `internal/config/config.go` | Keep utils.go version | LOW |
| `RemoveDuplicates` | ❌ Missing | ✅ utils.go | `internal/config/config.go` | Keep utils.go version | LOW |

## Type Integration Matrix

| Type Name | Original File | Current Location | Target Package | Conflict Resolution | Priority |
|-----------|---------------|------------------|----------------|-------------------|----------|
| `Organization` | ✅ Original | ✅ types.go | `internal/types/types.go` | Keep types.go version | LOW |
| `AlternateContactConfig` | ✅ Original | ✅ types.go | `internal/types/types.go` | Keep types.go version | LOW |
| `SESTopicConfig` | ✅ Original | ✅ ses.go | `internal/types/types.go` | Consolidate to types.go | MEDIUM |
| `SESConfig` | ✅ Original | ✅ ses.go | `internal/types/types.go` | Consolidate to types.go | MEDIUM |
| `SESBackup` | ✅ Original | ✅ ses.go | `internal/types/types.go` | Consolidate to types.go | MEDIUM |
| `RateLimiter` | ✅ Original | ✅ ses.go | `internal/types/types.go` | Consolidate to types.go | LOW |
| `GraphAuthResponse` | ✅ Original | ✅ list_management.go | `internal/types/types.go` | Consolidate to types.go | LOW |
| `GraphError` | ✅ Original | ✅ list_management.go | `internal/types/types.go` | Consolidate to types.go | LOW |
| `GraphMeetingResponse` | ✅ Original | ✅ list_management.go | `internal/types/types.go` | Consolidate to types.go | LOW |
| `ApprovalRequestMetadata` | ✅ Original | ✅ types.go | `internal/types/types.go` | Keep types.go version | LOW |
| `IdentityCenterUser` | ✅ Original | ❌ Missing | `internal/types/types.go` | Extract from original | MEDIUM |

## Conflict Resolution Strategies

### 1. Extract from Original (HIGH Priority)
**Strategy:** Functions that exist only in the original file and are critical for functionality.
**Action:** Extract complete implementation from original file.
**Functions:** All core AWS utilities, alternate contact CRUD, workflow orchestration.
**Count:** 26+ functions
**Risk:** Low - no conflicts, direct extraction

### 2. Compare and Use Better Version (MEDIUM Priority)
**Strategy:** Functions that exist in both files - compare implementations and use the more complete version.
**Action:** Analyze both implementations, choose the better one, document decision.
**Functions:** SES core operations, some email template functions.
**Count:** 8 functions
**Risk:** Medium - requires careful comparison

### 3. Enhance with Original (HIGH/MEDIUM Priority)
**Strategy:** Functions that exist in modular files but are incomplete compared to original.
**Action:** Enhance existing implementation with missing features from original.
**Functions:** Microsoft Graph functions, email template functions.
**Count:** 3 functions
**Risk:** Medium - requires integration of features

### 4. Keep Modular Version (LOW/MEDIUM Priority)
**Strategy:** Functions that exist only in modular files or are better implemented there.
**Action:** Keep existing implementation, move to appropriate internal package.
**Functions:** Utility functions, some SES helper functions.
**Count:** 6 functions
**Risk:** Low - simple relocation

### 5. Consolidate to Types (LOW/MEDIUM Priority)
**Strategy:** Type definitions scattered across multiple files.
**Action:** Move all types to `internal/types/types.go`.
**Types:** All struct definitions.
**Count:** 10 types
**Risk:** Low - straightforward consolidation

## Function Consolidation Approach

### Step 1: Dependency Analysis
Before moving functions, analyze their dependencies:
1. **Import Dependencies:** What packages each function requires
2. **Type Dependencies:** What types each function uses
3. **Function Dependencies:** What other functions each function calls
4. **Cross-Package Dependencies:** How functions will interact across packages

### Step 2: Conflict Resolution Matrix
For each conflicting function:
1. **Compare Signatures:** Ensure compatibility
2. **Compare Implementations:** Identify differences in logic
3. **Assess Completeness:** Determine which version is more complete
4. **Document Decision:** Record why one version was chosen
5. **Plan Migration:** How to replace the inferior version

### Step 3: Integration Order
Process functions in dependency order:
1. **Types First:** Move all types to avoid import issues
2. **Utilities Second:** Core AWS and configuration utilities
3. **Domain Functions Third:** Alternate contacts, SES operations
4. **Workflow Functions Last:** High-level orchestration functions

## Implementation Order

### Phase 1: High Priority Functions
1. Core AWS utilities (CreateConnectionConfiguration, etc.)
2. Alternate contact CRUD operations
3. Workflow orchestration functions
4. Complete Microsoft Graph integration
5. Complete email template functions

### Phase 2: Medium Priority Functions
1. Advanced SES operations
2. Type consolidation
3. Configuration functions
4. Enhanced SES core operations

### Phase 3: Low Priority Functions
1. Utility functions
2. Rate limiting functions
3. Helper functions
4. Code cleanup and optimization

## Additional Functions to Investigate

Based on the requirements and design documents, there may be additional functions in the original file that need to be cataloged:

### Identity Center Integration Functions
- Functions for Identity Center user management
- Contact import/export from Identity Center
- User synchronization functions

### S3 and Lambda Processing Functions
- S3 payload parsing functions
- S3 metadata extraction functions
- Lambda event processing functions
- S3 change metadata to email message mapping

### Advanced Email Template Functions
- Template rendering and processing utilities
- ICS calendar generation functions
- Advanced meeting creation functions

### Missing Helper Functions
- Additional utility functions not yet cataloged
- Error handling and logging functions
- Configuration validation functions

**Action Required:** Complete function catalog by scanning entire original file for any missed functions.

## Validation Checklist

### Pre-Integration Validation
- [ ] Complete function catalog is created
- [ ] All functions from original file are accounted for
- [ ] All conflicts are identified and resolution planned
- [ ] Target packages are clearly defined
- [ ] Dependencies are mapped

### Integration Validation
- [ ] Functions are moved in correct order
- [ ] All imports are properly resolved
- [ ] No functionality is lost during integration
- [ ] Function signatures remain compatible
- [ ] All types are properly consolidated

### Post-Integration Validation
- [ ] Code compiles successfully
- [ ] No circular dependencies exist
- [ ] Both CLI and Lambda modes work
- [ ] All workflow functions are operational
- [ ] Original files can be safely removed

## Detailed Integration Plan

### Package Creation Order
1. **`internal/types/types.go`** - Create first to avoid import issues
2. **`internal/config/config.go`** - Configuration utilities
3. **`internal/aws/utils.go`** - Core AWS utilities
4. **`internal/contacts/alternate_contacts.go`** - Contact management
5. **`internal/ses/operations.go`** - SES operations
6. **`internal/ses/list_management.go`** - Email templates and Microsoft Graph
7. **`internal/lambda/handlers.go`** - Lambda-specific handlers (if needed)

### File Elimination Strategy
After successful integration, eliminate these root-level files:
- `ses.go` - Functions moved to `internal/ses/`
- `list_management.go` - Functions moved to `internal/ses/`
- `types.go` - Types moved to `internal/types/`
- `utils.go` - Functions distributed to domain packages
- `ccoe-customer-contact-manager-original.go.bak` - Source file (after validation)

### Import Update Strategy
Update imports in this order:
1. **main.go** - Update to use internal packages
2. **CLI handlers** - Update function calls
3. **Lambda handlers** - Update function calls
4. **Test files** - Update test imports (if any)

## Risk Mitigation

### Compilation Risks
- **Circular Dependencies:** Carefully design package dependencies
- **Missing Imports:** Validate all required imports are included
- **Type Conflicts:** Ensure types are properly consolidated

### Functionality Risks
- **Lost Features:** Validate all functions are properly integrated
- **Signature Changes:** Ensure backward compatibility
- **Behavior Changes:** Test that integrated functions work identically

### Integration Risks
- **Incomplete Extraction:** Use systematic approach to avoid missing functions
- **Conflict Resolution:** Document all decisions for future reference
- **Testing Gaps:** Validate both CLI and Lambda modes after integration

## Success Criteria

### Functional Success
- [ ] All functions from original file are integrated
- [ ] No functionality is lost during integration
- [ ] Both CLI and Lambda modes work correctly
- [ ] All workflow orchestration functions work

### Technical Success
- [ ] Clean compilation with no errors
- [ ] No circular dependencies
- [ ] Proper separation of concerns
- [ ] Clean import structure

### Organizational Success
- [ ] Logical package organization
- [ ] Clear function categorization
- [ ] Maintainable code structure
- [ ] Documentation of changes

## Notes

1. **Original File Priority:** As stated in requirements, functions in the original file are presumed to be "more complete"
2. **Systematic Approach:** Process functions in priority order to minimize integration issues
3. **Compilation Validation:** Test compilation after each major integration step
4. **Backup Strategy:** Keep original files until integration is complete and validated
5. **Incremental Progress:** Complete one package at a time to maintain working state
6. **Dependency Management:** Resolve dependencies before moving dependent functions