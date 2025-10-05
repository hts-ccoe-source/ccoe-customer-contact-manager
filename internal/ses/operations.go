// Package ses provides SES operations and email management functionality.
package ses

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"aws-alternate-contact-manager/internal/types"
)

// CredentialManager interface for dependency injection
type CredentialManager interface {
	GetCustomerConfig(customerCode string) (aws.Config, error)
	GetCustomerInfo(customerCode string) (types.CustomerAccountInfo, error)
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

// CreateContactList creates a new contact list in SES
func CreateContactList(sesClient *sesv2.Client, listName string, description string, topicConfigs []types.SESTopicConfig) error {
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

	_, err := sesClient.DeleteContact(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to remove contact %s from list %s: %w", email, listName, err)
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
func ExpandTopicsWithGroups(sesConfig types.SESConfig) []types.SESTopicConfig {
	var expandedTopics []types.SESTopicConfig

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

			expandedTopic := types.SESTopicConfig{
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

// DescribeTopic provides detailed information about a specific topic
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

// validateContactListTopics checks if the required topics exist in the contact list
func validateContactListTopics(sesClient *sesv2.Client, listName string, config types.ContactImportConfig) error {
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

// AddOrUpdateContactToList adds a contact to a list or updates existing contact's topic subscriptions (idempotent)
// Returns: "created", "updated", or "unchanged" to indicate what action was taken
func AddOrUpdateContactToList(sesClient *sesv2.Client, listName string, email string, explicitTopics []string) (string, error) {
	// First, try to get the existing contact
	getInput := &sesv2.GetContactInput{
		ContactListName: aws.String(listName),
		EmailAddress:    aws.String(email),
	}

	existingContact, err := sesClient.GetContact(context.Background(), getInput)
	if err != nil {
		// Contact doesn't exist, create it
		err = AddContactToList(sesClient, listName, email, explicitTopics)
		if err != nil {
			return "", err
		}
		return "created", nil
	}

	// Contact exists, update their explicit topic subscriptions
	// Get current explicit subscriptions
	currentExplicitTopics := []string{}
	for _, pref := range existingContact.TopicPreferences {
		if pref.SubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
			currentExplicitTopics = append(currentExplicitTopics, *pref.TopicName)
		}
	}

	// Check if the explicit topics are already the same
	if areTopicListsEqual(currentExplicitTopics, explicitTopics) {
		fmt.Printf("✅ Contact %s already has the correct topic subscriptions\n", email)
		return "unchanged", nil
	}

	// Update the contact's explicit topic subscriptions
	err = updateContactSubscription(sesClient, listName, email, explicitTopics)
	if err != nil {
		return "", fmt.Errorf("failed to update existing contact %s: %w", email, err)
	}

	fmt.Printf("🔄 Updated existing contact %s with new topic subscriptions\n", email)
	return "updated", nil
}

// areTopicListsEqual checks if two topic lists contain the same topics (order doesn't matter)
func areTopicListsEqual(list1, list2 []string) bool {
	if len(list1) != len(list2) {
		return false
	}

	set1 := make(map[string]bool)
	for _, topic := range list1 {
		set1[topic] = true
	}

	for _, topic := range list2 {
		if !set1[topic] {
			return false
		}
	}

	return true
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
		err = AddContactToList(sesClient, listName, email, topics)
		if err != nil {
			return fmt.Errorf("failed to re-add contact with updated subscriptions: %w", err)
		}
	}

	return nil
}

// AddContactTopics adds explicit topic subscriptions to an existing contact (idempotent)
func AddContactTopics(sesClient *sesv2.Client, listName string, email string, topics []string) error {
	if len(topics) == 0 {
		return fmt.Errorf("no topics specified")
	}

	// Get current contact details
	getInput := &sesv2.GetContactInput{
		ContactListName: aws.String(listName),
		EmailAddress:    aws.String(email),
	}

	contact, err := sesClient.GetContact(context.Background(), getInput)
	if err != nil {
		return fmt.Errorf("failed to get contact %s: %w", email, err)
	}

	// Create a list of existing explicit topic subscriptions
	existingTopics := []string{}
	for _, pref := range contact.TopicPreferences {
		if pref.SubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
			existingTopics = append(existingTopics, *pref.TopicName)
		}
	}

	// Check which topics need to be added
	topicsToAdd := []string{}
	existingSet := make(map[string]bool)
	for _, topic := range existingTopics {
		existingSet[topic] = true
	}

	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic != "" && !existingSet[topic] {
			topicsToAdd = append(topicsToAdd, topic)
		}
	}

	if len(topicsToAdd) == 0 {
		fmt.Printf("✅ Contact %s already subscribed to all specified topics\n", email)
		return nil
	}

	// Create new topics list with existing + new topics
	allTopics := append(existingTopics, topicsToAdd...)

	// Use the existing updateContactSubscription function
	err = updateContactSubscription(sesClient, listName, email, allTopics)
	if err != nil {
		return fmt.Errorf("failed to update contact %s topic subscriptions: %w", email, err)
	}

	fmt.Printf("✅ Successfully added topic subscriptions for %s: %v\n", email, topicsToAdd)
	return nil
}

// RemoveContactTopics removes explicit topic subscriptions from an existing contact (idempotent)
func RemoveContactTopics(sesClient *sesv2.Client, listName string, email string, topics []string) error {
	if len(topics) == 0 {
		return fmt.Errorf("no topics specified")
	}

	// Get current contact details
	getInput := &sesv2.GetContactInput{
		ContactListName: aws.String(listName),
		EmailAddress:    aws.String(email),
	}

	contact, err := sesClient.GetContact(context.Background(), getInput)
	if err != nil {
		return fmt.Errorf("failed to get contact %s: %w", email, err)
	}

	// Create a map of topics to remove
	topicsToRemove := make(map[string]bool)
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic != "" {
			topicsToRemove[topic] = true
		}
	}

	// Build list of remaining explicit topics
	remainingTopics := []string{}
	removedTopics := []string{}

	for _, pref := range contact.TopicPreferences {
		if pref.SubscriptionStatus == sesv2Types.SubscriptionStatusOptIn {
			if topicsToRemove[*pref.TopicName] {
				removedTopics = append(removedTopics, *pref.TopicName)
			} else {
				remainingTopics = append(remainingTopics, *pref.TopicName)
			}
		}
	}

	if len(removedTopics) == 0 {
		fmt.Printf("✅ Contact %s was not explicitly subscribed to any of the specified topics\n", email)
		return nil
	}

	// Use the existing updateContactSubscription function
	err = updateContactSubscription(sesClient, listName, email, remainingTopics)
	if err != nil {
		return fmt.Errorf("failed to update contact %s topic subscriptions: %w", email, err)
	}

	fmt.Printf("✅ Successfully removed topic subscriptions for %s: %v\n", email, removedTopics)
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
	backup := types.SESBackup{}

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
		contactBackup := types.SESContactBackup{
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

	// Use config package to get path
	configPath := "." // Default path, should be replaced with proper config function
	backupPath := configPath + "/" + backupFilename

	backupJson, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal backup data: %w", err)
	}

	err = os.WriteFile(backupPath, backupJson, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write backup file: %w", err)
	}

	fmt.Printf("✅ Backup saved to: %s\n", backupFilename)
	fmt.Printf("📊 Backed up %d contacts and %d topics\n", len(backup.Contacts), len(listResult.Topics))

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

// ManageTopics manages topics in the account's contact list based on configuration
func ManageTopics(sesClient *sesv2.Client, configTopics []types.SESTopicConfig, dryRun bool) error {
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

	configTopicsMap := make(map[string]types.SESTopicConfig)
	for _, topic := range configTopics {
		configTopicsMap[topic.TopicName] = topic
	}

	// Track changes needed
	var topicsToAdd []types.SESTopicConfig
	var topicsToUpdate []types.SESTopicConfig
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

	fmt.Printf("📋 Changes needed:\n")
	if len(topicsToAdd) > 0 {
		fmt.Printf("  Topics to add: %d\n", len(topicsToAdd))
		for _, topic := range topicsToAdd {
			fmt.Printf("    + %s\n", topic.TopicName)
		}
	}
	if len(topicsToUpdate) > 0 {
		fmt.Printf("  Topics to update: %d\n", len(topicsToUpdate))
		for _, topic := range topicsToUpdate {
			fmt.Printf("    ~ %s\n", topic.TopicName)
		}
	}
	if len(topicsToRemove) > 0 {
		fmt.Printf("  Topics to remove: %d\n", len(topicsToRemove))
		for _, topicName := range topicsToRemove {
			fmt.Printf("    - %s\n", topicName)
		}
	}

	if dryRun {
		fmt.Printf("\nDRY RUN: No changes were made\n")
		return nil
	}

	// Note: Full topic management implementation would require additional SES API calls
	// that are not currently available in the AWS SDK for updating topics in place.
	// This is a simplified version that shows what would be done.
	fmt.Printf("\nNote: Topic management requires recreating the contact list with new topics.\n")
	fmt.Printf("This is a complex operation that should be done carefully in production.\n")

	return nil
}

// DescribeAllTopics provides detailed information about all topics
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

// EmailManager handles email operations
type EmailManager struct {
	credentialManager CredentialManager
	contactConfig     types.AlternateContactConfig
}

// NewEmailManager creates a new email manager
func NewEmailManager(credentialManager CredentialManager, contactConfig types.AlternateContactConfig) *EmailManager {
	return &EmailManager{
		credentialManager: credentialManager,
		contactConfig:     contactConfig,
	}
}

// ValidateEmailConfiguration validates the email configuration
func (em *EmailManager) ValidateEmailConfiguration(customerCode string) error {
	// Validate customer exists
	_, err := em.credentialManager.GetCustomerInfo(customerCode)
	if err != nil {
		return err
	}

	// Validate we can get SES client
	customerConfig, err := em.credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config: %w", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Test SES access by getting account sending enabled status
	_, err = sesClient.GetAccount(context.Background(), &sesv2.GetAccountInput{})
	if err != nil {
		return fmt.Errorf("failed to access SES for customer %s: %w", customerCode, err)
	}

	fmt.Printf("Email configuration validated for customer %s\n", customerCode)
	return nil
}

// SendAlternateContactNotification sends alternate contact update notification
func (em *EmailManager) SendAlternateContactNotification(customerCode string, changeDetails map[string]interface{}) error {
	customerConfig, err := em.credentialManager.GetCustomerConfig(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer config: %w", err)
	}

	customerInfo, err := em.credentialManager.GetCustomerInfo(customerCode)
	if err != nil {
		return fmt.Errorf("failed to get customer info: %w", err)
	}

	sesClient := sesv2.NewFromConfig(customerConfig)

	// Prepare email content
	subject := fmt.Sprintf("AWS Alternate Contact Update - %s", customerInfo.CustomerName)
	body := fmt.Sprintf(`AWS Alternate Contact Update Notification

Customer: %s
Account ID: %s
Environment: %s

The following alternate contacts have been updated:

Security Contact:
- Email: %s
- Name: %s
- Title: %s
- Phone: %s

Billing Contact:
- Email: %s
- Name: %s
- Title: %s
- Phone: %s

Operations Contact:
- Email: %s
- Name: %s
- Title: %s
- Phone: %s

This update was performed automatically by the AWS Alternate Contact Manager.

Best regards,
AWS Operations Team
`,
		customerInfo.CustomerName,
		customerInfo.GetAccountID(),
		customerInfo.Environment,
		em.contactConfig.SecurityEmail,
		em.contactConfig.SecurityName,
		em.contactConfig.SecurityTitle,
		em.contactConfig.SecurityPhone,
		em.contactConfig.BillingEmail,
		em.contactConfig.BillingName,
		em.contactConfig.BillingTitle,
		em.contactConfig.BillingPhone,
		em.contactConfig.OperationsEmail,
		em.contactConfig.OperationsName,
		em.contactConfig.OperationsTitle,
		em.contactConfig.OperationsPhone,
	)

	// Get recipients (use aws-announce topic by default)
	recipients, err := em.getTopicSubscribers(customerCode, "aws-announce", customerConfig)
	if err != nil {
		return fmt.Errorf("failed to get recipients: %w", err)
	}

	if len(recipients) == 0 {
		fmt.Printf("No recipients found for customer %s, skipping email notification\n", customerCode)
		return nil
	}

	// Send email
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String("ccoe@nonprod.ccoe.hearst.com"),
		Destination: &sesv2Types.Destination{
			ToAddresses: recipients,
		},
		Content: &sesv2Types.EmailContent{
			Simple: &sesv2Types.Message{
				Subject: &sesv2Types.Content{
					Data: aws.String(subject),
				},
				Body: &sesv2Types.Body{
					Text: &sesv2Types.Content{
						Data: aws.String(body),
					},
				},
			},
		},
	}

	result, err := sesClient.SendEmail(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	fmt.Printf("Email sent successfully to %v for customer %s (MessageId: %s)\n",
		recipients, customerCode, *result.MessageId)

	return nil
}

// getTopicSubscribers returns email addresses subscribed to a specific SES topic
func (em *EmailManager) getTopicSubscribers(customerCode, topicName string, customerConfig aws.Config) ([]string, error) {
	sesClient := sesv2.NewFromConfig(customerConfig)

	// Get the account's main contact list dynamically
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get account contact list: %w", err)
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
		return nil, fmt.Errorf("failed to list contacts for topic '%s': %w", topicName, err)
	}

	var subscribers []string
	for _, contact := range contactsResult.Contacts {
		if contact.EmailAddress != nil {
			subscribers = append(subscribers, *contact.EmailAddress)
		}
	}

	return subscribers, nil
}

// ManageSESLists handles SES list management operations
func ManageSESLists(action string, sesConfigFile string, backupFile string, email string, topics []string, suppressionReason string, topicName string, dryRun bool, sesRoleArn string, mgmtRoleArn string, identityCenterId string, userName string, maxConcurrency int, requestsPerSecond int, senderEmail string) error {
	// This is a simplified implementation for the integration
	// The full implementation would be moved from the root-level ses.go file
	return fmt.Errorf("ManageSESLists not fully implemented in internal package yet - use root-level ses.go for now")
}
