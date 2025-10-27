# Typeform Workspace Configuration

## Quick Start

**To configure workspaces:**

1. Get your workspace IDs from Typeform (see "Getting Workspace IDs" section below)
2. Update the `workspaceNames` map in `internal/typeform/create.go` with your actual workspace IDs
3. Redeploy the application

## Overview

Surveys are automatically organized into different Typeform workspaces based on their type. This allows for better organization and management of survey forms.

## Workspace Mapping

Each survey type is assigned to a specific workspace name:

| Survey Type | Workspace Name | Code Location |
|------------|----------------|---------------|
| Changes | `changes` | `internal/typeform/create.go` |
| CIC Announcements | `cic` | `internal/typeform/create.go` |
| InnerSource Announcements | `innersource` | `internal/typeform/create.go` |
| FinOps Announcements | `finops` | `internal/typeform/create.go` |
| General Announcements | `general` | `internal/typeform/create.go` |

## Implementation

The workspace mapping is hardcoded in `internal/typeform/create.go`:

```go
var workspaceNames = map[SurveyType]string{
    SurveyTypeChange:      "changes",
    SurveyTypeCIC:         "cic",
    SurveyTypeInnerSource: "innersource",
    SurveyTypeFinOps:      "finops",
    SurveyTypeGeneral:     "general",
}
```

## Behavior

When a survey is created:
1. The system determines the survey type (change, cic, innersource, finops, or general)
2. It looks up the corresponding workspace name from the map
3. The survey is created with a workspace reference: `https://api.typeform.com/workspaces/{workspace_name}`
4. If the workspace name is not found, it defaults to "general"

## Getting Workspace IDs from Typeform

### Method 1: Via Typeform UI

1. Log in to your Typeform account at https://admin.typeform.com
2. Click on your workspace name in the left sidebar
3. Look at the URL in your browser - it will be: `https://admin.typeform.com/workspace/{WORKSPACE_ID}`
4. Copy the `WORKSPACE_ID` from the URL (it's a string of letters and numbers)

### Method 2: Via Typeform API

You can also list all your workspaces using the Typeform API:

```bash
curl -X GET https://api.typeform.com/workspaces \
  -H "Authorization: Bearer YOUR_TYPEFORM_TOKEN"
```

This will return a JSON response with all your workspaces:

```json
{
  "items": [
    {
      "id": "abc123xyz",
      "name": "Changes Workspace",
      "href": "https://api.typeform.com/workspaces/abc123xyz"
    },
    {
      "id": "def456uvw",
      "name": "CIC Workspace",
      "href": "https://api.typeform.com/workspaces/def456uvw"
    }
  ]
}
```

Copy the `id` field for each workspace you want to use.

## Updating the Code with Workspace IDs

Once you have the workspace IDs, update the `workspaceNames` map in `internal/typeform/create.go`:

```go
var workspaceNames = map[SurveyType]string{
    SurveyTypeChange:      "abc123xyz",      // Replace with your changes workspace ID
    SurveyTypeCIC:         "def456uvw",      // Replace with your CIC workspace ID
    SurveyTypeInnerSource: "ghi789rst",      // Replace with your innersource workspace ID
    SurveyTypeFinOps:      "jkl012mno",      // Replace with your finops workspace ID
    SurveyTypeGeneral:     "pqr345stu",      // Replace with your general workspace ID
}
```

## Creating Workspaces in Typeform

If you don't have workspaces yet:

1. Log in to your Typeform account
2. Click on "Workspaces" in the left sidebar
3. Click "Create workspace"
4. Name each workspace (suggested names: "Changes", "CIC", "InnerSource", "FinOps", "General")
5. After creating each workspace, follow the steps above to get its ID

## Modifying Workspace Names

If you need to use different workspace names:
1. Edit `internal/typeform/create.go`
2. Update the `workspaceNames` map with your desired names
3. Redeploy the application

## Example

For a change completion survey:
1. Survey type is determined as `SurveyTypeChange`
2. System looks up workspace name: `"changes"`
3. Survey is created with workspace href: `https://api.typeform.com/workspaces/changes`
4. The survey appears in the "changes" workspace in Typeform
