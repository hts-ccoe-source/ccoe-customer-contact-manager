# CCOE Customer Contact Manager - Documentation Summaries

This directory contains high-level summaries and architecture diagrams for the CCOE Customer Contact Manager project.

## Quick Start

**New to the project?** Start here:

1. **[Project Status Summary](./PROJECT_STATUS_SUMMARY.md)** - Complete overview of the project, features, and current status
2. **[Diagrams Index](./DIAGRAMS_INDEX.md)** - Visual architecture diagrams with explanations

## Documents in This Directory

### PROJECT_STATUS_SUMMARY.md
**Purpose:** Comprehensive project overview  
**Audience:** All team members, stakeholders, new developers  
**Contents:**
- Executive summary
- Architecture overview with diagrams
- Current status (completed, in-progress, planned features)
- Technical stack
- Configuration files
- Deployment procedures
- Security model
- Monitoring and logging
- Testing approach
- Future roadmap

**When to read:** First document to read when joining the project or getting an overview.

---

### DIAGRAMS_INDEX.md
**Purpose:** Index of all architecture diagrams  
**Audience:** Technical team, architects, developers  
**Contents:**
- High-level architecture overview
- Component architecture (detailed)
- Change request processing flow
- Multi-customer distribution
- Authentication flow (SAML SSO)
- Identity Center integration

**When to read:** When you need visual understanding of system architecture or specific flows.

---

## Related Documentation

### Main Documentation (`../docs/`)

The `docs/` directory contains detailed technical documentation:

- **SOLUTION_OVERVIEW.md** - Multi-account architecture and deployment
- **LAMBDA_BACKEND_ARCHITECTURE.md** - Backend Lambda design
- **API_ENDPOINTS.md** - Complete API reference
- **DEPLOYMENT_GUIDE.md** - Step-by-step deployment
- **TRANSIENT_TRIGGER_PATTERN.md** - Event processing pattern
- **DATETIME_MIGRATION_GUIDE.md** - Datetime standardization
- **MEETING_FUNCTIONALITY_CONSOLIDATION.md** - Meeting features

### Code Documentation

- **README.md** (project root) - Build and usage instructions
- **Makefile** - Build targets and commands
- **go.mod** - Go dependencies
- **package.json** - Node.js dependencies

## Document Hierarchy

```
Project Root
├── README.md                    # Build and usage
├── summaries/                   # High-level overviews (YOU ARE HERE)
│   ├── README.md               # This file
│   ├── PROJECT_STATUS_SUMMARY.md
│   └── DIAGRAMS_INDEX.md
├── docs/                        # Detailed technical docs
│   ├── SOLUTION_OVERVIEW.md
│   ├── LAMBDA_BACKEND_ARCHITECTURE.md
│   ├── API_ENDPOINTS.md
│   └── ... (40+ documents)
└── generated-diagrams/          # Architecture diagrams
    ├── ccoe-architecture-overview.png
    ├── component-architecture.png
    ├── change-request-flow.png
    └── ... (6 diagrams)
```

## How to Use This Documentation

### For New Team Members

1. Read **PROJECT_STATUS_SUMMARY.md** for complete overview
2. Review diagrams in **DIAGRAMS_INDEX.md** for visual understanding
3. Read **SOLUTION_OVERVIEW.md** in `docs/` for deployment details
4. Read **API_ENDPOINTS.md** for API reference
5. Follow **DEPLOYMENT_GUIDE.md** for hands-on deployment

### For Developers

1. Review **Component Architecture** diagram for code organization
2. Read **LAMBDA_BACKEND_ARCHITECTURE.md** for backend design
3. Read **API_ENDPOINTS.md** for API contracts
4. Review **TRANSIENT_TRIGGER_PATTERN.md** for event processing
5. Check **DATETIME_MIGRATION_GUIDE.md** for datetime handling

### For Architects

1. Review all diagrams in **DIAGRAMS_INDEX.md**
2. Read **SOLUTION_OVERVIEW.md** for multi-account architecture
3. Review **PROJECT_STATUS_SUMMARY.md** for technology stack
4. Check security section in **PROJECT_STATUS_SUMMARY.md**

### For Operations

1. Read **Deployment** section in **PROJECT_STATUS_SUMMARY.md**
2. Follow **DEPLOYMENT_GUIDE.md** for deployment procedures
3. Review **Monitoring & Logging** section for CloudWatch logs
4. Check **Troubleshooting** section for common issues

## Keeping Documentation Updated

### When to Update Summaries

Update **PROJECT_STATUS_SUMMARY.md** when:
- New features are completed
- Architecture changes
- New components are added
- Technology stack changes
- Deployment procedures change

Update **DIAGRAMS_INDEX.md** when:
- New diagrams are created
- Existing diagrams are updated
- Diagram purposes change

### How to Update Diagrams

Diagrams are generated using the AWS Diagram MCP Server. To regenerate:

1. Use the MCP server with the appropriate Python code
2. Save to `../generated-diagrams/`
3. Update references in markdown files
4. Commit both diagram and markdown changes

## Questions?

For questions about:
- **Architecture:** Review diagrams and SOLUTION_OVERVIEW.md
- **API:** Check API_ENDPOINTS.md
- **Deployment:** Follow DEPLOYMENT_GUIDE.md
- **Features:** Read PROJECT_STATUS_SUMMARY.md
- **Code:** Check inline comments and internal package docs

---

**Last Updated:** October 22, 2025  
**Maintained By:** CCOE Platform Team
