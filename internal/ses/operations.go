// Package ses provides SES operations and email management functionality.
package ses

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	awsic "ccoe-customer-contact-manager/internal/aws"
	"ccoe-customer-contact-manager/internal/ses/templates"
	"ccoe-customer-contact-manager/internal/types"
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
		// No contact list exists - create one with topics from SESConfig.json
		fmt.Printf("⚠️  No contact list found - creating default contact list\n")

		// Load SES config to get topics
		configPath := GetConfigPath()
		sesConfigFile := GetSESConfigFilePath()
		sesJson, err := os.ReadFile(configPath + sesConfigFile)
		if err != nil {
			return "", fmt.Errorf("no contact lists found and failed to read SES config file: %w", err)
		}

		var sesConfig types.SESConfig
		err = json.Unmarshal(sesJson, &sesConfig)
		if err != nil {
			return "", fmt.Errorf("no contact lists found and failed to parse SES config: %w", err)
		}

		// Expand topics with groups
		expandedTopics := ExpandTopicsWithGroups(sesConfig)

		// Create default contact list name
		listName := "ccoe-customer-contacts"
		description := "CCOE Customer Contact List"

		// Create the contact list with topics
		err = CreateContactList(sesClient, listName, description, expandedTopics)
		if err != nil {
			return "", fmt.Errorf("no contact lists found and failed to create default contact list: %w", err)
		}

		fmt.Printf("✅ Created default contact list: %s with %d topics\n", listName, len(expandedTopics))
		return listName, nil
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

// DescribeContactListBuffered returns the contact list description as a string instead of printing
func DescribeContactListBuffered(sesClient *sesv2.Client, listName string) (string, error) {
	var buf strings.Builder

	// Get contact list details
	listInput := &sesv2.GetContactListInput{
		ContactListName: aws.String(listName),
	}

	listResult, err := sesClient.GetContactList(context.Background(), listInput)
	if err != nil {
		return "", fmt.Errorf("failed to get contact list details for %s: %w", listName, err)
	}

	buf.WriteString("=== Contact List Details ===\n")
	buf.WriteString(fmt.Sprintf("Name: %s\n", *listResult.ContactListName))
	if listResult.Description != nil {
		buf.WriteString(fmt.Sprintf("Description: %s\n", *listResult.Description))
	}
	buf.WriteString(fmt.Sprintf("Created: %s\n", listResult.CreatedTimestamp.Format("2006-01-02 15:04:05 UTC")))
	buf.WriteString(fmt.Sprintf("Last Modified: %s\n", listResult.LastUpdatedTimestamp.Format("2006-01-02 15:04:05 UTC")))

	// Display topics
	if len(listResult.Topics) > 0 {
		buf.WriteString("\nTopics:\n")
		for _, topic := range listResult.Topics {
			buf.WriteString(fmt.Sprintf("  - %s", *topic.TopicName))
			if topic.DisplayName != nil && *topic.DisplayName != *topic.TopicName {
				buf.WriteString(fmt.Sprintf(" (%s)", *topic.DisplayName))
			}
			buf.WriteString(fmt.Sprintf(" - Default: %s\n", topic.DefaultSubscriptionStatus))
		}
	} else {
		buf.WriteString("\nTopics: None\n")
	}

	// Get contact count
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(listName),
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		buf.WriteString(fmt.Sprintf("\nContacts: Unable to retrieve count (%v)\n", err))
	} else {
		buf.WriteString(fmt.Sprintf("\nTotal Contacts: %d\n", len(contactsResult.Contacts)))

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
				buf.WriteString("\nSubscription Statistics:\n")
				for topicName, stats := range topicStats {
					optIn := stats["OPT_IN"]
					optOut := stats["OPT_OUT"]
					total := optIn + optOut
					buf.WriteString(fmt.Sprintf("  %s: %d opted in, %d opted out (of %d contacts)\n",
						topicName, optIn, optOut, total))
				}
			}

			if unsubscribedCount > 0 {
				buf.WriteString(fmt.Sprintf("\nUnsubscribed from all: %d contacts\n", unsubscribedCount))
			}

			buf.WriteString("\nRecent Contacts (up to 5):\n")
			limit := len(contactsResult.Contacts)
			if limit > 5 {
				limit = 5
			}
			for i := 0; i < limit; i++ {
				contact := contactsResult.Contacts[i]
				buf.WriteString(fmt.Sprintf("  - %s", *contact.EmailAddress))
				if contact.LastUpdatedTimestamp != nil {
					buf.WriteString(fmt.Sprintf(" (updated: %s)", contact.LastUpdatedTimestamp.Format("2006-01-02")))
				}
				buf.WriteString("\n")
			}
			if len(contactsResult.Contacts) > 5 {
				buf.WriteString(fmt.Sprintf("  ... and %d more contacts (use 'list-contacts' to see all)\n", len(contactsResult.Contacts)-5))
			}
		}
	}

	return buf.String(), nil
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
	backup.BackupMetadata.Tool = "ccoe-customer-contact-manager"
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

	// Display current topics for verification
	fmt.Printf("📋 Current Topics in Contact List (%d total):\n", len(currentTopics))
	if len(currentTopics) == 0 {
		fmt.Printf("   (none)\n")
	} else {
		// Sort topic names for consistent display
		var topicNames []string
		for name := range currentTopics {
			topicNames = append(topicNames, name)
		}
		sort.Strings(topicNames)

		for i, name := range topicNames {
			topic := currentTopics[name]
			displayName := name
			if topic.DisplayName != nil && *topic.DisplayName != "" {
				displayName = *topic.DisplayName
			}
			fmt.Printf("   %d. %s", i+1, name)
			if displayName != name {
				fmt.Printf(" (%s)", displayName)
			}
			fmt.Printf(" - Default: %s\n", topic.DefaultSubscriptionStatus)
		}
	}
	fmt.Printf("\n")

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

// stdoutMutex ensures only one goroutine redirects stdout at a time
var stdoutMutex sync.Mutex

// ManageTopicsBuffered manages topics and returns the output as a string instead of printing
// This uses a mutex to ensure stdout redirection doesn't conflict between concurrent goroutines
func ManageTopicsBuffered(sesClient *sesv2.Client, configTopics []types.SESTopicConfig, dryRun bool) (string, error) {
	stdoutMutex.Lock()
	defer stdoutMutex.Unlock()

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Channel to capture output
	outputChan := make(chan string)
	go func() {
		var buf strings.Builder
		io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	// Call the original function
	err := ManageTopics(sesClient, configTopics, dryRun)

	// Restore stdout and get output
	w.Close()
	os.Stdout = oldStdout
	output := <-outputChan

	return output, err
}

// DescribeAllTopicsBuffered provides detailed information about all topics and returns the output as a string
// This is useful for concurrent operations where output needs to be displayed sequentially
func DescribeAllTopicsBuffered(sesClient *sesv2.Client) (string, error) {
	var output strings.Builder

	// First get the account's main contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return "", fmt.Errorf("error finding account contact list: %w", err)
	}

	// Get contact list details to access topics
	listInput := &sesv2.GetContactListInput{
		ContactListName: aws.String(accountListName),
	}

	listResult, err := sesClient.GetContactList(context.Background(), listInput)
	if err != nil {
		return "", fmt.Errorf("failed to get contact list details: %w", err)
	}

	if len(listResult.Topics) == 0 {
		output.WriteString(fmt.Sprintf("No topics found in contact list '%s'\n", accountListName))
		return output.String(), nil
	}

	// Get all contacts to calculate subscription statistics
	contactsInput := &sesv2.ListContactsInput{
		ContactListName: aws.String(accountListName),
	}

	contactsResult, err := sesClient.ListContacts(context.Background(), contactsInput)
	if err != nil {
		return "", fmt.Errorf("failed to list contacts: %w", err)
	}

	output.WriteString(fmt.Sprintf("=== All Topics in Contact List: %s ===\n\n", accountListName))

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

		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, topicName))
		if topic.DisplayName != nil && *topic.DisplayName != topicName {
			output.WriteString(fmt.Sprintf("   Display Name: %s\n", *topic.DisplayName))
		}
		output.WriteString(fmt.Sprintf("   Default Subscription: %s\n", topic.DefaultSubscriptionStatus))
		output.WriteString(fmt.Sprintf("   Subscriptions: %d opted in, %d opted out (%d total)\n",
			optInCount, optOutCount, optInCount+optOutCount))
		output.WriteString("\n")
	}

	return output.String(), nil
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

This update was performed automatically by the CCOE Customer Contact Manager.

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

// autoDetectIdentityCenterId detects Identity Center ID from existing files
func autoDetectIdentityCenterId(configPath string) (string, error) {
	files, err := os.ReadDir(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config directory: %w", err)
	}

	for _, file := range files {
		if strings.Contains(file.Name(), "identity-center-users-") {
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

	sort.Strings(matchingFiles)
	return matchingFiles[len(matchingFiles)-1], nil
}

// LoadIdentityCenterDataFromFiles loads user and group membership data from JSON files
func LoadIdentityCenterDataFromFiles(identityCenterId string) ([]types.IdentityCenterUser, []types.IdentityCenterGroupMembership, string, error) {
	configPath := GetConfigPath()

	if identityCenterId == "" {
		detectedId, err := autoDetectIdentityCenterId(configPath)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to auto-detect identity center ID: %w", err)
		}
		identityCenterId = detectedId
		fmt.Printf("🔍 Auto-detected Identity Center ID: %s\n", identityCenterId)
	}

	userFile, err := findMostRecentFile(configPath, fmt.Sprintf("identity-center-users-%s-", identityCenterId))
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to find user data file: %w", err)
	}

	groupFile, err := findMostRecentFile(configPath, fmt.Sprintf("identity-center-group-memberships-user-centric-%s-", identityCenterId))
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to find group membership data file: %w", err)
	}

	userJson, err := os.ReadFile(configPath + userFile)
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to read user file %s: %w", userFile, err)
	}

	var users []types.IdentityCenterUser
	err = json.Unmarshal(userJson, &users)
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to parse user file %s: %w", userFile, err)
	}

	groupJson, err := os.ReadFile(configPath + groupFile)
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to read group membership file %s: %w", groupFile, err)
	}

	var memberships []types.IdentityCenterGroupMembership
	err = json.Unmarshal(groupJson, &memberships)
	if err != nil {
		return nil, nil, identityCenterId, fmt.Errorf("failed to parse group membership file %s: %w", groupFile, err)
	}

	fmt.Printf("📁 Loaded %d users from: %s\n", len(users), userFile)
	fmt.Printf("📁 Loaded %d group memberships from: %s\n", len(memberships), groupFile)

	return users, memberships, identityCenterId, nil
}

// GetConfigPath returns the config path (using current directory)
func GetConfigPath() string {
	configPath, exists := os.LookupEnv("CONFIG_PATH")
	if !exists || configPath == "" {
		return "./"
	}
	if !strings.HasSuffix(configPath, "/") {
		configPath += "/"
	}
	return configPath
}

// GetSESConfigFilePath returns the SES config file path
// Checks SES_CONFIG_FILE environment variable first, then defaults to SESConfig.json
// This allows Lambda deployments to specify a custom config file path
func GetSESConfigFilePath() string {
	// Check for environment variable override (for Lambda mode)
	sesConfigFile, exists := os.LookupEnv("SES_CONFIG_FILE")
	if exists && sesConfigFile != "" {
		return sesConfigFile
	}
	// Default to SESConfig.json
	return "SESConfig.json"
}

// DetermineUserTopics determines which topics a user should be subscribed to based on their group memberships
func DetermineUserTopics(user types.IdentityCenterUser, membership *types.IdentityCenterGroupMembership, config types.ContactImportConfig) []string {
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

// BuildContactImportConfigFromSES builds a ContactImportConfig from SES configuration
func BuildContactImportConfigFromSES(sesConfig types.SESConfig) types.ContactImportConfig {
	var roleMappings []types.RoleTopicMapping
	var defaultTopics []string

	// Get all expanded topics from the SES config
	expandedTopics := ExpandTopicsWithGroups(sesConfig)

	// Process each topic to build role mappings
	for _, topic := range expandedTopics {
		if len(topic.OptInRoles) > 0 {
			// Create a role mapping for this topic
			roleMappings = append(roleMappings, types.RoleTopicMapping{
				Roles:  topic.OptInRoles,
				Topics: []string{topic.TopicName},
			})
		}

		// If the topic has OPT_IN as default, add it to default topics
		if topic.DefaultSubscriptionStatus == "OPT_IN" {
			defaultTopics = append(defaultTopics, topic.TopicName)
		}
	}

	return types.ContactImportConfig{
		RoleMappings:       roleMappings,
		DefaultTopics:      defaultTopics,
		RequireActiveUsers: true,
	}
}

// GetDefaultContactImportConfig returns the default role-to-topic mapping configuration
func GetDefaultContactImportConfig() types.ContactImportConfig {
	return types.ContactImportConfig{
		RoleMappings: []types.RoleTopicMapping{
			{
				Roles:  []string{"security", "devops", "cloudeng", "networking"},
				Topics: []string{"aws-calendar", "aws-announce"},
			},
		},
		DefaultTopics:      []string{"general-updates"},
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
	var targetUser *types.IdentityCenterUser
	var targetMembership *types.IdentityCenterGroupMembership

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
	sesConfigFile := GetSESConfigFilePath()
	sesJson, err := os.ReadFile(configPath + sesConfigFile)
	if err != nil {
		return fmt.Errorf("error reading SES config file: %v", err)
	}

	var sesConfig types.SESConfig
	err = json.Unmarshal(sesJson, &sesConfig)
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

	// Check if contact already exists
	existingContacts, err := getExistingContacts(sesClient, accountListName)
	if err != nil {
		return fmt.Errorf("failed to check existing contacts: %w", err)
	}

	if _, exists := existingContacts[targetUser.Email]; exists {
		fmt.Printf("ℹ️  Contact %s (%s) already exists - skipping (users manage their own subscriptions)\n", targetUser.DisplayName, targetUser.Email)
		return nil
	}

	// Add contact to SES with rate limiting
	rateLimiter := NewRateLimiter(5)
	defer rateLimiter.Stop()

	rateLimiter.Wait()
	err = AddContactToListQuiet(sesClient, accountListName, targetUser.Email, topics)
	if err != nil {
		// Check if it's an AlreadyExistsException
		errMsg := err.Error()
		if strings.Contains(errMsg, "AlreadyExistsException") || strings.Contains(errMsg, "already exists") {
			fmt.Printf("ℹ️  Contact %s (%s) already exists - skipping\n", targetUser.DisplayName, targetUser.Email)
			return nil
		}
		return fmt.Errorf("failed to add contact %s to SES: %w", targetUser.Email, err)
	}

	fmt.Printf("✅ Successfully imported contact: %s (%s) with topics: %v\n", targetUser.DisplayName, targetUser.Email, topics)
	return nil
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

	_, err := sesClient.CreateContact(context.Background(), input)
	return err
}

// CCOECloudGroupParseResult represents parsed information from ccoe-cloud group names for import
type CCOECloudGroupParseResult struct {
	GroupName         string
	AccountName       string
	AccountId         string
	ApplicationPrefix string
	RoleName          string
	IsValid           bool
}

// ParseCCOECloudGroup parses ccoe-cloud group names to extract AWS account information
func ParseCCOECloudGroup(groupName string) CCOECloudGroupParseResult {
	result := CCOECloudGroupParseResult{
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

// isAllDigits checks if string contains only digits
func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
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

// slicesEqual checks if two string slices are equal
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

// Logger interface for flexible logging (can be buffered or direct)
type Logger interface {
	Printf(format string, args ...interface{})
}

// DefaultLogger uses fmt.Printf for direct output
type DefaultLogger struct{}

func (l *DefaultLogger) Printf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// ImportAllAWSContacts imports all users from Identity Center to SES
// If identityCenterData is provided, it uses the in-memory data; otherwise, it loads from files
func ImportAllAWSContacts(sesClient *sesv2.Client, identityCenterId string, identityCenterData *awsic.IdentityCenterData, dryRun bool, requestsPerSecond int) error {
	return ImportAllAWSContactsWithLogger(sesClient, identityCenterId, identityCenterData, dryRun, requestsPerSecond, &DefaultLogger{})
}

// ImportAllAWSContactsWithLogger imports all users with custom logger
func ImportAllAWSContactsWithLogger(sesClient *sesv2.Client, identityCenterId string, identityCenterData *awsic.IdentityCenterData, dryRun bool, requestsPerSecond int, logger Logger) error {
	logger.Printf("🔍 Importing all AWS contacts from Identity Center")

	var users []types.IdentityCenterUser
	var memberships []types.IdentityCenterGroupMembership
	dataSource := "file-based"

	// Use in-memory data if provided, otherwise load from files
	if identityCenterData != nil {
		dataSource = "in-memory"
		fmt.Printf("📊 Using in-memory Identity Center data (data source: %s)\n", dataSource)
		users = identityCenterData.Users
		memberships = identityCenterData.Memberships
		identityCenterId = identityCenterData.InstanceID
		fmt.Printf("📊 Loaded %d users and %d group memberships from memory (instance: %s)\n", len(users), len(memberships), identityCenterId)
	} else {
		fmt.Printf("📁 Loading Identity Center data from files (data source: %s)\n", dataSource)
		var actualId string
		var err error
		users, memberships, actualId, err = LoadIdentityCenterDataFromFiles(identityCenterId)
		if err != nil {
			return fmt.Errorf("failed to load Identity Center data: %w", err)
		}
		identityCenterId = actualId // Use the actual ID (either provided or auto-detected)
		fmt.Printf("📁 Loaded %d users and %d group memberships from files (instance: %s)\n", len(users), len(memberships), identityCenterId)
	}

	// Create membership lookup map
	membershipMap := make(map[string]*types.IdentityCenterGroupMembership)
	for i, membership := range memberships {
		membershipMap[membership.UserName] = &memberships[i]
	}

	// Load SES config and build configuration
	configPath := GetConfigPath()
	sesConfigFile := GetSESConfigFilePath()
	sesJson, err := os.ReadFile(configPath + sesConfigFile)
	if err != nil {
		return fmt.Errorf("error reading SES config file: %v", err)
	}

	var sesConfig types.SESConfig
	err = json.Unmarshal(sesJson, &sesConfig)
	if err != nil {
		return fmt.Errorf("error parsing SES config: %v", err)
	}

	config := BuildContactImportConfigFromSES(sesConfig)

	// Create rate limiter for SES operations
	// Use 1 request per second for contact operations to avoid AlreadyExistsException and rate limiting
	// Contact creation is particularly sensitive and needs aggressive rate limiting
	sesRateLimit := 1
	fmt.Printf("⚙️  Rate limiting: %d request per second (conservative rate for contact operations)\n", sesRateLimit)
	rateLimiter := NewRateLimiter(sesRateLimit)
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

	// Build sets for efficient lookup
	// Set of valid Identity Center users (email -> user data)
	validUsers := make(map[string]struct {
		user       types.IdentityCenterUser
		membership *types.IdentityCenterGroupMembership
		topics     []string
	})

	fmt.Printf("👥 Processing %d Identity Center users...\n", len(users))

	for _, user := range users {
		// Skip inactive users if required
		if config.RequireActiveUsers && !user.Active {
			continue
		}

		// Get user's membership
		membership := membershipMap[user.UserName]

		// Determine topics
		topics := DetermineUserTopics(user, membership, config)

		// Skip users with no valid topics
		hasValidTopics := false
		for _, topic := range topics {
			if strings.TrimSpace(topic) != "" {
				hasValidTopics = true
				break
			}
		}

		if !hasValidTopics {
			continue
		}

		validUsers[user.Email] = struct {
			user       types.IdentityCenterUser
			membership *types.IdentityCenterGroupMembership
			topics     []string
		}{user, membership, topics}
	}

	fmt.Printf("✅ Found %d valid Identity Center users\n", len(validUsers))

	// Determine who to add and who to remove
	var usersToAdd []string
	var contactsToRemove []string

	// Find users to add (in Identity Center but not in SES)
	for email := range validUsers {
		if _, exists := existingContacts[email]; !exists {
			usersToAdd = append(usersToAdd, email)
		}
	}

	// Find contacts to remove (in SES but not in Identity Center)
	for email := range existingContacts {
		if _, exists := validUsers[email]; !exists {
			contactsToRemove = append(contactsToRemove, email)
		}
	}

	fmt.Printf("\n📊 Sync Summary:\n")
	fmt.Printf("   ➕ Users to add: %d\n", len(usersToAdd))
	fmt.Printf("   ➖ Contacts to remove: %d\n", len(contactsToRemove))
	fmt.Printf("   ✅ Already in sync: %d\n", len(validUsers)-len(usersToAdd))

	if dryRun {
		if len(usersToAdd) > 0 {
			fmt.Printf("\n🔍 Would add these users:\n")
			for i, email := range usersToAdd {
				if i < 5 { // Show first 5
					userData := validUsers[email]
					fmt.Printf("   - %s → topics: %v\n", email, userData.topics)
				}
			}
			if len(usersToAdd) > 5 {
				fmt.Printf("   ... and %d more\n", len(usersToAdd)-5)
			}
		}
		if len(contactsToRemove) > 0 {
			fmt.Printf("\n🔍 Would remove these contacts:\n")
			for i, email := range contactsToRemove {
				if i < 5 { // Show first 5
					fmt.Printf("   - %s\n", email)
				}
			}
			if len(contactsToRemove) > 5 {
				fmt.Printf("   ... and %d more\n", len(contactsToRemove)-5)
			}
		}
		return nil
	}

	// Add new users
	addedCount := 0
	addErrorCount := 0
	if len(usersToAdd) > 0 {
		fmt.Printf("\n➕ Adding %d new contacts...\n", len(usersToAdd))
		for i, email := range usersToAdd {
			// Show progress for large imports
			if len(usersToAdd) > 10 && (i+1)%10 == 0 {
				fmt.Printf("📊 Progress: %d/%d contacts added (%d%% complete)\n",
					i+1, len(usersToAdd), (i+1)*100/len(usersToAdd))
			}

			userData := validUsers[email]

			// Rate limit SES operations
			rateLimiter.Wait()

			// Add contact to SES
			err = AddContactToListQuiet(sesClient, accountListName, email, userData.topics)
			if err != nil {
				// Check if it's an AlreadyExistsException
				errMsg := err.Error()
				if strings.Contains(errMsg, "AlreadyExistsException") || strings.Contains(errMsg, "already exists") {
					// Already exists, count as success
					addedCount++
					continue
				}
				// Log first few errors
				if addErrorCount < 3 {
					fmt.Printf("   ❌ Failed to add contact %s: %v\n", email, err)
				}
				addErrorCount++
				continue
			}

			addedCount++
		}
	}

	// Remove old contacts
	removedCount := 0
	removeErrorCount := 0
	if len(contactsToRemove) > 0 {
		fmt.Printf("\n➖ Removing %d old contacts...\n", len(contactsToRemove))
		for i, email := range contactsToRemove {
			// Show progress for large removals
			if len(contactsToRemove) > 10 && (i+1)%10 == 0 {
				fmt.Printf("📊 Progress: %d/%d contacts removed (%d%% complete)\n",
					i+1, len(contactsToRemove), (i+1)*100/len(contactsToRemove))
			}

			// Rate limit SES operations
			rateLimiter.Wait()

			// Remove contact from SES
			err = RemoveContactFromList(sesClient, accountListName, email)
			if err != nil {
				// Log first few errors
				if removeErrorCount < 3 {
					fmt.Printf("   ❌ Failed to remove contact %s: %v\n", email, err)
				}
				removeErrorCount++
				continue
			}

			removedCount++
		}
	}

	fmt.Printf("\n📊 Final Summary:\n")
	fmt.Printf("   ➕ Added: %d\n", addedCount)
	fmt.Printf("   ➖ Removed: %d\n", removedCount)
	fmt.Printf("   ❌ Add errors: %d\n", addErrorCount)
	fmt.Printf("   ❌ Remove errors: %d\n", removeErrorCount)
	fmt.Printf("   ✅ Total in sync: %d\n", len(validUsers))

	if addErrorCount > 0 || removeErrorCount > 0 {
		return fmt.Errorf("failed to sync %d contacts", addErrorCount+removeErrorCount)
	}

	return nil
}

// SendEmailWithTemplate sends an email using the new template system
// This function integrates with the template registry to generate standardized emails
func SendEmailWithTemplate(
	ctx context.Context,
	sesClient *sesv2.Client,
	emailConfig types.EmailConfig,
	eventType string,
	notificationType templates.NotificationType,
	data interface{},
	topicName string,
) error {
	// Initialize template registry with email config
	registry := templates.NewTemplateRegistry(emailConfig)

	// Get the template
	emailTemplate, err := registry.GetTemplate(eventType, notificationType, data)
	if err != nil {
		return fmt.Errorf("failed to get template: %w", err)
	}

	// Get account contact list
	accountListName, err := GetAccountContactList(sesClient)
	if err != nil {
		return fmt.Errorf("failed to get account contact list: %w", err)
	}

	// Get subscribed contacts for the topic
	subscribedContacts, err := getSubscribedContactsForTopic(sesClient, accountListName, topicName)
	if err != nil {
		return fmt.Errorf("failed to get subscribed contacts for topic '%s': %w", topicName, err)
	}

	if len(subscribedContacts) == 0 {
		log.Printf("⚠️  No contacts are subscribed to topic '%s'", topicName)
		return nil
	}

	log.Printf("📧 Sending email to topic '%s' (%d subscribers)", topicName, len(subscribedContacts))

	// Send email to each subscribed contact
	successCount := 0
	errorCount := 0

	for _, contact := range subscribedContacts {
		sendInput := &sesv2.SendEmailInput{
			FromEmailAddress: aws.String(emailConfig.SenderAddress),
			Destination: &sesv2Types.Destination{
				ToAddresses: []string{*contact.EmailAddress},
			},
			Content: &sesv2Types.EmailContent{
				Simple: &sesv2Types.Message{
					Subject: &sesv2Types.Content{
						Data: aws.String(emailTemplate.Subject),
					},
					Body: &sesv2Types.Body{
						Html: &sesv2Types.Content{
							Data: aws.String(emailTemplate.HTMLBody),
						},
						Text: &sesv2Types.Content{
							Data: aws.String(emailTemplate.TextBody),
						},
					},
				},
			},
			ListManagementOptions: &sesv2Types.ListManagementOptions{
				ContactListName: aws.String(accountListName),
				TopicName:       aws.String(topicName),
			},
		}

		_, err := sesClient.SendEmail(ctx, sendInput)
		if err != nil {
			log.Printf("❌ Failed to send email to %s: %v", *contact.EmailAddress, err)
			errorCount++
		} else {
			log.Printf("✅ Sent email to %s", *contact.EmailAddress)
			successCount++
		}
	}

	log.Printf("📊 Email Summary: %d successful, %d errors", successCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("failed to send email to %d recipients", errorCount)
	}

	return nil
}
