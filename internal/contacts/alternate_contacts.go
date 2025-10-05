// Package contacts handles all alternate contact CRUD operations and workflow orchestration.
package contacts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/account"
	accountTypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	awsutils "aws-alternate-contact-manager/internal/aws"
	"aws-alternate-contact-manager/internal/config"
	"aws-alternate-contact-manager/internal/types"
)

// GetAlternateContact retrieves the alternate contact information for an account
func GetAlternateContact(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType) (*accountTypes.AlternateContact, error) {
	input := &account.GetAlternateContactInput{
		AccountId:            aws.String(accountId),
		AlternateContactType: contactType,
	}

	result, err := AccountServiceConnection.GetAlternateContact(context.Background(), input)
	if err != nil {
		// If the contact doesn't exist, return nil without error
		var nfe *accountTypes.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get alternate contact: %w", err)
	}

	return result.AlternateContact, nil
}

// SetAlternateContact sets or updates the alternate contact information for an account
func SetAlternateContact(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType, name, title, email, phone string) error {
	input := &account.PutAlternateContactInput{
		AccountId:            aws.String(accountId),
		AlternateContactType: contactType,
		Name:                 aws.String(name),
		Title:                aws.String(title),
		EmailAddress:         aws.String(email),
		PhoneNumber:          aws.String(phone),
	}

	_, err := AccountServiceConnection.PutAlternateContact(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to set alternate contact: %w", err)
	}

	return nil
}

// DeleteAlternateContact removes the alternate contact information for an account
func DeleteAlternateContact(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType) error {
	input := &account.DeleteAlternateContactInput{
		AccountId:            aws.String(accountId),
		AlternateContactType: contactType,
	}

	_, err := AccountServiceConnection.DeleteAlternateContact(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to delete alternate contact: %w", err)
	}

	return nil
}

// CheckIfContactExists checks if an alternate contact exists and returns true/false
func CheckIfContactExists(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType) (bool, error) {
	contact, err := GetAlternateContact(AccountServiceConnection, accountId, contactType)
	if err != nil {
		return false, err
	}
	return contact != nil, nil
}

// SetAlternateContactIfNotExists sets an alternate contact only if it doesn't already exist
func SetAlternateContactIfNotExists(AccountServiceConnection *account.Client, accountId string, contactType accountTypes.AlternateContactType, name, title, email, phone string, overwrite bool) error {
	exists, err := CheckIfContactExists(AccountServiceConnection, accountId, contactType)
	if err != nil {
		return fmt.Errorf("failed to check if contact exists: %w", err)
	}

	if exists && !overwrite {
		fmt.Printf("Alternate contact %s already exists for account %s, will not overwrite.\n", contactType, accountId)
		return nil
	}

	err = SetAlternateContact(AccountServiceConnection, accountId, contactType, name, title, email, phone)
	if err != nil {
		return fmt.Errorf("failed to set alternate contact %s for account %s: %w", contactType, accountId, err)
	}

	if exists {
		fmt.Printf("Successfully updated alternate contact %s for account: %s\n", contactType, accountId)
	} else {
		fmt.Printf("Successfully added alternate contact %s for account: %s\n", contactType, accountId)
	}
	return nil
}

// SetContactsForSingleOrganization sets contacts for a single organization
func SetContactsForSingleOrganization(contactConfigFile *string, orgPrefix *string, overwrite *bool) {
	ConfigPath := config.GetConfigPath()
	fmt.Println("Working in Config Path: " + ConfigPath)

	//Read the Contact Config Json File
	ContactJson, err := os.ReadFile(ConfigPath + *contactConfigFile)
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + *contactConfigFile)

	var ContactConfig types.AlternateContactConfig
	json.NewDecoder(bytes.NewReader(ContactJson)).Decode(&ContactConfig)

	//Read the Org Json File
	OrgJson, err := os.ReadFile(ConfigPath + "OrgConfig.json")
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + "OrgConfig.json")

	var OrgConfig []types.Organization
	json.NewDecoder(bytes.NewReader(OrgJson)).Decode(&OrgConfig)

	ManagementAccountId, err := awsutils.GetManagementAccountIdByPrefix(*orgPrefix, OrgConfig)
	if err != nil {
		fmt.Printf("failed to get management account ID: %v\n", err)
		return
	}
	fmt.Printf("Management Account ID for prefix %s: %s\n", *orgPrefix, ManagementAccountId)

	var SecureTokenServiceConnection *sts.Client
	var CurrentAccountId string

	// Load the default AWS configuration
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Printf("failed to load AWS configuration: %v\n", err)
		return
	}

	// Use the default credentials from the role this ECS task is running as
	creds, err := cfg.Credentials.Retrieve(context.TODO())
	if err != nil {
		fmt.Printf("failed to retrieve AWS credentials: %v\n", err)
		return
	}

	fmt.Println("Using credentials from the role this ECS task is running as")
	fmt.Printf("Access Key ID: %s\n", creds.AccessKeyID)

	InitialCreds := aws.Credentials{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Source:          "environment",
	}

	cfg, err = awsutils.CreateConnectionConfiguration(InitialCreds)
	if err != nil {
		fmt.Println("failed to load AWS API configuration : %w", err)
		return
	}

	SecureTokenServiceConnection = sts.NewFromConfig(cfg)
	OrganizationsServiceConnection := organizations.NewFromConfig(cfg)
	CurrentAccountId = awsutils.GetCurrentAccountId(SecureTokenServiceConnection)

	fmt.Println("Processing Organization: " + *orgPrefix)
	fmt.Println()

	var finalCfg aws.Config

	if awsutils.IsManagementAccount(OrganizationsServiceConnection, CurrentAccountId) {
		fmt.Println(CurrentAccountId + " IS Organization Management Account")
		fmt.Println("Will use initial credentials: " + InitialCreds.AccessKeyID)
		finalCfg = cfg
	} else {
		fmt.Println(CurrentAccountId + " NOT Organization Management Account")
		fmt.Println("Need a Role inside the Organization Management Account")
		ManagementRoleArn := "arn:aws:iam::" + ManagementAccountId + ":role/otc/hts-ccoe-mocb-alt-contact-manager"
		ManagementSessionName := *orgPrefix + "-alt-contact-manager"
		fmt.Println("Attempting to switch into Role: " + ManagementRoleArn)

		ManagementAssumedCreds, err := awsutils.AssumeRole(SecureTokenServiceConnection, ManagementRoleArn, ManagementSessionName)
		if err != nil {
			fmt.Println("failed to assume role: %w", err)
			return
		}
		fmt.Println("Assumed Management role credentials:", *ManagementAssumedCreds.AccessKeyId)

		ManagementAwsCreds := aws.Credentials{
			AccessKeyID:     *ManagementAssumedCreds.AccessKeyId,
			SecretAccessKey: *ManagementAssumedCreds.SecretAccessKey,
			SessionToken:    *ManagementAssumedCreds.SessionToken,
			Source:          "AssumeRole",
		}

		ManagementCfg, err := awsutils.CreateConnectionConfiguration(ManagementAwsCreds)
		if err != nil {
			fmt.Printf("failed to load AWS API configuration with assumed role in Management account: %v\n", err)
			return
		}
		finalCfg = ManagementCfg
	}

	OrganizationsServiceConnection = organizations.NewFromConfig(finalCfg)

	accounts, err := awsutils.GetAllAccountsInOrganization(OrganizationsServiceConnection)
	if err != nil {
		fmt.Printf("failed to get the accounts in the organization: %v\n", err)
		return
	}

	fmt.Println("Accounts in the organization:")
	for _, account := range accounts {
		fmt.Println(*account.Name + " - " + *account.Id)
	}
	fmt.Println()

	// Create Account service connection with the final configuration
	AccountServiceConnection := account.NewFromConfig(finalCfg)

	// Set alternate contacts for each account in the organization
	for _, acct := range accounts {
		accountId := *acct.Id
		fmt.Println("Processing account: " + accountId)

		// Set Security Contact
		if ContactConfig.SecurityEmail != "" {
			err = SetAlternateContactIfNotExists(AccountServiceConnection, accountId, accountTypes.AlternateContactTypeSecurity,
				ContactConfig.SecurityName, ContactConfig.SecurityTitle, ContactConfig.SecurityEmail, ContactConfig.SecurityPhone, *overwrite)
			if err != nil {
				fmt.Printf("failed to set security contact for account %s: %v\n", accountId, err)
			}
		}

		// Set Billing Contact
		if ContactConfig.BillingEmail != "" {
			err = SetAlternateContactIfNotExists(AccountServiceConnection, accountId, accountTypes.AlternateContactTypeBilling,
				ContactConfig.BillingName, ContactConfig.BillingTitle, ContactConfig.BillingEmail, ContactConfig.BillingPhone, *overwrite)
			if err != nil {
				fmt.Printf("failed to set billing contact for account %s: %v\n", accountId, err)
			}
		}

		// Set Operations Contact
		if ContactConfig.OperationsEmail != "" {
			err = SetAlternateContactIfNotExists(AccountServiceConnection, accountId, accountTypes.AlternateContactTypeOperations,
				ContactConfig.OperationsName, ContactConfig.OperationsTitle, ContactConfig.OperationsEmail, ContactConfig.OperationsPhone, *overwrite)
			if err != nil {
				fmt.Printf("failed to set operations contact for account %s: %v\n", accountId, err)
			}
		}

		fmt.Println()
	}

	// Reinitialize the AWS configuration with the initial credentials
	cfg, err = awsutils.CreateConnectionConfiguration(InitialCreds)
	if err != nil {
		fmt.Printf("failed to reload AWS API configuration: %v\n", err)
		return
	}

	SecureTokenServiceConnection = sts.NewFromConfig(cfg)
	CurrentAccountId = awsutils.GetCurrentAccountId(SecureTokenServiceConnection)
	fmt.Println("Switched back to initial credentials in " + CurrentAccountId)
}

// SetContactsForAllOrganizations sets contacts for all organizations in the config file
func SetContactsForAllOrganizations(contactConfigFile *string, overwrite *bool) {
	ConfigPath := config.GetConfigPath()
	fmt.Println("Working in Config Path: " + ConfigPath)

	//Read the Contact Config Json File
	ContactJson, err := os.ReadFile(ConfigPath + *contactConfigFile)
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + *contactConfigFile)

	var ContactConfig types.AlternateContactConfig
	json.NewDecoder(bytes.NewReader(ContactJson)).Decode(&ContactConfig)

	//Read the Org Json File
	OrgJson, err := os.ReadFile(ConfigPath + "OrgConfig.json")
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + "OrgConfig.json")

	var OrgConfig []types.Organization
	json.NewDecoder(bytes.NewReader(OrgJson)).Decode(&OrgConfig)

	fmt.Printf("Found %d organizations in config file\n", len(OrgConfig))
	fmt.Println()

	// Process each organization
	for i, org := range OrgConfig {
		fmt.Printf("Processing organization %d of %d: %s (prefix: %s)\n", i+1, len(OrgConfig), org.FriendlyName, org.Prefix)

		// Call the existing single organization function
		orgPrefix := org.Prefix
		SetContactsForSingleOrganization(contactConfigFile, &orgPrefix, overwrite)

		fmt.Printf("Completed processing organization: %s\n", org.FriendlyName)
		fmt.Println("=" + strings.Repeat("=", 50))
		fmt.Println()
	}

	fmt.Printf("Successfully processed all %d organizations\n", len(OrgConfig))
}

// DeleteContactsFromOrganization deletes contacts from an organization
func DeleteContactsFromOrganization(orgPrefix *string, contactTypes *string) {
	ConfigPath := config.GetConfigPath()
	fmt.Println("Working in Config Path: " + ConfigPath)

	//Read the Org Json File
	OrgJson, err := os.ReadFile(ConfigPath + "OrgConfig.json")
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + "OrgConfig.json")

	var OrgConfig []types.Organization
	json.NewDecoder(bytes.NewReader(OrgJson)).Decode(&OrgConfig)

	ManagementAccountId, err := awsutils.GetManagementAccountIdByPrefix(*orgPrefix, OrgConfig)
	if err != nil {
		fmt.Printf("failed to get management account ID: %v\n", err)
		return
	}
	fmt.Printf("Management Account ID for prefix %s: %s\n", *orgPrefix, ManagementAccountId)

	var SecureTokenServiceConnection *sts.Client
	var CurrentAccountId string

	// Load the default AWS configuration
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Printf("failed to load AWS configuration: %v\n", err)
		return
	}

	// Use the default credentials from the role this ECS task is running as
	creds, err := cfg.Credentials.Retrieve(context.TODO())
	if err != nil {
		fmt.Printf("failed to retrieve AWS credentials: %v\n", err)
		return
	}

	fmt.Println("Using credentials from the role this ECS task is running as")
	fmt.Printf("Access Key ID: %s\n", creds.AccessKeyID)

	InitialCreds := aws.Credentials{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Source:          "environment",
	}

	cfg, err = awsutils.CreateConnectionConfiguration(InitialCreds)
	if err != nil {
		fmt.Println("failed to load AWS API configuration : %w", err)
		return
	}

	OrganizationsServiceConnection := organizations.NewFromConfig(cfg)
	SecureTokenServiceConnection = sts.NewFromConfig(cfg)
	CurrentAccountId = awsutils.GetCurrentAccountId(SecureTokenServiceConnection)

	if awsutils.IsManagementAccount(OrganizationsServiceConnection, CurrentAccountId) {
		fmt.Println(CurrentAccountId + " IS Organization Management Account")
		fmt.Println("Will use initial credentials: " + InitialCreds.AccessKeyID)
	} else {
		fmt.Println(CurrentAccountId + " NOT Organization Management Account")
		fmt.Println("Need a Role inside the Organization Management Account")
		roleArn := "arn:aws:iam::" + ManagementAccountId + ":role/otc/hts-ccoe-mocb-alt-contact-manager"
		sessionName := *orgPrefix + "-alt-contact-manager"
		fmt.Println("Attempting to switch into Role: " + roleArn)

		AssumedCreds, err := awsutils.AssumeRole(SecureTokenServiceConnection, roleArn, sessionName)
		if err != nil {
			fmt.Println("failed to assume role: %w", err)
			return
		}
		fmt.Println("Assumed role credentials:", *AssumedCreds.AccessKeyId)

		// Transform *ststypes.Credentials returned by AssumeRole function into aws.Credentials
		// required by CreateConnectionConfiguration function
		awsCreds := aws.Credentials{
			AccessKeyID:     *AssumedCreds.AccessKeyId,
			SecretAccessKey: *AssumedCreds.SecretAccessKey,
			SessionToken:    *AssumedCreds.SessionToken,
			Source:          "AssumeRole",
		}

		cfg, err = awsutils.CreateConnectionConfiguration(awsCreds)
		if err != nil {
			fmt.Println("failed to assume role: %w", err)
			return
		}

		OrganizationsServiceConnection = organizations.NewFromConfig(cfg)

		//Check the Contacts to Delete Exist
		accounts, err := awsutils.GetAllAccountsInOrganization(OrganizationsServiceConnection)
		if err != nil {
			fmt.Printf("failed to get the accounts in the organization: %v\n", err)
			return
		}

		fmt.Println("Accounts in the organization:")
		for _, account := range accounts {
			fmt.Println(*account.Name + " - " + *account.Id)
		}
		fmt.Println()

		// Split the input into a slice of strings
		contactTypesList := strings.Split(*contactTypes, ",")

		// Create Account service connection
		AccountServiceConnection := account.NewFromConfig(cfg)

		// Check if the account has the contacts to delete
		for _, acct := range accounts {
			accountId := *acct.Id

			for _, contactTypeStr := range contactTypesList {
				contactTypeStr = strings.TrimSpace(contactTypeStr)
				var contactType accountTypes.AlternateContactType

				switch strings.ToLower(contactTypeStr) {
				case "security":
					contactType = accountTypes.AlternateContactTypeSecurity
				case "billing":
					contactType = accountTypes.AlternateContactTypeBilling
				case "operations":
					contactType = accountTypes.AlternateContactTypeOperations
				default:
					fmt.Printf("Invalid contact type: %s\n", contactTypeStr)
					continue
				}

				contactExists, err := CheckIfContactExists(AccountServiceConnection, accountId, contactType)
				if err != nil {
					fmt.Printf("failed to check if contact exists for account: %v\n", err)
					return
				}

				if contactExists {
					err = DeleteAlternateContact(AccountServiceConnection, accountId, contactType)
					if err != nil {
						fmt.Printf("failed to remove contact from account: %v\n", err)
						return
					}
					fmt.Printf("Removed contact "+contactTypeStr+" from the account: %s\n", accountId)
				} else {
					fmt.Printf("Contact "+contactTypeStr+" does not exist for the account: %s\n", accountId)
				}
			}
		}
	}
}

// CredentialManager interface for dependency injection
type CredentialManager interface {
	GetCustomerConfig(customerCode string) (aws.Config, error)
	GetCustomerInfo(customerCode string) (types.CustomerAccountInfo, error)
}

// UpdateAlternateContacts updates alternate contacts for a customer account
func UpdateAlternateContacts(customerCode string, credentialManager CredentialManager, contactConfig types.AlternateContactConfig) error {
	customerConfig, err := credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config: %w", err)
	}

	accountClient := account.NewFromConfig(customerConfig)

	// Update Security Contact
	if err := updateContact(accountClient, accountTypes.AlternateContactTypeSecurity, contactConfig.SecurityEmail, contactConfig.SecurityName, contactConfig.SecurityTitle, contactConfig.SecurityPhone); err != nil {
		return fmt.Errorf("failed to update security contact: %w", err)
	}

	// Update Billing Contact
	if err := updateContact(accountClient, accountTypes.AlternateContactTypeBilling, contactConfig.BillingEmail, contactConfig.BillingName, contactConfig.BillingTitle, contactConfig.BillingPhone); err != nil {
		return fmt.Errorf("failed to update billing contact: %w", err)
	}

	// Update Operations Contact
	if err := updateContact(accountClient, accountTypes.AlternateContactTypeOperations, contactConfig.OperationsEmail, contactConfig.OperationsName, contactConfig.OperationsTitle, contactConfig.OperationsPhone); err != nil {
		return fmt.Errorf("failed to update operations contact: %w", err)
	}

	fmt.Printf("Successfully updated alternate contacts for customer %s\n", customerCode)
	return nil
}

// updateContact updates a specific alternate contact
func updateContact(client *account.Client, contactType accountTypes.AlternateContactType, email, name, title, phone string) error {
	input := &account.PutAlternateContactInput{
		AlternateContactType: contactType,
		EmailAddress:         &email,
		Name:                 &name,
		Title:                &title,
		PhoneNumber:          &phone,
	}

	_, err := client.PutAlternateContact(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to put alternate contact %s: %w", contactType, err)
	}

	fmt.Printf("Updated %s contact: %s (%s)\n", contactType, name, email)
	return nil
}

// GetAlternateContacts retrieves current alternate contacts for a customer
func GetAlternateContacts(customerCode string, credentialManager CredentialManager) (map[string]interface{}, error) {
	customerConfig, err := credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer config: %w", err)
	}

	accountClient := account.NewFromConfig(customerConfig)
	contacts := make(map[string]interface{})

	// Get Security Contact
	if contact, err := getContact(accountClient, accountTypes.AlternateContactTypeSecurity); err == nil {
		contacts["security"] = contact
	}

	// Get Billing Contact
	if contact, err := getContact(accountClient, accountTypes.AlternateContactTypeBilling); err == nil {
		contacts["billing"] = contact
	}

	// Get Operations Contact
	if contact, err := getContact(accountClient, accountTypes.AlternateContactTypeOperations); err == nil {
		contacts["operations"] = contact
	}

	return contacts, nil
}

// getContact retrieves a specific alternate contact
func getContact(client *account.Client, contactType accountTypes.AlternateContactType) (map[string]string, error) {
	input := &account.GetAlternateContactInput{
		AlternateContactType: contactType,
	}

	result, err := client.GetAlternateContact(context.Background(), input)
	if err != nil {
		return nil, err
	}

	contact := map[string]string{
		"type":  string(contactType),
		"email": *result.AlternateContact.EmailAddress,
		"name":  *result.AlternateContact.Name,
		"title": *result.AlternateContact.Title,
		"phone": *result.AlternateContact.PhoneNumber,
	}

	return contact, nil
}
