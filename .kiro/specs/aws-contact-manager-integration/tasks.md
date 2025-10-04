# Implementation Plan

- [ ] 1. Project restructuring and initial setup
  - Create internal directory structure with domain-specific packages
  - Update Makefile to handle new package structure
  - _Requirements: 5.1, 6.1_

- [ ] 1.1 Create internal package directory structure
  - Create `internal/` directory with subdirectories: `aws/`, `contacts/`, `ses/`, `lambda/`, `config/`, `types/`
  - Initialize each package with proper Go package declarations
  - _Requirements: 5.1_

- [ ] 1.2 Update Makefile for new structure
  - Modify build targets to include internal packages
  - Update test commands to cover all internal packages
  - Update linting and formatting commands for internal directory
  - _Requirements: 6.1_

- [ ] 2. Function analysis and categorization
  - Systematically catalog all functions in original backup file
  - Analyze existing modular files for function distribution
  - Create function mapping matrix for integration planning
  - _Requirements: 1.1, 1.2, 1.3_

- [ ] 2.1 Catalog functions from original file
  - Extract complete list of function definitions from `aws-alternate-contact-manager-original.go.bak`
  - Document function signatures, dependencies, and purposes
  - _Requirements: 1.1_

- [ ] 2.2 Analyze existing modular files
  - Inventory functions in current `ses.go`, `list_management.go`, `types.go`, `utils.go`
  - Identify duplicate implementations between original and modular files
  - _Requirements: 1.2, 1.3_

- [ ] 2.3 Create integration mapping
  - Map each function to target internal package
  - Identify conflicts and resolution strategies
  - Plan function consolidation approach
  - _Requirements: 1.4_

- [ ] 3. AWS utilities package implementation
  - Extract and integrate core AWS utility functions into `internal/aws/utils.go`
  - Resolve conflicts with existing implementations
  - _Requirements: 2.1, 2.2, 2.3_

- [ ] 3.1 Create AWS utilities package
  - Implement `CreateConnectionConfiguration` function
  - Implement organization management functions (`GetManagementAccountIdByPrefix`, `IsManagementAccount`, `GetAllAccountsInOrganization`)
  - Implement credential functions (`AssumeRole`, `GetCurrentAccountId`)
  - _Requirements: 2.1, 2.2_

- [ ] 3.2 Resolve AWS utility conflicts
  - Consolidate duplicate `GetConfigPath` implementations
  - Ensure no duplicate AWS utility functions exist
  - _Requirements: 2.4, 2.5_

- [ ] 4. Alternate contacts package implementation
  - Extract and integrate alternate contact management functions into `internal/contacts/alternate_contacts.go`
  - Implement workflow orchestration functions
  - _Requirements: 3.1, 3.2, 3.3_

- [ ] 4.1 Implement contact CRUD operations
  - Extract `GetAlternateContact`, `SetAlternateContact`, `DeleteAlternateContact` functions
  - Extract `CheckIfContactExists`, `SetAlternateContactIfNotExists` functions
  - _Requirements: 3.1, 3.2_

- [ ] 4.2 Implement workflow orchestration functions
  - Extract `SetContactsForSingleOrganization` function
  - Extract `SetContactsForAllOrganizations` function  
  - Extract `DeleteContactsFromOrganization` function
  - _Requirements: 3.3_

- [ ] 4.3 Resolve contact function dependencies
  - Update imports to use `internal/aws` package
  - Ensure proper integration with AWS utilities
  - _Requirements: 3.4, 3.5_

- [ ] 5. Enhanced SES package implementation
  - Migrate existing SES functions to `internal/ses/operations.go`
  - Enhance with missing functionality from original file
  - _Requirements: 4.1, 4.2, 4.3_

- [ ] 5.1 Migrate existing SES functions
  - Move functions from current `ses.go` to `internal/ses/operations.go`
  - Update package declarations and imports
  - _Requirements: 4.1, 4.3_

- [ ] 5.2 Enhance SES operations with original functionality
  - Compare existing SES functions with original implementations
  - Add missing advanced SES features from original file
  - Consolidate duplicate implementations
  - _Requirements: 4.2, 4.4, 4.5_

- [ ] 6. List management package implementation
  - Migrate email template functions to `internal/ses/list_management.go`
  - Complete Microsoft Graph integration functions
  - Add S3 payload processing for Lambda mode
  - _Requirements: 4.1, 4.2, 4.3, 4.4_

- [ ] 6.1 Migrate and complete email template functions
  - Move functions from current `list_management.go` to `internal/ses/list_management.go`
  - Complete missing email template processing functions from original file
  - _Requirements: 4.1, 4.2_

- [ ] 6.2 Complete Microsoft Graph integration
  - Finish Microsoft Graph meeting creation functions
  - Add ICS calendar generation functions
  - _Requirements: 4.2_

- [ ] 6.3 Add S3 payload processing for Lambda mode
  - Implement S3 payload parsing and metadata extraction functions
  - Implement S3 change metadata to email message mapping functions
  - _Requirements: 4.3, 4.4_

- [ ] 7. Configuration and types package implementation
  - Create `internal/config/config.go` with configuration management functions
  - Migrate types to `internal/types/types.go`
  - _Requirements: 5.1, 5.2, 5.3_

- [ ] 7.1 Implement configuration package
  - Consolidate configuration loading functions
  - Move `GetConfigPath` and related functions to `internal/config/config.go`
  - _Requirements: 5.2, 5.4_

- [ ] 7.2 Migrate types package
  - Move type definitions from current `types.go` to `internal/types/types.go`
  - Add any missing types from original file
  - _Requirements: 5.1, 5.3_

- [ ] 8. Lambda package implementation (if needed)
  - Extract Lambda-specific handlers to `internal/lambda/handlers.go`
  - Ensure Lambda mode functionality is preserved
  - _Requirements: 5.1, 5.5_

- [ ] 8.1 Create Lambda handlers package
  - Extract Lambda-specific event handling code
  - Implement S3 event processing functions
  - _Requirements: 5.5_

- [ ] 9. Main.go integration and import updates
  - Update main.go to import from internal packages
  - Update CLI and Lambda handlers to use new package structure
  - _Requirements: 6.1, 6.2, 6.3, 7.1, 7.2_

- [ ] 9.1 Update main.go imports
  - Replace old imports with internal package imports
  - Update function calls to use new package structure
  - _Requirements: 6.1, 6.2_

- [ ] 9.2 Update CLI handlers
  - Modify CLI command handlers to use internal packages
  - Ensure CLI mode functionality is preserved
  - _Requirements: 7.1, 7.2_

- [ ] 9.3 Update Lambda handlers
  - Modify Lambda mode handlers to use internal packages
  - Ensure Lambda mode functionality is preserved
  - _Requirements: 7.2_

- [ ] 10. Dependency resolution and compilation validation
  - Resolve all import dependencies
  - Ensure clean compilation of entire codebase
  - _Requirements: 6.1, 6.2, 6.3, 6.4_

- [ ] 10.1 Resolve import dependencies
  - Add all required imports to internal packages
  - Remove unused imports and consolidate duplicates
  - _Requirements: 6.1, 6.2_

- [ ] 10.2 Validate compilation
  - Compile each internal package individually
  - Compile entire codebase and resolve any errors
  - _Requirements: 6.3, 6.4_

- [ ] 11. Function signature compatibility validation
  - Ensure function signatures remain compatible with existing usage
  - Validate backward compatibility is maintained
  - _Requirements: 7.1, 7.2, 7.3_

- [ ] 11.1 Validate function signatures
  - Check that integrated functions maintain compatible signatures
  - Ensure no breaking changes are introduced
  - _Requirements: 7.1, 7.2_

- [ ] 11.2 Test compatibility
  - Verify existing function calls continue to work
  - Test both CLI and Lambda modes for functionality
  - _Requirements: 7.3, 7.4_

- [ ] 12. Final cleanup and validation
  - Remove old modular files and original backup file
  - Perform comprehensive testing of both operation modes
  - Update documentation and build processes
  - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [ ] 12.1 Remove obsolete files
  - Delete old root-level modular files (`ses.go`, `list_management.go`, `types.go`, `utils.go`)
  - Remove `aws-alternate-contact-manager-original.go.bak`
  - _Requirements: 8.1, 8.2_

- [ ] 12.2 Comprehensive testing
  - Test CLI mode with all subcommands
  - Test Lambda mode with S3 event processing
  - Validate all functionality is preserved
  - _Requirements: 8.3, 8.4_

- [ ] 12.3 Final validation and documentation
  - Ensure Makefile works with new structure
  - Update any documentation references to old file structure
  - Create summary of integration changes
  - _Requirements: 8.5_