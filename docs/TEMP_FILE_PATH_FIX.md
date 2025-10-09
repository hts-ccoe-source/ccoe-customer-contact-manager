# Temporary File Path Fix Summary

## Problem Identified
The Lambda function was failing to send emails with this error:
```
2025/10/06 05:52:27 Failed to send approval request email for customer htsnonprod: failed to send approval request email: failed to load metadata file .//tmp/change-metadata-1306382752.json: no such file or directory
```

## Root Cause Analysis

### 1. **Path Concatenation Issue**
The SES functions expect relative paths and add a config path prefix:
```go
// In SES functions:
configPath := config.GetConfigPath()  // Returns "./"
metadataFile := configPath + jsonMetadataPath  // Results in "./filename"
```

### 2. **Temporary File Location Mismatch**
The original code created temp files in the system temp directory:
```go
// BEFORE (BROKEN):
tempFile, err := os.CreateTemp("", "change-metadata-*.json")  // Creates in /tmp/
return tempFile.Name()  // Returns "/tmp/change-metadata-*.json"
```

### 3. **Path Resolution Problem**
When SES functions tried to read the file:
```go
// SES function does:
configPath + jsonMetadataPath = "./" + "/tmp/change-metadata-*.json"
// Results in: ".//tmp/change-metadata-*.json" (invalid path)
```

## Solution Implemented

### 1. **Create Temp Files in Config Directory**
```go
// AFTER (FIXED):
func createTempMetadataFile(changeDetails map[string]interface{}) (string, error) {
    // Get config path where SES functions expect files to be
    configPath := config.GetConfigPath()
    
    // Create temporary file in the config directory
    tempFile, err := os.CreateTemp(configPath, "change-metadata-*.json")
    if err != nil {
        return "", fmt.Errorf("failed to create temp file in %s: %w", configPath, err)
    }
    defer tempFile.Close()

    // Write metadata to file
    encoder := json.NewEncoder(tempFile)
    encoder.SetIndent("", "  ")
    if err := encoder.Encode(metadata); err != nil {
        os.Remove(tempFile.Name())
        return "", fmt.Errorf("failed to write metadata to temp file: %w", err)
    }

    // Return just the filename (without path) since SES functions add configPath
    filename := filepath.Base(tempFile.Name())
    return filename, nil
}
```

### 2. **Fixed Cleanup Path**
```go
// BEFORE (BROKEN):
defer os.Remove(tempFile) // tempFile is just filename

// AFTER (FIXED):
configPath := config.GetConfigPath()
fullTempPath := filepath.Join(configPath, tempFile)
defer os.Remove(fullTempPath) // Use full path for cleanup
```

### 3. **Applied to Both Functions**
Fixed both email sending functions:
- `SendApprovalRequestEmail()` - for approval requests
- `SendApprovedAnnouncementEmail()` - for approved announcements

## Technical Details

### File Creation Flow:
1. **Get Config Path**: `config.GetConfigPath()` returns `"./"` (current directory)
2. **Create Temp File**: `os.CreateTemp(configPath, "change-metadata-*.json")` creates file in `./`
3. **Return Filename**: `filepath.Base(tempFile.Name())` returns just the filename
4. **SES Function**: Concatenates `configPath + filename` = `"./change-metadata-*.json"`
5. **File Access**: SES function can now find the file at the correct path

### Cleanup Flow:
1. **Reconstruct Path**: `filepath.Join(configPath, tempFile)` creates full path
2. **Remove File**: `os.Remove(fullTempPath)` cleans up the temporary file

## Expected Behavior After Fix

### ✅ **Successful File Creation**
```
Temp file created: ./change-metadata-1306382752.json
SES function looks for: ./ + change-metadata-1306382752.json = ./change-metadata-1306382752.json
Result: File found and processed successfully
```

### ✅ **Successful Email Sending**
```
2025/10/06 05:52:27 📧 Sending approval request email for change CHG-1234567890
2025/10/06 05:52:27 ✅ Approval request email sent to 5 members of topic aws-approval from ccoe@nonprod.ccoe.hearst.com
```

### ✅ **Proper Cleanup**
```
Temp file removed: ./change-metadata-1306382752.json
No leftover temporary files in the working directory
```

## Both Functions Verified

### SendApprovalRequestEmail ✅
- **Topic**: "aws-approval"
- **SES Function**: `ses.SendApprovalRequest()`
- **Temp File**: Created in config directory
- **Cleanup**: Uses full path
- **Logging**: Shows recipient count

### SendApprovedAnnouncementEmail ✅
- **Topic**: "aws-announce"  
- **SES Function**: `ses.SendChangeNotificationWithTemplate()`
- **Temp File**: Created in config directory
- **Cleanup**: Uses full path
- **Logging**: Shows recipient count

## File Path Examples

### Before Fix (Broken):
```
Temp file: /tmp/change-metadata-1306382752.json
SES looks for: .//tmp/change-metadata-1306382752.json
Result: File not found error
```

### After Fix (Working):
```
Temp file: ./change-metadata-1306382752.json
SES looks for: ./change-metadata-1306382752.json
Result: File found and processed
```

## Benefits

### 🐛 **Fixed File Not Found Error**
- Temporary files are now created in the correct directory
- SES functions can find and read the metadata files
- No more path concatenation issues

### 🧹 **Proper Cleanup**
- Temporary files are correctly removed after use
- No leftover files in the working directory
- Full path used for reliable cleanup

### 📧 **Reliable Email Sending**
- Both approval requests and announcements work correctly
- Consistent implementation across both functions
- Proper error handling and logging

### 🔧 **Maintainable Code**
- Clear separation between filename and full path
- Consistent pattern for both email functions
- Proper use of filepath utilities

The temporary file path issue has been resolved for both approval requests and approved announcements, ensuring reliable email delivery.