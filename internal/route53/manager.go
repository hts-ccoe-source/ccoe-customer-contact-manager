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

// DeliverabilityDNSConfig holds DNS configuration for email deliverability
type DeliverabilityDNSConfig struct {
	ZoneID           string
	ZoneName         string
	DomainName       string
	MailFromDomain   string
	DMARCPolicy      string
	DMARCReportEmail string
	Region           string
	ExistingSPF      []string // Existing SPF includes (e.g., "_spf.google.com")
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
					// Store ALL TXT values (verification records can have multiple tokens)
					for _, rr := range record.ResourceRecords {
						if rr.Value != nil {
							value := *rr.Value
							value = strings.Trim(value, "\"")
							// Use a unique key for each value: recordName + value
							key := recordName + "|" + value
							existingRecords[key] = value
						}
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
	for key, value := range existingRecords {
		// Keys are in format: "recordName|value"
		if strings.HasPrefix(key, recordName+"|") {
			existingTokens[value] = true
		}
	}

	dm.logger.Debug("checking verification tokens",
		"record_name", recordName,
		"existing_count", len(existingTokens),
		"required_count", len(orgs))

	// Check if all required tokens already exist
	allTokensExist := true
	for _, org := range orgs {
		if !existingTokens[org.VerificationToken] {
			allTokensExist = false
			dm.logger.Debug("missing verification token",
				"organization", org.Name,
				"token", org.VerificationToken)
			break
		}
	}

	// If all required tokens exist, no update needed (extra tokens are OK)
	if allTokensExist {
		dm.logger.Info("all verification tokens already exist",
			"record_name", recordName,
			"required_tokens", len(orgs),
			"existing_tokens", len(existingTokens))
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

// CreateMXRecord creates or updates an MX record
func (dm *DNSManager) CreateMXRecord(ctx context.Context, zoneID, name, value string, ttl int64) (types.Change, error) {
	change := types.Change{
		Action: types.ChangeActionUpsert,
		ResourceRecordSet: &types.ResourceRecordSet{
			Name: awssdk.String(name),
			Type: types.RRTypeMx,
			TTL:  awssdk.Int64(ttl),
			ResourceRecords: []types.ResourceRecord{
				{
					Value: awssdk.String(value),
				},
			},
		},
	}

	dm.logger.Debug("created MX record change",
		"name", name,
		"value", value,
		"ttl", ttl)

	return change, nil
}

// CreateGenericTXTRecord creates or updates a TXT record
func (dm *DNSManager) CreateGenericTXTRecord(ctx context.Context, zoneID, name, value string, ttl int64) (types.Change, error) {
	// Ensure value is quoted
	if !strings.HasPrefix(value, "\"") {
		value = fmt.Sprintf("\"%s\"", value)
	}

	change := types.Change{
		Action: types.ChangeActionUpsert,
		ResourceRecordSet: &types.ResourceRecordSet{
			Name: awssdk.String(name),
			Type: types.RRTypeTxt,
			TTL:  awssdk.Int64(ttl),
			ResourceRecords: []types.ResourceRecord{
				{
					Value: awssdk.String(value),
				},
			},
		},
	}

	dm.logger.Debug("created TXT record change",
		"name", name,
		"value", value,
		"ttl", ttl)

	return change, nil
}

// ConfigureDeliverabilityRecords creates DNS records for email deliverability (SPF, DMARC, MAIL FROM)
func (dm *DNSManager) ConfigureDeliverabilityRecords(ctx context.Context, config DeliverabilityDNSConfig) error {
	dm.logger.Info("starting deliverability DNS configuration",
		"zone_id", config.ZoneID,
		"domain", config.DomainName,
		"mail_from_domain", config.MailFromDomain,
		"dry_run", dm.dryRun)

	// Look up zone name if not provided
	zoneName := config.ZoneName
	if zoneName == "" {
		var err error
		zoneName, err = dm.GetHostedZoneName(ctx, config.ZoneID)
		if err != nil {
			return fmt.Errorf("failed to get hosted zone name: %w", err)
		}
		config.ZoneName = zoneName
	}

	// Get existing records for idempotency check
	existingRecords, err := dm.getDeliverabilityRecords(ctx, config.ZoneID)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	var allChanges []types.Change

	// 1. SPF record for main domain
	spfValue := "v=spf1 include:amazonses.com"
	for _, include := range config.ExistingSPF {
		spfValue += " include:" + include
	}
	spfValue += " ~all"

	if !dm.recordExists(existingRecords, config.DomainName, "TXT", spfValue) {
		spfChange, err := dm.CreateGenericTXTRecord(ctx, config.ZoneID, config.DomainName, spfValue, 300)
		if err != nil {
			return fmt.Errorf("failed to create SPF record: %w", err)
		}
		allChanges = append(allChanges, spfChange)
		dm.logger.Info("prepared SPF record", "domain", config.DomainName, "value", spfValue)
	} else {
		dm.logger.Info("SPF record already exists with correct value", "domain", config.DomainName)
	}

	// 2. DMARC record
	dmarcName := fmt.Sprintf("_dmarc.%s", config.DomainName)
	dmarcValue := fmt.Sprintf("v=DMARC1; p=%s; rua=mailto:%s; pct=100; adkim=s; aspf=s",
		config.DMARCPolicy, config.DMARCReportEmail)

	if !dm.recordExists(existingRecords, dmarcName, "TXT", dmarcValue) {
		dmarcChange, err := dm.CreateGenericTXTRecord(ctx, config.ZoneID, dmarcName, dmarcValue, 300)
		if err != nil {
			return fmt.Errorf("failed to create DMARC record: %w", err)
		}
		allChanges = append(allChanges, dmarcChange)
		dm.logger.Info("prepared DMARC record", "name", dmarcName, "policy", config.DMARCPolicy)
	} else {
		dm.logger.Info("DMARC record already exists with correct value", "name", dmarcName)
	}

	// 3. Custom MAIL FROM domain records (if configured)
	if config.MailFromDomain != "" {
		// MX record for bounce handling
		mxValue := fmt.Sprintf("10 feedback-smtp.%s.amazonses.com", config.Region)
		if !dm.recordExists(existingRecords, config.MailFromDomain, "MX", mxValue) {
			mxChange, err := dm.CreateMXRecord(ctx, config.ZoneID, config.MailFromDomain, mxValue, 300)
			if err != nil {
				return fmt.Errorf("failed to create MX record: %w", err)
			}
			allChanges = append(allChanges, mxChange)
			dm.logger.Info("prepared MX record", "domain", config.MailFromDomain, "value", mxValue)
		} else {
			dm.logger.Info("MX record already exists with correct value", "domain", config.MailFromDomain)
		}

		// SPF record for MAIL FROM domain
		mailFromSPF := "v=spf1 include:amazonses.com ~all"
		if !dm.recordExists(existingRecords, config.MailFromDomain, "TXT", mailFromSPF) {
			mailFromSPFChange, err := dm.CreateGenericTXTRecord(ctx, config.ZoneID, config.MailFromDomain, mailFromSPF, 300)
			if err != nil {
				return fmt.Errorf("failed to create MAIL FROM SPF record: %w", err)
			}
			allChanges = append(allChanges, mailFromSPFChange)
			dm.logger.Info("prepared MAIL FROM SPF record", "domain", config.MailFromDomain)
		} else {
			dm.logger.Info("MAIL FROM SPF record already exists with correct value", "domain", config.MailFromDomain)
		}
	}

	// Apply changes only if there are any
	if len(allChanges) == 0 {
		dm.logger.Info("no DNS changes needed - all records already exist with correct values",
			"zone_id", config.ZoneID,
			"domain", config.DomainName)
		return nil
	}

	// Apply all changes
	if err := dm.applyChanges(ctx, config.ZoneID, allChanges); err != nil {
		return fmt.Errorf("failed to apply deliverability DNS changes: %w", err)
	}

	if dm.dryRun {
		dm.logger.Info("dry-run: would complete deliverability DNS configuration",
			"zone_id", config.ZoneID,
			"domain", config.DomainName,
			"total_changes", len(allChanges))
	} else {
		dm.logger.Info("deliverability DNS configuration completed",
			"zone_id", config.ZoneID,
			"domain", config.DomainName,
			"records_created", len(allChanges))
	}

	return nil
}

// GetDeliverabilityDNSRecords returns a list of DNS records needed for deliverability (for display purposes)
func (dm *DNSManager) GetDeliverabilityDNSRecords(config DeliverabilityDNSConfig) []string {
	var records []string

	// SPF record
	spfValue := "v=spf1 include:amazonses.com"
	for _, include := range config.ExistingSPF {
		spfValue += " include:" + include
	}
	spfValue += " ~all"
	records = append(records, fmt.Sprintf("TXT %s \"%s\"", config.DomainName, spfValue))

	// DMARC record
	dmarcName := fmt.Sprintf("_dmarc.%s", config.DomainName)
	dmarcValue := fmt.Sprintf("v=DMARC1; p=%s; rua=mailto:%s; pct=100; adkim=s; aspf=s",
		config.DMARCPolicy, config.DMARCReportEmail)
	records = append(records, fmt.Sprintf("TXT %s \"%s\"", dmarcName, dmarcValue))

	// MAIL FROM records
	if config.MailFromDomain != "" {
		mxValue := fmt.Sprintf("10 feedback-smtp.%s.amazonses.com", config.Region)
		records = append(records, fmt.Sprintf("MX %s %s", config.MailFromDomain, mxValue))

		mailFromSPF := "v=spf1 include:amazonses.com ~all"
		records = append(records, fmt.Sprintf("TXT %s \"%s\"", config.MailFromDomain, mailFromSPF))
	}

	return records
}

// getDeliverabilityRecords retrieves existing deliverability DNS records from a hosted zone
func (dm *DNSManager) getDeliverabilityRecords(ctx context.Context, zoneID string) (map[string]map[string][]string, error) {
	// Map structure: recordName -> recordType -> []values
	records := make(map[string]map[string][]string)

	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: awssdk.String(zoneID),
	}

	paginator := route53.NewListResourceRecordSetsPaginator(dm.client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list resource record sets: %w", err)
		}

		for _, recordSet := range output.ResourceRecordSets {
			name := strings.TrimSuffix(awssdk.ToString(recordSet.Name), ".")
			recordType := string(recordSet.Type)

			if records[name] == nil {
				records[name] = make(map[string][]string)
			}

			// Extract values from ResourceRecords
			var values []string
			for _, rr := range recordSet.ResourceRecords {
				value := awssdk.ToString(rr.Value)
				// Remove quotes from TXT records for comparison
				if recordType == "TXT" {
					value = strings.Trim(value, "\"")
				}
				values = append(values, value)
			}

			records[name][recordType] = values
		}
	}

	return records, nil
}

// recordExists checks if a DNS record exists with the specified value
func (dm *DNSManager) recordExists(existingRecords map[string]map[string][]string, name string, recordType string, expectedValue string) bool {
	// Normalize the name (remove trailing dot if present)
	name = strings.TrimSuffix(name, ".")

	// Check if record name exists
	if existingRecords[name] == nil {
		return false
	}

	// Check if record type exists for this name
	if existingRecords[name][recordType] == nil {
		return false
	}

	// For TXT records, remove quotes from expected value for comparison
	if recordType == "TXT" {
		expectedValue = strings.Trim(expectedValue, "\"")
	}

	// Check if the expected value exists in the record values
	for _, value := range existingRecords[name][recordType] {
		if value == expectedValue {
			return true
		}
	}

	return false
}
