package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/account"
	accountTypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationsTypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
)

type Organization struct {
	FriendlyName        string `json:"mocb_org_friendly_name"`
	Prefix              string `json:"mocb_org_prefix"`
	Environment         string `json:"environment"`
	ManagementAccountId string `json:"management_account_id"`
}

type AlternateContactConfig struct {
	SecurityEmail   string `json:"security_email"`
	SecurityName    string `json:"security_name"`
	SecurityTitle   string `json:"security_title"`
	SecurityPhone   string `json:"security_phone"`
	BillingEmail    string `json:"billing_email"`
	BillingName     string `json:"billing_name"`
	BillingTitle    string `json:"billing_title"`
	BillingPhone    string `json:"billing_phone"`
	OperationsEmail string `json:"operations_email"`
	OperationsName  string `json:"operations_name"`
	OperationsTitle string `json:"operations_title"`
	OperationsPhone string `json:"operations_phone"`
}

type SESTopicConfig struct {
	TopicName                 string `json:"TopicName"`
	DisplayName               string `json:"DisplayName"`
	Description               string `json:"Description"`
	DefaultSubscriptionStatus string `json:"DefaultSubscriptionStatus"`
}

type SESConfig struct {
	TopicGroupPrefix  []string         `json:"topic_group_prefix"`
	TopicGroupMembers []SESTopicConfig `json:"topic_group_members"`
	Topics            []SESTopicConfig `json:"topics"`
}

type SESBackup struct {
	ContactList struct {
		Name        string             `json:"name"`
		Description *string            `json:"description"`
		Topics      []sesv2Types.Topic `json:"topics"`
		CreatedAt   string             `json:"created_at"`
		UpdatedAt   string             `json:"updated_at"`
	} `json:"contact_list"`
	Contacts []struct {
		EmailAddress     string                       `json:"email_address"`
		TopicPreferences []sesv2Types.TopicPreference `json:"topic_preferences"`
		UnsubscribeAll   bool                         `json:"unsubscribe_all"`
	} `json:"contacts"`
	BackupMetadata struct {
		Timestamp string `json:"timestamp"`
		Tool      string `json:"tool"`
		Action    string `json:"action"`
	} `json:"backup_metadata"`
}

func CheckForCreds() {
	key, id := os.LookupEnv("AWS_ACCESS_KEY_ID")
	if !id {
		panic("AWS_ACCESS_KEY_ID unset")
	} else {
		fmt.Println("Environment provided AWS_ACCESS_KEY_ID : " + key)
	}

	_, secret := os.LookupEnv("AWS_SECRET_ACCESS_KEY")
	if !secret {
		panic("AWS_SECRET_ACCESS_KEY unset")
	} else {
		fmt.Println("Environment provided AWS_SECRET_ACCESS_KEY : xxxx")
	}

	_, token := os.LookupEnv("AWS_SESSION_TOKEN")
	if !token {
		panic("AWS_SESSION_TOKEN unset")
	} else {
		fmt.Println("Environment provided AWS_SESSION_TOKEN      : xxxx")
	}
}

// CreateConnectionConfiguration creates an AWS configuration using the provided credentials.
func CreateConnectionConfiguration(creds aws.Credentials) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: creds,
		}),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	return cfg, nil
}

func GetManagementAccountIdByPrefix(prefix string, orgConfig []Organization) (string, error) {
	for _, org := range orgConfig {
		if org.Prefix == prefix {
			return org.ManagementAccountId, nil
		}
	}
	return "", fmt.Errorf("management account ID not found for prefix: %s", prefix)
}

// GetConfigPath returns the CONFIG_PATH environment variable or defaults to current directory
func GetConfigPath() string {
	configPath, exists := os.LookupEnv("CONFIG_PATH")
	if !exists || configPath == "" {
		return "./"
	}
	// Ensure the path ends with a slash for proper file concatenation
	if !strings.HasSuffix(configPath, "/") {
		configPath += "/"
	}
	return configPath
}

// Get the current account ID
func GetCurrentAccountId(StsServiceConnection *sts.Client) string {
	stsinput := &sts.GetCallerIdentityInput{}
	stsresult, stserr := StsServiceConnection.GetCallerIdentity(context.Background(), stsinput)
	if stserr != nil {
		fmt.Println("Error getting current Account ID")
		os.Exit(1)
	} else {
		fmt.Println("Account ID: " + *stsresult.Account)
		fmt.Println()
	}
	return *stsresult.Account
}

// Check if the current account is the management account
func IsManagementAccount(OrganizationsServiceConnection *organizations.Client, AccountId string) bool {
	input := &organizations.DescribeOrganizationInput{}
	result, err := OrganizationsServiceConnection.DescribeOrganization(context.Background(), input)
	if err != nil {
		fmt.Println("Error", err)
	}

	if *result.Organization.MasterAccountId == AccountId {
		return true
	} else {
		return false
	}
}

// AssumeRole assumes an AWS IAM role and returns the assumed role's credentials.
func AssumeRole(stsClient *sts.Client, roleArn string, sessionName string) (*ststypes.Credentials, error) {
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleArn),
		RoleSessionName: aws.String(sessionName),
	}

	result, err := stsClient.AssumeRole(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role: %w", err)
	}

	return result.Credentials, nil
}

func GetAllAccountsInOrganization(OrganizationsServiceConnection *organizations.Client) ([]organizationsTypes.Account, error) {
	input := &organizations.ListAccountsInput{}
	result, err := OrganizationsServiceConnection.ListAccounts(context.Background(), input)
	if err != nil {
		fmt.Println("Error", err)
	}

	OrgAccounts := result.Accounts

	return OrgAccounts, nil
}

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

func SetContactsForOrganization(contactConfig *AlternateContactConfig, orgPrefix *string, overwrite *bool) {
	ConfigPath := GetConfigPath()
	fmt.Println("Working in Config Path: " + ConfigPath)

	//Read the Org Json File
	OrgJson, err := os.ReadFile(ConfigPath + "OrgConfig.json")
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + "OrgConfig.json")

	var OrgConfig []Organization
	json.NewDecoder(bytes.NewReader(OrgJson)).Decode(&OrgConfig)

	ManagementAccountId, err := GetManagementAccountIdByPrefix(*orgPrefix, OrgConfig)
	if err != nil {
		fmt.Printf("failed to get management account ID: %v\n", err)
		return
	}
	fmt.Printf("Management Account ID for prefix %s: %s\n", *orgPrefix, ManagementAccountId)

	var SecureTokenServiceConnection *sts.Client
	var CurrentAccountId string

	// Load the default AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
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

	cfg, err = CreateConnectionConfiguration(InitialCreds)
	if err != nil {
		fmt.Println("failed to load AWS API configuration : %w", err)
		return
	}

	SecureTokenServiceConnection = sts.NewFromConfig(cfg)
	OrganizationsServiceConnection := organizations.NewFromConfig(cfg)
	CurrentAccountId = GetCurrentAccountId(SecureTokenServiceConnection)

	var finalCfg aws.Config

	if IsManagementAccount(OrganizationsServiceConnection, CurrentAccountId) {
		fmt.Println(CurrentAccountId + " IS Organization Management Account")
		fmt.Println("Will use initial credentials: " + InitialCreds.AccessKeyID)
		finalCfg = cfg
	} else {
		fmt.Println(CurrentAccountId + " NOT Organization Management Account")
		fmt.Println("Need a Role inside the Organization Management Account")
		roleArn := "arn:aws:iam::" + ManagementAccountId + ":role/otc/hts-ccoe-mocb-alt-contact-manager"
		sessionName := *orgPrefix + "-alt-contact-manager"
		fmt.Println("Attempting to switch into Role: " + roleArn)

		AssumedCreds, err := AssumeRole(SecureTokenServiceConnection, roleArn, sessionName)
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

		finalCfg, err = CreateConnectionConfiguration(awsCreds)
		if err != nil {
			fmt.Println("failed to assume role: %w", err)
			return
		}
	}

	OrganizationsServiceConnection = organizations.NewFromConfig(finalCfg)

	//Check that we can list accounts
	accounts, err := GetAllAccountsInOrganization(OrganizationsServiceConnection)
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
		if contactConfig.SecurityEmail != "" {
			err = SetAlternateContactIfNotExists(AccountServiceConnection, accountId, accountTypes.AlternateContactTypeSecurity,
				contactConfig.SecurityName, contactConfig.SecurityTitle, contactConfig.SecurityEmail, contactConfig.SecurityPhone, *overwrite)
			if err != nil {
				fmt.Printf("failed to set security contact for account %s: %v\n", accountId, err)
			}
		}

		// Set Billing Contact
		if contactConfig.BillingEmail != "" {
			err = SetAlternateContactIfNotExists(AccountServiceConnection, accountId, accountTypes.AlternateContactTypeBilling,
				contactConfig.BillingName, contactConfig.BillingTitle, contactConfig.BillingEmail, contactConfig.BillingPhone, *overwrite)
			if err != nil {
				fmt.Printf("failed to set billing contact for account %s: %v\n", accountId, err)
			}
		}

		// Set Operations Contact
		if contactConfig.OperationsEmail != "" {
			err = SetAlternateContactIfNotExists(AccountServiceConnection, accountId, accountTypes.AlternateContactTypeOperations,
				contactConfig.OperationsName, contactConfig.OperationsTitle, contactConfig.OperationsEmail, contactConfig.OperationsPhone, *overwrite)
			if err != nil {
				fmt.Printf("failed to set operations contact for account %s: %v\n", accountId, err)
			}
		}

		fmt.Println()
	}
}

func SetContactsForSingleOrganization(contactConfigFile *string, orgPrefix *string, overwrite *bool) {
	ConfigPath := GetConfigPath()
	fmt.Println("Working in Config Path: " + ConfigPath)

	//Read the Contact Config Json File
	ContactJson, err := os.ReadFile(ConfigPath + *contactConfigFile)
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + *contactConfigFile)

	var ContactConfig AlternateContactConfig
	json.NewDecoder(bytes.NewReader(ContactJson)).Decode(&ContactConfig)

	//Read the Org Json File
	OrgJson, err := os.ReadFile(ConfigPath + "OrgConfig.json")
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + "OrgConfig.json")

	var OrgConfig []Organization
	json.NewDecoder(bytes.NewReader(OrgJson)).Decode(&OrgConfig)

	ManagementAccountId, err := GetManagementAccountIdByPrefix(*orgPrefix, OrgConfig)
	if err != nil {
		fmt.Printf("failed to get management account ID: %v\n", err)
		return
	}
	fmt.Printf("Management Account ID for prefix %s: %s\n", *orgPrefix, ManagementAccountId)

	var SecureTokenServiceConnection *sts.Client
	var CurrentAccountId string

	// Load the default AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
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

	cfg, err = CreateConnectionConfiguration(InitialCreds)
	if err != nil {
		fmt.Println("failed to load AWS API configuration : %w", err)
		return
	}

	SecureTokenServiceConnection = sts.NewFromConfig(cfg)
	OrganizationsServiceConnection := organizations.NewFromConfig(cfg)
	CurrentAccountId = GetCurrentAccountId(SecureTokenServiceConnection)

	fmt.Println("Processing Organization: " + *orgPrefix)
	fmt.Println()

	var finalCfg aws.Config

	if IsManagementAccount(OrganizationsServiceConnection, CurrentAccountId) {
		fmt.Println(CurrentAccountId + " IS Organization Management Account")
		fmt.Println("Will use initial credentials: " + InitialCreds.AccessKeyID)
		finalCfg = cfg
	} else {
		fmt.Println(CurrentAccountId + " NOT Organization Management Account")
		fmt.Println("Need a Role inside the Organization Management Account")
		ManagementRoleArn := "arn:aws:iam::" + ManagementAccountId + ":role/otc/hts-ccoe-mocb-alt-contact-manager"
		ManagementSessionName := *orgPrefix + "-alt-contact-manager"
		fmt.Println("Attempting to switch into Role: " + ManagementRoleArn)

		ManagementAssumedCreds, err := AssumeRole(SecureTokenServiceConnection, ManagementRoleArn, ManagementSessionName)
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

		ManagementCfg, err := CreateConnectionConfiguration(ManagementAwsCreds)
		if err != nil {
			fmt.Printf("failed to load AWS API configuration with assumed role in Management account: %v\n", err)
			return
		}
		finalCfg = ManagementCfg
	}

	OrganizationsServiceConnection = organizations.NewFromConfig(finalCfg)

	accounts, err := GetAllAccountsInOrganization(OrganizationsServiceConnection)
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
	cfg, err = CreateConnectionConfiguration(InitialCreds)
	if err != nil {
		fmt.Printf("failed to reload AWS API configuration: %v\n", err)
		return
	}

	SecureTokenServiceConnection = sts.NewFromConfig(cfg)
	CurrentAccountId = GetCurrentAccountId(SecureTokenServiceConnection)
	fmt.Println("Switched back to initial credentials in " + CurrentAccountId)
}

func DeleteContactsFromOrganization(orgPrefix *string, contactTypes *string) {
	ConfigPath := GetConfigPath()
	fmt.Println("Working in Config Path: " + ConfigPath)

	//Read the Org Json File
	OrgJson, err := os.ReadFile(ConfigPath + "OrgConfig.json")
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + "OrgConfig.json")

	var OrgConfig []Organization
	json.NewDecoder(bytes.NewReader(OrgJson)).Decode(&OrgConfig)

	ManagementAccountId, err := GetManagementAccountIdByPrefix(*orgPrefix, OrgConfig)
	if err != nil {
		fmt.Printf("failed to get management account ID: %v\n", err)
		return
	}
	fmt.Printf("Management Account ID for prefix %s: %s\n", *orgPrefix, ManagementAccountId)

	var SecureTokenServiceConnection *sts.Client
	var CurrentAccountId string

	// Load the default AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
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

	cfg, err = CreateConnectionConfiguration(InitialCreds)
	if err != nil {
		fmt.Println("failed to load AWS API configuration : %w", err)
		return
	}

	OrganizationsServiceConnection := organizations.NewFromConfig(cfg)
	SecureTokenServiceConnection = sts.NewFromConfig(cfg)
	CurrentAccountId = GetCurrentAccountId(SecureTokenServiceConnection)

	if IsManagementAccount(OrganizationsServiceConnection, CurrentAccountId) {
		fmt.Println(CurrentAccountId + " IS Organization Management Account")
		fmt.Println("Will use initial credentials: " + InitialCreds.AccessKeyID)
	} else {
		fmt.Println(CurrentAccountId + " NOT Organization Management Account")
		fmt.Println("Need a Role inside the Organization Management Account")
		roleArn := "arn:aws:iam::" + ManagementAccountId + ":role/otc/hts-ccoe-mocb-alt-contact-manager"
		sessionName := *orgPrefix + "-alt-contact-manager"
		fmt.Println("Attempting to switch into Role: " + roleArn)

		AssumedCreds, err := AssumeRole(SecureTokenServiceConnection, roleArn, sessionName)
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

		cfg, err = CreateConnectionConfiguration(awsCreds)
		if err != nil {
			fmt.Println("failed to assume role: %w", err)
			return
		}

		OrganizationsServiceConnection = organizations.NewFromConfig(cfg)

		//Check the Contacts to Delete Exist
		accounts, err := GetAllAccountsInOrganization(OrganizationsServiceConnection)
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

// SetContactsForAllOrganizations sets contacts for all organizations in the config file
func SetContactsForAllOrganizations(contactConfigFile *string, overwrite *bool) {
	ConfigPath := GetConfigPath()
	fmt.Println("Working in Config Path: " + ConfigPath)

	//Read the Contact Config Json File
	ContactJson, err := os.ReadFile(ConfigPath + *contactConfigFile)
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + *contactConfigFile)

	var ContactConfig AlternateContactConfig
	json.NewDecoder(bytes.NewReader(ContactJson)).Decode(&ContactConfig)

	//Read the Org Json File
	OrgJson, err := os.ReadFile(ConfigPath + "OrgConfig.json")
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully opened " + ConfigPath + "OrgConfig.json")

	var OrgConfig []Organization
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

// SES Management Functions

// CreateContactList creates a new contact list in SES
func CreateContactList(sesClient *sesv2.Client, listName string, description string, topicConfigs []SESTopicConfig) error {
	var topicPreferences []sesv2Types.Topic
	for _, topicConfig := range topicConfigs {
		var defaultStatus sesv2Types.SubscriptionStatus
		if topicConfig.DefaultSubscriptionStatus == "OPT_IN" {
			defaultStatus = sesv2Types.SubscriptionStatusOptIn
		} else {
			defaultStatus = sesv2Types.SubscriptionStatusOptOut
		}

		topicPreferences = append(topicPreferences, sesv2Types.Topic{
			TopicName:                 aws.String(topicConfig.TopicName),
			DisplayName:               aws.String(topicConfig.DisplayName),
			Description:               aws.String(topicConfig.Description),
			DefaultSubscriptionStatus: defaultStatus,
		})
	}

	input := &sesv2.CreateContactListInput{
		ContactListName: aws.String(listName),
		Description:     aws.String(description),
		Topics:          topicPreferences,
	}

	_, err := sesClient.CreateContactList(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to create contact list: %w", err)
	}

	fmt.Printf("Successfully created contact list: %s\n", listName)
	return nil
}

// AddContactToList adds an email contact to a contact list
func AddContactToList(sesClient *sesv2.Client, listName string, email string, topics []string) error {
	var topicPreferences []sesv2Types.TopicPreference
	for _, topic := range topics {
		topicPreferences = append(topicPreferences, sesv2Types.TopicPreference{
			TopicName:          aws.String(topic),
			SubscriptionStatus: sesv2Types.SubscriptionStatusOptIn,
		})
	}

	input := &sesv2.CreateContactInput{
		ContactListName:  aws.String(listName),
		EmailAddress:     aws.String(email),
		TopicPreferences: topicPreferences,
	}

	_, err := sesClient.CreateContact(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to add contact %s to list %s: %w", email, listName, err)
	}

	fmt.Printf("Successfully added contact %s to list %s\n", email, listName)
	return nil
}

// RemoveContactFromList removes an email contact from a contact list
func RemoveContactFromList(sesClient *sesv2.Client, listName string, email string) error {
	input := &sesv2.DeleteContactInput{
		ContactListName: aws.String(listName),
		EmailAddress:    aws.String(email),
	}

	_, err := sesClient.DeleteContact(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to remove contact %s from list %s: %w", email, listName, err)
	}

	fmt.Printf("Successfully removed contact %s from list %s\n", email, listName)
	return nil
}

// CreateContactListBackup creates a backup of a contact list with all contacts and topics
func CreateContactListBackup(sesClient *sesv2.Client, listName string, action string) (string, error) {
	// Get contact list details
	listInput := &sesv2.GetContactListInput{
		ContactListName: aws.String(listName),
	}

	listResult, err := sesClient.GetContactList(context.Background(), listInput)
	if err != nil {
		return "", fmt.Errorf("failed to get contact list details: %w", err)
	}

	// Get all contacts
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(listName),
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return "", fmt.Errorf("failed to list contacts for backup: %w", err)
	}

	// Create backup structure
	backup := SESBackup{}

	// Fill contact list info
	backup.ContactList.Name = listName
	backup.ContactList.Description = listResult.Description
	backup.ContactList.Topics = listResult.Topics
	if listResult.CreatedTimestamp != nil {
		backup.ContactList.CreatedAt = listResult.CreatedTimestamp.Format("2006-01-02T15:04:05Z")
	}
	if listResult.LastUpdatedTimestamp != nil {
		backup.ContactList.UpdatedAt = listResult.LastUpdatedTimestamp.Format("2006-01-02T15:04:05Z")
	}

	// Fill contacts info
	for _, contact := range contactsResult.Contacts {
		contactBackup := struct {
			EmailAddress     string                       `json:"email_address"`
			TopicPreferences []sesv2Types.TopicPreference `json:"topic_preferences"`
			UnsubscribeAll   bool                         `json:"unsubscribe_all"`
		}{
			EmailAddress:     *contact.EmailAddress,
			TopicPreferences: contact.TopicPreferences,
			UnsubscribeAll:   contact.UnsubscribeAll,
		}

		backup.Contacts = append(backup.Contacts, contactBackup)
	}

	// Fill backup metadata
	backup.BackupMetadata.Timestamp = time.Now().Format("2006-01-02T15:04:05Z")
	backup.BackupMetadata.Tool = "aws-alternate-contact-manager"
	backup.BackupMetadata.Action = action

	// Save backup to file
	backupFilename := fmt.Sprintf("ses-backup-%s-%s.json",
		listName,
		time.Now().Format("20060102-150405"))

	backupPath := GetConfigPath() + backupFilename

	backupJson, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal backup data: %w", err)
	}

	err = os.WriteFile(backupPath, backupJson, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write backup file: %w", err)
	}

	fmt.Printf("✅ Backup saved to: %s\n", backupFilename)
	fmt.Printf("📊 Backed up %d contacts and %d topics\n", len(backup.Contacts), len(backup.ContactList.Topics))

	return backupFilename, nil
}

// RemoveAllContactsFromList removes all contacts from a contact list after creating a backup
func RemoveAllContactsFromList(sesClient *sesv2.Client, listName string) error {
	fmt.Printf("🔍 Checking contacts in list %s...\n", listName)

	// First, get all contacts in the list to check if there are any
	input := &sesv2.ListContactsInput{
		ContactListName: aws.String(listName),
	}

	result, err := sesClient.ListContacts(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to list contacts in %s: %w", listName, err)
	}

	if len(result.Contacts) == 0 {
		fmt.Printf("No contacts found in list %s - nothing to remove\n", listName)
		return nil
	}

	fmt.Printf("Found %d contacts in list %s\n", len(result.Contacts), listName)

	// Create backup before removing contacts
	fmt.Printf("📦 Creating backup before removing contacts...\n")
	backupFilename, err := CreateContactListBackup(sesClient, listName, "remove-contact-all")
	if err != nil {
		return fmt.Errorf("failed to create backup before removing contacts: %w", err)
	}

	fmt.Printf("🗑️  Proceeding to remove all %d contacts...\n", len(result.Contacts))

	// Remove each contact
	successCount := 0
	errorCount := 0
	for i, contact := range result.Contacts {
		fmt.Printf("Removing contact %d/%d: %s\n", i+1, len(result.Contacts), *contact.EmailAddress)
		err := RemoveContactFromList(sesClient, listName, *contact.EmailAddress)
		if err != nil {
			fmt.Printf("❌ Error removing contact %s: %v\n", *contact.EmailAddress, err)
			errorCount++
		} else {
			successCount++
		}
	}

	fmt.Printf("\n✅ Removal complete: %d successful, %d errors\n", successCount, errorCount)
	fmt.Printf("📁 Backup available at: %s\n", backupFilename)

	if errorCount > 0 {
		return fmt.Errorf("failed to remove %d contacts from list %s (backup saved: %s)", errorCount, listName, backupFilename)
	}

	return nil
}

// AddToSuppressionList adds an email to the account-level suppression list
func AddToSuppressionList(sesClient *sesv2.Client, email string, reason sesv2Types.SuppressionListReason) error {
	input := &sesv2.PutSuppressedDestinationInput{
		EmailAddress: aws.String(email),
		Reason:       reason,
	}

	_, err := sesClient.PutSuppressedDestination(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to add %s to suppression list: %w", email, err)
	}

	fmt.Printf("Successfully added %s to suppression list with reason: %s\n", email, reason)
	return nil
}

// RemoveFromSuppressionList removes an email from the account-level suppression list
func RemoveFromSuppressionList(sesClient *sesv2.Client, email string) error {
	input := &sesv2.DeleteSuppressedDestinationInput{
		EmailAddress: aws.String(email),
	}

	_, err := sesClient.DeleteSuppressedDestination(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to remove %s from suppression list: %w", email, err)
	}

	fmt.Printf("Successfully removed %s from suppression list\n", email)
	return nil
}

// GetAccountContactList gets the first/main contact list for the account
func GetAccountContactList(sesClient *sesv2.Client) (string, error) {
	input := &sesv2.ListContactListsInput{}

	result, err := sesClient.ListContactLists(context.Background(), input)
	if err != nil {
		return "", fmt.Errorf("failed to list contact lists: %w", err)
	}

	if len(result.ContactLists) == 0 {
		return "", fmt.Errorf("no contact lists found in this account")
	}

	// Return the first contact list (typically the main one)
	return *result.ContactLists[0].ContactListName, nil
}

// DescribeContactList provides detailed information about a contact list
func DescribeContactList(sesClient *sesv2.Client, listName string) error {
	// Get contact list details
	listInput := &sesv2.GetContactListInput{
		ContactListName: aws.String(listName),
	}

	listResult, err := sesClient.GetContactList(context.Background(), listInput)
	if err != nil {
		return fmt.Errorf("failed to get contact list details for %s: %w", listName, err)
	}

	fmt.Printf("=== Contact List Details ===\n")
	fmt.Printf("Name: %s\n", *listResult.ContactListName)
	if listResult.Description != nil {
		fmt.Printf("Description: %s\n", *listResult.Description)
	}
	fmt.Printf("Created: %s\n", listResult.CreatedTimestamp.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("Last Modified: %s\n", listResult.LastUpdatedTimestamp.Format("2006-01-02 15:04:05 UTC"))

	// Display topics
	if len(listResult.Topics) > 0 {
		fmt.Printf("\nTopics:\n")
		for _, topic := range listResult.Topics {
			fmt.Printf("  - %s", *topic.TopicName)
			if topic.DisplayName != nil && *topic.DisplayName != *topic.TopicName {
				fmt.Printf(" (%s)", *topic.DisplayName)
			}
			fmt.Printf(" - Default: %s\n", topic.DefaultSubscriptionStatus)
		}
	} else {
		fmt.Printf("\nTopics: None\n")
	}

	// Get contact count
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(listName),
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		fmt.Printf("\nContacts: Unable to retrieve count (%v)\n", err)
	} else {
		fmt.Printf("\nTotal Contacts: %d\n", len(contactsResult.Contacts))

		if len(contactsResult.Contacts) > 0 {
			// Calculate subscription statistics
			topicStats := make(map[string]map[string]int)
			unsubscribedCount := 0

			for _, contact := range contactsResult.Contacts {
				if contact.UnsubscribeAll {
					unsubscribedCount++
				}

				for _, topicPref := range contact.TopicPreferences {
					topicName := *topicPref.TopicName
					if topicStats[topicName] == nil {
						topicStats[topicName] = make(map[string]int)
					}

					if topicPref.SubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
						topicStats[topicName]["OPT_IN"]++
					} else {
						topicStats[topicName]["OPT_OUT"]++
					}
				}
			}

			// Show subscription statistics
			if len(topicStats) > 0 {
				fmt.Printf("\nSubscription Statistics:\n")
				for topicName, stats := range topicStats {
					optIn := stats["OPT_IN"]
					optOut := stats["OPT_OUT"]
					total := optIn + optOut
					fmt.Printf("  %s: %d opted in, %d opted out (of %d contacts)\n",
						topicName, optIn, optOut, total)
				}
			}

			if unsubscribedCount > 0 {
				fmt.Printf("\nUnsubscribed from all: %d contacts\n", unsubscribedCount)
			}

			fmt.Printf("\nRecent Contacts (up to 5):\n")
			limit := len(contactsResult.Contacts)
			if limit > 5 {
				limit = 5
			}
			for i := 0; i < limit; i++ {
				contact := contactsResult.Contacts[i]
				fmt.Printf("  - %s", *contact.EmailAddress)
				if contact.LastUpdatedTimestamp != nil {
					fmt.Printf(" (updated: %s)", contact.LastUpdatedTimestamp.Format("2006-01-02"))
				}
				fmt.Printf("\n")
			}
			if len(contactsResult.Contacts) > 5 {
				fmt.Printf("  ... and %d more contacts (use 'list-contacts' to see all)\n", len(contactsResult.Contacts)-5)
			}
		}
	}

	return nil
}

// ListContactsInList lists all contacts in a specific contact list with topic subscriptions
func ListContactsInList(sesClient *sesv2.Client, listName string) error {
	input := &sesv2.ListContactsInput{
		ContactListName: aws.String(listName),
	}

	result, err := sesClient.ListContacts(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to list contacts in %s: %w", listName, err)
	}

	if len(result.Contacts) == 0 {
		fmt.Printf("No contacts found in list '%s'\n", listName)
		return nil
	}

	fmt.Printf("Contacts in list '%s' (%d total):\n\n", listName, len(result.Contacts))

	for i, contact := range result.Contacts {
		fmt.Printf("%d. %s\n", i+1, *contact.EmailAddress)

		if contact.LastUpdatedTimestamp != nil {
			fmt.Printf("   Last Updated: %s\n", contact.LastUpdatedTimestamp.Format("2006-01-02 15:04:05 UTC"))
		}

		// Show topic preferences
		if len(contact.TopicPreferences) > 0 {
			fmt.Printf("   Topic Subscriptions:\n")
			for _, topic := range contact.TopicPreferences {
				status := "OPT_OUT"
				if topic.SubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
					status = "OPT_IN"
				}
				fmt.Printf("     - %s: %s\n", *topic.TopicName, status)
			}
		} else {
			fmt.Printf("   Topic Subscriptions: None (using list defaults)\n")
		}

		// Show unsubscribe status if available
		if contact.UnsubscribeAll {
			fmt.Printf("   Status: UNSUBSCRIBED FROM ALL\n")
		}

		fmt.Printf("\n")
	}

	return nil
}

// DescribeTopic provides detailed information about a specific topic in the account's contact list
func DescribeTopic(sesClient *sesv2.Client, topicName string) error {
	// First get the account's main contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("error finding account contact list: %w", err)
	}

	// Get contact list details to access topics
	listInput := &sesv2.GetContactListInput{
		ContactListName: aws.String(accountListName),
	}

	listResult, err := sesClient.GetContactList(context.Background(), listInput)
	if err != nil {
		return fmt.Errorf("failed to get contact list details: %w", err)
	}

	// Find the specific topic
	var targetTopic *sesv2Types.Topic
	for _, topic := range listResult.Topics {
		if *topic.TopicName == topicName {
			targetTopic = &topic
			break
		}
	}

	if targetTopic == nil {
		return fmt.Errorf("topic '%s' not found in contact list '%s'", topicName, accountListName)
	}

	// Get all contacts to calculate subscription statistics for this topic
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return fmt.Errorf("failed to list contacts: %w", err)
	}

	// Calculate statistics for this topic
	optInCount := 0
	optOutCount := 0
	subscribedContacts := []string{}
	unsubscribedContacts := []string{}

	for _, contact := range contactsResult.Contacts {
		found := false
		for _, topicPref := range contact.TopicPreferences {
			if *topicPref.TopicName == topicName {
				found = true
				if topicPref.SubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
					optInCount++
					subscribedContacts = append(subscribedContacts, *contact.EmailAddress)
				} else {
					optOutCount++
					unsubscribedContacts = append(unsubscribedContacts, *contact.EmailAddress)
				}
				break
			}
		}
		if !found {
			// Contact doesn't have explicit preference, uses default
			if targetTopic.DefaultSubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
				optInCount++
				subscribedContacts = append(subscribedContacts, *contact.EmailAddress+" (default)")
			} else {
				optOutCount++
				unsubscribedContacts = append(unsubscribedContacts, *contact.EmailAddress+" (default)")
			}
		}
	}

	// Display topic information
	fmt.Printf("=== Topic Details: %s ===\n", topicName)
	if targetTopic.DisplayName != nil && *targetTopic.DisplayName != topicName {
		fmt.Printf("Display Name: %s\n", *targetTopic.DisplayName)
	}
	fmt.Printf("Default Subscription: %s\n", targetTopic.DefaultSubscriptionStatus)
	fmt.Printf("Contact List: %s\n", accountListName)

	fmt.Printf("\nSubscription Statistics:\n")
	fmt.Printf("  Opted In: %d contacts\n", optInCount)
	fmt.Printf("  Opted Out: %d contacts\n", optOutCount)
	fmt.Printf("  Total: %d contacts\n", optInCount+optOutCount)

	if len(subscribedContacts) > 0 {
		fmt.Printf("\nSubscribed Contacts:\n")
		for _, email := range subscribedContacts {
			fmt.Printf("  - %s\n", email)
		}
	}

	if len(unsubscribedContacts) > 0 {
		fmt.Printf("\nUnsubscribed Contacts:\n")
		for _, email := range unsubscribedContacts {
			fmt.Printf("  - %s\n", email)
		}
	}

	return nil
}

// DescribeContact provides detailed information about a specific contact
func DescribeContact(sesClient *sesv2.Client, email string) error {
	// First get the account's main contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("error finding account contact list: %w", err)
	}

	// Get contact details
	contactInput := &sesv2.GetContactInput{
		ContactListName: aws.String(accountListName),
		EmailAddress:    aws.String(email),
	}

	contactResult, err := sesClient.GetContact(context.Background(), contactInput)
	if err != nil {
		return fmt.Errorf("failed to get contact details for %s: %w", email, err)
	}

	// Get contact list details to access available topics
	listInput := &sesv2.GetContactListInput{
		ContactListName: aws.String(accountListName),
	}

	listResult, err := sesClient.GetContactList(context.Background(), listInput)
	if err != nil {
		return fmt.Errorf("failed to get contact list details: %w", err)
	}

	// Display contact information
	fmt.Printf("=== Contact Details: %s ===\n", email)
	fmt.Printf("Contact List: %s\n", accountListName)

	if contactResult.CreatedTimestamp != nil {
		fmt.Printf("Added: %s\n", contactResult.CreatedTimestamp.Format("2006-01-02 15:04:05 UTC"))
	}
	if contactResult.LastUpdatedTimestamp != nil {
		fmt.Printf("Last Updated: %s\n", contactResult.LastUpdatedTimestamp.Format("2006-01-02 15:04:05 UTC"))
	}

	// Show unsubscribe status
	if contactResult.UnsubscribeAll {
		fmt.Printf("Status: UNSUBSCRIBED FROM ALL TOPICS\n")
	} else {
		fmt.Printf("Status: Active\n")
	}

	// Create a map of contact's topic preferences
	contactPrefs := make(map[string]sesv2Types.SubscriptionStatus)
	for _, pref := range contactResult.TopicPreferences {
		contactPrefs[*pref.TopicName] = pref.SubscriptionStatus
	}

	// Display topic subscriptions
	fmt.Printf("\nTopic Subscriptions:\n")
	if len(listResult.Topics) == 0 {
		fmt.Printf("  No topics available in this contact list\n")
	} else {
		for _, topic := range listResult.Topics {
			topicName := *topic.TopicName

			// Check if contact has explicit preference
			if status, hasExplicit := contactPrefs[topicName]; hasExplicit {
				statusStr := "OPT_OUT"
				if status == sesv2Types.SubscriptionStatusOptIn {
					statusStr = "OPT_IN"
				}
				fmt.Printf("  - %s: %s (explicit)\n", topicName, statusStr)
			} else {
				// Use default from topic
				defaultStr := "OPT_OUT"
				if topic.DefaultSubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
					defaultStr = "OPT_IN"
				}
				fmt.Printf("  - %s: %s (default)\n", topicName, defaultStr)
			}

			// Show display name if different
			if topic.DisplayName != nil && *topic.DisplayName != topicName {
				fmt.Printf("    Display Name: %s\n", *topic.DisplayName)
			}
		}
	}

	// Show summary statistics
	explicitOptIns := 0
	explicitOptOuts := 0
	defaultSubscriptions := 0

	for _, topic := range listResult.Topics {
		topicName := *topic.TopicName
		if _, hasExplicit := contactPrefs[topicName]; hasExplicit {
			if contactPrefs[topicName] == sesv2Types.SubscriptionStatusOptIn {
				explicitOptIns++
			} else {
				explicitOptOuts++
			}
		} else {
			defaultSubscriptions++
		}
	}

	fmt.Printf("\nSubscription Summary:\n")
	fmt.Printf("  Explicit Opt-ins: %d\n", explicitOptIns)
	fmt.Printf("  Explicit Opt-outs: %d\n", explicitOptOuts)
	fmt.Printf("  Using Defaults: %d\n", defaultSubscriptions)
	fmt.Printf("  Total Topics: %d\n", len(listResult.Topics))

	return nil
}

// DescribeAllTopics provides detailed information about all topics in the account's contact list
func DescribeAllTopics(sesClient *sesv2.Client) error {
	// First get the account's main contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("error finding account contact list: %w", err)
	}

	// Get contact list details to access topics
	listInput := &sesv2.GetContactListInput{
		ContactListName: aws.String(accountListName),
	}

	listResult, err := sesClient.GetContactList(context.Background(), listInput)
	if err != nil {
		return fmt.Errorf("failed to get contact list details: %w", err)
	}

	if len(listResult.Topics) == 0 {
		fmt.Printf("No topics found in contact list '%s'\n", accountListName)
		return nil
	}

	// Get all contacts to calculate subscription statistics
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return fmt.Errorf("failed to list contacts: %w", err)
	}

	fmt.Printf("=== All Topics in Contact List: %s ===\n\n", accountListName)

	for i, topic := range listResult.Topics {
		topicName := *topic.TopicName

		// Calculate statistics for this topic
		optInCount := 0
		optOutCount := 0

		for _, contact := range contactsResult.Contacts {
			found := false
			for _, topicPref := range contact.TopicPreferences {
				if *topicPref.TopicName == topicName {
					found = true
					if topicPref.SubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
						optInCount++
					} else {
						optOutCount++
					}
					break
				}
			}
			if !found {
				// Contact doesn't have explicit preference, uses default
				if topic.DefaultSubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
					optInCount++
				} else {
					optOutCount++
				}
			}
		}

		fmt.Printf("%d. %s\n", i+1, topicName)
		if topic.DisplayName != nil && *topic.DisplayName != topicName {
			fmt.Printf("   Display Name: %s\n", *topic.DisplayName)
		}
		fmt.Printf("   Default Subscription: %s\n", topic.DefaultSubscriptionStatus)
		fmt.Printf("   Subscriptions: %d opted in, %d opted out (%d total)\n",
			optInCount, optOutCount, optInCount+optOutCount)
		fmt.Printf("\n")
	}

	return nil
}

// ManageTopics manages topics in the account's contact list based on configuration
func ManageTopics(sesClient *sesv2.Client, configTopics []SESTopicConfig, dryRun bool) error {
	// Get the account's main contact list, or create one if none exists
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		// Check if the error is because no contact lists exist
		if strings.Contains(err.Error(), "no contact lists found") {
			fmt.Printf("📝 No contact list found in account. Creating new contact list...\n")

			if dryRun {
				fmt.Printf("DRY RUN: Would create new contact list 'main-contact-list' with %d topics\n", len(configTopics))
				return nil
			}

			// Create a new contact list with the configured topics
			listName := "main-contact-list"
			err = CreateContactList(sesClient, listName, "Managed contact list", configTopics)
			if err != nil {
				return fmt.Errorf("failed to create new contact list: %w", err)
			}

			fmt.Printf("✅ Created new contact list: %s with %d topics\n", listName, len(configTopics))
			fmt.Printf("🎉 Topic management completed successfully!\n")
			fmt.Printf("   - Created new list: %s\n", listName)
			fmt.Printf("   - Added %d topics\n", len(configTopics))
			return nil
		}

		return fmt.Errorf("error finding account contact list: %w", err)
	}

	// Get current contact list details
	listInput := &sesv2.GetContactListInput{
		ContactListName: aws.String(accountListName),
	}

	listResult, err := sesClient.GetContactList(context.Background(), listInput)
	if err != nil {
		return fmt.Errorf("failed to get contact list details: %w", err)
	}

	fmt.Printf("=== Topic Management for Contact List: %s ===\n", accountListName)
	if dryRun {
		fmt.Printf("DRY RUN MODE - No changes will be made\n")
	}
	fmt.Printf("\n")

	// Create maps for easier comparison
	currentTopics := make(map[string]sesv2Types.Topic)
	for _, topic := range listResult.Topics {
		currentTopics[*topic.TopicName] = topic
	}

	configTopicsMap := make(map[string]SESTopicConfig)
	for _, topic := range configTopics {
		configTopicsMap[topic.TopicName] = topic
	}

	// Track changes needed
	var topicsToAdd []SESTopicConfig
	var topicsToUpdate []SESTopicConfig
	var topicsToRemove []string

	// Check for topics to add or update
	for _, configTopic := range configTopics {
		if currentTopic, exists := currentTopics[configTopic.TopicName]; exists {
			// Topic exists, check if it needs updating
			needsUpdate := false

			currentDisplayName := ""
			if currentTopic.DisplayName != nil {
				currentDisplayName = *currentTopic.DisplayName
			}

			currentDescription := ""
			if currentTopic.Description != nil {
				currentDescription = *currentTopic.Description
			}

			currentDefaultStatus := string(currentTopic.DefaultSubscriptionStatus)

			if currentDisplayName != configTopic.DisplayName ||
				currentDescription != configTopic.Description ||
				currentDefaultStatus != configTopic.DefaultSubscriptionStatus {
				needsUpdate = true
			}

			if needsUpdate {
				topicsToUpdate = append(topicsToUpdate, configTopic)
			}
		} else {
			// Topic doesn't exist, needs to be added
			topicsToAdd = append(topicsToAdd, configTopic)
		}
	}

	// Check for topics to remove (exist in current but not in config)
	for topicName := range currentTopics {
		if _, exists := configTopicsMap[topicName]; !exists {
			topicsToRemove = append(topicsToRemove, topicName)
		}
	}

	// Display planned changes
	if len(topicsToAdd) == 0 && len(topicsToUpdate) == 0 && len(topicsToRemove) == 0 {
		fmt.Printf("✅ All topics are already in sync with configuration\n")
		return nil
	}

	fmt.Printf("Changes needed:\n\n")

	// Show topics to add
	if len(topicsToAdd) > 0 {
		fmt.Printf("📝 Topics to ADD:\n")
		for _, topic := range topicsToAdd {
			fmt.Printf("  + %s (%s)\n", topic.TopicName, topic.DisplayName)
			fmt.Printf("    Description: %s\n", topic.Description)
			fmt.Printf("    Default: %s\n", topic.DefaultSubscriptionStatus)
			fmt.Printf("\n")
		}
	}

	// Show topics to update
	if len(topicsToUpdate) > 0 {
		fmt.Printf("🔄 Topics to UPDATE:\n")
		for _, topic := range topicsToUpdate {
			currentTopic := currentTopics[topic.TopicName]
			fmt.Printf("  ~ %s\n", topic.TopicName)

			if currentTopic.DisplayName == nil || *currentTopic.DisplayName != topic.DisplayName {
				fmt.Printf("    Display Name: %s → %s\n",
					aws.ToString(currentTopic.DisplayName), topic.DisplayName)
			}

			if currentTopic.Description == nil || *currentTopic.Description != topic.Description {
				fmt.Printf("    Description: %s → %s\n",
					aws.ToString(currentTopic.Description), topic.Description)
			}

			if string(currentTopic.DefaultSubscriptionStatus) != topic.DefaultSubscriptionStatus {
				fmt.Printf("    Default: %s → %s\n",
					currentTopic.DefaultSubscriptionStatus, topic.DefaultSubscriptionStatus)
			}
			fmt.Printf("\n")
		}
	}

	// Show topics to remove
	if len(topicsToRemove) > 0 {
		fmt.Printf("🗑️  Topics to REMOVE:\n")
		for _, topicName := range topicsToRemove {
			currentTopic := currentTopics[topicName]
			fmt.Printf("  - %s (%s)\n", topicName, aws.ToString(currentTopic.DisplayName))
		}
		fmt.Printf("\n")
	}

	if dryRun {
		fmt.Printf("DRY RUN: No changes were made. Use without --dry-run to apply changes.\n")
		return nil
	}

	// Confirmation prompt for destructive operation
	// Apply changes (backup will be created automatically)
	fmt.Printf("Applying changes...\n\n")

	// If we need to update or remove topics, we need to recreate the contact list
	if len(topicsToUpdate) > 0 || len(topicsToRemove) > 0 || len(topicsToAdd) > 0 {
		fmt.Printf("🔄 Recreating contact list with updated topics...\n")

		// Step 1: Get all current contacts
		fmt.Printf("1. Retrieving all contacts from current list...\n")
		contactsInput := &sesv2.ListContactsInput{
			ContactListName: aws.String(accountListName),
		}

		contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
		if err != nil {
			return fmt.Errorf("failed to list contacts for migration: %w", err)
		}

		fmt.Printf("   Found %d contacts to migrate\n", len(contactsResult.Contacts))

		// Step 2: Create backup of contact list and all contacts
		fmt.Printf("2. Creating backup of contact list and contacts...\n")

		_, err = CreateContactListBackup(sesClient, accountListName, "manage-topic")
		if err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}

		// Step 3: Delete old contact list first (SES doesn't allow duplicate names)
		fmt.Printf("3. Deleting old contact list: %s\n", accountListName)

		deleteInput := &sesv2.DeleteContactListInput{
			ContactListName: aws.String(accountListName),
		}

		_, err = sesClient.DeleteContactList(context.Background(), deleteInput)
		if err != nil {
			return fmt.Errorf("failed to delete old contact list: %w", err)
		}

		fmt.Printf("   ✅ Deleted old contact list\n")

		// Step 4: Create new contact list with correct topics
		fmt.Printf("4. Creating new contact list with updated topics: %s\n", accountListName)

		var newTopics []sesv2Types.Topic
		for _, configTopic := range configTopics {
			var defaultStatus sesv2Types.SubscriptionStatus
			if configTopic.DefaultSubscriptionStatus == "OPT_IN" {
				defaultStatus = sesv2Types.SubscriptionStatusOptIn
			} else {
				defaultStatus = sesv2Types.SubscriptionStatusOptOut
			}

			newTopics = append(newTopics, sesv2Types.Topic{
				TopicName:                 aws.String(configTopic.TopicName),
				DisplayName:               aws.String(configTopic.DisplayName),
				Description:               aws.String(configTopic.Description),
				DefaultSubscriptionStatus: defaultStatus,
			})
		}

		createInput := &sesv2.CreateContactListInput{
			ContactListName: aws.String(accountListName),
			Description:     listResult.Description,
			Topics:          newTopics,
		}

		_, err = sesClient.CreateContactList(context.Background(), createInput)
		if err != nil {
			return fmt.Errorf("failed to create new contact list: %w", err)
		}

		fmt.Printf("   ✅ Created new contact list with %d topics\n", len(newTopics))

		// Step 5: Migrate all contacts to the new list
		fmt.Printf("5. Migrating contacts to updated list...\n")
		migratedCount := 0

		for _, contact := range contactsResult.Contacts {
			// Create topic preferences for the new list
			var newTopicPrefs []sesv2Types.TopicPreference

			// Map old preferences to new topics
			oldPrefsMap := make(map[string]sesv2Types.SubscriptionStatus)
			for _, pref := range contact.TopicPreferences {
				oldPrefsMap[*pref.TopicName] = pref.SubscriptionStatus
			}

			// Create preferences for all new topics
			for _, configTopic := range configTopics {
				var status sesv2Types.SubscriptionStatus

				// Use old preference if it exists, otherwise use new default
				if oldStatus, exists := oldPrefsMap[configTopic.TopicName]; exists {
					status = oldStatus
				} else {
					if configTopic.DefaultSubscriptionStatus == "OPT_IN" {
						status = sesv2Types.SubscriptionStatusOptIn
					} else {
						status = sesv2Types.SubscriptionStatusOptOut
					}
				}

				newTopicPrefs = append(newTopicPrefs, sesv2Types.TopicPreference{
					TopicName:          aws.String(configTopic.TopicName),
					SubscriptionStatus: status,
				})
			}

			// Add contact to new list
			addContactInput := &sesv2.CreateContactInput{
				ContactListName:  aws.String(accountListName),
				EmailAddress:     contact.EmailAddress,
				TopicPreferences: newTopicPrefs,
				UnsubscribeAll:   contact.UnsubscribeAll,
			}

			_, err = sesClient.CreateContact(context.Background(), addContactInput)
			if err != nil {
				fmt.Printf("   ⚠️  Failed to migrate contact %s: %v\n", *contact.EmailAddress, err)
				continue
			}

			migratedCount++
		}

		fmt.Printf("   ✅ Migrated %d/%d contacts successfully\n", migratedCount, len(contactsResult.Contacts))

		fmt.Printf("\n🎉 Topic management completed successfully!\n")
		fmt.Printf("   - Updated %d topics\n", len(topicsToUpdate))
		fmt.Printf("   - Added %d topics\n", len(topicsToAdd))
		fmt.Printf("   - Removed %d topics\n", len(topicsToRemove))
		fmt.Printf("   - Migrated %d contacts\n", migratedCount)
	}

	return nil
}

// ExpandTopicsWithGroups expands topics for each topic group with proper prefixes and includes standalone topics
func ExpandTopicsWithGroups(sesConfig SESConfig) []SESTopicConfig {
	var expandedTopics []SESTopicConfig

	// First, expand grouped topics
	for _, group := range sesConfig.TopicGroupPrefix {
		for _, topic := range sesConfig.TopicGroupMembers {
			// TopicName: lowercase group + dash + topic name
			expandedTopicName := strings.ToLower(group) + "-" + topic.TopicName

			// DisplayName: prepend uppercase group + space
			expandedDisplayName := strings.ToUpper(group) + " " + topic.DisplayName

			// Description: insert uppercase group at index 1 of space-separated words
			descriptionWords := strings.Fields(topic.Description)
			var expandedDescription string
			if len(descriptionWords) >= 2 {
				// Insert group at index 1
				newWords := make([]string, 0, len(descriptionWords)+1)
				newWords = append(newWords, descriptionWords[0])     // First word
				newWords = append(newWords, strings.ToUpper(group))  // Insert group
				newWords = append(newWords, descriptionWords[1:]...) // Rest of words
				expandedDescription = strings.Join(newWords, " ")
			} else if len(descriptionWords) == 1 {
				// Only one word, append group after it
				expandedDescription = descriptionWords[0] + " " + strings.ToUpper(group)
			} else {
				// Empty description, just use group
				expandedDescription = strings.ToUpper(group)
			}

			expandedTopic := SESTopicConfig{
				TopicName:                 expandedTopicName,
				DisplayName:               expandedDisplayName,
				Description:               expandedDescription,
				DefaultSubscriptionStatus: topic.DefaultSubscriptionStatus,
			}
			expandedTopics = append(expandedTopics, expandedTopic)
		}
	}

	// Then, add standalone topics as-is
	for _, topic := range sesConfig.Topics {
		expandedTopics = append(expandedTopics, topic)
	}

	return expandedTopics
}

// CreateContactListFromBackup creates a contact list and restores all contacts from a backup file
func CreateContactListFromBackup(sesClient *sesv2.Client, backupFilePath string) error {
	ConfigPath := GetConfigPath()

	// Read backup file
	fmt.Printf("📁 Reading backup file: %s\n", backupFilePath)
	backupJson, err := os.ReadFile(ConfigPath + backupFilePath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	var backup SESBackup
	err = json.Unmarshal(backupJson, &backup)
	if err != nil {
		return fmt.Errorf("failed to parse backup file: %w", err)
	}

	fmt.Printf("📋 Backup contains:\n")
	fmt.Printf("   - Contact List: %s\n", backup.ContactList.Name)
	fmt.Printf("   - Topics: %d\n", len(backup.ContactList.Topics))
	fmt.Printf("   - Contacts: %d\n", len(backup.Contacts))
	fmt.Printf("   - Backup Date: %s\n", backup.BackupMetadata.Timestamp)
	fmt.Printf("\n")

	// Step 1: Create contact list with topics from backup
	fmt.Printf("1. Creating contact list: %s\n", backup.ContactList.Name)

	createInput := &sesv2.CreateContactListInput{
		ContactListName: aws.String(backup.ContactList.Name),
		Description:     backup.ContactList.Description,
		Topics:          backup.ContactList.Topics,
	}

	_, err = sesClient.CreateContactList(context.Background(), createInput)
	if err != nil {
		return fmt.Errorf("failed to create contact list: %w", err)
	}

	fmt.Printf("   ✅ Created contact list with %d topics\n", len(backup.ContactList.Topics))

	// Step 2: Restore all contacts
	fmt.Printf("2. Restoring contacts...\n")
	restoredCount := 0

	for _, contact := range backup.Contacts {
		addContactInput := &sesv2.CreateContactInput{
			ContactListName:  aws.String(backup.ContactList.Name),
			EmailAddress:     aws.String(contact.EmailAddress),
			TopicPreferences: contact.TopicPreferences,
			UnsubscribeAll:   contact.UnsubscribeAll,
		}

		_, err = sesClient.CreateContact(context.Background(), addContactInput)
		if err != nil {
			fmt.Printf("   ⚠️  Failed to restore contact %s: %v\n", contact.EmailAddress, err)
			continue
		}

		restoredCount++
	}

	fmt.Printf("   ✅ Restored %d/%d contacts successfully\n", restoredCount, len(backup.Contacts))

	fmt.Printf("\n🎉 Contact list restoration completed successfully!\n")
	fmt.Printf("   - List Name: %s\n", backup.ContactList.Name)
	fmt.Printf("   - Topics: %d\n", len(backup.ContactList.Topics))
	fmt.Printf("   - Contacts: %d\n", restoredCount)

	return nil
}

// printSESHelp displays detailed help information for SES actions
func printSESHelp() {
	fmt.Println("AWS SES Contact List Management - Available Actions")
	fmt.Println("=" + strings.Repeat("=", 50))
	fmt.Println()

	fmt.Println("📋 CONTACT LIST MANAGEMENT:")
	fmt.Println("  create-list          Create a new contact list")
	fmt.Println("                       • From config: -ses-config-file SESConfig.json")
	fmt.Println("                       • From backup: -backup-file backup.json")
	fmt.Println()
	fmt.Println("  describe-list        Show contact list details and topics")
	fmt.Println("  describe-account     Show account's main contact list details")
	fmt.Println()

	fmt.Println("👥 CONTACT MANAGEMENT:")
	fmt.Println("  add-contact          Add email to contact list")
	fmt.Println("                       • Required: -email user@example.com")
	fmt.Println("                       • Optional: -topics topic1,topic2")
	fmt.Println()
	fmt.Println("  remove-contact       Remove specific email from list")
	fmt.Println("                       • Required: -email user@example.com")
	fmt.Println()
	fmt.Println("  remove-contact-all   Remove ALL contacts from list (creates backup)")
	fmt.Println("                       • ⚠️  Creates automatic backup before removal")
	fmt.Println("                       • 📁 Backup: ses-backup-{list}-{timestamp}.json")
	fmt.Println()
	fmt.Println("  list-contacts        List all contacts in the contact list")
	fmt.Println()

	fmt.Println("🔍 CONTACT INFORMATION:")
	fmt.Println("  describe-contact     Show contact details and subscriptions")
	fmt.Println("                       • Required: -email user@example.com")
	fmt.Println()

	fmt.Println("📧 SUPPRESSION LIST:")
	fmt.Println("  suppress             Add email to suppression list")
	fmt.Println("                       • Required: -email user@example.com")
	fmt.Println("                       • Optional: -suppression-reason bounce|complaint")
	fmt.Println()
	fmt.Println("  unsuppress           Remove email from suppression list")
	fmt.Println("                       • Required: -email user@example.com")
	fmt.Println()

	fmt.Println("🏷️  TOPIC MANAGEMENT:")
	fmt.Println("  describe-topic       Show specific topic details")
	fmt.Println("                       • Required: -topic-name topic-name")
	fmt.Println()
	fmt.Println("  describe-topic-all   Show all topics and subscription stats")
	fmt.Println()
	fmt.Println("  manage-topic         Update contact list topics (creates backup)")
	fmt.Println("                       • Uses: -ses-config-file SESConfig.json")
	fmt.Println("                       • Optional: -dry-run (preview changes)")
	fmt.Println()

	fmt.Println("📖 USAGE EXAMPLES:")
	fmt.Println("  # Create contact list from config")
	fmt.Println("  ./aws-alternate-contact-manager ses -action create-list")
	fmt.Println()
	fmt.Println("  # Add contact with specific topics")
	fmt.Println("  ./aws-alternate-contact-manager ses -action add-contact -email user@example.com -topics aws-calendar,wiz-approval")
	fmt.Println()
	fmt.Println("  # Remove all contacts (with backup)")
	fmt.Println("  ./aws-alternate-contact-manager ses -action remove-contact-all")
	fmt.Println()
	fmt.Println("  # Preview topic changes")
	fmt.Println("  ./aws-alternate-contact-manager ses -action manage-topic -dry-run")
	fmt.Println()
	fmt.Println("  # Restore from backup")
	fmt.Println("  ./aws-alternate-contact-manager ses -action create-list -backup-file ses-backup-list-20250915-214033.json")
	fmt.Println()

	fmt.Println("⚙️  CONFIGURATION:")
	fmt.Println("  -ses-config-file     Path to SES config (default: SESConfig.json)")
	fmt.Println("  -backup-file         Path to backup file for restore operations")
	fmt.Println("  -email               Email address for contact operations")
	fmt.Println("  -topics              Comma-separated topic list")
	fmt.Println("  -topic-name          Specific topic name")
	fmt.Println("  -suppression-reason  Reason for suppression (bounce|complaint)")
	fmt.Println("  -dry-run             Preview changes without applying")
	fmt.Println()

	fmt.Println("🔒 SAFETY FEATURES:")
	fmt.Println("  • Automatic backups for destructive operations")
	fmt.Println("  • Dry-run mode for preview")
	fmt.Println("  • Progress tracking and error reporting")
	fmt.Println("  • Backup files include complete restoration data")
	fmt.Println()
}

// ManageSESLists handles SES list management operations
func ManageSESLists(action string, sesConfigFile string, backupFile string, email string, topics []string, suppressionReason string, topicName string, dryRun bool) {
	ConfigPath := GetConfigPath()
	fmt.Println("Working in Config Path: " + ConfigPath)

	// Read SES config file
	sesJson, err := os.ReadFile(ConfigPath + sesConfigFile)
	if err != nil {
		fmt.Printf("Error reading SES config file: %v\n", err)
		return
	}
	fmt.Println("Successfully opened " + ConfigPath + sesConfigFile)

	var sesConfig SESConfig
	err = json.NewDecoder(bytes.NewReader(sesJson)).Decode(&sesConfig)
	if err != nil {
		fmt.Printf("Error parsing SES config: %v\n", err)
		return
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Printf("Failed to load AWS configuration: %v\n", err)
		return
	}

	sesClient := sesv2.NewFromConfig(cfg)

	// Get the account's main contact list for operations that need it
	var accountListName string
	if action == "add-contact" || action == "remove-contact" || action == "list-contacts" || action == "describe-list" {
		accountListName, err = GetAccountContactList(sesClient)
		if err != nil {
			fmt.Printf("Error finding account contact list: %v\n", err)
			return
		}
	}

	switch action {
	case "create-list":
		if backupFile != "" {
			// Create list from backup file
			err = CreateContactListFromBackup(sesClient, backupFile)
		} else {
			// Create list from SES config
			listName := "main-contact-list"
			var topicsToUse []SESTopicConfig
			if len(topics) == 0 {
				// Use expanded topics from config (with topic groups)
				topicsToUse = ExpandTopicsWithGroups(sesConfig)
			} else {
				// Convert string topics to topic configs with defaults
				for _, topicName := range topics {
					topicsToUse = append(topicsToUse, SESTopicConfig{
						TopicName:                 topicName,
						DisplayName:               topicName,
						Description:               "User-defined topic",
						DefaultSubscriptionStatus: "OPT_OUT",
					})
				}
			}
			err = CreateContactList(sesClient, listName, "Managed contact list", topicsToUse)
		}
	case "add-contact":
		var topicsToUse []string
		if len(topics) == 0 {
			// Extract topic names from expanded config
			expandedTopics := ExpandTopicsWithGroups(sesConfig)
			for _, topicConfig := range expandedTopics {
				topicsToUse = append(topicsToUse, topicConfig.TopicName)
			}
		} else {
			topicsToUse = topics
		}
		err = AddContactToList(sesClient, accountListName, email, topicsToUse)
	case "remove-contact":
		err = RemoveContactFromList(sesClient, accountListName, email)
	case "remove-contact-all":
		err = RemoveAllContactsFromList(sesClient, accountListName)
	case "suppress":
		var reason sesv2Types.SuppressionListReason
		switch suppressionReason {
		case "bounce":
			reason = sesv2Types.SuppressionListReasonBounce
		case "complaint":
			reason = sesv2Types.SuppressionListReasonComplaint
		default:
			reason = sesv2Types.SuppressionListReasonBounce
		}
		err = AddToSuppressionList(sesClient, email, reason)
	case "unsuppress":
		err = RemoveFromSuppressionList(sesClient, email)
	case "list-contacts":
		err = ListContactsInList(sesClient, accountListName)
	case "describe-list":
		err = DescribeContactList(sesClient, accountListName)
	case "describe-account":
		// Automatically find and describe the account's main contact list
		accountListName, err := GetAccountContactList(sesClient)
		if err != nil {
			fmt.Printf("Error finding account contact list: %v\n", err)
			return
		}
		fmt.Printf("Account's main contact list: %s\n\n", accountListName)
		err = DescribeContactList(sesClient, accountListName)
	case "describe-topic":
		if topicName == "" {
			fmt.Printf("Error: topic name is required for describe-topic action\n")
			return
		}
		err = DescribeTopic(sesClient, topicName)
	case "describe-topic-all":
		err = DescribeAllTopics(sesClient)
	case "describe-contact":
		if email == "" {
			fmt.Printf("Error: email is required for describe-contact action\n")
			return
		}
		err = DescribeContact(sesClient, email)
	case "manage-topic":
		expandedTopics := ExpandTopicsWithGroups(sesConfig)
		err = ManageTopics(sesClient, expandedTopics, dryRun)
	case "help":
		printSESHelp()
		return
	default:
		fmt.Printf("Unknown action: %s\n", action)
		fmt.Println("\nUse '-action help' to see available actions and usage examples.")
		return
	}

	if err != nil {
		fmt.Printf("Error executing action %s: %v\n", action, err)
	}
}

func main() {
	// Check if we have at least one argument (the subcommand)
	if len(os.Args) < 2 {
		fmt.Println("Usage: aws-alternate-contact-manager [subcommand] [options]")
		fmt.Println("Subcommands:")
		fmt.Println("  alt-contact       Manage AWS alternate contacts")
		fmt.Println("  ses               Manage SES mailing lists and suppression")
		fmt.Println("  help              Show this help message")
		os.Exit(1)
	}

	// Define FlagSets for each subcommand
	altContactCommand := flag.NewFlagSet("alt-contact", flag.ExitOnError)
	sesCommand := flag.NewFlagSet("ses", flag.ExitOnError)

	//define flags for the alt-contact subcommand
	altContactAction := altContactCommand.String("action", "", "Action to perform (set-all, set-one, delete)")
	altContactConfigFile := altContactCommand.String("contact-config-file", "ContactConfig.json", "Path to the contact configuration file (default: ContactConfig.json)")
	altContactOrgPrefix := altContactCommand.String("org-prefix", "", "Organization prefix (required for set-one and delete actions)")
	altContactOverwrite := altContactCommand.Bool("overwrite", false, "Overwrite existing contacts if true")
	altContactTypes := altContactCommand.String("contact-types", "", "Comma separated list of contact types to delete (security, billing, operations)")

	//define flags for the ses subcommand
	sesAction := sesCommand.String("action", "", "SES action (create-list, add-contact, remove-contact, remove-contact-all, suppress, unsuppress, list-contacts, describe-list, describe-account, describe-topic, describe-topic-all, describe-contact, manage-topic, help)")
	sesConfigFile := sesCommand.String("ses-config-file", "SESConfig.json", "Path to the SES configuration file (default: SESConfig.json)")
	sesBackupFile := sesCommand.String("backup-file", "", "Path to backup file for restore operations (for create-list action)")
	sesEmail := sesCommand.String("email", "", "Email address for contact operations")
	sesTopics := sesCommand.String("topics", "", "Comma-separated list of topics for contact subscription")
	sesSuppressionReason := sesCommand.String("suppression-reason", "bounce", "Reason for suppression (bounce or complaint)")
	sesTopicName := sesCommand.String("topic-name", "", "Topic name for topic-specific operations")
	sesDryRun := sesCommand.Bool("dry-run", false, "Show what would be done without making changes")

	// Switch on the subcommand
	switch os.Args[1] {
	case "alt-contact":
		altContactCommand.Parse(os.Args[2:])
	case "ses":
		sesCommand.Parse(os.Args[2:])
	case "help":
		fmt.Println("Usage: aws-alternate-contact-manager [subcommand] [options]")
		fmt.Println("Subcommands:")
		fmt.Println("  alt-contact       Manage AWS alternate contacts")
		fmt.Println("    Actions:")
		fmt.Println("      set-all       Set alternate contacts for all organizations in config file")
		fmt.Println("      set-one       Set alternate contacts for a single organization")
		fmt.Println("      delete        Delete alternate contacts")
		fmt.Println("  ses               Manage SES mailing lists and suppression")
		fmt.Println("  help              Show this help message")
		return
	default:
		fmt.Println("Unknown subcommand:", os.Args[1])
		fmt.Println("Usage: aws-alternate-contact-manager [subcommand] [options]")
		fmt.Println("Subcommands:")
		fmt.Println("  alt-contact       Manage AWS alternate contacts")
		fmt.Println("  ses               Manage SES mailing lists and suppression")
		fmt.Println("  help              Show this help message")
		os.Exit(1)
	}

	if altContactCommand.Parsed() {
		if *altContactAction == "" {
			fmt.Println("Error: action is required for alt-contact commands")
			fmt.Println("Available actions: set-all, set-one, delete")
			altContactCommand.PrintDefaults()
			os.Exit(1)
		}

		switch *altContactAction {
		case "set-all":
			SetContactsForAllOrganizations(altContactConfigFile, altContactOverwrite)
		case "set-one":
			if *altContactOrgPrefix == "" {
				fmt.Println("Error: org-prefix is required for set-one action")
				altContactCommand.PrintDefaults()
				os.Exit(1)
			}
			SetContactsForSingleOrganization(altContactConfigFile, altContactOrgPrefix, altContactOverwrite)
		case "delete":
			if *altContactOrgPrefix == "" || *altContactTypes == "" {
				fmt.Println("Error: org-prefix and contact-types are required for delete action")
				altContactCommand.PrintDefaults()
				os.Exit(1)
			}
			DeleteContactsFromOrganization(altContactOrgPrefix, altContactTypes)
		default:
			fmt.Printf("Unknown action: %s\n", *altContactAction)
			fmt.Println("Available actions: set-all, set-one, delete")
			os.Exit(1)
		}
	}

	if sesCommand.Parsed() {
		if *sesAction == "" {
			fmt.Println("Error: action is required for SES commands")
			sesCommand.PrintDefaults()
			os.Exit(1)
		}

		// Parse topics if provided
		var topics []string
		if *sesTopics != "" {
			topics = strings.Split(*sesTopics, ",")
			for i, topic := range topics {
				topics[i] = strings.TrimSpace(topic)
			}
		}

		ManageSESLists(*sesAction, *sesConfigFile, *sesBackupFile, *sesEmail, topics, *sesSuppressionReason, *sesTopicName, *sesDryRun)
	}
}
