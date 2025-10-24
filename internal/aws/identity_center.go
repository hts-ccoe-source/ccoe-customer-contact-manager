// Package aws provides Identity Center integration functions.
package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoreTypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"ccoe-customer-contact-manager/internal/config"
	"ccoe-customer-contact-manager/internal/types"
)

// IdentityCenterData holds user and group membership information retrieved from Identity Center
type IdentityCenterData struct {
	Users       []types.IdentityCenterUser            `json:"users"`
	Memberships []types.IdentityCenterGroupMembership `json:"memberships"`
	InstanceID  string                                `json:"instance_id"`
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

// extractInstanceIDFromArn extracts the Identity Center instance ID from an ARN
// Expected ARN format: arn:aws:sso:::instance/ssoins-xxxxxxxxxx
// Returns the instance ID in format: d-xxxxxxxxxx or ssoins-xxxxxxxxxx
func extractInstanceIDFromArn(instanceArn string) (string, error) {
	// Pattern to match instance ID in ARN
	// Supports both formats: ssoins-xxxxxxxxxx and d-xxxxxxxxxx
	re := regexp.MustCompile(`instance/((?:ssoins-|d-)[a-z0-9]+)`)
	matches := re.FindStringSubmatch(instanceArn)

	if len(matches) < 2 {
		return "", fmt.Errorf("failed to extract instance ID from ARN: %s", instanceArn)
	}

	return matches[1], nil
}

// DiscoverIdentityCenterInstanceID discovers the Identity Center instance ID from the account
// It validates that exactly one instance exists and returns its Identity Store ID
func DiscoverIdentityCenterInstanceID(cfg aws.Config) (string, error) {
	return DiscoverIdentityCenterInstanceIDWithLogger(cfg, &DefaultLogger{})
}

// DiscoverIdentityCenterInstanceIDWithLogger discovers the Identity Center instance ID with custom logger
func DiscoverIdentityCenterInstanceIDWithLogger(cfg aws.Config, logger Logger) (string, error) {
	ssoAdminClient := ssoadmin.NewFromConfig(cfg)

	result, err := ssoAdminClient.ListInstances(context.Background(),
		&ssoadmin.ListInstancesInput{})
	if err != nil {
		return "", fmt.Errorf("failed to list Identity Center instances: %w", err)
	}

	if len(result.Instances) == 0 {
		return "", fmt.Errorf("no Identity Center instances found in account")
	}

	if len(result.Instances) > 1 {
		return "", fmt.Errorf("multiple Identity Center instances found (%d), expected exactly one", len(result.Instances))
	}

	// Get the Identity Store ID directly from the instance
	// This is the correct ID format (d-xxxxxxxxxx) for Identity Store API calls
	if result.Instances[0].IdentityStoreId == nil {
		return "", fmt.Errorf("Identity Center instance has no Identity Store ID")
	}

	identityStoreID := *result.Instances[0].IdentityStoreId
	logger.Printf("🔍 Discovered Identity Center instance: %s (Identity Store ID: %s)",
		*result.Instances[0].InstanceArn, identityStoreID)
	return identityStoreID, nil
}

// ListIdentityCenterUser lists a specific user from Identity Center
func ListIdentityCenterUser(identityStoreClient *identitystore.Client, identityStoreId string, userName string) (*types.IdentityCenterUser, error) {
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
	identityCenterUser := convertToIdentityCenterUser(user)
	return &identityCenterUser, nil
}

// convertToIdentityCenterUser converts AWS SDK user type to our custom type
func convertToIdentityCenterUser(user identitystoreTypes.User) types.IdentityCenterUser {
	// Extract email from user attributes
	var email string
	if user.Emails != nil && len(user.Emails) > 0 {
		email = *user.Emails[0].Value
	}

	var firstName, lastName string
	if user.Name != nil {
		if user.Name.GivenName != nil {
			firstName = *user.Name.GivenName
		}
		if user.Name.FamilyName != nil {
			lastName = *user.Name.FamilyName
		}
	}

	return types.IdentityCenterUser{
		UserId:      *user.UserId,
		UserName:    *user.UserName,
		DisplayName: *user.DisplayName,
		Email:       email,
		GivenName:   firstName,
		FamilyName:  lastName,
		Active:      true, // Assume active when listing
	}
}

// ListIdentityCenterUsersAll lists all users from Identity Center with concurrency and rate limiting
func ListIdentityCenterUsersAll(identityStoreClient *identitystore.Client, identityStoreId string, maxConcurrency int, requestsPerSecond int) ([]types.IdentityCenterUser, error) {
	return ListIdentityCenterUsersAllWithLogger(identityStoreClient, identityStoreId, maxConcurrency, requestsPerSecond, &DefaultLogger{})
}

// ListIdentityCenterUsersAllWithLogger lists all users from Identity Center with concurrency, rate limiting, and custom logger
func ListIdentityCenterUsersAllWithLogger(identityStoreClient *identitystore.Client, identityStoreId string, maxConcurrency int, requestsPerSecond int, logger Logger) ([]types.IdentityCenterUser, error) {
	logger.Printf("🔍 Listing all users from Identity Center (ID: %s)", identityStoreId)
	logger.Printf("⚙️  Concurrency: %d workers, Rate limit: %d req/sec", maxConcurrency, requestsPerSecond)

	// Create rate limiter
	rateLimiter := NewRateLimiter(requestsPerSecond)
	defer rateLimiter.Stop()

	var allUsers []identitystoreTypes.User
	var nextToken *string

	// Paginate through all users
	for {
		rateLimiter.Wait()

		input := &identitystore.ListUsersInput{
			IdentityStoreId: aws.String(identityStoreId),
			NextToken:       nextToken,
		}

		result, err := identityStoreClient.ListUsers(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to list users: %w", err)
		}

		allUsers = append(allUsers, result.Users...)

		if result.NextToken == nil {
			break
		}
		nextToken = result.NextToken
	}

	logger.Printf("📊 Found %d total users", len(allUsers))

	// Process users with concurrency control
	identityCenterUsers := processUsersWithConcurrency(allUsers, maxConcurrency, rateLimiter)

	logger.Printf("✅ Successfully processed %d users", len(identityCenterUsers))
	return identityCenterUsers, nil
}

// processUsersWithConcurrency processes a batch of users with controlled concurrency
func processUsersWithConcurrency(users []identitystoreTypes.User, maxConcurrency int, rateLimiter *types.RateLimiter) []types.IdentityCenterUser {
	var wg sync.WaitGroup
	userChan := make(chan identitystoreTypes.User, len(users))
	resultChan := make(chan types.IdentityCenterUser, len(users))

	// Start workers
	for i := 0; i < maxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for user := range userChan {
				rateLimiter.Wait()
				identityCenterUser := convertToIdentityCenterUser(user)
				resultChan <- identityCenterUser
			}
		}()
	}

	// Send users to workers
	go func() {
		for _, user := range users {
			userChan <- user
		}
		close(userChan)
	}()

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []types.IdentityCenterUser
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// ListIdentityCenterUsersWithRole retrieves all users from Identity Center by assuming a role
// Returns the users slice for in-memory use
func ListIdentityCenterUsersWithRole(mgmtRoleArn string, identityCenterId string, maxConcurrency int, requestsPerSecond int) ([]types.IdentityCenterUser, error) {
	// Assume the management role
	cfg, err := assumeRoleAndGetConfig(mgmtRoleArn, "identity-center-user-listing")
	if err != nil {
		return nil, fmt.Errorf("failed to assume management role: %w", err)
	}

	// Create Identity Store client with assumed role
	identityStoreClient := identitystore.NewFromConfig(cfg)

	// List all users
	users, err := ListIdentityCenterUsersAll(identityStoreClient, identityCenterId, maxConcurrency, requestsPerSecond)
	if err != nil {
		return nil, fmt.Errorf("failed to list all users: %w", err)
	}

	return users, nil
}

// HandleIdentityCenterUserListing handles Identity Center user listing with role assumption
// This function maintains backward compatibility by writing files and displaying results
func HandleIdentityCenterUserListing(mgmtRoleArn string, identityCenterId string, userName string, listType string, maxConcurrency int, requestsPerSecond int) error {
	fmt.Printf("🔐 Assuming management role: %s\n", mgmtRoleArn)

	// Assume the management role
	cfg, err := assumeRoleAndGetConfig(mgmtRoleArn, "identity-center-user-listing")
	if err != nil {
		return fmt.Errorf("failed to assume management role: %w", err)
	}

	fmt.Printf("✅ Successfully assumed role\n")

	// Create Identity Store client with assumed role
	identityStoreClient := identitystore.NewFromConfig(cfg)

	if listType == "all" {
		// List all users
		users, err := ListIdentityCenterUsersAll(identityStoreClient, identityCenterId, maxConcurrency, requestsPerSecond)
		if err != nil {
			return fmt.Errorf("failed to list all users: %w", err)
		}

		// Save to JSON file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("identity-center-users-%s-%s.json", identityCenterId, timestamp)

		err = SaveIdentityCenterUsersToJSON(users, filename)
		if err != nil {
			return fmt.Errorf("failed to save users data: %w", err)
		}

		DisplayIdentityCenterUsers(users)
	} else {
		// List specific user
		user, err := ListIdentityCenterUser(identityStoreClient, identityCenterId, userName)
		if err != nil {
			return fmt.Errorf("failed to list user: %w", err)
		}

		// Save to JSON file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("identity-center-user-%s-%s-%s.json", identityCenterId, userName, timestamp)

		err = SaveIdentityCenterUserToJSON(user, filename)
		if err != nil {
			return fmt.Errorf("failed to save user data: %w", err)
		}

		DisplayIdentityCenterUser(user)
	}

	return nil
}

// SaveIdentityCenterUsersToJSON saves users data to a JSON file
func SaveIdentityCenterUsersToJSON(users []types.IdentityCenterUser, filename string) error {
	jsonData, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users data: %w", err)
	}

	configPath := config.GetConfigPath()
	filePath := configPath + filename

	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write users data to file: %w", err)
	}

	fmt.Printf("💾 Saved %d users to %s\n", len(users), filename)
	return nil
}

// SaveIdentityCenterUserToJSON saves single user data to a JSON file
func SaveIdentityCenterUserToJSON(user *types.IdentityCenterUser, filename string) error {
	jsonData, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	configPath := config.GetConfigPath()
	filePath := configPath + filename

	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write user data to file: %w", err)
	}

	fmt.Printf("💾 Saved user data to %s\n", filename)
	return nil
}

// DisplayIdentityCenterUser displays a single user in a formatted way
func DisplayIdentityCenterUser(user *types.IdentityCenterUser) {
	fmt.Printf("👤 User: %s\n", user.DisplayName)
	fmt.Printf("   User ID: %s\n", user.UserId)
	fmt.Printf("   Username: %s\n", user.UserName)
	fmt.Printf("   Email: %s\n", user.Email)
	fmt.Printf("   First Name: %s\n", user.GivenName)
	fmt.Printf("   Last Name: %s\n", user.FamilyName)
}

// DisplayIdentityCenterUsers displays multiple users in a formatted table
func DisplayIdentityCenterUsers(users []types.IdentityCenterUser) {
	if len(users) == 0 {
		fmt.Println("No users found.")
		return
	}

	fmt.Printf("\n📊 Identity Center Users (%d total):\n", len(users))
	fmt.Printf("%-40s %-25s %-30s %-20s\n", "Display Name", "Username", "Email", "User ID")
	fmt.Printf("%s\n", strings.Repeat("-", 115))

	for _, user := range users {
		email := user.Email
		if len(email) > 28 {
			email = email[:25] + "..."
		}

		fmt.Printf("%-40s %-25s %-30s %-20s\n",
			user.DisplayName,
			user.UserName,
			email,
			user.UserId[:8]+"...")
	}
}

// NewRateLimiter creates a new rate limiter with the specified requests per second
func NewRateLimiter(requestsPerSecond int) *types.RateLimiter {
	rl := &types.RateLimiter{
		Ticker:   time.NewTicker(time.Second / time.Duration(requestsPerSecond)),
		Requests: make(chan struct{}, requestsPerSecond),
	}

	// Fill the initial bucket
	for i := 0; i < requestsPerSecond; i++ {
		rl.Requests <- struct{}{}
	}

	// Start the ticker to refill the bucket
	go func() {
		for range rl.Ticker.C {
			select {
			case rl.Requests <- struct{}{}:
			default:
				// Bucket is full, skip
			}
		}
	}()

	return rl
}

// assumeRoleAndGetConfig assumes an IAM role and returns an AWS config with the assumed credentials
func assumeRoleAndGetConfig(roleArn string, sessionName string) (aws.Config, error) {
	// Load default AWS configuration
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Assume the role
	assumeRoleInput := &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleArn),
		RoleSessionName: aws.String(sessionName),
	}

	assumeRoleResult, err := stsClient.AssumeRole(context.Background(), assumeRoleInput)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to assume role %s: %w", roleArn, err)
	}

	// Create new config with assumed role credentials
	assumedCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     *assumeRoleResult.Credentials.AccessKeyId,
				SecretAccessKey: *assumeRoleResult.Credentials.SecretAccessKey,
				SessionToken:    *assumeRoleResult.Credentials.SessionToken,
			},
		}),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create config with assumed credentials: %w", err)
	}

	return assumedCfg, nil
}

// listAllGroups retrieves all groups from Identity Center and returns a map of groupId -> groupName
func listAllGroups(client *identitystore.Client, identityStoreId string, rateLimiter *types.RateLimiter) (map[string]string, error) {
	fmt.Printf("📋 Pre-fetching all groups from Identity Center...\n")

	groupMap := make(map[string]string)
	var nextToken *string
	groupCount := 0

	for {
		rateLimiter.Wait()

		input := &identitystore.ListGroupsInput{
			IdentityStoreId: aws.String(identityStoreId),
			NextToken:       nextToken,
		}

		result, err := client.ListGroups(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to list groups: %w", err)
		}

		for _, group := range result.Groups {
			if group.GroupId != nil && group.DisplayName != nil {
				groupMap[*group.GroupId] = *group.DisplayName
				groupCount++
			}
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	fmt.Printf("✅ Pre-fetched %d groups\n", groupCount)
	return groupMap, nil
}

// getUserGroups retrieves all group names for a given user ID using a pre-fetched group map
func getUserGroups(client *identitystore.Client, identityStoreId string, userId string, rateLimiter *types.RateLimiter, groupMap map[string]string) ([]string, error) {
	var groups []string
	var nextToken *string

	for {
		rateLimiter.Wait()

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

		// Get group names for each membership using the pre-fetched map
		for _, membership := range result.GroupMemberships {
			if membership.GroupId != nil {
				groupId := *membership.GroupId

				// Look up group name in the map
				if groupName, ok := groupMap[groupId]; ok {
					groups = append(groups, groupName)
				} else {
					// Fallback to group ID if not found in map
					fmt.Printf("⚠️  Warning: Group ID %s not found in group map, using ID as name\n", groupId)
					groups = append(groups, groupId)
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

// getGroupName retrieves the display name for a group ID
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

// ListIdentityCenterGroupMembershipsWithRole retrieves all group memberships from Identity Center by assuming a role
// Returns the memberships slice for in-memory use
func ListIdentityCenterGroupMembershipsWithRole(mgmtRoleArn string, identityCenterId string, maxConcurrency int, requestsPerSecond int) ([]types.IdentityCenterGroupMembership, error) {
	// Assume the management role
	cfg, err := assumeRoleAndGetConfig(mgmtRoleArn, "identity-center-group-membership")
	if err != nil {
		return nil, fmt.Errorf("failed to assume management role: %w", err)
	}

	// Create Identity Store client with assumed role
	identityStoreClient := identitystore.NewFromConfig(cfg)

	// List all users first
	users, err := ListIdentityCenterUsersAll(identityStoreClient, identityCenterId, maxConcurrency, requestsPerSecond)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// List all user group memberships
	memberships, err := ListIdentityCenterGroupMembershipsAll(identityStoreClient, identityCenterId, users, maxConcurrency, requestsPerSecond)
	if err != nil {
		return nil, fmt.Errorf("failed to list group memberships: %w", err)
	}

	return memberships, nil
}

// ListIdentityCenterGroupMembershipsAll retrieves all user group memberships from Identity Center
func ListIdentityCenterGroupMembershipsAll(identityStoreClient *identitystore.Client, identityStoreId string, users []types.IdentityCenterUser, maxConcurrency int, requestsPerSecond int) ([]types.IdentityCenterGroupMembership, error) {
	return ListIdentityCenterGroupMembershipsAllWithLogger(identityStoreClient, identityStoreId, users, maxConcurrency, requestsPerSecond, &DefaultLogger{})
}

// ListIdentityCenterGroupMembershipsAllWithLogger retrieves all user group memberships from Identity Center with custom logger
func ListIdentityCenterGroupMembershipsAllWithLogger(identityStoreClient *identitystore.Client, identityStoreId string, users []types.IdentityCenterUser, maxConcurrency int, requestsPerSecond int, logger Logger) ([]types.IdentityCenterGroupMembership, error) {
	logger.Printf("🔍 Retrieving group memberships for %d users", len(users))
	logger.Printf("⚙️  Concurrency: %d workers, Rate limit: %d req/sec", maxConcurrency, requestsPerSecond)

	// Create rate limiter
	rateLimiter := NewRateLimiter(requestsPerSecond)
	defer rateLimiter.Stop()

	// Pre-fetch all groups once to avoid repeated DescribeGroup calls
	logger.Printf("📋 Pre-fetching all groups from Identity Center...")
	groupMap, err := listAllGroups(identityStoreClient, identityStoreId, rateLimiter)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}
	logger.Printf("✅ Pre-fetched %d groups", len(groupMap))

	var memberships []types.IdentityCenterGroupMembership
	var mu sync.Mutex
	var wg sync.WaitGroup
	var processed int32

	// Create worker pool
	userChan := make(chan types.IdentityCenterUser, len(users))

	// Start workers
	for i := 0; i < maxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for user := range userChan {
				groups, err := getUserGroups(identityStoreClient, identityStoreId, user.UserId, rateLimiter, groupMap)
				if err != nil {
					logger.Printf("⚠️  Warning: Failed to get groups for user %s: %v", user.UserName, err)
					continue
				}

				membership := types.IdentityCenterGroupMembership{
					UserId:      user.UserId,
					UserName:    user.UserName,
					DisplayName: user.DisplayName,
					Email:       user.Email,
					Groups:      groups,
				}

				mu.Lock()
				memberships = append(memberships, membership)
				processed++
				// Show progress every 10 users
				if processed%10 == 0 || processed == int32(len(users)) {
					logger.Printf("📊 Progress: %d/%d users processed (%.1f%%)",
						processed, len(users), float64(processed)/float64(len(users))*100)
				}
				mu.Unlock()
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

	logger.Printf("✅ Successfully retrieved group memberships for %d users", len(memberships))
	return memberships, nil
}

// RetrieveIdentityCenterData retrieves users and group memberships in-memory from Identity Center
func RetrieveIdentityCenterData(
	roleArn string,
	maxConcurrency int,
	requestsPerSecond int,
) (*IdentityCenterData, error) {
	return RetrieveIdentityCenterDataWithLogger(roleArn, maxConcurrency, requestsPerSecond, &DefaultLogger{})
}

// RetrieveIdentityCenterDataWithLogger retrieves users and group memberships with custom logger
func RetrieveIdentityCenterDataWithLogger(
	roleArn string,
	maxConcurrency int,
	requestsPerSecond int,
	logger Logger,
) (*IdentityCenterData, error) {
	logger.Printf("🔐 Assuming Identity Center role: %s", roleArn)

	// 1. Assume the Identity Center role
	cfg, err := assumeRoleAndGetConfig(roleArn, "identity-center-data-retrieval")
	if err != nil {
		return nil, fmt.Errorf("failed to assume Identity Center role: %w", err)
	}

	logger.Printf("✅ Successfully assumed role")

	// 2. Discover Identity Center instance ID with logger
	instanceID, err := DiscoverIdentityCenterInstanceIDWithLogger(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to discover Identity Center instance: %w", err)
	}

	// 3. Create Identity Store client
	identityStoreClient := identitystore.NewFromConfig(cfg)

	// 4. Retrieve all users with logger
	users, err := ListIdentityCenterUsersAllWithLogger(identityStoreClient, instanceID, maxConcurrency, requestsPerSecond, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve users: %w", err)
	}

	// 5. Retrieve all group memberships with logger
	memberships, err := ListIdentityCenterGroupMembershipsAllWithLogger(identityStoreClient, instanceID, users, maxConcurrency, requestsPerSecond, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve group memberships: %w", err)
	}

	logger.Printf("✅ Identity Center data retrieval complete: %d users, %d memberships", len(users), len(memberships))

	return &IdentityCenterData{
		Users:       users,
		Memberships: memberships,
		InstanceID:  instanceID,
	}, nil
}

// HandleIdentityCenterGroupMembership handles Identity Center group membership listing with role assumption
func HandleIdentityCenterGroupMembership(mgmtRoleArn string, identityCenterId string, userName string, listType string, maxConcurrency int, requestsPerSecond int) error {
	fmt.Printf("🔐 Assuming management role: %s\n", mgmtRoleArn)

	// Assume the management role
	cfg, err := assumeRoleAndGetConfig(mgmtRoleArn, "identity-center-group-membership")
	if err != nil {
		return fmt.Errorf("failed to assume management role: %w", err)
	}

	fmt.Printf("✅ Successfully assumed role\n")

	// Create Identity Store client with assumed role
	identityStoreClient := identitystore.NewFromConfig(cfg)

	// Create rate limiter
	rateLimiter := NewRateLimiter(requestsPerSecond)
	defer rateLimiter.Stop()

	if listType == "all" {
		// List all users first
		users, err := ListIdentityCenterUsersAll(identityStoreClient, identityCenterId, maxConcurrency, requestsPerSecond)
		if err != nil {
			return fmt.Errorf("failed to list users: %w", err)
		}

		// List all user group memberships
		fmt.Printf("📋 Listing group memberships for all users...\n")
		memberships, err := ListIdentityCenterGroupMembershipsAll(identityStoreClient, identityCenterId, users, maxConcurrency, requestsPerSecond)
		if err != nil {
			return fmt.Errorf("failed to list all user group memberships: %w", err)
		}

		// Display memberships
		fmt.Printf("\n📊 Found group memberships for %d users:\n", len(memberships))
		for i, membership := range memberships {
			fmt.Printf("%d. %s (%s) - %d groups\n", i+1, membership.DisplayName, membership.UserName, len(membership.Groups))
			for _, group := range membership.Groups {
				fmt.Printf("   - %s\n", group)
			}
		}

		// Save to JSON file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("identity-center-group-memberships-user-centric-%s-%s.json", identityCenterId, timestamp)
		err = SaveGroupMembershipsToJSON(memberships, filename)
		if err != nil {
			return fmt.Errorf("failed to save group memberships: %w", err)
		}
	} else {
		// List specific user's group membership
		fmt.Printf("🔍 Looking up group membership for user: %s\n", userName)

		// Find the user first
		user, err := ListIdentityCenterUser(identityStoreClient, identityCenterId, userName)
		if err != nil {
			return fmt.Errorf("failed to find user: %w", err)
		}

		// Pre-fetch all groups
		groupMap, err := listAllGroups(identityStoreClient, identityCenterId, rateLimiter)
		if err != nil {
			return fmt.Errorf("failed to list groups: %w", err)
		}

		// Get user's groups
		groups, err := getUserGroups(identityStoreClient, identityCenterId, user.UserId, rateLimiter, groupMap)
		if err != nil {
			return fmt.Errorf("failed to get user groups: %w", err)
		}

		membership := types.IdentityCenterGroupMembership{
			UserId:      user.UserId,
			UserName:    user.UserName,
			DisplayName: user.DisplayName,
			Email:       user.Email,
			Groups:      groups,
		}

		// Display membership
		fmt.Printf("\n👤 User: %s (%s)\n", membership.DisplayName, membership.UserName)
		fmt.Printf("   Email: %s\n", membership.Email)
		fmt.Printf("   Groups (%d):\n", len(membership.Groups))
		for _, group := range membership.Groups {
			fmt.Printf("     - %s\n", group)
		}

		// Save to JSON file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("identity-center-group-membership-%s-%s-%s.json", identityCenterId, userName, timestamp)
		err = SaveGroupMembershipToJSON(membership, filename)
		if err != nil {
			return fmt.Errorf("failed to save group membership: %w", err)
		}
	}

	return nil
}

// SaveGroupMembershipsToJSON saves group memberships data to a JSON file
func SaveGroupMembershipsToJSON(memberships []types.IdentityCenterGroupMembership, filename string) error {
	jsonData, err := json.MarshalIndent(memberships, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal group memberships data: %w", err)
	}

	configPath := config.GetConfigPath()
	filePath := configPath + filename

	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write group memberships data to file: %w", err)
	}

	fmt.Printf("💾 Saved %d group memberships to %s\n", len(memberships), filename)
	return nil
}

// SaveGroupMembershipToJSON saves single group membership data to a JSON file
func SaveGroupMembershipToJSON(membership types.IdentityCenterGroupMembership, filename string) error {
	jsonData, err := json.MarshalIndent(membership, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal group membership data: %w", err)
	}

	configPath := config.GetConfigPath()
	filePath := configPath + filename

	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write group membership data to file: %w", err)
	}

	fmt.Printf("💾 Saved group membership data to %s\n", filename)
	return nil
}
