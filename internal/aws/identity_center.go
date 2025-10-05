// Package aws provides Identity Center integration functions.
package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoreTypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"aws-alternate-contact-manager/internal/config"
	"aws-alternate-contact-manager/internal/types"
)

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
		FirstName:   firstName,
		LastName:    lastName,
	}
}

// ListIdentityCenterUsersAll lists all users from Identity Center with concurrency and rate limiting
func ListIdentityCenterUsersAll(identityStoreClient *identitystore.Client, identityStoreId string, maxConcurrency int, requestsPerSecond int) ([]types.IdentityCenterUser, error) {
	fmt.Printf("🔍 Listing all users from Identity Center (ID: %s)\n", identityStoreId)
	fmt.Printf("⚙️  Concurrency: %d workers, Rate limit: %d req/sec\n", maxConcurrency, requestsPerSecond)

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

	fmt.Printf("📊 Found %d total users\n", len(allUsers))

	// Process users with concurrency control
	identityCenterUsers := processUsersWithConcurrency(allUsers, maxConcurrency, rateLimiter)

	fmt.Printf("✅ Successfully processed %d users\n", len(identityCenterUsers))
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

// HandleIdentityCenterUserListing handles Identity Center user listing with role assumption
func HandleIdentityCenterUserListing(mgmtRoleArn string, identityCenterId string, userName string, listType string, maxConcurrency int, requestsPerSecond int) error {
	fmt.Printf("🔐 Assuming management role: %s\n", mgmtRoleArn)

	// Create STS client with default config
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)

	// Assume the management role
	assumeRoleInput := &sts.AssumeRoleInput{
		RoleArn:         aws.String(mgmtRoleArn),
		RoleSessionName: aws.String("identity-center-user-listing"),
	}

	assumeRoleResult, err := stsClient.AssumeRole(context.Background(), assumeRoleInput)
	if err != nil {
		return fmt.Errorf("failed to assume role: %w", err)
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
		return fmt.Errorf("failed to create assumed role config: %w", err)
	}

	// Create Identity Store client with assumed role
	identityStoreClient := identitystore.NewFromConfig(assumedCfg)

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
	fmt.Printf("   First Name: %s\n", user.FirstName)
	fmt.Printf("   Last Name: %s\n", user.LastName)
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

// HandleIdentityCenterGroupMembership handles Identity Center group membership listing with role assumption
func HandleIdentityCenterGroupMembership(mgmtRoleArn string, identityCenterId string, userName string, listType string, maxConcurrency int, requestsPerSecond int) error {
	fmt.Printf("🔐 Assuming management role: %s\n", mgmtRoleArn)

	// Create STS client with default config
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)

	// Assume the management role
	assumeRoleInput := &sts.AssumeRoleInput{
		RoleArn:         aws.String(mgmtRoleArn),
		RoleSessionName: aws.String("identity-center-group-membership"),
	}

	assumeRoleResult, err := stsClient.AssumeRole(context.Background(), assumeRoleInput)
	if err != nil {
		return fmt.Errorf("failed to assume role: %w", err)
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
		return fmt.Errorf("failed to create assumed role config: %w", err)
	}

	// Create Identity Store client with assumed role
	identityStoreClient := identitystore.NewFromConfig(assumedCfg)
	_ = identityStoreClient // Suppress unused variable warning for now

	if listType == "all" {
		// List all user group memberships - placeholder for now
		fmt.Printf("📋 Listing group memberships for all users (placeholder implementation)\n")
		fmt.Printf("Note: Full group membership listing requires additional implementation\n")
	} else {
		// List specific user group membership - placeholder for now
		fmt.Printf("📋 Listing group memberships for user: %s (placeholder implementation)\n", userName)
		fmt.Printf("Note: User group membership listing requires additional implementation\n")
	}

	return nil
}
