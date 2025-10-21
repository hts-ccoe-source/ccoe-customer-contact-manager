package route53

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"ccoe-customer-contact-manager/internal/aws"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// DNSManager manages Route53 DNS records for SES validation
type DNSManager struct {
	client *route53.Client
	logger *slog.Logger
	dryRun bool
}

// DNSConfig holds Route53 configuration
type DNSConfig struct {
	ZoneID   string
	ZoneName string // Looked up from ZoneID via GetHostedZone API
}

// OrganizationDNS holds DNS configuration for one organization
type OrganizationDNS struct {
	Name              string
	DKIMTokens        []string
	VerificationToken string
}

// NewDNSManager creates a new DNS manager
func NewDNSManager(cfg awssdk.Config, dryRun bool, logger *slog.Logger) *DNSManager {
	return &DNSManager{
		client: route53.NewFromConfig(cfg),
		logger: logger,
		dryRun: dryRun,
	}
}

// GetHostedZoneName retrieves the zone name from a zone ID
func (dm *DNSManager) GetHostedZoneName(ctx context.Context, zoneID string) (string, error) {
	dm.logger.Info("retrieving hosted zone name", "zone_id", zoneID)

	input := &route53.GetHostedZoneInput{
		Id: awssdk.String(zoneID),
	}

	var zoneName string

	// Wrap API call with retry logic
	err := aws.RetryableOperation(ctx, "GetHostedZone", func() error {
		output, err := dm.client.GetHostedZone(ctx, input)
		if err != nil {
			return err
		}

		if output.HostedZone == nil || output.HostedZone.Name == nil {
			return fmt.Errorf("hosted zone name not found for zone ID: %s", zoneID)
		}

		zoneName = *output.HostedZone.Name
		// Strip trailing dot if present
		zoneName = strings.TrimSuffix(zoneName, ".")

		return nil
	}, dm.logger)

	if err != nil {
		return "", aws.WrapAWSError(err, fmt.Sprintf("get hosted zone %s", zoneID))
	}

	dm.logger.Info("retrieved hosted zone name", "zone_id", zoneID, "zone_name", zoneName)
	return zoneName, nil
}

// validateOrganization validates organization DNS configuration
func (dm *DNSManager) validateOrganization(org OrganizationDNS) error {
	// Check that exactly 3 DKIM tokens are present
	if len(org.DKIMTokens) != 3 {
		return fmt.Errorf("organization %s has invalid DKIM token count: expected 3, got %d", org.Name, len(org.DKIMTokens))
	}

	// Check that verification token is non-empty
	if org.VerificationToken == "" {
		return fmt.Errorf("organization %s has empty verification token", org.Name)
	}

	return nil
}

// applyChanges applies DNS changes in batches
func (dm *DNSManager) applyChanges(ctx context.Context, zoneID string, changes []types.Change) error {
	if len(changes) == 0 {
		dm.logger.Info("no DNS changes to apply")
		return nil
	}

	// Split changes into batches of 1000 (Route53 limit)
	const batchSize = 1000
	totalBatches := (len(changes) + batchSize - 1) / batchSize

	dm.logger.Info("applying DNS changes",
		"total_changes", len(changes),
		"batches", totalBatches,
		"dry_run", dm.dryRun)

	for i := 0; i < len(changes); i += batchSize {
		end := i + batchSize
		if end > len(changes) {
			end = len(changes)
		}

		batch := changes[i:end]
		batchNum := (i / batchSize) + 1

		if dm.dryRun {
			dm.logger.Info("dry-run: would apply DNS change batch",
				"batch", batchNum,
				"total_batches", totalBatches,
				"changes_in_batch", len(batch))
			continue
		}

		input := &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: awssdk.String(zoneID),
			ChangeBatch: &types.ChangeBatch{
				Changes: batch,
			},
		}

		// Wrap API call with retry logic
		err := aws.RetryableOperation(ctx, "ChangeResourceRecordSets", func() error {
			output, err := dm.client.ChangeResourceRecordSets(ctx, input)
			if err != nil {
				return err
			}

			dm.logger.Info("applied DNS change batch",
				"batch", batchNum,
				"total_batches", totalBatches,
				"changes_in_batch", len(batch),
				"change_id", *output.ChangeInfo.Id)

			return nil
		}, dm.logger)

		if err != nil {
			return aws.WrapAWSError(err, fmt.Sprintf("apply DNS change batch %d/%d", batchNum, totalBatches))
		}
	}

	return nil
}

// ConfigureRecords creates DNS records for SES validation
// Looks up zone name from zone ID before creating records
func (dm *DNSManager) ConfigureRecords(ctx context.Context, dnsConfig DNSConfig, orgs []OrganizationDNS) error {
	dm.logger.Info("starting Route53 DNS configuration",
		"zone_id", dnsConfig.ZoneID,
		"organizations", len(orgs),
		"dry_run", dm.dryRun)

	// Look up zone name from zone ID
	zoneName, err := dm.GetHostedZoneName(ctx, dnsConfig.ZoneID)
	if err != nil {
		return fmt.Errorf("failed to get hosted zone name: %w", err)
	}

	// Validate each organization configuration
	validOrgs := make([]OrganizationDNS, 0, len(orgs))
	skippedCount := 0

	for _, org := range orgs {
		if err := dm.validateOrganization(org); err != nil {
			dm.logger.Warn("skipping organization due to invalid configuration",
				"organization", org.Name,
				"error", err)
			skippedCount++
			continue
		}
		validOrgs = append(validOrgs, org)
	}

	if len(validOrgs) == 0 {
		return fmt.Errorf("no valid organizations to process")
	}

	dm.logger.Info("validated organizations",
		"valid", len(validOrgs),
		"skipped", skippedCount)

	// Get existing records to check what needs to be created/updated
	existingRecords, err := dm.getExistingRecords(ctx, dnsConfig.ZoneID, zoneName, validOrgs)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	// Build array of Route53 Change objects for all organizations
	var allChanges []types.Change
	dkimChangesCount := 0
	verificationChangesCount := 0

	// Create DKIM records for each organization
	for _, org := range validOrgs {
		dkimChanges := dm.createDKIMRecordsIfNeeded(ctx, zoneName, org.DKIMTokens, existingRecords)
		if len(dkimChanges) > 0 {
			dm.logger.Info("DKIM records need update",
				"organization", org.Name,
				"records_to_update", len(dkimChanges))
			allChanges = append(allChanges, dkimChanges...)
			dkimChangesCount += len(dkimChanges)
		} else {
			dm.logger.Info("DKIM records already exist and match",
				"organization", org.Name)
		}
	}

	// Create verification records for all organizations
	verificationChanges := dm.createVerificationRecordsIfNeeded(ctx, zoneName, validOrgs, existingRecords)
	if len(verificationChanges) > 0 {
		dm.logger.Info("verification records need update",
			"count", len(verificationChanges))
		allChanges = append(allChanges, verificationChanges...)
		verificationChangesCount = len(verificationChanges)
	} else {
		dm.logger.Info("verification records already exist and match",
			"count", len(validOrgs))
	}

	// Apply all changes
	if err := dm.applyChanges(ctx, dnsConfig.ZoneID, allChanges); err != nil {
		return fmt.Errorf("failed to apply DNS changes: %w", err)
	}

	// Log summary only if there were changes or in normal mode
	if dm.dryRun {
		// Only log summary if there would be changes
		if len(allChanges) > 0 {
			dm.logger.Info("dry-run: would complete Route53 DNS configuration",
				"zone_id", dnsConfig.ZoneID,
				"zone_name", zoneName,
				"organizations_processed", len(validOrgs),
				"organizations_skipped", skippedCount,
				"dkim_records_to_update", dkimChangesCount,
				"verification_records_to_update", verificationChangesCount,
				"total_changes", len(allChanges),
				"dry_run", dm.dryRun)
		}
	} else {
		// Always log in normal mode
		dm.logger.Info("Route53 DNS configuration completed",
			"zone_id", dnsConfig.ZoneID,
			"zone_name", zoneName,
			"organizations_processed", len(validOrgs),
			"organizations_skipped", skippedCount,
			"dkim_records_upserted", dkimChangesCount,
			"verification_records_upserted", verificationChangesCount,
			"total_changes", len(allChanges),
			"dry_run", dm.dryRun)
	}

	return nil
}

// getExistingRecords retrieves existing DNS records for comparison
func (dm *DNSManager) getExistingRecords(ctx context.Context, zoneID, zoneName string, orgs []OrganizationDNS) (map[string]string, error) {
	existingRecords := make(map[string]string)

	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: awssdk.String(zoneID),
	}

	// Wrap API call with retry logic
	err := aws.RetryableOperation(ctx, "ListResourceRecordSets", func() error {
		paginator := route53.NewListResourceRecordSetsPaginator(dm.client, input)

		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				return err
			}

			for _, record := range output.ResourceRecordSets {
				if record.Name == nil || record.Type == "" {
					continue
				}

				recordName := strings.TrimSuffix(*record.Name, ".")

				// Only track DKIM CNAME and verification TXT records
				if record.Type == types.RRTypeCname && strings.Contains(recordName, "._domainkey.") {
					if len(record.ResourceRecords) > 0 && record.ResourceRecords[0].Value != nil {
						existingRecords[recordName] = strings.TrimSuffix(*record.ResourceRecords[0].Value, ".")
					}
				} else if record.Type == types.RRTypeTxt && strings.HasPrefix(recordName, "_amazonses.") {
					if len(record.ResourceRecords) > 0 && record.ResourceRecords[0].Value != nil {
						// Store TXT value without quotes
						value := *record.ResourceRecords[0].Value
						value = strings.Trim(value, "\"")
						existingRecords[recordName] = value
					}
				}
			}
		}
		return nil
	}, dm.logger)

	if err != nil {
		return nil, aws.WrapAWSError(err, "list resource record sets")
	}

	dm.logger.Debug("retrieved existing DNS records",
		"zone_id", zoneID,
		"record_count", len(existingRecords))

	return existingRecords, nil
}

// createDKIMRecordsIfNeeded creates CNAME records for DKIM tokens only if they don't exist or differ
func (dm *DNSManager) createDKIMRecordsIfNeeded(ctx context.Context, zoneName string, dkimTokens []string, existingRecords map[string]string) []types.Change {
	changes := make([]types.Change, 0)

	for _, token := range dkimTokens {
		recordName := fmt.Sprintf("%s._domainkey.%s", token, zoneName)
		recordValue := fmt.Sprintf("%s.dkim.amazonses.com", token)

		// Check if record exists and matches
		if existingValue, exists := existingRecords[recordName]; exists && existingValue == recordValue {
			dm.logger.Debug("DKIM record already exists and matches",
				"name", recordName,
				"value", recordValue)
			continue
		}

		// Record doesn't exist or differs, add to changes
		change := types.Change{
			Action: types.ChangeActionUpsert,
			ResourceRecordSet: &types.ResourceRecordSet{
				Name: awssdk.String(recordName),
				Type: types.RRTypeCname,
				TTL:  awssdk.Int64(600),
				ResourceRecords: []types.ResourceRecord{
					{
						Value: awssdk.String(recordValue),
					},
				},
			},
		}

		changes = append(changes, change)
		dm.logger.Debug("DKIM record needs update",
			"name", recordName,
			"value", recordValue,
			"existing_value", existingRecords[recordName])
	}

	return changes
}

// createVerificationRecordsIfNeeded creates TXT records for verification only if they don't exist or differ
func (dm *DNSManager) createVerificationRecordsIfNeeded(ctx context.Context, zoneName string, orgs []OrganizationDNS, existingRecords map[string]string) []types.Change {
	changes := make([]types.Change, 0)

	// Group all verification tokens for the same record name
	recordName := fmt.Sprintf("_amazonses.%s", zoneName)

	// Get existing TXT values for this record
	existingTokens := make(map[string]bool)
	for name, value := range existingRecords {
		if name == recordName {
			existingTokens[value] = true
		}
	}

	// Check which tokens need to be added
	needsUpdate := false
	for _, org := range orgs {
		if !existingTokens[org.VerificationToken] {
			needsUpdate = true
			break
		}
	}

	if !needsUpdate {
		dm.logger.Debug("all verification tokens already exist",
			"record_name", recordName,
			"token_count", len(orgs))
		return changes
	}

	// Build the complete set of TXT values (existing + new)
	allTokens := make([]string, 0)
	for token := range existingTokens {
		allTokens = append(allTokens, token)
	}
	for _, org := range orgs {
		if !existingTokens[org.VerificationToken] {
			allTokens = append(allTokens, org.VerificationToken)
		}
	}

	// Create resource records for all tokens
	resourceRecords := make([]types.ResourceRecord, 0, len(allTokens))
	for _, token := range allTokens {
		recordValue := fmt.Sprintf("\"%s\"", token)
		resourceRecords = append(resourceRecords, types.ResourceRecord{
			Value: awssdk.String(recordValue),
		})
	}

	change := types.Change{
		Action: types.ChangeActionUpsert,
		ResourceRecordSet: &types.ResourceRecordSet{
			Name:            awssdk.String(recordName),
			Type:            types.RRTypeTxt,
			TTL:             awssdk.Int64(600),
			ResourceRecords: resourceRecords,
		},
	}

	changes = append(changes, change)
	dm.logger.Debug("verification record needs update",
		"record_name", recordName,
		"total_tokens", len(allTokens),
		"new_tokens", len(allTokens)-len(existingTokens))

	return changes
}
