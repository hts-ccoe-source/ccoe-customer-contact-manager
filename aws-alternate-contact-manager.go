package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/account"
	accountTypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoreTypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
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

// RateLimiter implements a simple rate limiter using a channel
type RateLimiter struct {
	ticker   *time.Ticker
	requests chan struct{}
}

// NewRateLimiter creates a new rate limiter with the specified requests per second
func NewRateLimiter(requestsPerSecond int) *RateLimiter {
	rl := &RateLimiter{
		ticker:   time.NewTicker(time.Second / time.Duration(requestsPerSecond)),
		requests: make(chan struct{}, requestsPerSecond),
	}

	// Fill the initial bucket
	for i := 0; i < requestsPerSecond; i++ {
		rl.requests <- struct{}{}
	}

	// Start the ticker to refill the bucket
	go func() {
		for range rl.ticker.C {
			select {
			case rl.requests <- struct{}{}:
			default:
				// Bucket is full, skip
			}
		}
	}()

	return rl
}

// Wait blocks until a request can be made
func (rl *RateLimiter) Wait() {
	<-rl.requests
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	rl.ticker.Stop()
}

// IdentityCenterUser represents a user from Identity Center
type IdentityCenterUser struct {
	UserId      string `json:"user_id"`
	UserName    string `json:"user_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	GivenName   string `json:"given_name"`
	FamilyName  string `json:"family_name"`
	Active      bool   `json:"active"`
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
		// Skip empty or blank topic names
		if strings.TrimSpace(topic) != "" {
			topicPreferences = append(topicPreferences, sesv2Types.TopicPreference{
				TopicName:          aws.String(topic),
				SubscriptionStatus: sesv2Types.SubscriptionStatusOptIn,
			})
		}
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

// validateContactListTopics checks if the required topics exist in the contact list
func validateContactListTopics(sesClient *sesv2.Client, listName string, config ContactImportConfig) error {
	// Get contact list details
	input := &sesv2.GetContactListInput{
		ContactListName: aws.String(listName),
	}

	result, err := sesClient.GetContactList(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to get contact list details: %w", err)
	}

	// Build set of existing topics
	existingTopics := make(map[string]bool)
	for _, topic := range result.Topics {
		existingTopics[*topic.TopicName] = true
	}

	// Check required topics
	var missingTopics []string

	// Check default topics
	for _, topic := range config.DefaultTopics {
		if !existingTopics[topic] {
			missingTopics = append(missingTopics, topic)
		}
	}

	// Check role mapping topics
	for _, mapping := range config.RoleMappings {
		for _, topic := range mapping.Topics {
			if !existingTopics[topic] {
				found := false
				for _, missing := range missingTopics {
					if missing == topic {
						found = true
						break
					}
				}
				if !found {
					missingTopics = append(missingTopics, topic)
				}
			}
		}
	}

	if len(missingTopics) > 0 {
		return fmt.Errorf("contact list '%s' is missing required topics: %v", listName, missingTopics)
	}

	fmt.Printf("✅ All required topics found in contact list\n")
	return nil
}

// AddContactToListQuiet adds an email contact to a contact list without verbose output
func AddContactToListQuiet(sesClient *sesv2.Client, listName string, email string, topics []string) error {
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

	// Create rate limiter (2 requests per second to avoid 429 errors)
	ticker := time.NewTicker(500 * time.Millisecond) // 2 requests per second
	defer ticker.Stop()

	// Remove each contact with rate limiting
	successCount := 0
	errorCount := 0
	for i, contact := range result.Contacts {
		// Wait for rate limiter (except for first request)
		if i > 0 {
			<-ticker.C
		}

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

// ListIdentityCenterUser lists a specific user from Identity Center
func ListIdentityCenterUser(identityStoreClient *identitystore.Client, identityStoreId string, userName string) (*IdentityCenterUser, error) {
	// Search for user by username
	input := &identitystore.ListUsersInput{
		IdentityStoreId: aws.String(identityStoreId),
		Filters: []identitystoreTypes.Filter{
			{
				AttributePath:  aws.String("UserName"),
				AttributeValue: aws.String(userName),
			},
		},
	}

	result, err := identityStoreClient.ListUsers(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	if len(result.Users) == 0 {
		return nil, fmt.Errorf("user %s not found in Identity Center", userName)
	}

	if len(result.Users) > 1 {
		return nil, fmt.Errorf("multiple users found with username %s", userName)
	}

	user := result.Users[0]

	// Extract email from user attributes
	var email string
	for _, emailAttr := range user.Emails {
		if emailAttr.Primary && emailAttr.Value != nil {
			email = *emailAttr.Value
			break
		}
	}
	if email == "" && len(user.Emails) > 0 && user.Emails[0].Value != nil {
		email = *user.Emails[0].Value
	}

	// Extract names
	var givenName, familyName string
	if user.Name != nil {
		if user.Name.GivenName != nil {
			givenName = *user.Name.GivenName
		}
		if user.Name.FamilyName != nil {
			familyName = *user.Name.FamilyName
		}
	}

	icUser := &IdentityCenterUser{
		UserId:      *user.UserId,
		UserName:    *user.UserName,
		DisplayName: *user.DisplayName,
		Email:       email,
		GivenName:   givenName,
		FamilyName:  familyName,
		Active:      true, // Identity Store users are active by default
	}

	return icUser, nil
}

// ListIdentityCenterUsersAll lists all users from Identity Center with concurrency and rate limiting
func ListIdentityCenterUsersAll(identityStoreClient *identitystore.Client, identityStoreId string, maxConcurrency int, requestsPerSecond int) ([]IdentityCenterUser, error) {
	fmt.Printf("🔍 Listing all users from Identity Center (ID: %s)\n", identityStoreId)
	fmt.Printf("⚙️  Concurrency: %d workers, Rate limit: %d req/sec\n", maxConcurrency, requestsPerSecond)

	// Create rate limiter
	rateLimiter := NewRateLimiter(requestsPerSecond)
	defer rateLimiter.Stop()

	var allUsers []IdentityCenterUser
	var nextToken *string
	pageCount := 0

	// First, get all user IDs with pagination
	for {
		pageCount++
		fmt.Printf("📄 Fetching page %d...\n", pageCount)

		rateLimiter.Wait() // Rate limit the API call

		input := &identitystore.ListUsersInput{
			IdentityStoreId: aws.String(identityStoreId),
			MaxResults:      aws.Int32(50), // AWS default max
		}

		if nextToken != nil {
			input.NextToken = nextToken
		}

		result, err := identityStoreClient.ListUsers(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to list users on page %d: %w", pageCount, err)
		}

		fmt.Printf("   Found %d users on page %d\n", len(result.Users), pageCount)

		// Process users with concurrency
		if len(result.Users) > 0 {
			pageUsers := processUsersWithConcurrency(result.Users, maxConcurrency, rateLimiter)
			allUsers = append(allUsers, pageUsers...)
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	fmt.Printf("✅ Total users retrieved: %d\n", len(allUsers))
	return allUsers, nil
}

// processUsersWithConcurrency processes a batch of users with controlled concurrency
func processUsersWithConcurrency(users []identitystoreTypes.User, maxConcurrency int, rateLimiter *RateLimiter) []IdentityCenterUser {
	var wg sync.WaitGroup
	userChan := make(chan identitystoreTypes.User, len(users))
	resultChan := make(chan IdentityCenterUser, len(users))

	// Start worker goroutines
	for i := 0; i < maxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for user := range userChan {
				rateLimiter.Wait() // Rate limit each processing
				processedUser := convertToIdentityCenterUser(user)
				resultChan <- processedUser
			}
		}()
	}

	// Send users to workers
	for _, user := range users {
		userChan <- user
	}
	close(userChan)

	// Wait for all workers to complete
	wg.Wait()
	close(resultChan)

	// Collect results
	var results []IdentityCenterUser
	for user := range resultChan {
		results = append(results, user)
	}

	return results
}

// convertToIdentityCenterUser converts AWS SDK user type to our custom type
func convertToIdentityCenterUser(user identitystoreTypes.User) IdentityCenterUser {
	// Extract email from user attributes
	var email string
	for _, emailAttr := range user.Emails {
		if emailAttr.Primary && emailAttr.Value != nil {
			email = *emailAttr.Value
			break
		}
	}
	if email == "" && len(user.Emails) > 0 && user.Emails[0].Value != nil {
		email = *user.Emails[0].Value
	}

	// Extract names
	var givenName, familyName string
	if user.Name != nil {
		if user.Name.GivenName != nil {
			givenName = *user.Name.GivenName
		}
		if user.Name.FamilyName != nil {
			familyName = *user.Name.FamilyName
		}
	}

	return IdentityCenterUser{
		UserId:      *user.UserId,
		UserName:    *user.UserName,
		DisplayName: *user.DisplayName,
		Email:       email,
		GivenName:   givenName,
		FamilyName:  familyName,
		Active:      true, // Identity Store users are active by default
	}
}

// SaveIdentityCenterUsersToJSON saves users data to a JSON file
func SaveIdentityCenterUsersToJSON(users []IdentityCenterUser, filename string) error {
	jsonData, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users data to JSON: %w", err)
	}

	configPath := GetConfigPath()
	fullPath := configPath + filename

	err = os.WriteFile(fullPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file %s: %w", fullPath, err)
	}

	fmt.Printf("📁 Users data saved to: %s\n", filename)
	return nil
}

// SaveIdentityCenterUserToJSON saves single user data to a JSON file
func SaveIdentityCenterUserToJSON(user *IdentityCenterUser, filename string) error {
	jsonData, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user data to JSON: %w", err)
	}

	configPath := GetConfigPath()
	fullPath := configPath + filename

	err = os.WriteFile(fullPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file %s: %w", fullPath, err)
	}

	fmt.Printf("📁 User data saved to: %s\n", filename)
	return nil
}

// SaveGroupMembershipsToJSON saves group membership data to a JSON file
func SaveGroupMembershipsToJSON(memberships []IdentityCenterGroupMembership, filename string) error {
	jsonData, err := json.MarshalIndent(memberships, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal group memberships to JSON: %w", err)
	}

	configPath := GetConfigPath()
	fullPath := configPath + filename

	err = os.WriteFile(fullPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file %s: %w", fullPath, err)
	}

	fmt.Printf("📁 Group memberships saved to: %s\n", filename)
	return nil
}

// ConvertToGroupCentric converts user-centric data to group-centric format
func ConvertToGroupCentric(memberships []IdentityCenterGroupMembership) []IdentityCenterGroupCentric {
	groupMap := make(map[string][]IdentityCenterUserInfo)

	// Build map of groups to users
	for _, membership := range memberships {
		userInfo := IdentityCenterUserInfo{
			UserId:      membership.UserId,
			UserName:    membership.UserName,
			DisplayName: membership.DisplayName,
			Email:       membership.Email,
		}

		for _, group := range membership.Groups {
			groupMap[group] = append(groupMap[group], userInfo)
		}
	}

	// Convert map to slice and sort by group name
	var groupCentric []IdentityCenterGroupCentric
	for groupName, members := range groupMap {
		groupCentric = append(groupCentric, IdentityCenterGroupCentric{
			GroupName: groupName,
			Members:   members,
		})
	}

	// Sort groups by name for consistent output
	for i := 0; i < len(groupCentric)-1; i++ {
		for j := i + 1; j < len(groupCentric); j++ {
			if groupCentric[i].GroupName > groupCentric[j].GroupName {
				groupCentric[i], groupCentric[j] = groupCentric[j], groupCentric[i]
			}
		}
	}

	return groupCentric
}

// ParseCCOECloudGroup parses ccoe-cloud group names to extract AWS account information
// Pattern: ccoe-cloud-{account-name}-{account-id}-idp-{application-prefix}-{role-name}
func ParseCCOECloudGroup(groupName string) CCOECloudGroupInfo {
	result := CCOECloudGroupInfo{
		GroupName: groupName,
		IsValid:   false,
	}

	// Check if group starts with ccoe-cloud-
	if !strings.HasPrefix(groupName, "ccoe-cloud-") {
		return result
	}

	// Remove the ccoe-cloud- prefix
	remaining := strings.TrimPrefix(groupName, "ccoe-cloud-")

	// Split by dashes
	parts := strings.Split(remaining, "-")
	if len(parts) < 5 {
		return result // Not enough parts
	}

	// Find the account ID (string of digits)
	accountIdIndex := -1
	for i, part := range parts {
		// Check if this part is all digits (account ID)
		if len(part) > 0 && isAllDigits(part) {
			accountIdIndex = i
			break
		}
	}

	if accountIdIndex == -1 {
		return result // No account ID found
	}

	// Account name is everything before the account ID
	if accountIdIndex == 0 {
		return result // No account name
	}

	accountNameParts := parts[:accountIdIndex]
	result.AccountName = strings.Join(accountNameParts, "-")
	result.AccountId = parts[accountIdIndex]

	// Check for -idp- separator after account ID
	if accountIdIndex+1 >= len(parts) || parts[accountIdIndex+1] != "idp" {
		return result // Missing idp separator
	}

	// Everything after -idp- should be application-prefix and role-name
	remainingParts := parts[accountIdIndex+2:]
	if len(remainingParts) < 2 {
		return result // Need at least application-prefix and role-name
	}

	// Last part is role name, everything before that is application prefix
	result.RoleName = remainingParts[len(remainingParts)-1]
	applicationPrefixParts := remainingParts[:len(remainingParts)-1]
	result.ApplicationPrefix = strings.Join(applicationPrefixParts, "-")

	result.IsValid = true
	return result
}

// isAllDigits checks if a string contains only digits
func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ParseAllCCOECloudGroups parses all ccoe-cloud groups from group membership data
func ParseAllCCOECloudGroups(memberships []IdentityCenterGroupMembership) []CCOECloudGroupInfo {
	var ccoeGroups []CCOECloudGroupInfo
	groupsSeen := make(map[string]bool)

	// Extract unique ccoe-cloud groups
	for _, membership := range memberships {
		for _, group := range membership.Groups {
			if strings.HasPrefix(group, "ccoe-cloud-") && !groupsSeen[group] {
				groupsSeen[group] = true
				parsed := ParseCCOECloudGroup(group)
				if parsed.IsValid {
					ccoeGroups = append(ccoeGroups, parsed)
				}
			}
		}
	}

	// Sort by account name, then by application prefix, then by role name
	for i := 0; i < len(ccoeGroups)-1; i++ {
		for j := i + 1; j < len(ccoeGroups); j++ {
			if shouldSwap(ccoeGroups[i], ccoeGroups[j]) {
				ccoeGroups[i], ccoeGroups[j] = ccoeGroups[j], ccoeGroups[i]
			}
		}
	}

	return ccoeGroups
}

// shouldSwap determines if two CCOE groups should be swapped for sorting
func shouldSwap(a, b CCOECloudGroupInfo) bool {
	if a.AccountName != b.AccountName {
		return a.AccountName > b.AccountName
	}
	if a.ApplicationPrefix != b.ApplicationPrefix {
		return a.ApplicationPrefix > b.ApplicationPrefix
	}
	return a.RoleName > b.RoleName
}

// SaveCCOECloudGroupsToJSON saves parsed CCOE cloud groups to a JSON file
func SaveCCOECloudGroupsToJSON(ccoeGroups []CCOECloudGroupInfo, filename string) error {
	jsonData, err := json.MarshalIndent(ccoeGroups, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal CCOE cloud groups to JSON: %w", err)
	}

	configPath := GetConfigPath()
	fullPath := configPath + filename

	err = os.WriteFile(fullPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file %s: %w", fullPath, err)
	}

	fmt.Printf("📁 CCOE cloud groups saved to: %s\n", filename)
	return nil
}

// GetDefaultContactImportConfig returns the default role-to-topic mapping configuration
func GetDefaultContactImportConfig() ContactImportConfig {
	return ContactImportConfig{
		RoleMappings: []RoleTopicMapping{
			{
				Roles:  []string{"security", "devops", "cloudeng", "networking"},
				Topics: []string{"aws-calendar", "aws-announce"},
			},
		},
		DefaultTopics:      []string{"general-updates"},
		RequireActiveUsers: true,
	}
}

// LoadIdentityCenterDataFromFiles loads user and group membership data from JSON files
// If identityCenterId is empty, it will attempt to auto-detect from existing files
func LoadIdentityCenterDataFromFiles(identityCenterId string) ([]IdentityCenterUser, []IdentityCenterGroupMembership, string, error) {
	configPath := GetConfigPath()

	// Auto-detect identity center ID if not provided
	if identityCenterId == "" {
		detectedId, err := autoDetectIdentityCenterId()
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to auto-detect identity center ID: %w", err)
		}
		identityCenterId = detectedId
		fmt.Printf("🔍 Auto-detected Identity Center ID: %s\n", identityCenterId)
	}

	// Find the most recent user and group membership files
	userFile, err := findMostRecentFile(configPath, fmt.Sprintf("identity-center-users-%s-", identityCenterId))
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to find user data file: %w", err)
	}

	groupFile, err := findMostRecentFile(configPath, fmt.Sprintf("identity-center-group-memberships-user-centric-%s-", identityCenterId))
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to find group membership data file: %w", err)
	}

	// Load users
	userJson, err := os.ReadFile(configPath + userFile)
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to read user file %s: %w", userFile, err)
	}

	var users []IdentityCenterUser
	err = json.Unmarshal(userJson, &users)
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to parse user file %s: %w", userFile, err)
	}

	// Load group memberships
	groupJson, err := os.ReadFile(configPath + groupFile)
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to read group membership file %s: %w", groupFile, err)
	}

	var memberships []IdentityCenterGroupMembership
	err = json.Unmarshal(groupJson, &memberships)
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to parse group membership file %s: %w", groupFile, err)
	}

	fmt.Printf("📁 Loaded %d users from: %s\n", len(users), userFile)
	fmt.Printf("📁 Loaded %d group memberships from: %s\n", len(memberships), groupFile)

	return users, memberships, identityCenterId, nil
}

// autoDetectIdentityCenterId attempts to find identity center ID from existing files
func autoDetectIdentityCenterId() (string, error) {
	configPath := GetConfigPath()
	files, err := os.ReadDir(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", configPath, err)
	}

	// Look for identity center files and extract ID
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			// Check for user files: identity-center-users-{id}-{timestamp}.json
			if strings.HasPrefix(name, "identity-center-users-") && strings.HasSuffix(name, ".json") {
				// Remove prefix and suffix, then extract ID
				// Format: identity-center-users-d-906638888d-20250915-232635.json
				withoutPrefix := strings.TrimPrefix(name, "identity-center-users-")
				withoutSuffix := strings.TrimSuffix(withoutPrefix, ".json")
				// Split by dash and find the ID (starts with 'd-')
				parts := strings.Split(withoutSuffix, "-")
				for i, part := range parts {
					if part == "d" && i+1 < len(parts) {
						// ID is d-{next part}
						id := "d-" + parts[i+1]
						return id, nil
					}
				}
			}
			// Check for group membership files: identity-center-group-memberships-user-centric-{id}-{timestamp}.json
			if strings.HasPrefix(name, "identity-center-group-memberships-user-centric-") && strings.HasSuffix(name, ".json") {
				// Remove prefix and suffix, then extract ID
				withoutPrefix := strings.TrimPrefix(name, "identity-center-group-memberships-user-centric-")
				withoutSuffix := strings.TrimSuffix(withoutPrefix, ".json")
				// Split by dash and find the ID (starts with 'd-')
				parts := strings.Split(withoutSuffix, "-")
				for i, part := range parts {
					if part == "d" && i+1 < len(parts) {
						// ID is d-{next part}
						id := "d-" + parts[i+1]
						return id, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("no identity center files found in %s", configPath)
}

// findMostRecentFile finds the most recent file matching a prefix
func findMostRecentFile(directory, prefix string) (string, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", directory, err)
	}

	var matchingFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), prefix) {
			matchingFiles = append(matchingFiles, file.Name())
		}
	}

	if len(matchingFiles) == 0 {
		return "", fmt.Errorf("no files found with prefix %s", prefix)
	}

	// Sort files by name (which includes timestamp) to get the most recent
	for i := 0; i < len(matchingFiles)-1; i++ {
		for j := i + 1; j < len(matchingFiles); j++ {
			if matchingFiles[i] < matchingFiles[j] {
				matchingFiles[i], matchingFiles[j] = matchingFiles[j], matchingFiles[i]
			}
		}
	}

	return matchingFiles[0], nil
}

// DetermineUserTopics determines which topics a user should be subscribed to based on their group memberships
func DetermineUserTopics(user IdentityCenterUser, membership *IdentityCenterGroupMembership, config ContactImportConfig) []string {
	var topics []string
	topicSet := make(map[string]bool)

	// Add default topics for all active users
	if !config.RequireActiveUsers || user.Active {
		for _, topic := range config.DefaultTopics {
			if !topicSet[topic] {
				topics = append(topics, topic)
				topicSet[topic] = true
			}
		}
	}

	// Check role mappings if user has group memberships
	if membership != nil {
		// Parse CCOE cloud groups to extract roles
		var userRoles []string
		for _, group := range membership.Groups {
			if strings.HasPrefix(group, "ccoe-cloud-") {
				parsed := ParseCCOECloudGroup(group)
				if parsed.IsValid {
					userRoles = append(userRoles, parsed.RoleName)
				}
			}
		}

		// Check each role mapping
		for _, mapping := range config.RoleMappings {
			for _, userRole := range userRoles {
				for _, mappingRole := range mapping.Roles {
					if strings.EqualFold(userRole, mappingRole) {
						// User has a matching role, add the topics
						for _, topic := range mapping.Topics {
							if !topicSet[topic] {
								topics = append(topics, topic)
								topicSet[topic] = true
							}
						}
						break
					}
				}
			}
		}
	}

	return topics
}

// ImportAWSContact imports a specific user to SES contact list based on their Identity Center group memberships
func ImportAWSContact(sesClient *sesv2.Client, identityCenterId string, userName string, dryRun bool) error {
	fmt.Printf("🔍 Importing AWS contact for user: %s\n", userName)

	// Load Identity Center data from files
	users, memberships, actualId, err := LoadIdentityCenterDataFromFiles(identityCenterId)
	if err != nil {
		return fmt.Errorf("failed to load Identity Center data: %w", err)
	}
	identityCenterId = actualId // Use the actual ID (either provided or auto-detected)

	// Find the specific user
	var targetUser *IdentityCenterUser
	var targetMembership *IdentityCenterGroupMembership

	for _, user := range users {
		if user.UserName == userName {
			targetUser = &user
			break
		}
	}

	if targetUser == nil {
		return fmt.Errorf("user %s not found in Identity Center data", userName)
	}

	// Find user's group membership
	for _, membership := range memberships {
		if membership.UserName == userName {
			targetMembership = &membership
			break
		}
	}

	// Get default configuration
	config := GetDefaultContactImportConfig()

	// Determine topics for this user
	topics := DetermineUserTopics(*targetUser, targetMembership, config)

	if len(topics) == 0 {
		fmt.Printf("⚠️  No topics determined for user %s\n", userName)
		return nil
	}

	fmt.Printf("📋 User %s will be subscribed to topics: %v\n", userName, topics)

	if dryRun {
		fmt.Printf("🔍 DRY RUN: Would add %s (%s) to topics: %v\n", targetUser.DisplayName, targetUser.Email, topics)
		return nil
	}

	// Get account contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Add contact to SES
	err = AddContactToListQuiet(sesClient, accountListName, targetUser.Email, topics)
	if err != nil {
		return fmt.Errorf("failed to add contact %s to SES: %w", targetUser.Email, err)
	}

	fmt.Printf("✅ Successfully imported contact: %s (%s) with topics: %v\n", targetUser.DisplayName, targetUser.Email, topics)
	return nil
}

// ApprovalRequestMetadata represents the metadata from the collector
type ApprovalRequestMetadata struct {
	ChangeMetadata struct {
		Title        string `json:"title"`
		CustomerName string `json:"customerName"`
		Tickets      struct {
			ServiceNow string `json:"serviceNow"`
			Jira       string `json:"jira"`
		} `json:"tickets"`
		ChangeReason           string `json:"changeReason"`
		ImplementationPlan     string `json:"implementationPlan"`
		TestPlan               string `json:"testPlan"`
		ExpectedCustomerImpact string `json:"expectedCustomerImpact"`
		RollbackPlan           string `json:"rollbackPlan"`
		Schedule               struct {
			ImplementationStart string `json:"implementationStart"`
			ImplementationEnd   string `json:"implementationEnd"`
			BeginDate           string `json:"beginDate"`
			BeginTime           string `json:"beginTime"`
			EndDate             string `json:"endDate"`
			EndTime             string `json:"endTime"`
		} `json:"schedule"`
	} `json:"changeMetadata"`
	EmailNotification struct {
		Subject         string `json:"subject"`
		Customer        string `json:"customer"`
		ScheduledWindow struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"scheduledWindow"`
		Tickets struct {
			Snow string `json:"snow"`
			Jira string `json:"jira"`
		} `json:"tickets"`
	} `json:"emailNotification"`
	MeetingInvite *struct {
		Title     string   `json:"title"`
		StartTime string   `json:"startTime"`
		Duration  int      `json:"duration"`
		Attendees []string `json:"attendees"`
		Location  string   `json:"location"`
	} `json:"meetingInvite,omitempty"`
	GeneratedAt string `json:"generatedAt"`
	GeneratedBy string `json:"generatedBy"`
}

// SendCalendarInvite sends a calendar invite with ICS attachment based on metadata
func SendCalendarInvite(sesClient *sesv2.Client, topicName string, jsonMetadataPath string, senderEmail string, dryRun bool) error {
	// Validate required parameters
	if topicName == "" {
		return fmt.Errorf("topic name is required for send-calendar-invite action")
	}
	if jsonMetadataPath == "" {
		return fmt.Errorf("json-metadata file path is required for send-calendar-invite action")
	}
	if senderEmail == "" {
		return fmt.Errorf("sender email is required for send-calendar-invite action")
	}

	// Load metadata from JSON file
	metadata, err := loadApprovalMetadata(jsonMetadataPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Check if meeting information exists
	if metadata.MeetingInvite == nil {
		return fmt.Errorf("no meeting information found in metadata - calendar invite cannot be created")
	}

	// Generate ICS file content
	icsContent, err := generateICSFile(metadata)
	if err != nil {
		return fmt.Errorf("failed to generate ICS file: %w", err)
	}

	// Get account contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get contacts subscribed to the specified topic
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
		Filter: &sesv2Types.ListContactsFilter{
			FilteredStatus: sesv2Types.SubscriptionStatusOptIn,
			TopicFilter: &sesv2Types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	if len(contactsResult.Contacts) == 0 {
		fmt.Printf("⚠️  No contacts are subscribed to topic '%s'\n", topicName)
		return nil
	}

	fmt.Printf("📅 Sending calendar invite to topic '%s' (%d subscribers)\n", topicName, len(contactsResult.Contacts))
	fmt.Printf("📋 Using SES contact list: %s\n", accountListName)
	fmt.Printf("📄 Change: %s\n", metadata.ChangeMetadata.Title)
	fmt.Printf("🕐 Meeting: %s\n", metadata.MeetingInvite.Title)

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No emails will be sent\n")
	}

	// Create email content
	subject := fmt.Sprintf("Calendar Invite: %s", metadata.MeetingInvite.Title)

	// Output raw email message to console for debugging
	fmt.Printf("\n📧 Calendar Invite Preview:\n")
	fmt.Printf("=" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("From: %s\n", senderEmail)
	fmt.Printf("Subject: %s\n", subject)
	fmt.Printf("Contact List: %s\n", accountListName)
	fmt.Printf("Topic: %s\n", topicName)
	fmt.Printf("Content-Type: text/calendar; method=REQUEST\n")
	fmt.Printf("\n--- CALENDAR INVITE (ICS) ---\n")
	// Show sample with first recipient
	sampleIcs := strings.ReplaceAll(icsContent, "%%ATTENDEE_EMAIL%%", *contactsResult.Contacts[0].EmailAddress)
	sampleIcs = strings.ReplaceAll(sampleIcs, "%ATTENDEE_EMAIL%", *contactsResult.Contacts[0].EmailAddress)
	fmt.Printf("%s\n", sampleIcs)
	fmt.Printf("=" + strings.Repeat("=", 60) + "\n\n")

	if dryRun {
		fmt.Printf("📊 Calendar Invite Summary (DRY RUN):\n")
		fmt.Printf("   📧 Would send to: %d recipients\n", len(contactsResult.Contacts))
		fmt.Printf("   📋 Recipients:\n")
		for _, contact := range contactsResult.Contacts {
			fmt.Printf("      - %s\n", *contact.EmailAddress)
		}
		return nil
	}

	successCount := 0
	errorCount := 0

	// Send to each subscribed contact as proper calendar invite
	for _, contact := range contactsResult.Contacts {
		// Generate calendar invite with attendee email
		calendarInvite := strings.ReplaceAll(icsContent, "%%ATTENDEE_EMAIL%%", *contact.EmailAddress)
		calendarInvite = strings.ReplaceAll(calendarInvite, "%ATTENDEE_EMAIL%", *contact.EmailAddress)

		// Generate raw calendar invite email
		rawEmail, err := generateCalendarInviteEmail(
			senderEmail,
			*contact.EmailAddress,
			subject,
			calendarInvite,
		)
		if err != nil {
			fmt.Printf("   ❌ Failed to generate calendar invite for %s: %v\n", *contact.EmailAddress, err)
			errorCount++
			continue
		}

		// Send raw email
		sendRawInput := &sesv2.SendEmailInput{
			FromEmailAddress: aws.String(senderEmail),
			Destination: &sesv2Types.Destination{
				ToAddresses: []string{*contact.EmailAddress},
			},
			Content: &sesv2Types.EmailContent{
				Raw: &sesv2Types.RawMessage{
					Data: []byte(rawEmail),
				},
			},
			ListManagementOptions: &sesv2Types.ListManagementOptions{
				ContactListName: aws.String(accountListName),
				TopicName:       aws.String(topicName),
			},
		}

		_, err = sesClient.SendEmail(context.Background(), sendRawInput)
		if err != nil {
			fmt.Printf("   ❌ Failed to send to %s: %v\n", *contact.EmailAddress, err)
			errorCount++
		} else {
			fmt.Printf("   ✅ Sent to %s\n", *contact.EmailAddress)
			successCount++
		}
	}

	fmt.Printf("\n📊 Calendar Invite Summary:\n")
	fmt.Printf("   ✅ Successful: %d\n", successCount)
	fmt.Printf("   ❌ Errors: %d\n", errorCount)
	fmt.Printf("   📋 Total recipients: %d\n", len(contactsResult.Contacts))

	if errorCount > 0 {
		return fmt.Errorf("failed to send calendar invite to %d recipients", errorCount)
	}

	return nil
}

// generateICSFile creates an ICS calendar file from metadata
func generateICSFile(metadata *ApprovalRequestMetadata) (string, error) {
	if metadata.MeetingInvite == nil {
		return "", fmt.Errorf("no meeting information available")
	}

	// Parse the meeting start time (try multiple formats)
	var startTime time.Time
	var err error

	// Try multiple time formats
	timeFormats := []string{
		time.RFC3339,          // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05", // 2006-01-02T15:04:05
		"2006-01-02T15:04",    // 2006-01-02T15:04
		time.RFC3339Nano,      // 2006-01-02T15:04:05.999999999Z07:00
	}

	for _, format := range timeFormats {
		startTime, err = time.Parse(format, metadata.MeetingInvite.StartTime)
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to parse meeting start time '%s' with any supported format: %w", metadata.MeetingInvite.StartTime, err)
	}

	// Calculate end time
	endTime := startTime.Add(time.Duration(metadata.MeetingInvite.Duration) * time.Minute)

	// Generate unique UID
	uid := fmt.Sprintf("%d@aws-alternate-contact-manager", time.Now().Unix())

	// Create ICS content with proper attendee information
	icsContent := fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//AWS Alternate Contact Manager//Calendar Invite//EN
CALSCALE:GREGORIAN
METHOD:REQUEST
BEGIN:VEVENT
UID:%s
DTSTART:%s
DTEND:%s
DTSTAMP:%s
SUMMARY:%s
DESCRIPTION:Change: %s\n\nCustomer: %s\n\nImplementation Window:\nStart: %s\nEnd: %s\n\nLocation: %s\n\nThis meeting is related to the approved change request.
LOCATION:%s
ORGANIZER:MAILTO:aws-contact-manager@hearst.com
ATTENDEE;ROLE=REQ-PARTICIPANT;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:MAILTO:%%ATTENDEE_EMAIL%%
STATUS:CONFIRMED
SEQUENCE:0
CREATED:%s
LAST-MODIFIED:%s
CLASS:PUBLIC
TRANSP:OPAQUE
END:VEVENT
END:VCALENDAR`,
		uid,
		startTime.UTC().Format("20060102T150405Z"),
		endTime.UTC().Format("20060102T150405Z"),
		time.Now().UTC().Format("20060102T150405Z"),
		metadata.MeetingInvite.Title,
		metadata.ChangeMetadata.Title,
		metadata.ChangeMetadata.CustomerName,
		formatDateTime(metadata.ChangeMetadata.Schedule.ImplementationStart),
		formatDateTime(metadata.ChangeMetadata.Schedule.ImplementationEnd),
		metadata.MeetingInvite.Location,
		metadata.MeetingInvite.Location,
		time.Now().UTC().Format("20060102T150405Z"),
		time.Now().UTC().Format("20060102T150405Z"),
	)

	return icsContent, nil
}

// generateCalendarInviteHTML creates HTML email content for calendar invite
func generateCalendarInviteHTML(metadata *ApprovalRequestMetadata) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Calendar Invite</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #28a745 0%%, #20c997 100%%); color: white; padding: 25px; border-radius: 8px; margin-bottom: 25px; text-align: center; }
        .header h2 { margin: 0 0 10px 0; font-size: 1.8rem; }
        .content { background: #f8f9fa; padding: 20px; border-radius: 8px; border-left: 4px solid #28a745; margin-bottom: 20px; }
        .meeting-details { background: #e7f3ff; padding: 15px; border-radius: 5px; margin: 15px 0; border-left: 4px solid #007bff; }
        .footer { margin-top: 20px; padding-top: 15px; border-top: 1px solid #dee2e6; font-size: 12px; color: #6c757d; text-align: center; }
        .unsubscribe { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px; border-left: 4px solid #6c757d; }
    </style>
</head>
<body>
    <div class="header">
        <h2>✅ Change Approved - Calendar Invite</h2>
        <p><strong>%s</strong></p>
    </div>

    <div class="content">
        <h3>🎉 Good News!</h3>
        <p>The change request "<strong>%s</strong>" has been approved and is ready for implementation.</p>
        <p>You are invited to join the coordination bridge during the implementation window.</p>
    </div>

    <div class="meeting-details">
        <h3>📅 Meeting Details</h3>
        <p><strong>Meeting:</strong> %s</p>
        <p><strong>Location:</strong> %s</p>
        <p><strong>Duration:</strong> %d minutes</p>
        <p><strong>Implementation Window:</strong><br>
           %s to %s</p>
    </div>

    <div class="unsubscribe">
        <h3>📧 Subscription Information</h3>
        <p>You can manage your subscription preferences using the unsubscribe link at the bottom of this email.</p>
    </div>

    <div class="footer">
        <p><strong>AWS Alternate Contact Manager</strong></p>
        <p>Calendar invite attached - please check your calendar application.</p>
        <p><a href="{{amazonSESUnsubscribeUrl}}" style="color: #007bff; text-decoration: none; font-size: 0.9rem;">
            📧 Manage your email preferences or unsubscribe
        </a></p>
    </div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		metadata.ChangeMetadata.Title,
		metadata.MeetingInvite.Title,
		metadata.MeetingInvite.Location,
		metadata.MeetingInvite.Duration,
		formatDateTime(metadata.ChangeMetadata.Schedule.ImplementationStart),
		formatDateTime(metadata.ChangeMetadata.Schedule.ImplementationEnd),
	)
}

// generateCalendarInviteText creates plain text email content for calendar invite
func generateCalendarInviteText(metadata *ApprovalRequestMetadata) string {
	return fmt.Sprintf(`✅ CHANGE APPROVED - CALENDAR INVITE

Good News!

The change request "%s" has been approved and is ready for implementation.

You are invited to join the coordination bridge during the implementation window.

MEETING DETAILS:
Meeting: %s
Location: %s
Duration: %d minutes

Implementation Window:
%s to %s

---
AWS Alternate Contact Manager
Calendar invite attached - please check your calendar application.

You can manage your subscription preferences using the unsubscribe link at the bottom of this email.`,
		metadata.ChangeMetadata.Title,
		metadata.MeetingInvite.Title,
		metadata.MeetingInvite.Location,
		metadata.MeetingInvite.Duration,
		formatDateTime(metadata.ChangeMetadata.Schedule.ImplementationStart),
		formatDateTime(metadata.ChangeMetadata.Schedule.ImplementationEnd),
	)
}

// sanitizeFilename removes invalid characters from filename
func sanitizeFilename(filename string) string {
	// Replace invalid characters with dashes
	sanitized := strings.ReplaceAll(filename, " ", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, "*", "-")
	sanitized = strings.ReplaceAll(sanitized, "?", "-")
	sanitized = strings.ReplaceAll(sanitized, "\"", "-")
	sanitized = strings.ReplaceAll(sanitized, "<", "-")
	sanitized = strings.ReplaceAll(sanitized, ">", "-")
	sanitized = strings.ReplaceAll(sanitized, "|", "-")

	// Remove multiple consecutive dashes
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}

	// Trim dashes from start and end
	sanitized = strings.Trim(sanitized, "-")

	return sanitized
}

// generateRawEmailWithAttachment creates a raw MIME email with ICS attachment
func generateRawEmailWithAttachment(from, to, subject, htmlBody, textBody, icsContent, icsFilename string) (string, error) {
	// Replace attendee email placeholder in ICS content
	icsContent = strings.ReplaceAll(icsContent, "%%ATTENDEE_EMAIL%%", to)
	icsContent = strings.ReplaceAll(icsContent, "%ATTENDEE_EMAIL%", to)
	// Generate boundary for multipart message
	boundary := fmt.Sprintf("boundary_%d", time.Now().Unix())

	// Create raw email with MIME headers
	rawEmail := fmt.Sprintf(`From: %s
To: %s
Subject: %s
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="%s"

--%s
Content-Type: multipart/alternative; boundary="alt_%s"

--alt_%s
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: 7bit

%s

--alt_%s
Content-Type: text/html; charset=UTF-8
Content-Transfer-Encoding: 7bit

%s

--alt_%s
Content-Type: text/calendar; charset=UTF-8; method=REQUEST
Content-Transfer-Encoding: 7bit

%s

--alt_%s--

--%s
Content-Type: text/calendar; charset=UTF-8; method=REQUEST; name="%s"
Content-Disposition: attachment; filename="%s"
Content-Transfer-Encoding: base64

%s

--%s--
`,
		from,
		to,
		subject,
		boundary,
		boundary,
		boundary,
		boundary,
		textBody,
		boundary,
		htmlBody,
		boundary,
		boundary,
		icsFilename,
		base64Encode(icsContent),
		boundary,
	)

	return rawEmail, nil
}

// base64Encode encodes content to base64 with line breaks
func base64Encode(content string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	// Add line breaks every 76 characters (RFC 2045)
	var result strings.Builder
	for i, char := range encoded {
		if i > 0 && i%76 == 0 {
			result.WriteString("\r\n")
		}
		result.WriteRune(char)
	}

	return result.String()
}

// generateCalendarInviteEmail creates a raw email that is primarily a calendar invite
func generateCalendarInviteEmail(from, to, subject, icsContent string) (string, error) {
	// Create raw calendar invite email (primary content is the calendar invite)
	rawEmail := fmt.Sprintf(`From: %s
To: %s
Subject: %s
MIME-Version: 1.0
Content-Type: text/calendar; charset=UTF-8; method=REQUEST
Content-Transfer-Encoding: 7bit

%s`,
		from,
		to,
		subject,
		icsContent,
	)

	return rawEmail, nil
}

// SendApprovalRequest sends an approval request email using metadata and template
func SendApprovalRequest(sesClient *sesv2.Client, topicName string, jsonMetadataPath string, htmlTemplatePath string, senderEmail string, dryRun bool) error {
	// Validate required parameters
	if topicName == "" {
		return fmt.Errorf("topic name is required for send-approval-request action")
	}
	if jsonMetadataPath == "" {
		return fmt.Errorf("json-metadata file path is required for send-approval-request action")
	}
	if senderEmail == "" {
		return fmt.Errorf("sender email is required for send-approval-request action")
	}

	// Load metadata from JSON file
	metadata, err := loadApprovalMetadata(jsonMetadataPath)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Generate or load HTML template
	var htmlContent string
	if htmlTemplatePath != "" {
		htmlContent, err = loadHtmlTemplate(htmlTemplatePath)
		if err != nil {
			return fmt.Errorf("failed to load HTML template: %w", err)
		}
	} else {
		htmlContent = generateDefaultHtmlTemplate(metadata)
	}

	// Process template with metadata
	processedHtml := processTemplate(htmlContent, metadata)

	// Get account contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get contacts subscribed to the specified topic
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
		Filter: &sesv2Types.ListContactsFilter{
			FilteredStatus: sesv2Types.SubscriptionStatusOptIn,
			TopicFilter: &sesv2Types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	if len(contactsResult.Contacts) == 0 {
		fmt.Printf("⚠️  No contacts are subscribed to topic '%s'\n", topicName)
		return nil
	}

	fmt.Printf("📧 Sending approval request to topic '%s' (%d subscribers)\n", topicName, len(contactsResult.Contacts))
	fmt.Printf("📋 Using SES contact list: %s\n", accountListName)
	fmt.Printf("📄 Change: %s\n", metadata.ChangeMetadata.Title)
	fmt.Printf("👤 Customer: %s\n", metadata.ChangeMetadata.CustomerName)

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No emails will be sent\n")
	}

	// Output raw email message to console for debugging
	fmt.Printf("\n📧 Raw Email Message Preview:\n")
	fmt.Printf("=" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("From: %s\n", senderEmail)
	fmt.Printf("Subject: %s\n", metadata.EmailNotification.Subject)
	fmt.Printf("Contact List: %s\n", accountListName)
	fmt.Printf("Topic: %s\n", topicName)
	fmt.Printf("\n--- EMAIL BODY ---\n")
	fmt.Printf("%s\n", processedHtml)
	fmt.Printf("=" + strings.Repeat("=", 60) + "\n\n")

	if dryRun {
		fmt.Printf("📊 Approval Request Summary (DRY RUN):\n")
		fmt.Printf("   📧 Would send to: %d recipients\n", len(contactsResult.Contacts))
		fmt.Printf("   📋 Recipients:\n")
		for _, contact := range contactsResult.Contacts {
			fmt.Printf("      - %s\n", *contact.EmailAddress)
		}
		return nil
	}

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderEmail),
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(metadata.EmailNotification.Subject),
				},
				Body: &sesv2Types.Body{
					Html: &sesv2Types.Content{
						Data: aws.String(processedHtml),
					},
				},
			},
		},
		ListManagementOptions: &sesv2Types.ListManagementOptions{
			ContactListName: aws.String(accountListName),
			TopicName:       aws.String(topicName),
		},
	}

	successCount := 0
	errorCount := 0

	// Send to each subscribed contact
	for _, contact := range contactsResult.Contacts {
		sendInput.Destination.ToAddresses = []string{*contact.EmailAddress}

		_, err := sesClient.SendEmail(context.Background(), sendInput)
		if err != nil {
			fmt.Printf("   ❌ Failed to send to %s: %v\n", *contact.EmailAddress, err)
			errorCount++
		} else {
			fmt.Printf("   ✅ Sent to %s\n", *contact.EmailAddress)
			successCount++
		}
	}

	fmt.Printf("\n📊 Approval Request Summary:\n")
	fmt.Printf("   ✅ Successful: %d\n", successCount)
	fmt.Printf("   ❌ Errors: %d\n", errorCount)
	fmt.Printf("   📋 Total recipients: %d\n", len(contactsResult.Contacts))

	if errorCount > 0 {
		return fmt.Errorf("failed to send approval request to %d recipients", errorCount)
	}

	return nil
}

// loadApprovalMetadata loads and parses the JSON metadata file
func loadApprovalMetadata(filePath string) (*ApprovalRequestMetadata, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file %s: %w", filePath, err)
	}

	var metadata ApprovalRequestMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	return &metadata, nil
}

// loadHtmlTemplate loads an HTML template from file
func loadHtmlTemplate(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read HTML template file %s: %w", filePath, err)
	}
	return string(data), nil
}

// generateDefaultHtmlTemplate creates a default HTML template for approval requests
func generateDefaultHtmlTemplate(metadata *ApprovalRequestMetadata) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Change Approval Request</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background-color: #f8f9fa; padding: 20px; border-radius: 5px; margin-bottom: 20px; border-left: 4px solid #007bff; }
        .section { margin-bottom: 25px; }
        .section h3 { color: #007bff; margin-bottom: 10px; border-bottom: 2px solid #e9ecef; padding-bottom: 5px; }
        .info-grid { display: grid; grid-template-columns: 150px 1fr; gap: 10px; margin-bottom: 15px; }
        .info-label { font-weight: bold; color: #495057; }
        .schedule { background-color: #e7f3ff; padding: 15px; border-radius: 5px; margin: 15px 0; }
        .tickets { background-color: #f8f9fa; padding: 10px; border-radius: 5px; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #dee2e6; font-size: 12px; color: #6c757d; }
        .unsubscribe { background-color: #e9ecef; padding: 15px; border-radius: 5px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="header">
        <h2>🔔 Change Approval Request</h2>
        <p><strong>%s</strong></p>
        <p>Customer: %s</p>
    </div>

    <div class="section">
        <h3>📋 Change Details</h3>
        <div class="info-grid">
            <div class="info-label">Title:</div>
            <div>%s</div>
            <div class="info-label">Customer:</div>
            <div>%s</div>
        </div>
        
        <div class="tickets">
            <strong>Tracking Numbers:</strong><br>
            ServiceNow: %s<br>
            JIRA: %s
        </div>
    </div>

    <div class="section">
        <h3>📅 Implementation Schedule</h3>
        <div class="schedule">
            <strong>Start:</strong> %s<br>
            <strong>End:</strong> %s
        </div>
    </div>

    <div class="section">
        <h3>❓ Why This Change?</h3>
        <div>%s</div>
    </div>

    <div class="section">
        <h3>🔧 Implementation Plan</h3>
        <div>%s</div>
    </div>

    <div class="section">
        <h3>🧪 Test Plan</h3>
        <div>%s</div>
    </div>

    <div class="section">
        <h3>👥 Expected Customer Impact</h3>
        <div>%s</div>
    </div>

    <div class="section">
        <h3>🔄 Rollback Plan</h3>
        <div>%s</div>
    </div>

    <div class="unsubscribe">
        <h3>📧 Subscription Information</h3>
        <p>You are receiving this approval request because you are subscribed to change notifications.</p>
        <p>You can manage your subscription preferences using the unsubscribe link at the bottom of this email.</p>
        <p>If you have questions about this change, please contact the change requestor or your system administrator.</p>
    </div>

    <div class="footer">
        <p>This approval request was generated automatically from the AWS Alternate Contact Manager.</p>
        <p>Generated at: %s</p>
        <p><a href="{{amazonSESUnsubscribeUrl}}" style="color: #007bff; text-decoration: none;">Unsubscribe from these notifications</a></p>
    </div>
</body>
</html>`,
		metadata.ChangeMetadata.Title,
		metadata.ChangeMetadata.CustomerName,
		metadata.ChangeMetadata.Title,
		metadata.ChangeMetadata.CustomerName,
		getValueOrDefault(metadata.ChangeMetadata.Tickets.ServiceNow, "Not specified"),
		getValueOrDefault(metadata.ChangeMetadata.Tickets.Jira, "Not specified"),
		formatDateTime(metadata.ChangeMetadata.Schedule.ImplementationStart),
		formatDateTime(metadata.ChangeMetadata.Schedule.ImplementationEnd),
		convertTextToHtml(metadata.ChangeMetadata.ChangeReason),
		convertTextToHtml(metadata.ChangeMetadata.ImplementationPlan),
		convertTextToHtml(metadata.ChangeMetadata.TestPlan),
		convertTextToHtml(metadata.ChangeMetadata.ExpectedCustomerImpact),
		convertTextToHtml(metadata.ChangeMetadata.RollbackPlan),
		metadata.GeneratedAt,
	)
}

// processTemplate processes template placeholders with metadata values
func processTemplate(template string, metadata *ApprovalRequestMetadata) string {
	// Simple template processing - replace common placeholders
	processed := template
	processed = strings.ReplaceAll(processed, "{{CHANGE_TITLE}}", metadata.ChangeMetadata.Title)
	processed = strings.ReplaceAll(processed, "{{CUSTOMER_NAME}}", metadata.ChangeMetadata.CustomerName)
	processed = strings.ReplaceAll(processed, "{{CHANGE_REASON}}", convertTextToHtml(metadata.ChangeMetadata.ChangeReason))
	processed = strings.ReplaceAll(processed, "{{IMPLEMENTATION_PLAN}}", convertTextToHtml(metadata.ChangeMetadata.ImplementationPlan))
	processed = strings.ReplaceAll(processed, "{{TEST_PLAN}}", convertTextToHtml(metadata.ChangeMetadata.TestPlan))
	processed = strings.ReplaceAll(processed, "{{CUSTOMER_IMPACT}}", convertTextToHtml(metadata.ChangeMetadata.ExpectedCustomerImpact))
	processed = strings.ReplaceAll(processed, "{{ROLLBACK_PLAN}}", convertTextToHtml(metadata.ChangeMetadata.RollbackPlan))
	processed = strings.ReplaceAll(processed, "{{IMPLEMENTATION_START}}", formatDateTime(metadata.ChangeMetadata.Schedule.ImplementationStart))
	processed = strings.ReplaceAll(processed, "{{IMPLEMENTATION_END}}", formatDateTime(metadata.ChangeMetadata.Schedule.ImplementationEnd))
	processed = strings.ReplaceAll(processed, "{{SNOW_TICKET}}", getValueOrDefault(metadata.ChangeMetadata.Tickets.ServiceNow, "Not specified"))
	processed = strings.ReplaceAll(processed, "{{JIRA_TICKET}}", getValueOrDefault(metadata.ChangeMetadata.Tickets.Jira, "Not specified"))
	processed = strings.ReplaceAll(processed, "{{GENERATED_AT}}", metadata.GeneratedAt)

	return processed
}

// Helper functions
func getValueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// convertTextToHtml converts plain text with line breaks to HTML format
func convertTextToHtml(text string) string {
	if text == "" {
		return ""
	}

	// Replace double line breaks with paragraph breaks
	text = strings.ReplaceAll(text, "\n\n", "</p><p>")

	// Replace single line breaks with <br> tags
	text = strings.ReplaceAll(text, "\n", "<br>")

	// Wrap in paragraph tags if not empty
	if strings.TrimSpace(text) != "" {
		text = "<p>" + text + "</p>"
	}

	return text
}

func formatDateTime(dateTimeStr string) string {
	if dateTimeStr == "" {
		return "Not specified"
	}

	// Try to parse and format the datetime
	if t, err := time.Parse(time.RFC3339, dateTimeStr); err == nil {
		return t.Format("January 2, 2006 at 3:04 PM MST")
	}

	// If parsing fails, return as-is
	return dateTimeStr
}

// SubscriptionConfig represents the subscription configuration file structure
type SubscriptionConfig map[string][]string

// LoadSubscriptionConfig loads the subscription configuration from a JSON file
func LoadSubscriptionConfig(configPath string) (SubscriptionConfig, error) {
	configJson, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read subscription config file %s: %w", configPath, err)
	}

	var config SubscriptionConfig
	err = json.Unmarshal(configJson, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subscription config: %w", err)
	}

	return config, nil
}

// ManageSubscriptions handles bulk subscription/unsubscription operations
func ManageSubscriptions(sesClient *sesv2.Client, configPath string, subscribe bool, dryRun bool) error {
	// Load subscription configuration
	config, err := LoadSubscriptionConfig(configPath)
	if err != nil {
		return err
	}

	// Get account contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	action := "subscribe"
	if !subscribe {
		action = "unsubscribe"
	}

	fmt.Printf("📧 %s operation using config: %s\n", strings.Title(action), configPath)
	fmt.Printf("📋 Using SES contact list: %s\n", accountListName)

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No changes will be made\n")
	}

	// Get existing contacts for validation
	existingContacts, err := getExistingContacts(sesClient, accountListName)
	if err != nil {
		return fmt.Errorf("failed to get existing contacts: %w", err)
	}

	totalOperations := 0
	successCount := 0
	errorCount := 0
	skippedCount := 0

	// Process each topic and its subscribers
	for topicName, emails := range config {
		fmt.Printf("\n🏷️  Processing topic: %s (%d emails)\n", topicName, len(emails))

		for _, email := range emails {
			totalOperations++

			// Check if contact exists
			existingTopics, contactExists := existingContacts[email]
			if !contactExists {
				fmt.Printf("   ⚠️  Contact %s does not exist in contact list, skipping\n", email)
				skippedCount++
				continue
			}

			// Check current subscription status
			isCurrentlySubscribed := false
			for _, topic := range existingTopics {
				if topic == topicName {
					isCurrentlySubscribed = true
					break
				}
			}

			// Determine if action is needed
			actionNeeded := false
			if subscribe && !isCurrentlySubscribed {
				actionNeeded = true
			} else if !subscribe && isCurrentlySubscribed {
				actionNeeded = true
			}

			if !actionNeeded {
				status := "subscribed"
				if !isCurrentlySubscribed {
					status = "unsubscribed"
				}
				fmt.Printf("   ⏭️  %s already %s to %s, skipping\n", email, status, topicName)
				skippedCount++
				continue
			}

			if dryRun {
				if subscribe {
					fmt.Printf("   🔍 Would subscribe %s to %s\n", email, topicName)
				} else {
					fmt.Printf("   🔍 Would unsubscribe %s from %s\n", email, topicName)
				}
				successCount++
				continue
			}

			// Perform the actual subscription/unsubscription
			if subscribe {
				// Add topic to existing subscriptions
				newTopics := append(existingTopics, topicName)
				err = updateContactSubscription(sesClient, accountListName, email, newTopics)
				if err != nil {
					fmt.Printf("   ❌ Failed to subscribe %s to %s: %v\n", email, topicName, err)
					errorCount++
				} else {
					fmt.Printf("   ✅ Subscribed %s to %s\n", email, topicName)
					successCount++
					// Update local cache
					existingContacts[email] = newTopics
				}
			} else {
				// Remove topic from existing subscriptions
				var newTopics []string
				for _, topic := range existingTopics {
					if topic != topicName {
						newTopics = append(newTopics, topic)
					}
				}
				err = updateContactSubscription(sesClient, accountListName, email, newTopics)
				if err != nil {
					fmt.Printf("   ❌ Failed to unsubscribe %s from %s: %v\n", email, topicName, err)
					errorCount++
				} else {
					fmt.Printf("   ✅ Unsubscribed %s from %s\n", email, topicName)
					successCount++
					// Update local cache
					existingContacts[email] = newTopics
				}
			}
		}
	}

	fmt.Printf("\n📊 %s Summary:\n", strings.Title(action))
	fmt.Printf("   ✅ Successful: %d\n", successCount)
	fmt.Printf("   ❌ Errors: %d\n", errorCount)
	fmt.Printf("   ⏭️  Skipped: %d\n", skippedCount)
	fmt.Printf("   📋 Total processed: %d\n", totalOperations)

	if errorCount > 0 {
		return fmt.Errorf("failed to %s %d contacts", action, errorCount)
	}

	return nil
}

// updateContactSubscription updates a contact's topic subscriptions
func updateContactSubscription(sesClient *sesv2.Client, listName string, email string, topics []string) error {
	// Remove the contact first
	err := RemoveContactFromList(sesClient, listName, email)
	if err != nil {
		return fmt.Errorf("failed to remove contact before updating: %w", err)
	}

	// Add the contact back with new topic subscriptions
	if len(topics) > 0 {
		err = AddContactToListQuiet(sesClient, listName, email, topics)
		if err != nil {
			return fmt.Errorf("failed to re-add contact with updated subscriptions: %w", err)
		}
	}

	return nil
}

// SendTopicTestEmail sends a test email to a specific topic
func SendTopicTestEmail(sesClient *sesv2.Client, topicName string, senderEmail string) error {
	// Get account contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get contact list details to verify topic exists
	listInput := &sesv2.GetContactListInput{
		ContactListName: aws.String(accountListName),
	}

	listResult, err := sesClient.GetContactList(context.Background(), listInput)
	if err != nil {
		return fmt.Errorf("failed to get contact list details: %w", err)
	}

	// Verify topic exists
	topicExists := false
	for _, topic := range listResult.Topics {
		if *topic.TopicName == topicName {
			topicExists = true
			break
		}
	}

	if !topicExists {
		return fmt.Errorf("topic '%s' not found in contact list '%s'", topicName, accountListName)
	}

	// Get contacts subscribed to this topic
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
		Filter: &sesv2Types.ListContactsFilter{
			FilteredStatus: sesv2Types.SubscriptionStatusOptIn,
			TopicFilter: &sesv2Types.TopicFilter{
				TopicName: aws.String(topicName),
			},
		},
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	if len(contactsResult.Contacts) == 0 {
		fmt.Printf("⚠️  No contacts are subscribed to topic '%s'\n", topicName)
		return nil
	}

	fmt.Printf("📧 Sending test email to topic '%s' (%d subscribers)\n", topicName, len(contactsResult.Contacts))

	// Create simple test email content (text-only)
	subject := fmt.Sprintf("Test Email for Topic: %s", topicName)
	textBody := fmt.Sprintf(`Test Email for Topic: %s

This is a test email to verify that the topic subscription is working correctly.

Topic: %s
Sent: %s
Contact List: %s

You are receiving this email because you are subscribed to the "%s" topic.

AWS SES unsubscribe link:
{{amazonSESUnsubscribeUrl}}
`, topicName, topicName, time.Now().Format("2006-01-02 15:04:05"), accountListName, topicName)

	// Validate sender email
	if senderEmail == "" {
		return fmt.Errorf("sender email is required for sending test emails (use -sender-email parameter)")
	}

	// Send email using SES v2 SendEmail API
	sendInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderEmail),
		Destination: &sesv2Types.Destination{
			ToAddresses: []string{}, // Will be populated per contact
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(subject),
				},
				Body: &sesv2Types.Body{
					Text: &sesv2Types.Content{
						Data: aws.String(textBody),
					},
				},
			},
		},
		ListManagementOptions: &sesv2Types.ListManagementOptions{
			ContactListName: aws.String(accountListName),
			TopicName:       aws.String(topicName),
		},
	}

	// Output raw email message to console for debugging
	fmt.Printf("\n📧 Raw Email Message Preview:\n")
	fmt.Printf("=" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("From: %s\n", senderEmail)
	fmt.Printf("Subject: %s\n", subject)
	fmt.Printf("Contact List: %s\n", accountListName)
	fmt.Printf("Topic: %s\n", topicName)
	fmt.Printf("\n--- EMAIL BODY ---\n")
	fmt.Printf("%s\n", textBody)
	fmt.Printf("=" + strings.Repeat("=", 60) + "\n\n")

	successCount := 0
	errorCount := 0

	// Send to each subscribed
	for _, contact := range contactsResult.Contacts {
		sendInput.Destination.ToAddresses = []string{*contact.EmailAddress}

		_, err := sesClient.SendEmail(context.Background(), sendInput)
		if err != nil {
			fmt.Printf("   ❌ Failed to send to %s: %v\n", *contact.EmailAddress, err)
			errorCount++
		} else {
			fmt.Printf("   ✅ Sent to %s\n", *contact.EmailAddress)
			successCount++
		}
	}

	fmt.Printf("\n📊 Test Email Summary:\n")
	fmt.Printf("   ✅ Successful: %d\n", successCount)
	fmt.Printf("   ❌ Errors: %d\n", errorCount)
	fmt.Printf("   📋 Total recipients: %d\n", len(contactsResult.Contacts))

	if errorCount > 0 {
		return fmt.Errorf("failed to send test email to %d recipients", errorCount)
	}

	return nil
}

// getExistingContacts returns a map of existing contacts and their topic subscriptions
func getExistingContacts(sesClient *sesv2.Client, listName string) (map[string][]string, error) {
	input := &sesv2.ListContactsInput{
		ContactListName: aws.String(listName),
	}

	result, err := sesClient.ListContacts(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing contacts: %w", err)
	}

	existingContacts := make(map[string][]string)
	for _, contact := range result.Contacts {
		email := *contact.EmailAddress
		var topics []string

		// Extract subscribed topics
		for _, pref := range contact.TopicPreferences {
			if pref.SubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
				topics = append(topics, *pref.TopicName)
			}
		}

		existingContacts[email] = topics
	}

	return existingContacts, nil
}

// slicesEqual compares two string slices for equality
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// ImportAllAWSContacts imports all users to SES contact list based on their Identity Center group memberships
func ImportAllAWSContacts(sesClient *sesv2.Client, identityCenterId string, dryRun bool) error {
	fmt.Printf("🔍 Importing all AWS contacts from Identity Center\n")

	// Load Identity Center data from files
	users, memberships, actualId, err := LoadIdentityCenterDataFromFiles(identityCenterId)
	if err != nil {
		return fmt.Errorf("failed to load Identity Center data: %w", err)
	}
	identityCenterId = actualId // Use the actual ID (either provided or auto-detected)

	// Create membership lookup map
	membershipMap := make(map[string]*IdentityCenterGroupMembership)
	for i, membership := range memberships {
		membershipMap[membership.UserName] = &memberships[i]
	}

	// Get default configuration
	config := GetDefaultContactImportConfig()

	// Get account contact list
	var accountListName string
	var existingContacts map[string][]string

	if !dryRun {
		accountListName, err = GetAccountContactList(sesClient)
		if err != nil {
			return fmt.Errorf("failed to get account contact list: %w", err)
		}
		fmt.Printf("📋 Using SES contact list: %s\n", accountListName)

		// Validate that required topics exist in the contact list
		err = validateContactListTopics(sesClient, accountListName, config)
		if err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
		}

		// Get existing contacts for idempotent operation
		fmt.Printf("📋 Checking existing contacts...\n")
		var err error
		existingContacts, err = getExistingContacts(sesClient, accountListName)
		if err != nil {
			return fmt.Errorf("failed to get existing contacts: %w", err)
		}
		fmt.Printf("📋 Found %d existing contacts\n", len(existingContacts))
	} else {
		// In dry-run mode, we can't check existing contacts
		existingContacts = make(map[string][]string)
	}

	// Process each user
	successCount := 0
	errorCount := 0
	skippedCount := 0
	updatedCount := 0

	fmt.Printf("👥 Processing %d users...\n", len(users))

	// Show sample of first few users for debugging
	if len(users) > 0 {
		fmt.Printf("🔍 Sample users:\n")
		sampleCount := 3
		if len(users) < sampleCount {
			sampleCount = len(users)
		}
		for i := 0; i < sampleCount; i++ {
			user := users[i]
			membership := membershipMap[user.UserName]
			topics := DetermineUserTopics(user, membership, config)
			fmt.Printf("   - %s (%s) → topics: %v\n", user.UserName, user.Email, topics)
		}
		fmt.Println()
	}

	for _, user := range users {
		// Skip inactive users if required
		if config.RequireActiveUsers && !user.Active {
			skippedCount++
			continue
		}

		// Get user's membership
		membership := membershipMap[user.UserName]

		// Determine topics
		topics := DetermineUserTopics(user, membership, config)

		// Skip users with no topics or only empty topics
		hasValidTopics := false
		for _, topic := range topics {
			if strings.TrimSpace(topic) != "" {
				hasValidTopics = true
				break
			}
		}

		if !hasValidTopics {
			skippedCount++
			continue
		}

		if dryRun {
			successCount++
			continue
		}

		// Check if contact already exists with same topics (idempotent operation)
		if existingTopics, exists := existingContacts[user.Email]; exists {
			// Sort both slices for comparison
			sort.Strings(topics)
			sort.Strings(existingTopics)

			// Compare topics
			if slicesEqual(topics, existingTopics) {
				// Contact already exists with same topics, skip
				skippedCount++
				continue
			} else {
				// Contact exists but with different topics, need to update
				fmt.Printf("   🔄 Updating contact %s (topics changed)\n", user.Email)
				// Remove existing contact first
				err = RemoveContactFromList(sesClient, accountListName, user.Email)
				if err != nil {
					fmt.Printf("   ❌ Failed to remove existing contact %s: %v\n", user.Email, err)
					errorCount++
					continue
				}
				updatedCount++
			}
		}

		// Add contact to SES
		err = AddContactToListQuiet(sesClient, accountListName, user.Email, topics)
		if err != nil {
			// Log first few errors for debugging
			if errorCount < 3 {
				fmt.Printf("   ❌ Failed to add contact %s: %v\n", user.Email, err)
			}
			errorCount++
			continue
		}

		successCount++
	}

	fmt.Printf("\n📊 Import Summary:\n")
	fmt.Printf("   ✅ Successful: %d\n", successCount)
	fmt.Printf("   🔄 Updated: %d\n", updatedCount)
	fmt.Printf("   ❌ Errors: %d\n", errorCount)
	fmt.Printf("   ⏭️  Skipped: %d\n", skippedCount)
	fmt.Printf("   📋 Total processed: %d\n", len(users))

	if errorCount > 0 {
		return fmt.Errorf("failed to import %d contacts", errorCount)
	}

	return nil
}

// DisplayCCOECloudGroups displays parsed CCOE cloud groups in a formatted table
func DisplayCCOECloudGroups(ccoeGroups []CCOECloudGroupInfo) {
	if len(ccoeGroups) == 0 {
		fmt.Println("No CCOE cloud groups found.")
		return
	}

	fmt.Printf("\n📊 CCOE Cloud Groups (%d total)\n", len(ccoeGroups))
	fmt.Println(strings.Repeat("=", 120))
	fmt.Printf("%-25s %-15s %-20s %-15s %s\n", "Account Name", "Account ID", "App Prefix", "Role Name", "Group Name")
	fmt.Println(strings.Repeat("-", 120))

	for _, group := range ccoeGroups {
		// Truncate long fields for table display
		accountName := group.AccountName
		if len(accountName) > 23 {
			accountName = accountName[:20] + "..."
		}

		appPrefix := group.ApplicationPrefix
		if len(appPrefix) > 18 {
			appPrefix = appPrefix[:15] + "..."
		}

		roleName := group.RoleName
		if len(roleName) > 13 {
			roleName = roleName[:10] + "..."
		}

		fmt.Printf("%-25s %-15s %-20s %-15s %s\n",
			accountName, group.AccountId, appPrefix, roleName, group.GroupName)
	}

	fmt.Println(strings.Repeat("=", 120))
	fmt.Printf("Total: %d CCOE cloud groups\n", len(ccoeGroups))
}

// SaveGroupCentricToJSON saves group-centric data to a JSON file
func SaveGroupCentricToJSON(groupCentric []IdentityCenterGroupCentric, filename string) error {
	jsonData, err := json.MarshalIndent(groupCentric, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal group-centric data to JSON: %w", err)
	}

	configPath := GetConfigPath()
	fullPath := configPath + filename

	err = os.WriteFile(fullPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file %s: %w", fullPath, err)
	}

	fmt.Printf("📁 Group-centric data saved to: %s\n", filename)
	return nil
}

// SaveGroupMembershipToJSON saves single user group membership to a JSON file
func SaveGroupMembershipToJSON(membership *IdentityCenterGroupMembership, filename string) error {
	jsonData, err := json.MarshalIndent(membership, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal group membership to JSON: %w", err)
	}

	configPath := GetConfigPath()
	fullPath := configPath + filename

	err = os.WriteFile(fullPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file %s: %w", fullPath, err)
	}

	fmt.Printf("📁 Group membership saved to: %s\n", filename)
	return nil
}

// DisplayIdentityCenterUser displays a single user in a formatted way
func DisplayIdentityCenterUser(user *IdentityCenterUser) {
	fmt.Printf("👤 User: %s\n", user.DisplayName)
	fmt.Printf("   User ID: %s\n", user.UserId)
	fmt.Printf("   Username: %s\n", user.UserName)
	fmt.Printf("   Email: %s\n", user.Email)
	fmt.Printf("   Given Name: %s\n", user.GivenName)
	fmt.Printf("   Family Name: %s\n", user.FamilyName)
	fmt.Printf("   Active: %t\n", user.Active)
	fmt.Println()
}

// IdentityCenterGroupMembership represents a user's group membership
type IdentityCenterGroupMembership struct {
	UserId      string   `json:"user_id"`
	UserName    string   `json:"user_name"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Groups      []string `json:"groups"`
}

// IdentityCenterGroupCentric represents groups with their members
type IdentityCenterGroupCentric struct {
	GroupName string                   `json:"group_name"`
	Members   []IdentityCenterUserInfo `json:"members"`
}

// IdentityCenterUserInfo represents user info for group membership
type IdentityCenterUserInfo struct {
	UserId      string `json:"user_id"`
	UserName    string `json:"user_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

// CCOECloudGroupInfo represents parsed information from ccoe-cloud group names
type CCOECloudGroupInfo struct {
	GroupName         string `json:"group_name"`
	AccountName       string `json:"account_name"`
	AccountId         string `json:"account_id"`
	ApplicationPrefix string `json:"application_prefix"`
	RoleName          string `json:"role_name"`
	IsValid           bool   `json:"is_valid"`
}

// RoleTopicMapping defines which roles should be subscribed to which topics
type RoleTopicMapping struct {
	Roles  []string `json:"roles"`
	Topics []string `json:"topics"`
}

// ContactImportConfig defines the mapping configuration for importing contacts
type ContactImportConfig struct {
	RoleMappings       []RoleTopicMapping `json:"role_mappings"`
	DefaultTopics      []string           `json:"default_topics"`
	RequireActiveUsers bool               `json:"require_active_users"`
}

// ListUserGroupMembership gets group memberships for a specific user
func ListUserGroupMembership(identityStoreClient *identitystore.Client, identityStoreId string, userName string) (*IdentityCenterGroupMembership, error) {
	// First get the user details
	user, err := ListIdentityCenterUser(identityStoreClient, identityStoreId, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to get user details: %w", err)
	}

	// Get group memberships for the user
	input := &identitystore.ListGroupMembershipsForMemberInput{
		IdentityStoreId: aws.String(identityStoreId),
		MemberId: &identitystoreTypes.MemberIdMemberUserId{
			Value: user.UserId,
		},
	}

	var allGroups []string
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		result, err := identityStoreClient.ListGroupMembershipsForMember(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to list group memberships for user %s: %w", userName, err)
		}

		// Get group details for each membership
		for _, membership := range result.GroupMemberships {
			groupInput := &identitystore.DescribeGroupInput{
				IdentityStoreId: aws.String(identityStoreId),
				GroupId:         membership.GroupId,
			}

			groupResult, err := identityStoreClient.DescribeGroup(context.Background(), groupInput)
			if err != nil {
				fmt.Printf("Warning: Failed to get details for group %s: %v\n", *membership.GroupId, err)
				allGroups = append(allGroups, *membership.GroupId) // Use GroupId as fallback
			} else {
				allGroups = append(allGroups, *groupResult.DisplayName)
			}
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	membership := &IdentityCenterGroupMembership{
		UserId:      user.UserId,
		UserName:    user.UserName,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Groups:      allGroups,
	}

	return membership, nil
}

// ListAllUserGroupMemberships gets group memberships for all users with concurrency and rate limiting
func ListAllUserGroupMemberships(identityStoreClient *identitystore.Client, identityStoreId string, maxConcurrency int, requestsPerSecond int) ([]IdentityCenterGroupMembership, error) {
	fmt.Printf("🔍 Getting all users and their group memberships from Identity Center (ID: %s)\n", identityStoreId)
	fmt.Printf("⚙️  Concurrency: %d workers, Rate limit: %d req/sec\n", maxConcurrency, requestsPerSecond)

	// First get all users
	users, err := ListIdentityCenterUsersAll(identityStoreClient, identityStoreId, maxConcurrency, requestsPerSecond)
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}

	fmt.Printf("👥 Found %d users, now getting group memberships...\n", len(users))

	// Create rate limiter for group membership operations
	rateLimiter := NewRateLimiter(requestsPerSecond)
	defer rateLimiter.Stop()

	// Process users with concurrency to get their group memberships
	var wg sync.WaitGroup
	userChan := make(chan IdentityCenterUser, len(users))
	resultChan := make(chan IdentityCenterGroupMembership, len(users))
	errorChan := make(chan error, len(users))

	// Start worker goroutines
	for i := 0; i < maxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for user := range userChan {
				rateLimiter.Wait() // Rate limit each operation

				membership, err := getUserGroupMembershipFromUser(identityStoreClient, identityStoreId, user, rateLimiter)
				if err != nil {
					errorChan <- fmt.Errorf("failed to get group membership for user %s: %w", user.UserName, err)
					continue
				}
				resultChan <- *membership
			}
		}()
	}

	// Send users to workers
	for _, user := range users {
		userChan <- user
	}
	close(userChan)

	// Wait for all workers to complete
	wg.Wait()
	close(resultChan)
	close(errorChan)

	// Collect results and errors
	var memberships []IdentityCenterGroupMembership
	var errors []error

	for membership := range resultChan {
		memberships = append(memberships, membership)
	}

	for err := range errorChan {
		errors = append(errors, err)
	}

	// Report errors but continue with successful results
	if len(errors) > 0 {
		fmt.Printf("⚠️  Encountered %d errors while processing group memberships:\n", len(errors))
		for i, err := range errors {
			if i < 5 { // Show first 5 errors
				fmt.Printf("   - %v\n", err)
			}
		}
		if len(errors) > 5 {
			fmt.Printf("   ... and %d more errors\n", len(errors)-5)
		}
	}

	fmt.Printf("✅ Successfully retrieved group memberships for %d users\n", len(memberships))
	return memberships, nil
}

// getUserGroupMembershipFromUser gets group membership for a single user (helper function)
func getUserGroupMembershipFromUser(identityStoreClient *identitystore.Client, identityStoreId string, user IdentityCenterUser, rateLimiter *RateLimiter) (*IdentityCenterGroupMembership, error) {
	// Get group memberships for the user
	input := &identitystore.ListGroupMembershipsForMemberInput{
		IdentityStoreId: aws.String(identityStoreId),
		MemberId: &identitystoreTypes.MemberIdMemberUserId{
			Value: user.UserId,
		},
	}

	var allGroups []string
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		rateLimiter.Wait() // Rate limit API calls
		result, err := identityStoreClient.ListGroupMembershipsForMember(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to list group memberships: %w", err)
		}

		// Get group details for each membership
		for _, membership := range result.GroupMemberships {
			rateLimiter.Wait() // Rate limit group detail calls

			groupInput := &identitystore.DescribeGroupInput{
				IdentityStoreId: aws.String(identityStoreId),
				GroupId:         membership.GroupId,
			}

			groupResult, err := identityStoreClient.DescribeGroup(context.Background(), groupInput)
			if err != nil {
				// Use GroupId as fallback if we can't get group details
				allGroups = append(allGroups, *membership.GroupId)
			} else {
				allGroups = append(allGroups, *groupResult.DisplayName)
			}
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	membership := &IdentityCenterGroupMembership{
		UserId:      user.UserId,
		UserName:    user.UserName,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Groups:      allGroups,
	}

	return membership, nil
}

// DisplayUserGroupMembership displays a single user's group membership
func DisplayUserGroupMembership(membership *IdentityCenterGroupMembership) {
	fmt.Printf("👤 User: %s\n", membership.DisplayName)
	fmt.Printf("   User ID: %s\n", membership.UserId)
	fmt.Printf("   Username: %s\n", membership.UserName)
	fmt.Printf("   Email: %s\n", membership.Email)
	fmt.Printf("   Groups (%d):\n", len(membership.Groups))

	if len(membership.Groups) == 0 {
		fmt.Printf("     (No group memberships)\n")
	} else {
		for _, group := range membership.Groups {
			fmt.Printf("     - %s\n", group)
		}
	}
	fmt.Println()
}

// DisplayAllUserGroupMemberships displays multiple users' group memberships in a formatted table
func DisplayAllUserGroupMemberships(memberships []IdentityCenterGroupMembership) {
	if len(memberships) == 0 {
		fmt.Println("No user group memberships found.")
		return
	}

	fmt.Printf("\n📊 Identity Center User Group Memberships (%d users)\n", len(memberships))
	fmt.Println(strings.Repeat("=", 100))
	fmt.Printf("%-20s %-30s %-40s %-8s\n", "Username", "Display Name", "Email", "Groups")
	fmt.Println(strings.Repeat("-", 100))

	for _, membership := range memberships {
		// Truncate long fields for table display
		username := membership.UserName
		if len(username) > 18 {
			username = username[:15] + "..."
		}

		displayName := membership.DisplayName
		if len(displayName) > 28 {
			displayName = displayName[:25] + "..."
		}

		email := membership.Email
		if len(email) > 38 {
			email = email[:35] + "..."
		}

		groupCount := fmt.Sprintf("%d", len(membership.Groups))

		fmt.Printf("%-20s %-30s %-40s %-8s\n", username, displayName, email, groupCount)

		// Show groups indented
		if len(membership.Groups) > 0 {
			for i, group := range membership.Groups {
				if i < 3 { // Show first 3 groups
					fmt.Printf("%-20s   └─ %s\n", "", group)
				} else if i == 3 && len(membership.Groups) > 3 {
					fmt.Printf("%-20s   └─ ... and %d more groups\n", "", len(membership.Groups)-3)
					break
				}
			}
		} else {
			fmt.Printf("%-20s   └─ (No groups)\n", "")
		}
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 100))
	fmt.Printf("Total: %d users\n", len(memberships))
}

// DisplayIdentityCenterUsers displays multiple users in a formatted table
func DisplayIdentityCenterUsers(users []IdentityCenterUser) {
	if len(users) == 0 {
		fmt.Println("No users found.")
		return
	}

	fmt.Printf("\n📊 Identity Center Users (%d total)\n", len(users))
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("%-20s %-30s %-40s %-8s\n", "Username", "Display Name", "Email", "Active")
	fmt.Println(strings.Repeat("-", 80))

	for _, user := range users {
		activeStatus := "✅"
		if !user.Active {
			activeStatus = "❌"
		}

		// Truncate long fields for table display
		username := user.UserName
		if len(username) > 18 {
			username = username[:15] + "..."
		}

		displayName := user.DisplayName
		if len(displayName) > 28 {
			displayName = displayName[:25] + "..."
		}

		email := user.Email
		if len(email) > 38 {
			email = email[:35] + "..."
		}

		fmt.Printf("%-20s %-30s %-40s %-8s\n", username, displayName, email, activeStatus)
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Total: %d users\n", len(users))
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
	fmt.Println("  send-topic-test      Send test email to specific topic subscribers")
	fmt.Println("                       • Required: -topic-name topic-name")
	fmt.Println("                       • Required: -sender-email verified@domain.com")
	fmt.Println("                       • Sends test email to all subscribed contacts")
	fmt.Println()
	fmt.Println("  manage-topic         Update contact list topics (creates backup)")
	fmt.Println("                       • Uses: -ses-config-file SESConfig.json")
	fmt.Println("                       • Optional: -dry-run (preview changes)")
	fmt.Println()
	fmt.Println("  subscribe            Subscribe contacts to topics based on config")
	fmt.Println("                       • Uses: -subscription-config SubscriptionConfig.json")
	fmt.Println("                       • Optional: -dry-run (preview changes)")
	fmt.Println()
	fmt.Println("  unsubscribe          Unsubscribe contacts from topics based on config")
	fmt.Println("                       • Uses: -subscription-config SubscriptionConfig.json")
	fmt.Println("                       • Optional: -dry-run (preview changes)")
	fmt.Println()
	fmt.Println("  send-approval-request Send approval request email to topic subscribers")
	fmt.Println("                       • Required: -topic-name topic-name")
	fmt.Println("                       • Required: -json-metadata metadata.json")
	fmt.Println("                       • Required: -sender-email verified@domain.com")
	fmt.Println("                       • Optional: -html-template template.html")
	fmt.Println("                       • Optional: -dry-run (preview email)")
	fmt.Println()
	fmt.Println("  send-calendar-invite Send calendar invite with ICS attachment")
	fmt.Println("                       • Required: -topic-name topic-name")
	fmt.Println("                       • Required: -json-metadata metadata.json")
	fmt.Println("                       • Required: -sender-email verified@domain.com")
	fmt.Println("                       • Optional: -dry-run (preview email)")
	fmt.Println("                       • Creates ICS file from meeting metadata")
	fmt.Println()

	fmt.Println("👥 IDENTITY CENTER INTEGRATION:")
	fmt.Println("  list-identity-center-user     List specific user from Identity Center")
	fmt.Println("                                • Required: -mgmt-role-arn arn:aws:iam::123:role/MyRole")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Required: -username john.doe")
	fmt.Println("                                • Outputs: JSON file with user data")
	fmt.Println()
	fmt.Println("  list-identity-center-user-all List ALL users from Identity Center")
	fmt.Println("                                • Required: -mgmt-role-arn arn:aws:iam::123:role/MyRole")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Optional: -max-concurrency 10 (workers)")
	fmt.Println("                                • Optional: -requests-per-second 10 (rate limit)")
	fmt.Println("                                • Outputs: JSON file with all users data")
	fmt.Println()
	fmt.Println("  list-group-membership         List group memberships for specific user")
	fmt.Println("                                • Required: -mgmt-role-arn arn:aws:iam::123:role/MyRole")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Required: -username john.doe")
	fmt.Println("                                • Outputs: JSON file with user's group memberships")
	fmt.Println()
	fmt.Println("  list-group-membership-all     List group memberships for ALL users")
	fmt.Println("                                • Required: -mgmt-role-arn arn:aws:iam::123:role/MyRole")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Optional: -max-concurrency 10 (workers)")
	fmt.Println("                                • Optional: -requests-per-second 10 (rate limit)")
	fmt.Println("                                • Outputs: Three JSON files (user-centric, group-centric, and CCOE cloud groups)")
	fmt.Println()

	fmt.Println("📥 AWS CONTACT IMPORT:")
	fmt.Println("  import-aws-contact            Import specific user to SES based on group memberships")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Required: -username john.doe")
	fmt.Println("                                • Optional: -mgmt-role-arn (if data files don't exist)")
	fmt.Println("                                • Optional: -dry-run (preview import)")
	fmt.Println()
	fmt.Println("  import-aws-contact-all        Import ALL users to SES based on group memberships")
	fmt.Println("                                • Required: -identity-center-id d-1234567890")
	fmt.Println("                                • Optional: -mgmt-role-arn (if data files don't exist)")
	fmt.Println("                                • Optional: -dry-run (preview import)")
	fmt.Println("                                • Optional: -max-concurrency 10 (for data generation)")
	fmt.Println("                                • Optional: -requests-per-second 10 (for data generation)")
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
	fmt.Println("  # List specific Identity Center user")
	fmt.Println("  ./aws-alternate-contact-manager ses -action list-identity-center-user -mgmt-role-arn arn:aws:iam::123456789012:role/IdentityCenterRole -identity-center-id d-1234567890 -username john.doe")
	fmt.Println()
	fmt.Println("  # List all Identity Center users with custom concurrency")
	fmt.Println("  ./aws-alternate-contact-manager ses -action list-identity-center-user-all -mgmt-role-arn arn:aws:iam::123456789012:role/IdentityCenterRole -identity-center-id d-1234567890 -max-concurrency 10 -requests-per-second 15")
	fmt.Println()
	fmt.Println("  # List group memberships for specific user")
	fmt.Println("  ./aws-alternate-contact-manager ses -action list-group-membership -mgmt-role-arn arn:aws:iam::123456789012:role/IdentityCenterRole -identity-center-id d-1234567890 -username john.doe")
	fmt.Println()
	fmt.Println("  # List group memberships for all users")
	fmt.Println("  ./aws-alternate-contact-manager ses -action list-group-membership-all -mgmt-role-arn arn:aws:iam::123456789012:role/IdentityCenterRole -identity-center-id d-1234567890 -max-concurrency 10 -requests-per-second 10")
	fmt.Println()
	fmt.Println("  # Import specific user to SES (uses existing JSON files)")
	fmt.Println("  ./aws-alternate-contact-manager ses -action import-aws-contact -identity-center-id d-1234567890 -username john.doe")
	fmt.Println()
	fmt.Println("  # Import all users to SES with dry run")
	fmt.Println("  ./aws-alternate-contact-manager ses -action import-aws-contact-all -identity-center-id d-1234567890 -dry-run")
	fmt.Println()
	fmt.Println("  # Import all users (will generate data files if missing)")
	fmt.Println("  ./aws-alternate-contact-manager ses -action import-aws-contact-all -identity-center-id d-1234567890 -mgmt-role-arn arn:aws:iam::123456789012:role/IdentityCenterRole")
	fmt.Println()
	fmt.Println("  # Use SES with role assumption")
	fmt.Println("  ./aws-alternate-contact-manager ses -action list-contacts -ses-role-arn arn:aws:iam::123456789012:role/SESRole")
	fmt.Println()

	fmt.Println("⚙️  CONFIGURATION:")
	fmt.Println("  -ses-config-file     Path to SES config (default: SESConfig.json)")
	fmt.Println("  -backup-file         Path to backup file for restore operations")
	fmt.Println("  -email               Email address for contact operations")
	fmt.Println("  -topics              Comma-separated topic list")
	fmt.Println("  -topic-name          Specific topic name")
	fmt.Println("  -suppression-reason  Reason for suppression (bounce|complaint)")
	fmt.Println("  -dry-run             Preview changes without applying")
	fmt.Println("  -ses-role-arn        Optional IAM role ARN for SES operations")
	fmt.Println("  -mgmt-role-arn       Management IAM role ARN for Identity Center operations")
	fmt.Println("  -identity-center-id  Identity Center instance ID (d-xxxxxxxxxx)")
	fmt.Println("  -username            Username to search in Identity Center")
	fmt.Println("  -max-concurrency     Max concurrent workers (default: 10)")
	fmt.Println("  -requests-per-second API rate limit (default: 10 req/sec)")
	fmt.Println()

	fmt.Println("🔒 SAFETY FEATURES:")
	fmt.Println("  • Automatic backups for destructive operations")
	fmt.Println("  • Dry-run mode for preview")
	fmt.Println("  • Progress tracking and error reporting")
	fmt.Println("  • Backup files include complete restoration data")
	fmt.Println("  • JSON output files for Identity Center data")
	fmt.Println()
}

// handleIdentityCenterUserListing handles Identity Center user listing with role assumption
func handleIdentityCenterUserListing(mgmtRoleArn string, identityCenterId string, userName string, listType string, maxConcurrency int, requestsPerSecond int) error {
	fmt.Printf("🔐 Assuming management role: %s\n", mgmtRoleArn)

	// Load default AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Create STS client to assume role
	stsClient := sts.NewFromConfig(cfg)

	// Assume the specified management role
	assumeRoleInput := &sts.AssumeRoleInput{
		RoleArn:         aws.String(mgmtRoleArn),
		RoleSessionName: aws.String("identity-center-user-listing"),
	}

	assumeRoleResult, err := stsClient.AssumeRole(context.Background(), assumeRoleInput)
	if err != nil {
		return fmt.Errorf("failed to assume management role %s: %w", mgmtRoleArn, err)
	}

	fmt.Printf("✅ Successfully assumed role\n")

	// Create new config with assumed role credentials
	assumedCreds := aws.Credentials{
		AccessKeyID:     *assumeRoleResult.Credentials.AccessKeyId,
		SecretAccessKey: *assumeRoleResult.Credentials.SecretAccessKey,
		SessionToken:    *assumeRoleResult.Credentials.SessionToken,
		Source:          "AssumeRole",
	}

	assumedCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: assumedCreds,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to create config with assumed role: %w", err)
	}

	// Create Identity Store client with assumed role
	identityStoreClient := identitystore.NewFromConfig(assumedCfg)

	if listType == "all" {
		// List all users
		users, err := ListIdentityCenterUsersAll(identityStoreClient, identityCenterId, maxConcurrency, requestsPerSecond)
		if err != nil {
			return fmt.Errorf("failed to list all Identity Center users: %w", err)
		}

		// Display users
		DisplayIdentityCenterUsers(users)

		// Save to JSON file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("identity-center-users-%s-%s.json", identityCenterId, timestamp)
		err = SaveIdentityCenterUsersToJSON(users, filename)
		if err != nil {
			fmt.Printf("Warning: Failed to save users to JSON file: %v\n", err)
		}
	} else {
		// List specific user
		user, err := ListIdentityCenterUser(identityStoreClient, identityCenterId, userName)
		if err != nil {
			return fmt.Errorf("failed to list Identity Center user %s: %w", userName, err)
		}

		// Display user
		DisplayIdentityCenterUser(user)

		// Save to JSON file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("identity-center-user-%s-%s-%s.json", identityCenterId, userName, timestamp)
		err = SaveIdentityCenterUserToJSON(user, filename)
		if err != nil {
			fmt.Printf("Warning: Failed to save user to JSON file: %v\n", err)
		}
	}

	return nil
}

// handleIdentityCenterGroupMembership handles Identity Center group membership listing with role assumption
func handleIdentityCenterGroupMembership(mgmtRoleArn string, identityCenterId string, userName string, listType string, maxConcurrency int, requestsPerSecond int) error {
	fmt.Printf("🔐 Assuming management role: %s\n", mgmtRoleArn)

	// Load default AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Create STS client to assume role
	stsClient := sts.NewFromConfig(cfg)

	// Assume the specified management role
	assumeRoleInput := &sts.AssumeRoleInput{
		RoleArn:         aws.String(mgmtRoleArn),
		RoleSessionName: aws.String("identity-center-group-membership"),
	}

	assumeRoleResult, err := stsClient.AssumeRole(context.Background(), assumeRoleInput)
	if err != nil {
		return fmt.Errorf("failed to assume management role %s: %w", mgmtRoleArn, err)
	}

	fmt.Printf("✅ Successfully assumed role\n")

	// Create new config with assumed role credentials
	assumedCreds := aws.Credentials{
		AccessKeyID:     *assumeRoleResult.Credentials.AccessKeyId,
		SecretAccessKey: *assumeRoleResult.Credentials.SecretAccessKey,
		SessionToken:    *assumeRoleResult.Credentials.SessionToken,
		Source:          "AssumeRole",
	}

	assumedCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: assumedCreds,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to create config with assumed role: %w", err)
	}

	// Create Identity Store client with assumed role
	identityStoreClient := identitystore.NewFromConfig(assumedCfg)

	if listType == "all" {
		// List all users and their group memberships
		memberships, err := ListAllUserGroupMemberships(identityStoreClient, identityCenterId, maxConcurrency, requestsPerSecond)
		if err != nil {
			return fmt.Errorf("failed to list all user group memberships: %w", err)
		}

		// Display memberships
		DisplayAllUserGroupMemberships(memberships)

		// Save user-centric JSON file
		timestamp := time.Now().Format("20060102-150405")
		userCentricFilename := fmt.Sprintf("identity-center-group-memberships-user-centric-%s-%s.json", identityCenterId, timestamp)
		err = SaveGroupMembershipsToJSON(memberships, userCentricFilename)
		if err != nil {
			fmt.Printf("Warning: Failed to save user-centric group memberships to JSON file: %v\n", err)
		}

		// Convert to group-centric format and save
		groupCentric := ConvertToGroupCentric(memberships)
		groupCentricFilename := fmt.Sprintf("identity-center-group-memberships-group-centric-%s-%s.json", identityCenterId, timestamp)
		err = SaveGroupCentricToJSON(groupCentric, groupCentricFilename)
		if err != nil {
			fmt.Printf("Warning: Failed to save group-centric data to JSON file: %v\n", err)
		}

		// Parse and save CCOE cloud groups
		ccoeGroups := ParseAllCCOECloudGroups(memberships)
		if len(ccoeGroups) > 0 {
			fmt.Printf("\n🏢 Found %d CCOE cloud groups\n", len(ccoeGroups))
			DisplayCCOECloudGroups(ccoeGroups)

			ccoeFilename := fmt.Sprintf("identity-center-ccoe-cloud-groups-%s-%s.json", identityCenterId, timestamp)
			err = SaveCCOECloudGroupsToJSON(ccoeGroups, ccoeFilename)
			if err != nil {
				fmt.Printf("Warning: Failed to save CCOE cloud groups to JSON file: %v\n", err)
			}
		} else {
			fmt.Printf("\n🏢 No CCOE cloud groups found\n")
		}
	} else {
		// List specific user's group membership
		membership, err := ListUserGroupMembership(identityStoreClient, identityCenterId, userName)
		if err != nil {
			return fmt.Errorf("failed to list group membership for user %s: %w", userName, err)
		}

		// Display membership
		DisplayUserGroupMembership(membership)

		// Save to JSON file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("identity-center-group-membership-%s-%s-%s.json", identityCenterId, userName, timestamp)
		err = SaveGroupMembershipToJSON(membership, filename)
		if err != nil {
			fmt.Printf("Warning: Failed to save group membership to JSON file: %v\n", err)
		}
	}

	return nil
}

// handleAWSContactImport handles AWS contact import with automatic data generation if needed
func handleAWSContactImport(sesClient *sesv2.Client, mgmtRoleArn string, identityCenterId string, userName string, importType string, maxConcurrency int, requestsPerSecond int, dryRun bool) error {
	// Check if required JSON files exist, if not generate them
	configPath := GetConfigPath()

	userFileExists := false
	groupFileExists := false

	// Auto-detect identity center ID if not provided
	if identityCenterId == "" {
		detectedId, err := autoDetectIdentityCenterId()
		if err == nil {
			identityCenterId = detectedId
			fmt.Printf("🔍 Auto-detected Identity Center ID: %s\n", identityCenterId)
		}
	}

	// Check for user file
	if identityCenterId != "" {
		if userFile, err := findMostRecentFile(configPath, fmt.Sprintf("identity-center-users-%s-", identityCenterId)); err == nil {
			fmt.Printf("📁 Found existing user data file: %s\n", userFile)
			userFileExists = true
		}

		// Check for group membership file
		if groupFile, err := findMostRecentFile(configPath, fmt.Sprintf("identity-center-group-memberships-user-centric-%s-", identityCenterId)); err == nil {
			fmt.Printf("📁 Found existing group membership file: %s\n", groupFile)
			groupFileExists = true
		}
	}

	// Generate missing files if needed
	if !userFileExists || !groupFileExists {
		if identityCenterId == "" {
			return fmt.Errorf("identity-center-id is required when no existing data files are found")
		}
		if mgmtRoleArn == "" {
			return fmt.Errorf("mgmt-role-arn is required to generate Identity Center data files")
		}

		fmt.Printf("📥 Missing Identity Center data files, generating them...\n")

		if !userFileExists {
			fmt.Printf("🔍 Generating user data...\n")
			err := handleIdentityCenterUserListing(mgmtRoleArn, identityCenterId, "", "all", maxConcurrency, requestsPerSecond)
			if err != nil {
				return fmt.Errorf("failed to generate user data: %w", err)
			}
		}

		if !groupFileExists {
			fmt.Printf("🔍 Generating group membership data...\n")
			err := handleIdentityCenterGroupMembership(mgmtRoleArn, identityCenterId, "", "all", maxConcurrency, requestsPerSecond)
			if err != nil {
				return fmt.Errorf("failed to generate group membership data: %w", err)
			}
		}

		fmt.Printf("✅ Identity Center data files generated successfully\n")
	}

	// Now perform the import
	if importType == "all" {
		return ImportAllAWSContacts(sesClient, identityCenterId, dryRun)
	} else {
		return ImportAWSContact(sesClient, identityCenterId, userName, dryRun)
	}
}

// ManageSESLists handles SES list management operations
func ManageSESLists(action string, sesConfigFile string, backupFile string, email string, topics []string, suppressionReason string, topicName string, dryRun bool, sesRoleArn string, mgmtRoleArn string, identityCenterId string, userName string, maxConcurrency int, requestsPerSecond int, senderEmail string, subscriptionConfig string, jsonMetadata string, htmlTemplate string) {
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

	// Handle SES role assumption if specified (except for Identity Center actions)
	if sesRoleArn != "" && !strings.HasPrefix(action, "list-identity-center-") {
		fmt.Printf("🔐 Assuming SES role: %s\n", sesRoleArn)

		// Create STS client with default config
		stsClient := sts.NewFromConfig(cfg)

		// Assume the specified SES role
		assumeRoleInput := &sts.AssumeRoleInput{
			RoleArn:         aws.String(sesRoleArn),
			RoleSessionName: aws.String("ses-operations"),
		}

		assumeRoleResult, err := stsClient.AssumeRole(context.Background(), assumeRoleInput)
		if err != nil {
			fmt.Printf("Failed to assume SES role %s: %v\n", sesRoleArn, err)
			return
		}

		fmt.Printf("✅ Successfully assumed SES role\n")

		// Create new config with assumed role credentials
		assumedCreds := aws.Credentials{
			AccessKeyID:     *assumeRoleResult.Credentials.AccessKeyId,
			SecretAccessKey: *assumeRoleResult.Credentials.SecretAccessKey,
			SessionToken:    *assumeRoleResult.Credentials.SessionToken,
			Source:          "AssumeRole",
		}

		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
				Value: assumedCreds,
			}),
		)
		if err != nil {
			fmt.Printf("Failed to create config with assumed SES role: %v\n", err)
			return
		}
	}

	sesClient := sesv2.NewFromConfig(cfg)

	// Get the account's main contact list for operations that need it
	var accountListName string
	if action == "add-contact" || action == "remove-contact" || action == "remove-contact-all" || action == "list-contacts" || action == "describe-list" {
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
	case "send-topic-test":
		if topicName == "" {
			fmt.Printf("Error: topic name is required for send-topic-test action\n")
			return
		}
		err = SendTopicTestEmail(sesClient, topicName, senderEmail)
	case "describe-contact":
		if email == "" {
			fmt.Printf("Error: email is required for describe-contact action\n")
			return
		}
		err = DescribeContact(sesClient, email)
	case "manage-topic":
		expandedTopics := ExpandTopicsWithGroups(sesConfig)
		err = ManageTopics(sesClient, expandedTopics, dryRun)
	case "subscribe":
		err = ManageSubscriptions(sesClient, subscriptionConfig, true, dryRun)
	case "unsubscribe":
		err = ManageSubscriptions(sesClient, subscriptionConfig, false, dryRun)
	case "send-approval-request":
		if topicName == "" {
			fmt.Printf("Error: topic-name is required for send-approval-request action\n")
			return
		}
		err = SendApprovalRequest(sesClient, topicName, jsonMetadata, htmlTemplate, senderEmail, dryRun)
	case "send-calendar-invite":
		if topicName == "" {
			fmt.Printf("Error: topic-name is required for send-calendar-invite action\n")
			return
		}
		err = SendCalendarInvite(sesClient, topicName, jsonMetadata, senderEmail, dryRun)
	case "list-identity-center-user":
		if userName == "" {
			fmt.Printf("Error: username is required for list-identity-center-user action\n")
			return
		}
		if identityCenterId == "" {
			fmt.Printf("Error: identity-center-id is required for list-identity-center-user action\n")
			return
		}
		if mgmtRoleArn == "" {
			fmt.Printf("Error: mgmt-role-arn is required for list-identity-center-user action\n")
			return
		}
		err = handleIdentityCenterUserListing(mgmtRoleArn, identityCenterId, userName, "", maxConcurrency, requestsPerSecond)
	case "list-identity-center-user-all":
		if identityCenterId == "" {
			fmt.Printf("Error: identity-center-id is required for list-identity-center-user-all action\n")
			return
		}
		if mgmtRoleArn == "" {
			fmt.Printf("Error: mgmt-role-arn is required for list-identity-center-user-all action\n")
			return
		}
		err = handleIdentityCenterUserListing(mgmtRoleArn, identityCenterId, "", "all", maxConcurrency, requestsPerSecond)
	case "list-group-membership":
		if userName == "" {
			fmt.Printf("Error: username is required for list-group-membership action\n")
			return
		}
		if identityCenterId == "" {
			fmt.Printf("Error: identity-center-id is required for list-group-membership action\n")
			return
		}
		if mgmtRoleArn == "" {
			fmt.Printf("Error: mgmt-role-arn is required for list-group-membership action\n")
			return
		}
		err = handleIdentityCenterGroupMembership(mgmtRoleArn, identityCenterId, userName, "", maxConcurrency, requestsPerSecond)
	case "list-group-membership-all":
		if identityCenterId == "" {
			fmt.Printf("Error: identity-center-id is required for list-group-membership-all action\n")
			return
		}
		if mgmtRoleArn == "" {
			fmt.Printf("Error: mgmt-role-arn is required for list-group-membership-all action\n")
			return
		}
		err = handleIdentityCenterGroupMembership(mgmtRoleArn, identityCenterId, "", "all", maxConcurrency, requestsPerSecond)
	case "import-aws-contact":
		if userName == "" {
			fmt.Printf("Error: username is required for import-aws-contact action\n")
			return
		}
		if identityCenterId == "" {
			fmt.Printf("Error: identity-center-id is required for import-aws-contact action\n")
			return
		}
		err = handleAWSContactImport(sesClient, mgmtRoleArn, identityCenterId, userName, "", maxConcurrency, requestsPerSecond, dryRun)
	case "import-aws-contact-all":
		// identity-center-id is optional for import-aws-contact-all - will auto-detect from files
		err = handleAWSContactImport(sesClient, mgmtRoleArn, identityCenterId, "", "all", maxConcurrency, requestsPerSecond, dryRun)
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
	sesAction := sesCommand.String("action", "", "SES action (create-list, add-contact, remove-contact, remove-contact-all, suppress, unsuppress, list-contacts, describe-list, describe-account, describe-topic, describe-topic-all, describe-contact, manage-topic, subscribe, unsubscribe, send-approval-request, send-calendar-invite, list-identity-center-user, list-identity-center-user-all, list-group-membership, list-group-membership-all, import-aws-contact, import-aws-contact-all, help)")
	sesConfigFile := sesCommand.String("ses-config-file", "SESConfig.json", "Path to the SES configuration file (default: SESConfig.json)")
	sesBackupFile := sesCommand.String("backup-file", "", "Path to backup file for restore operations (for create-list action)")
	sesEmail := sesCommand.String("email", "", "Email address for contact operations")
	sesTopics := sesCommand.String("topics", "", "Comma-separated list of topics for contact subscription")
	sesSuppressionReason := sesCommand.String("suppression-reason", "bounce", "Reason for suppression (bounce or complaint)")
	sesTopicName := sesCommand.String("topic-name", "", "Topic name for topic-specific operations")
	sesDryRun := sesCommand.Bool("dry-run", false, "Show what would be done without making changes")
	sesSESRoleArn := sesCommand.String("ses-role-arn", "", "Optional IAM role ARN to assume for SES operations")
	sesMgmtRoleArn := sesCommand.String("mgmt-role-arn", "", "Management account IAM role ARN to assume for Identity Center operations")
	sesIdentityCenterId := sesCommand.String("identity-center-id", "", "Identity Center instance ID")
	sesUserName := sesCommand.String("username", "", "Username to search for in Identity Center")
	sesMaxConcurrency := sesCommand.Int("max-concurrency", 10, "Maximum concurrent workers for Identity Center operations (default: 10)")
	sesRequestsPerSecond := sesCommand.Int("requests-per-second", 10, "API requests per second rate limit (default: 10)")
	sesSenderEmail := sesCommand.String("sender-email", "", "Sender email address for test emails (must be verified in SES)")
	sesSubscriptionConfig := sesCommand.String("subscription-config", "SubscriptionConfig.json", "Path to subscription configuration file (default: SubscriptionConfig.json)")
	sesJsonMetadata := sesCommand.String("json-metadata", "", "Path to JSON metadata file from metadata collector")
	sesHtmlTemplate := sesCommand.String("html-template", "", "Path to HTML email template file")

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

		ManageSESLists(*sesAction, *sesConfigFile, *sesBackupFile, *sesEmail, topics, *sesSuppressionReason, *sesTopicName, *sesDryRun, *sesSESRoleArn, *sesMgmtRoleArn, *sesIdentityCenterId, *sesUserName, *sesMaxConcurrency, *sesRequestsPerSecond, *sesSenderEmail, *sesSubscriptionConfig, *sesJsonMetadata, *sesHtmlTemplate)
	}
}
