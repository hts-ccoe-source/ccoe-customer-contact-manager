package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoreTypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// SES-related types from the original file
type SESTopicConfig struct {
	TopicName                 string   `json:"TopicName"`
	DisplayName               string   `json:"DisplayName"`
	Description               string   `json:"Description"`
	DefaultSubscriptionStatus string   `json:"DefaultSubscriptionStatus"`
	OptInRoles                []string `json:"OptInRoles"`
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

// GetConfigPath returns the configuration path from environment or default
func GetConfigPath() string {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./"
	}
	if !strings.HasSuffix(configPath, "/") {
		configPath += "/"
	}
	return configPath
}

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

// RemoveContactFromList removes an email contact from a contact list
func RemoveContactFromList(sesClient *sesv2.Client, listName string, email string) error {
	input := &sesv2.DeleteContactInput{
		ContactListName: aws.String(listName),
		EmailAddress:    aws.String(email),
	}

	// Implement exponential backoff with retries
	err := retryWithExponentialBackoff(func() error {
		_, err := sesClient.DeleteContact(context.Background(), input)
		return err
	}, fmt.Sprintf("remove contact %s from list %s", email, listName))

	if err != nil {
		return err
	}

	fmt.Printf("Successfully removed contact %s from list %s\n", email, listName)
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
				OptInRoles:                topic.OptInRoles,
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

// ManageSESLists handles SES list management operations
func ManageSESLists(action string, sesConfigFile string, backupFile string, email string, topics []string, suppressionReason string, topicName string, dryRun bool, sesRoleArn string, mgmtRoleArn string, identityCenterId string, userName string, maxConcurrency int, requestsPerSecond int, senderEmail string) error {
	ConfigPath := GetConfigPath()
	fmt.Println("Working in Config Path: " + ConfigPath)

	// Read SES config file
	if sesConfigFile == "" {
		sesConfigFile = "SESConfig.json"
	}

	sesJson, err := os.ReadFile(ConfigPath + sesConfigFile)
	if err != nil {
		return fmt.Errorf("error reading SES config file: %v", err)
	}
	fmt.Println("Successfully opened " + ConfigPath + sesConfigFile)

	var sesConfig SESConfig
	err = json.NewDecoder(bytes.NewReader(sesJson)).Decode(&sesConfig)
	if err != nil {
		return fmt.Errorf("error parsing SES config: %v", err)
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load AWS configuration: %v", err)
	}

	// Handle SES role assumption if specified
	if sesRoleArn != "" {
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
			return fmt.Errorf("failed to assume SES role %s: %v", sesRoleArn, err)
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
			return fmt.Errorf("failed to create config with assumed SES role: %v", err)
		}
	}

	sesClient := sesv2.NewFromConfig(cfg)

	// Get the account's main contact list for operations that need it
	var accountListName string
	if action == "add-contact" || action == "remove-contact" || action == "remove-contact-all" || action == "list-contacts" || action == "describe-list" || action == "delete-list" {
		accountListName, err = GetAccountContactList(sesClient)
		if err != nil {
			return fmt.Errorf("error finding account contact list: %v", err)
		}
	}

	switch action {
	case "create-list":
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
	case "delete-list":
		err = DeleteContactList(sesClient, accountListName, dryRun)
	case "describe-account":
		// Automatically find and describe the account's main contact list
		accountListName, err := GetAccountContactList(sesClient)
		if err != nil {
			return fmt.Errorf("error finding account contact list: %v", err)
		}
		fmt.Printf("Account's main contact list: %s\n\n", accountListName)
		err = DescribeContactList(sesClient, accountListName)
	case "describe-topic":
		if topicName == "" {
			return fmt.Errorf("topic name is required for describe-topic action")
		}
		err = DescribeTopic(sesClient, topicName)
	case "describe-topic-all":
		err = DescribeAllTopics(sesClient)
	case "describe-contact":
		if email == "" {
			return fmt.Errorf("email is required for describe-contact action")
		}
		err = DescribeContact(sesClient, email)
	case "manage-topic":
		expandedTopics := ExpandTopicsWithGroups(sesConfig)
		err = ManageTopics(sesClient, expandedTopics, dryRun)
	case "send-topic-test":
		if topicName == "" {
			return fmt.Errorf("topic name is required for send-topic-test action")
		}
		err = SendTopicTestEmail(sesClient, topicName, senderEmail)
	case "remove-contact-all":
		err = RemoveAllContactsFromList(sesClient, accountListName)
	case "list-identity-center-user":
		if userName == "" {
			return fmt.Errorf("username is required for list-identity-center-user action")
		}
		if identityCenterId == "" {
			return fmt.Errorf("identity-center-id is required for list-identity-center-user action")
		}
		if mgmtRoleArn == "" {
			return fmt.Errorf("mgmt-role-arn is required for list-identity-center-user action")
		}
		err = handleIdentityCenterUserListing(mgmtRoleArn, identityCenterId, userName, "", maxConcurrency, requestsPerSecond)
	case "list-identity-center-user-all":
		if identityCenterId == "" {
			return fmt.Errorf("identity-center-id is required for list-identity-center-user-all action")
		}
		if mgmtRoleArn == "" {
			return fmt.Errorf("mgmt-role-arn is required for list-identity-center-user-all action")
		}
		err = handleIdentityCenterUserListing(mgmtRoleArn, identityCenterId, "", "all", maxConcurrency, requestsPerSecond)
	case "list-group-membership":
		if userName == "" {
			return fmt.Errorf("username is required for list-group-membership action")
		}
		if identityCenterId == "" {
			return fmt.Errorf("identity-center-id is required for list-group-membership action")
		}
		if mgmtRoleArn == "" {
			return fmt.Errorf("mgmt-role-arn is required for list-group-membership action")
		}
		err = handleIdentityCenterGroupMembership(mgmtRoleArn, identityCenterId, userName, "", maxConcurrency, requestsPerSecond)
	case "list-group-membership-all":
		if identityCenterId == "" {
			return fmt.Errorf("identity-center-id is required for list-group-membership-all action")
		}
		if mgmtRoleArn == "" {
			return fmt.Errorf("mgmt-role-arn is required for list-group-membership-all action")
		}
		err = handleIdentityCenterGroupMembership(mgmtRoleArn, identityCenterId, "", "all", maxConcurrency, requestsPerSecond)
	case "import-aws-contact":
		if userName == "" {
			return fmt.Errorf("username is required for import-aws-contact action")
		}
		if identityCenterId == "" {
			return fmt.Errorf("identity-center-id is required for import-aws-contact action")
		}
		err = handleAWSContactImport(sesClient, mgmtRoleArn, identityCenterId, userName, "", maxConcurrency, requestsPerSecond, dryRun)
	case "import-aws-contact-all":
		// identity-center-id is optional for import-aws-contact-all - will auto-detect from files
		err = handleAWSContactImport(sesClient, mgmtRoleArn, identityCenterId, "", "all", maxConcurrency, requestsPerSecond, dryRun)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	return err
}

// Placeholder functions - these need to be extracted from the original file

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

// Identity Center functions - these require additional AWS SDK dependencies
// and are complex functions that would need the identitystore service

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
		// List all users - simplified implementation
		fmt.Printf("📋 Listing all Identity Center users...\n")
		users, err := listAllIdentityCenterUsers(identityStoreClient, identityCenterId)
		if err != nil {
			return fmt.Errorf("failed to list all Identity Center users: %w", err)
		}

		// Display users
		fmt.Printf("Found %d users:\n", len(users))
		for i, user := range users {
			fmt.Printf("%d. %s (%s) - %s\n", i+1, user.DisplayName, user.UserName, user.Email)
		}

		// Save to JSON file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("identity-center-users-%s-%s.json", identityCenterId, timestamp)
		err = saveUsersToJSON(users, filename)
		if err != nil {
			fmt.Printf("Warning: Failed to save users to JSON file: %v\n", err)
		} else {
			fmt.Printf("✅ Saved users to: %s\n", filename)
		}
	} else {
		// List specific user
		fmt.Printf("🔍 Looking up user: %s\n", userName)
		user, err := findIdentityCenterUser(identityStoreClient, identityCenterId, userName)
		if err != nil {
			return fmt.Errorf("failed to find Identity Center user %s: %w", userName, err)
		}

		// Display user
		fmt.Printf("User Details:\n")
		fmt.Printf("  Display Name: %s\n", user.DisplayName)
		fmt.Printf("  Username: %s\n", user.UserName)
		fmt.Printf("  Email: %s\n", user.Email)
		fmt.Printf("  User ID: %s\n", user.UserId)
		fmt.Printf("  Active: %t\n", user.Active)

		// Save to JSON file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("identity-center-user-%s-%s-%s.json", identityCenterId, userName, timestamp)
		err = saveUserToJSON(user, filename)
		if err != nil {
			fmt.Printf("Warning: Failed to save user to JSON file: %v\n", err)
		} else {
			fmt.Printf("✅ Saved user to: %s\n", filename)
		}
	}

	return nil
}

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
		fmt.Printf("📋 Listing group memberships for all users...\n")
		memberships, err := listAllUserGroupMemberships(identityStoreClient, identityCenterId)
		if err != nil {
			return fmt.Errorf("failed to list all user group memberships: %w", err)
		}

		// Display memberships
		fmt.Printf("Found group memberships for %d users:\n", len(memberships))
		for i, membership := range memberships {
			fmt.Printf("%d. %s (%s) - %d groups\n", i+1, membership.DisplayName, membership.UserName, len(membership.Groups))
			for _, group := range membership.Groups {
				fmt.Printf("   - %s\n", group)
			}
		}

		// Save user-centric JSON file
		timestamp := time.Now().Format("20060102-150405")
		userCentricFilename := fmt.Sprintf("identity-center-group-memberships-user-centric-%s-%s.json", identityCenterId, timestamp)
		err = saveGroupMembershipsToJSON(memberships, userCentricFilename)
		if err != nil {
			fmt.Printf("Warning: Failed to save user-centric group memberships to JSON file: %v\n", err)
		} else {
			fmt.Printf("✅ Saved user-centric data to: %s\n", userCentricFilename)
		}

		// Convert to group-centric format and save
		groupCentric := convertToGroupCentric(memberships)
		groupCentricFilename := fmt.Sprintf("identity-center-group-memberships-group-centric-%s-%s.json", identityCenterId, timestamp)
		err = saveGroupCentricToJSON(groupCentric, groupCentricFilename)
		if err != nil {
			fmt.Printf("Warning: Failed to save group-centric data to JSON file: %v\n", err)
		} else {
			fmt.Printf("✅ Saved group-centric data to: %s\n", groupCentricFilename)
		}
	} else {
		// List specific user's group membership
		fmt.Printf("🔍 Looking up group membership for user: %s\n", userName)
		membership, err := getUserGroupMembership(identityStoreClient, identityCenterId, userName)
		if err != nil {
			return fmt.Errorf("failed to get group membership for user %s: %w", userName, err)
		}

		// Display membership
		fmt.Printf("User: %s (%s)\n", membership.DisplayName, membership.UserName)
		fmt.Printf("Email: %s\n", membership.Email)
		fmt.Printf("Groups (%d):\n", len(membership.Groups))
		for _, group := range membership.Groups {
			fmt.Printf("  - %s\n", group)
		}

		// Save to JSON file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("identity-center-group-membership-%s-%s-%s.json", identityCenterId, userName, timestamp)
		err = saveGroupMembershipToJSON(membership, filename)
		if err != nil {
			fmt.Printf("Warning: Failed to save group membership to JSON file: %v\n", err)
		} else {
			fmt.Printf("✅ Saved group membership to: %s\n", filename)
		}
	}

	return nil
}

func handleAWSContactImport(sesClient *sesv2.Client, mgmtRoleArn string, identityCenterId string, userName string, importType string, maxConcurrency int, requestsPerSecond int, dryRun bool) error {
	fmt.Printf("📥 AWS Contact Import\n")

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No actual imports will be performed\n")
	}

	// Check if required JSON files exist
	configPath := GetConfigPath()

	// Auto-detect identity center ID if not provided
	if identityCenterId == "" {
		detectedId, err := autoDetectIdentityCenterId(configPath)
		if err == nil {
			identityCenterId = detectedId
			fmt.Printf("🔍 Auto-detected Identity Center ID: %s\n", identityCenterId)
		}
	}

	userFileExists := false
	groupFileExists := false

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

		fmt.Printf("📋 Generating missing Identity Center data files...\n")

		if !userFileExists {
			fmt.Printf("🔄 Generating user data...\n")
			err := handleIdentityCenterUserListing(mgmtRoleArn, identityCenterId, "", "all", maxConcurrency, requestsPerSecond)
			if err != nil {
				return fmt.Errorf("failed to generate user data: %w", err)
			}
		}

		if !groupFileExists {
			fmt.Printf("🔄 Generating group membership data...\n")
			err := handleIdentityCenterGroupMembership(mgmtRoleArn, identityCenterId, "", "all", maxConcurrency, requestsPerSecond)
			if err != nil {
				return fmt.Errorf("failed to generate group membership data: %w", err)
			}
		}
	}

	if importType == "all" {
		return BulkImportContacts(sesClient, identityCenterId, dryRun)
	} else {
		fmt.Printf("📥 Importing specific user: %s\n", userName)
		if userName == "" {
			return fmt.Errorf("username is required for single user import")
		}
		return ImportSingleAWSContact(sesClient, identityCenterId, userName, dryRun)
	}
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

// Identity Center types
type IdentityCenterUser struct {
	UserId      string `json:"user_id"`
	UserName    string `json:"user_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	GivenName   string `json:"given_name"`
	FamilyName  string `json:"family_name"`
	Active      bool   `json:"active"`
}

type IdentityCenterGroupMembership struct {
	UserId      string   `json:"user_id"`
	UserName    string   `json:"user_name"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Groups      []string `json:"groups"`
}

type IdentityCenterGroupCentric struct {
	GroupName string                   `json:"group_name"`
	Members   []IdentityCenterUserInfo `json:"members"`
}

type IdentityCenterUserInfo struct {
	UserId      string `json:"user_id"`
	UserName    string `json:"user_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type CCOECloudGroupInfo struct {
	GroupName         string `json:"group_name"`
	AccountName       string `json:"account_name"`
	AccountId         string `json:"account_id"`
	ApplicationPrefix string `json:"application_prefix"`
	RoleName          string `json:"role_name"`
	IsValid           bool   `json:"is_valid"`
}

// Helper functions for Identity Center operations

func listAllIdentityCenterUsers(client *identitystore.Client, identityStoreId string) ([]IdentityCenterUser, error) {
	var allUsers []IdentityCenterUser
	var nextToken *string

	for {
		input := &identitystore.ListUsersInput{
			IdentityStoreId: aws.String(identityStoreId),
			NextToken:       nextToken,
			MaxResults:      aws.Int32(50), // AWS limit
		}

		result, err := client.ListUsers(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to list users: %w", err)
		}

		// Convert AWS SDK users to our custom type
		for _, user := range result.Users {
			icUser := convertToIdentityCenterUser(user)
			allUsers = append(allUsers, icUser)
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	return allUsers, nil
}

func findIdentityCenterUser(client *identitystore.Client, identityStoreId string, userName string) (IdentityCenterUser, error) {
	// Try to find user by username
	input := &identitystore.ListUsersInput{
		IdentityStoreId: aws.String(identityStoreId),
		MaxResults:      aws.Int32(50),
	}

	var nextToken *string
	for {
		input.NextToken = nextToken
		result, err := client.ListUsers(context.Background(), input)
		if err != nil {
			return IdentityCenterUser{}, fmt.Errorf("failed to list users: %w", err)
		}

		// Look for matching username
		for _, user := range result.Users {
			if user.UserName != nil && *user.UserName == userName {
				return convertToIdentityCenterUser(user), nil
			}
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	return IdentityCenterUser{}, fmt.Errorf("user %s not found", userName)
}

func convertToIdentityCenterUser(user identitystoreTypes.User) IdentityCenterUser {
	icUser := IdentityCenterUser{
		UserId: aws.ToString(user.UserId),
		Active: true, // Default to true since Active field may not be available
	}

	if user.UserName != nil {
		icUser.UserName = *user.UserName
	}
	if user.DisplayName != nil {
		icUser.DisplayName = *user.DisplayName
	}

	// Extract email from user attributes
	for _, email := range user.Emails {
		if email.Primary && email.Value != nil {
			icUser.Email = *email.Value
			break
		}
	}
	// If no primary email, use first email
	if icUser.Email == "" && len(user.Emails) > 0 && user.Emails[0].Value != nil {
		icUser.Email = *user.Emails[0].Value
	}

	// Extract names
	if user.Name != nil {
		if user.Name.GivenName != nil {
			icUser.GivenName = *user.Name.GivenName
		}
		if user.Name.FamilyName != nil {
			icUser.FamilyName = *user.Name.FamilyName
		}
	}

	return icUser
}

func saveUsersToJSON(users []IdentityCenterUser, filename string) error {
	configPath := GetConfigPath()
	fullPath := configPath + filename

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users: %w", err)
	}

	err = os.WriteFile(fullPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func saveUserToJSON(user IdentityCenterUser, filename string) error {
	configPath := GetConfigPath()
	fullPath := configPath + filename

	data, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	err = os.WriteFile(fullPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Helper functions for group membership operations

func listAllUserGroupMemberships(client *identitystore.Client, identityStoreId string) ([]IdentityCenterGroupMembership, error) {
	// First get all users
	users, err := listAllIdentityCenterUsers(client, identityStoreId)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	var memberships []IdentityCenterGroupMembership

	// For each user, get their group memberships
	for _, user := range users {
		groups, err := getUserGroups(client, identityStoreId, user.UserId)
		if err != nil {
			fmt.Printf("Warning: Failed to get groups for user %s: %v\n", user.UserName, err)
			continue
		}

		membership := IdentityCenterGroupMembership{
			UserId:      user.UserId,
			UserName:    user.UserName,
			DisplayName: user.DisplayName,
			Email:       user.Email,
			Groups:      groups,
		}

		memberships = append(memberships, membership)
	}

	return memberships, nil
}

func getUserGroupMembership(client *identitystore.Client, identityStoreId string, userName string) (IdentityCenterGroupMembership, error) {
	// Find the user first
	user, err := findIdentityCenterUser(client, identityStoreId, userName)
	if err != nil {
		return IdentityCenterGroupMembership{}, fmt.Errorf("failed to find user: %w", err)
	}

	// Get user's groups
	groups, err := getUserGroups(client, identityStoreId, user.UserId)
	if err != nil {
		return IdentityCenterGroupMembership{}, fmt.Errorf("failed to get user groups: %w", err)
	}

	membership := IdentityCenterGroupMembership{
		UserId:      user.UserId,
		UserName:    user.UserName,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Groups:      groups,
	}

	return membership, nil
}

func getUserGroups(client *identitystore.Client, identityStoreId string, userId string) ([]string, error) {
	var groups []string
	var nextToken *string

	for {
		input := &identitystore.ListGroupMembershipsForMemberInput{
			IdentityStoreId: aws.String(identityStoreId),
			MemberId: &identitystoreTypes.MemberIdMemberUserId{
				Value: userId,
			},
			NextToken:  nextToken,
			MaxResults: aws.Int32(50),
		}

		result, err := client.ListGroupMembershipsForMember(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to list group memberships: %w", err)
		}

		// Get group names for each membership
		for _, membership := range result.GroupMemberships {
			if membership.GroupId != nil {
				groupName, err := getGroupName(client, identityStoreId, *membership.GroupId)
				if err != nil {
					fmt.Printf("Warning: Failed to get group name for ID %s: %v\n", *membership.GroupId, err)
					groups = append(groups, *membership.GroupId) // Use ID as fallback
				} else {
					groups = append(groups, groupName)
				}
			}
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	return groups, nil
}

func getGroupName(client *identitystore.Client, identityStoreId string, groupId string) (string, error) {
	input := &identitystore.DescribeGroupInput{
		IdentityStoreId: aws.String(identityStoreId),
		GroupId:         aws.String(groupId),
	}

	result, err := client.DescribeGroup(context.Background(), input)
	if err != nil {
		return "", fmt.Errorf("failed to describe group: %w", err)
	}

	if result.DisplayName != nil {
		return *result.DisplayName, nil
	}

	return groupId, nil // Fallback to ID
}

func convertToGroupCentric(memberships []IdentityCenterGroupMembership) []IdentityCenterGroupCentric {
	groupMap := make(map[string][]IdentityCenterUserInfo)

	// Build group-centric map
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

	// Convert to slice
	var groupCentric []IdentityCenterGroupCentric
	for groupName, members := range groupMap {
		gc := IdentityCenterGroupCentric{
			GroupName: groupName,
			Members:   members,
		}
		groupCentric = append(groupCentric, gc)
	}

	// Sort by group name
	sort.Slice(groupCentric, func(i, j int) bool {
		return groupCentric[i].GroupName < groupCentric[j].GroupName
	})

	return groupCentric
}

func saveGroupMembershipsToJSON(memberships []IdentityCenterGroupMembership, filename string) error {
	configPath := GetConfigPath()
	fullPath := configPath + filename

	data, err := json.MarshalIndent(memberships, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal memberships: %w", err)
	}

	err = os.WriteFile(fullPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func saveGroupCentricToJSON(groupCentric []IdentityCenterGroupCentric, filename string) error {
	configPath := GetConfigPath()
	fullPath := configPath + filename

	data, err := json.MarshalIndent(groupCentric, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal group centric data: %w", err)
	}

	err = os.WriteFile(fullPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func saveGroupMembershipToJSON(membership IdentityCenterGroupMembership, filename string) error {
	configPath := GetConfigPath()
	fullPath := configPath + filename

	data, err := json.MarshalIndent(membership, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal membership: %w", err)
	}

	err = os.WriteFile(fullPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Helper functions for contact import

func autoDetectIdentityCenterId(configPath string) (string, error) {
	// Look for existing files with identity center ID pattern
	files, err := os.ReadDir(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config directory: %w", err)
	}

	for _, file := range files {
		if strings.Contains(file.Name(), "identity-center-users-") {
			// Extract ID from filename like "identity-center-users-d-1234567890-20231201-120000.json"
			parts := strings.Split(file.Name(), "-")
			if len(parts) >= 4 && strings.HasPrefix(parts[3], "d") {
				return parts[3], nil
			}
		}
	}

	return "", fmt.Errorf("no identity center ID found in existing files")
}

func findMostRecentFile(configPath string, prefix string) (string, error) {
	files, err := os.ReadDir(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config directory: %w", err)
	}

	var matchingFiles []string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), prefix) {
			matchingFiles = append(matchingFiles, file.Name())
		}
	}

	if len(matchingFiles) == 0 {
		return "", fmt.Errorf("no files found with prefix: %s", prefix)
	}

	// Sort to get most recent (assuming timestamp format)
	sort.Strings(matchingFiles)
	return matchingFiles[len(matchingFiles)-1], nil
}

// Contact import configuration types
type RoleTopicMapping struct {
	Roles  []string `json:"roles"`
	Topics []string `json:"topics"`
}

type ContactImportConfig struct {
	RoleMappings       []RoleTopicMapping `json:"role_mappings"`
	DefaultTopics      []string           `json:"default_topics"`
	RequireActiveUsers bool               `json:"require_active_users"`
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
func LoadIdentityCenterDataFromFiles(identityCenterId string) ([]IdentityCenterUser, []IdentityCenterGroupMembership, string, error) {
	configPath := GetConfigPath()

	// Auto-detect identity center ID if not provided
	if identityCenterId == "" {
		detectedId, err := autoDetectIdentityCenterId(configPath)
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

// ParseCCOECloudGroup parses ccoe-cloud group names to extract AWS account information
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

	// Find "idp" marker and extract application prefix and role name
	idpIndex := -1
	for i := accountIdIndex + 1; i < len(parts); i++ {
		if parts[i] == "idp" {
			idpIndex = i
			break
		}
	}

	if idpIndex == -1 || idpIndex+2 >= len(parts) {
		return result // No idp marker or not enough parts after it
	}

	result.ApplicationPrefix = parts[idpIndex+1]
	result.RoleName = parts[idpIndex+2]
	result.IsValid = true

	return result
}

// Helper function to check if string contains only digits
func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Helper function to check if two slices are equal
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
		var topics []string
		for _, pref := range contact.TopicPreferences {
			if pref.SubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
				topics = append(topics, *pref.TopicName)
			}
		}
		existingContacts[*contact.EmailAddress] = topics
	}

	return existingContacts, nil
}

// AddContactToListQuiet adds an email contact to a contact list without verbose output
func AddContactToListQuiet(sesClient *sesv2.Client, listName string, email string, topics []string) error {
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

	// Implement exponential backoff with retries
	return retryWithExponentialBackoff(func() error {
		_, err := sesClient.CreateContact(context.Background(), input)
		return err
	}, fmt.Sprintf("add contact %s to list %s", email, listName))
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

	// Create set of existing topics
	existingTopics := make(map[string]bool)
	for _, topic := range result.Topics {
		existingTopics[*topic.TopicName] = true
	}

	// Check if all required topics exist
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
				missingTopics = append(missingTopics, topic)
			}
		}
	}

	if len(missingTopics) > 0 {
		return fmt.Errorf("missing topics in contact list %s: %v", listName, missingTopics)
	}

	return nil
}

// ContactImportConfig defines the mapping configuration for importing contacts
// BulkImportContacts imports contacts from Identity Center to SES
func BulkImportContacts(sesClient *sesv2.Client, identityCenterId string, dryRun bool) error {
	// Load default SES config
	configPath := GetConfigPath()
	sesJson, err := os.ReadFile(configPath + "SESConfig.json")
	if err != nil {
		return fmt.Errorf("error reading SES config file: %v", err)
	}

	var sesConfig SESConfig
	err = json.NewDecoder(bytes.NewReader(sesJson)).Decode(&sesConfig)
	if err != nil {
		return fmt.Errorf("error parsing SES config: %v", err)
	}

	return BulkImportContactsWithConfig(sesClient, identityCenterId, dryRun, 5, sesConfig) // Default 5 requests per second
}

// BulkImportContactsWithRateLimit imports contacts from Identity Center to SES with configurable rate limiting
func BulkImportContactsWithRateLimit(sesClient *sesv2.Client, identityCenterId string, dryRun bool, requestsPerSecond int) error {
	// Load default SES config
	configPath := GetConfigPath()
	sesJson, err := os.ReadFile(configPath + "SESConfig.json")
	if err != nil {
		return fmt.Errorf("error reading SES config file: %v", err)
	}

	var sesConfig SESConfig
	err = json.NewDecoder(bytes.NewReader(sesJson)).Decode(&sesConfig)
	if err != nil {
		return fmt.Errorf("error parsing SES config: %v", err)
	}

	return BulkImportContactsWithConfig(sesClient, identityCenterId, dryRun, requestsPerSecond, sesConfig)
}

// BulkImportContactsWithConfig imports contacts from Identity Center to SES with configurable rate limiting and SES config
func BulkImportContactsWithConfig(sesClient *sesv2.Client, identityCenterId string, dryRun bool, requestsPerSecond int, sesConfig SESConfig) error {
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

	// Build configuration from SES config
	config := BuildContactImportConfigFromSES(sesConfig)

	// Create rate limiter for SES operations
	rateLimiter := NewRateLimiter(requestsPerSecond)
	fmt.Printf("⚙️  Rate limiting: %d requests per second\n", requestsPerSecond)
	defer rateLimiter.Stop()

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

	for i, user := range users {
		// Show progress for large imports
		if len(users) > 10 && (i+1)%10 == 0 {
			fmt.Printf("📊 Progress: %d/%d users processed (%d%% complete)\n",
				i+1, len(users), (i+1)*100/len(users))
		}

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
				// Rate limit SES operations
				rateLimiter.Wait()
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

		// Rate limit SES operations
		rateLimiter.Wait()

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

// DeleteContactList deletes a contact list from SES after creating a backup
func DeleteContactList(sesClient *sesv2.Client, listName string, dryRun bool) error {
	fmt.Printf("🗑️  Deleting SES contact list: %s\n", listName)

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No actual deletion will be performed\n")
	}

	// First, verify the contact list exists
	listInput := &sesv2.GetContactListInput{
		ContactListName: aws.String(listName),
	}

	listResult, err := sesClient.GetContactList(context.Background(), listInput)
	if err != nil {
		return fmt.Errorf("failed to get contact list details: %w", err)
	}

	// Get contact count for confirmation
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(listName),
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return fmt.Errorf("failed to list contacts: %w", err)
	}

	// Display contact list information
	fmt.Printf("📋 Contact List Details:\n")
	fmt.Printf("   Name: %s\n", listName)
	if listResult.Description != nil {
		fmt.Printf("   Description: %s\n", *listResult.Description)
	}
	fmt.Printf("   Topics: %d\n", len(listResult.Topics))
	fmt.Printf("   Contacts: %d\n", len(contactsResult.Contacts))

	if listResult.CreatedTimestamp != nil {
		fmt.Printf("   Created: %s\n", listResult.CreatedTimestamp.Format("2006-01-02 15:04:05 UTC"))
	}
	if listResult.LastUpdatedTimestamp != nil {
		fmt.Printf("   Last Updated: %s\n", listResult.LastUpdatedTimestamp.Format("2006-01-02 15:04:05 UTC"))
	}

	fmt.Printf("\n")

	if len(contactsResult.Contacts) > 0 {
		fmt.Printf("⚠️  WARNING: This contact list contains %d contacts that will be permanently deleted!\n", len(contactsResult.Contacts))
		fmt.Printf("📦 Creating backup before deletion...\n")

		if !dryRun {
			// Create backup before deletion
			backupFilename, err := CreateContactListBackup(sesClient, listName, "delete-list")
			if err != nil {
				return fmt.Errorf("failed to create backup before deletion: %w", err)
			}
			fmt.Printf("✅ Backup created: %s\n", backupFilename)
		} else {
			fmt.Printf("🔍 DRY RUN: Would create backup before deletion\n")
		}
	}

	if len(listResult.Topics) > 0 {
		fmt.Printf("📋 Topics that will be deleted:\n")
		for i, topic := range listResult.Topics {
			fmt.Printf("   %d. %s", i+1, *topic.TopicName)
			if topic.DisplayName != nil && *topic.DisplayName != *topic.TopicName {
				fmt.Printf(" (%s)", *topic.DisplayName)
			}
			fmt.Printf("\n")
		}
		fmt.Printf("\n")
	}

	if dryRun {
		fmt.Printf("🔍 DRY RUN: Would delete contact list '%s' with %d contacts and %d topics\n",
			listName, len(contactsResult.Contacts), len(listResult.Topics))
		fmt.Printf("🔍 DRY RUN: Use without --dry-run to perform actual deletion\n")
		return nil
	}

	// Perform the deletion
	fmt.Printf("🗑️  Proceeding with deletion of contact list: %s\n", listName)

	deleteInput := &sesv2.DeleteContactListInput{
		ContactListName: aws.String(listName),
	}

	_, err = sesClient.DeleteContactList(context.Background(), deleteInput)
	if err != nil {
		return fmt.Errorf("failed to delete contact list %s: %w", listName, err)
	}

	fmt.Printf("✅ Successfully deleted contact list: %s\n", listName)
	fmt.Printf("📊 Deletion Summary:\n")
	fmt.Printf("   🗑️  Deleted contact list: %s\n", listName)
	fmt.Printf("   📧 Contacts removed: %d\n", len(contactsResult.Contacts))
	fmt.Printf("   📋 Topics removed: %d\n", len(listResult.Topics))

	if len(contactsResult.Contacts) > 0 {
		fmt.Printf("   📁 Backup available for recovery\n")
	}

	return nil
}

// retryWithExponentialBackoff implements exponential backoff retry logic
func retryWithExponentialBackoff(operation func() error, operationName string) error {
	maxRetries := 5
	baseDelay := 1 * time.Second
	maxDelay := 30 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		// Check if this is a rate limiting error
		isRateLimitError := strings.Contains(err.Error(), "TooManyRequestsException") ||
			strings.Contains(err.Error(), "Rate exceeded") ||
			strings.Contains(err.Error(), "429")

		// Check if this is a throttling error
		isThrottlingError := strings.Contains(err.Error(), "Throttling") ||
			strings.Contains(err.Error(), "ThrottledException")

		// Only retry on rate limiting or throttling errors
		if !isRateLimitError && !isThrottlingError {
			return fmt.Errorf("failed to %s: %w", operationName, err)
		}

		// Don't retry on the last attempt
		if attempt == maxRetries {
			return fmt.Errorf("failed to %s after %d retries: %w", operationName, maxRetries, err)
		}

		// Calculate delay with exponential backoff and jitter
		delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
		if delay > maxDelay {
			delay = maxDelay
		}

		// Add jitter (random component) to avoid thundering herd
		jitter := time.Duration(rand.Float64() * float64(delay) * 0.1) // 10% jitter
		delay += jitter

		fmt.Printf("   ⏳ Rate limit hit for %s, retrying in %v (attempt %d/%d)\n",
			operationName, delay.Round(time.Millisecond*100)/100, attempt+1, maxRetries)

		time.Sleep(delay)
	}

	return fmt.Errorf("failed to %s after %d retries", operationName, maxRetries)
}

// BuildContactImportConfigFromSES builds a ContactImportConfig from SES configuration
func BuildContactImportConfigFromSES(sesConfig SESConfig) ContactImportConfig {
	var roleMappings []RoleTopicMapping
	var defaultTopics []string

	// Get all expanded topics from the SES config
	expandedTopics := ExpandTopicsWithGroups(sesConfig)

	// Process each topic to build role mappings
	for _, topic := range expandedTopics {
		if len(topic.OptInRoles) > 0 {
			// Create a role mapping for this topic
			roleMappings = append(roleMappings, RoleTopicMapping{
				Roles:  topic.OptInRoles,
				Topics: []string{topic.TopicName},
			})
		}

		// If the topic has OPT_IN as default, add it to default topics
		if topic.DefaultSubscriptionStatus == "OPT_IN" {
			defaultTopics = append(defaultTopics, topic.TopicName)
		}
	}

	return ContactImportConfig{
		RoleMappings:       roleMappings,
		DefaultTopics:      defaultTopics,
		RequireActiveUsers: true,
	}
}

// ImportSingleAWSContact imports a single user from Identity Center to SES
func ImportSingleAWSContact(sesClient *sesv2.Client, identityCenterId string, userName string, dryRun bool) error {
	fmt.Printf("🔍 Importing single AWS contact: %s\n", userName)

	// Load Identity Center data from files
	users, memberships, actualId, err := LoadIdentityCenterDataFromFiles(identityCenterId)
	if err != nil {
		return fmt.Errorf("failed to load Identity Center data: %w", err)
	}
	identityCenterId = actualId

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

	// Load SES config and build configuration
	configPath := GetConfigPath()
	sesJson, err := os.ReadFile(configPath + "SESConfig.json")
	if err != nil {
		return fmt.Errorf("error reading SES config file: %v", err)
	}

	var sesConfig SESConfig
	err = json.NewDecoder(bytes.NewReader(sesJson)).Decode(&sesConfig)
	if err != nil {
		return fmt.Errorf("error parsing SES config: %v", err)
	}

	config := BuildContactImportConfigFromSES(sesConfig)

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

	// Add contact to SES with rate limiting
	rateLimiter := NewRateLimiter(5)
	defer rateLimiter.Stop()

	rateLimiter.Wait()
	err = AddContactToListQuiet(sesClient, accountListName, targetUser.Email, topics)
	if err != nil {
		return fmt.Errorf("failed to add contact %s to SES: %w", targetUser.Email, err)
	}

	fmt.Printf("✅ Successfully imported contact: %s (%s) with topics: %v\n", targetUser.DisplayName, targetUser.Email, topics)
	return nil
}
