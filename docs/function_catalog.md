# Function Catalog from ccoe-customer-contact-manager-original.go.bak

## Overview
This document catalogs all functions found in the original monolithic file that need to be analyzed for integration into the modular codebase.

## Core AWS Utility Functions
These functions handle AWS service connections, credentials, and organization management:

1. **CreateConnectionConfiguration(creds aws.Credentials) (aws.Config, error)**
   - Creates AWS configuration using provided credentials
   - Target: `internal/aws/utils.go`

2. **GetManagementAccountIdByPrefix(prefix string, orgConfig []Organization) (string, error)**
   - Gets management account ID by organization prefix
   - Target: `internal/aws/utils.go`

3. **GetConfigPath() string**
   - Returns CONFIG_PATH environment variable or defaults to current directory
   - Target: `internal/config/config.go` (consolidate with existing)

4. **GetCurrentAccountId(StsServiceConnection *sts.Client) string**
   - Gets current AWS account ID
   - Target: `internal/aws/utils.go`

5. **IsManagementAccount(OrganizationsServiceConnection *organizations.Client, AccountId string) bool**
   - Checks if current account is management account
   - Target: `internal/aws/utils.go`

6. **AssumeRole(stsClient *sts.Client, roleArn string, sessionName string) (*ststypes.Credentials, error)**
   - Assumes AWS IAM role and returns credentials
   - Target: `internal/aws/utils.go`

7. **GetAllAccountsInOrganization(OrganizationsServiceConnection *organizations.Client) ([]organizationsTypes.Account, error)**
   - Lists all accounts in organization
   - Target: `internal/aws/utils.go`

## Alternate Contact Management Functions
These functions handle CRUD operations for alternate contacts:

8. **GetAlternateContact(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType) (*accountTypes.AlternateContact, error)**
   - Retrieves alternate contact information for an account
   - Target: `internal/contacts/alternate_contacts.go`

9. **SetAlternateContact(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType, name, title, email, phone string) error**
   - Sets or updates alternate contact information for an account
   - Target: `internal/contacts/alternate_contacts.go`

10. **DeleteAlternateContact(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType) error**
    - Removes alternate contact information for an account
    - Target: `internal/contacts/alternate_contacts.go`

11. **CheckIfContactExists(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType) (bool, error)**
    - Checks if an alternate contact exists
    - Target: `internal/contacts/alternate_contacts.go`

12. **SetAlternateContactIfNotExists(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType, name, title, email, phone string, overwrite bool) error**
    - Sets alternate contact only if it doesn't already exist
    - Target: `internal/contacts/alternate_contacts.go`

## Workflow Orchestration Functions
These functions handle bulk operations across organizations:

13. **SetContactsForSingleOrganization(contactConfigFile *string, orgPrefix *string, overwrite *bool)**
    - Sets contacts workflow for single organization
    - Target: `internal/contacts/alternate_contacts.go`

14. **DeleteContactsFromOrganization(orgPrefix *string, contactTypes *string)**
    - Delete contacts workflow for organization
    - Target: `internal/contacts/alternate_contacts.go`

15. **SetContactsForAllOrganizations(contactConfigFile *string, overwrite *bool)**
    - Sets contacts for all organizations in config file
    - Target: `internal/contacts/alternate_contacts.go`

## SES Management Functions
These functions handle SES contact lists, topics, and email management:

16. **CreateContactList(sesClient *sesv2.Client, listName string, description string, topicConfigs []SESTopicConfig) error**
    - Creates a new contact list in SES
    - Target: `internal/ses/operations.go`

17. **validateContactListTopics(sesClient *sesv2.Client, listName string, config ContactImportConfig) error**
    - Checks if required topics exist in contact list
    - Target: `internal/ses/operations.go`

18. **AddContactToList(sesClient *sesv2.Client, listName string, email string, explicitTopics []string) error**
    - Adds an email contact to a contact list
    - Target: `internal/ses/operations.go`

19. **AddOrUpdateContactToList(sesClient *sesv2.Client, listName string, email string, explicitTopics []string) (string, error)**
    - Adds contact or updates existing contact's topic subscriptions (idempotent)
    - Target: `internal/ses/operations.go`

20. **areTopicListsEqual(list1, list2 []string) bool**
    - Checks if two topic lists contain the same topics
    - Target: `internal/ses/operations.go`

21. **RemoveContactFromList(sesClient *sesv2.Client, listName string, email string) error**
    - Removes an email contact from a contact list
    - Target: `internal/ses/operations.go`

22. **AddContactTopics(sesClient *sesv2.Client, listName string, email string, topics []string) error**
    - Adds explicit topic subscriptions to existing contact
    - Target: `internal/ses/operations.go`

23. **RemoveContactTopics(sesClient *sesv2.Client, listName string, email string, topics []string) error**
    - Removes explicit topic subscriptions from existing contact
    - Target: `internal/ses/operations.go`

24. **CreateContactListBackup(sesClient *sesv2.Client, listName string, action string) (string, error)**
    - Creates backup of contact list with all contacts and topics
    - Target: `internal/ses/operations.go`

25. **RemoveAllContactsFromList(sesClient *sesv2.Client, listName string) error**
    - Removes all contacts from contact list after creating backup
    - Target: `internal/ses/operations.go`

26. **AddToSuppressionList(sesClient *sesv2.Client, email string, reason sesv2Types.SuppressionListReason) error**
    - Adds email to account-level suppression list
    - Target: `internal/ses/operations.go`

27. **RemoveFromSuppressionList(sesClient *sesv2.Client, email string) error**
    - Removes email from account-level suppression list
    - Target: `internal/ses/operations.go`

28. **GetAccountContactList(sesClient *sesv2.Client) (string, error)**
    - Gets the first/main contact list for the account
    - Target: `internal/ses/operations.go`

29. **DescribeContactList(sesClient *sesv2.Client, listName string) error**
    - Provides detailed information about a contact list
    - Target: `internal/ses/operations.go`

30. **ListContactsInList(sesClient *sesv2.Client, listName string) error**
    - Lists all contacts in specific contact list with topic subscriptions
    - Target: `internal/ses/operations.go`

31. **DescribeTopic(sesClient *sesv2.Client, topicName string) error**
    - Provides detailed information about specific topic
    - Target: `internal/ses/operations.go`

32. **DescribeContact(sesClient *sesv2.Client, email string) error**
    - Provides detailed information about specific contact
    - Target: `internal/ses/operations.go`

33. **DescribeAllTopics(sesClient *sesv2.Client) error**
    - Provides detailed information about all topics in account's contact list
    - Target: `internal/ses/operations.go`

34. **ManageTopics(sesClient *sesv2.Client, configTopics []SESTopicConfig, dryRun bool) error**
    - Manages topics in account's contact list based on configuration
    - Target: `internal/ses/operations.go`

## Rate Limiting Functions
These functions handle API rate limiting:

35. **NewRateLimiter(requestsPerSecond int) *RateLimiter**
    - Creates new rate limiter with specified requests per second
    - Target: `internal/aws/utils.go`

36. **(rl *RateLimiter) Wait()**
    - Blocks until a request can be made
    - Target: `internal/aws/utils.go`

37. **(rl *RateLimiter) Stop()**
    - Stops the rate limiter
    - Target: `internal/aws/utils.go`

## Additional Functions Found (Partial List)
The file contains many more functions that need to be cataloged. Based on the search results, there are additional functions for:

- Identity Center integration
- Microsoft Graph API integration
- Email template processing
- S3 payload processing for Lambda mode
- Meeting creation and calendar functions
- Contact import/export functions
- Advanced SES topic management
- Lambda event handling

## Type Definitions Found
The original file also contains several type definitions that need to be moved to `internal/types/types.go`:

- **Organization** - Organization configuration structure
- **AlternateContactConfig** - Alternate contact configuration structure
- **SESTopicConfig** - SES topic configuration structure
- **SESConfig** - SES configuration structure
- **SESBackup** - SES backup structure
- **GraphAuthResponse** - Microsoft Graph authentication response
- **GraphError** - Microsoft Graph error structure
- **GraphMeetingResponse** - Microsoft Graph meeting response
- **RateLimiter** - Rate limiter structure
- **IdentityCenterUser** - Identity Center user structure

## Next Steps
1. Complete the function catalog by analyzing the remaining functions in the file
2. Compare each function with existing implementations in modular files
3. Create integration mapping for conflict resolution
4. Begin systematic extraction and integration process