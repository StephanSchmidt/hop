# Hop

**This app is not an official one.**

A Go command-line tool to use [Bunny CDN](https://bunny.net) for static sites 

* Manage 302 redirects
* Upload files to CDN storage
* List DNS A and CNAME records for pull zones

## Installation

### From Source

1. Clone the repository:
```bash
git clone https://github.com/StephanSchmidt/hop.git
cd hop
```

2. Build the binary:
```bash
make
```

3. (Optional) Move the binary to your PATH:
```bash
sudo mv hop /usr/local/bin/
```

### Direct Installation with Go

```bash
go install github.com/StephanSchmidt/hop/cmd/hop@latest
```

## Usage

### Comprehensive Check
```bash
# Run all checks (rules, DNS, SSL) for a pull zone
hop check --key YOUR_API_KEY --zone PULL_ZONE_NAME [--skip-health]
```

### Redirect Rules Management
```bash
# Add a new redirect  
hop rules add --key YOUR_API_KEY --zone PULL_ZONE_NAME --from TRIGGER_PATH --to DESTINATION_URL [--desc DESCRIPTION]

# List existing redirects
hop rules list --key YOUR_API_KEY --zone PULL_ZONE_NAME

# Check redirect rules for issues
hop rules check --key YOUR_API_KEY --zone PULL_ZONE_NAME [--skip-health]
```

### CDN Content Management
```bash
# Push files to CDN storage
hop cdn push --key YOUR_API_KEY --zone PULL_ZONE_NAME --from LOCAL_DIRECTORY

# Check SSL configuration for all pull zone hostnames
hop cdn check --key YOUR_API_KEY --zone PULL_ZONE_NAME
```

### DNS Records Management
```bash
# List DNS A and CNAME records for pull zone
hop dns list --key YOUR_API_KEY --zone PULL_ZONE_NAME

# Check DNS records exist for pull zone hostnames
hop dns check --key YOUR_API_KEY --zone PULL_ZONE_NAME
```

## Commands

### `check` - Run all checks (rules, DNS, SSL) for a pull zone

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will automatically lookup the ID

**Optional Parameters:**
- `--skip-health`: Skip HTTP health checks for faster execution

**What it does:**
- Runs comprehensive redirect rule analysis (same as `rules check`)
- Validates DNS A and CNAME records exist for all pull zone hostnames
- Tests SSL/HTTPS connectivity and Force SSL redirect configuration
- Provides a unified summary of all issues found
- Exits with status code 1 if any errors are found

### `rules add` - Add a new 302 redirect

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will automatically lookup the ID
- `--from`: Trigger path pattern to match (e.g., "*/old-page" or "*/blog/*")
- `--to`: Destination URL to redirect to (e.g., "https://example.com/new-page")

**Optional Parameters:**
- `--desc`: Custom description for the redirect rule (auto-generated if not provided)

### `rules list` - List existing 302 redirects

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will automatically lookup the ID

### `rules check` - Check redirect rules for potential issues

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will automatically lookup the ID

**Optional Parameters:**
- `--skip-health`: Skip HTTP health checks for faster execution

### `cdn push` - Push files to CDN storage

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will automatically lookup associated storage zone
- `--from`: Local directory path to upload files from

**Notes:**
- Recursively uploads all files from the specified directory
- Automatically finds the storage zone associated with the pull zone
- Preserves directory structure in the CDN storage
- Shows upload progress and summary

### `cdn check` - Check SSL configuration for all pull zone hostnames

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will test SSL connectivity for all hostnames

**Notes:**
- Tests actual HTTPS connectivity by making requests to each hostname
- Tests Force SSL redirect by checking if HTTP requests redirect to HTTPS
- Automatically skips `.b-cdn.net` hostnames (SSL managed automatically by Bunny)
- Provides concise output: only shows issues that need attention
- Exits with status code 1 if HTTPS is not working
- Warns if HTTPS works but Force SSL redirect is not configured
- Uses text indicators: OK, WARN, ERROR (no emojis)

### `dns list` - List DNS A and CNAME records for pull zone

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will lookup DNS records for pull zone hostnames

**Notes:**
- Finds all hostnames associated with the pull zone
- Searches all DNS zones for A and CNAME records matching those hostnames
- Displays records in format: `hostname - record_type - value`
- Supports both full domain names and relative DNS record names

### `dns check` - Check DNS records exist for pull zone hostnames

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will validate DNS records for pull zone hostnames

**Notes:**
- Validates that DNS records exist for all hostnames associated with the pull zone
- Automatically skips `.b-cdn.net` hostnames (automatically managed by Bunny CDN)
- Uses text indicators: `OK` for found records, `MISSING` for missing records, `SKIP` for ignored hostnames
- Exits with status code 1 if any required DNS records are missing
- Use `--debug` flag for detailed hostname matching information

## Global Options

The following options can be used with any command:

### `--debug` - Enable debug output

Add `--debug` before any command for detailed troubleshooting output:

```bash
hop --debug COMMAND [OPTIONS]
```

## Examples

### Run comprehensive check for a pull zone
```bash
hop check --key your-api-key --zone amazingctosite
```

### Run comprehensive check without health checks (faster)
```bash
hop check --key your-api-key --zone amazingctosite --skip-health
```

### Add a redirect for a specific page
```bash
hop rules add --key your-api-key --zone amazingctosite --from "*/old-page" --to "https://amazingcto.com/new-page"
```

### Add a redirect with wildcard pattern
```bash
hop rules add --key your-api-key --zone amazingctosite --from "*/blog/*" --to "https://amazingcto.com/articles/$1"
```

### Add a redirect to external domain
```bash
hop rules add --key your-api-key --zone amazingctosite --from "*/external" --to "https://external-site.com/"
```

### Add a redirect with custom description
```bash
hop rules add --key your-api-key --zone amazingctosite --from "*/contact" --to "/contact-us" --desc "Redirect old contact page"
```

### List all existing redirects
```bash
hop rules list --key your-api-key --zone amazingctosite
```

### Check redirect rules for issues
```bash
hop rules check --key your-api-key --zone amazingctosite
```

### Check redirect rules without health checks
```bash
hop rules check --key your-api-key --zone amazingctosite --skip-health
```

### Push local directory to CDN storage
```bash
hop cdn push --key your-api-key --zone amazingctosite --from ./dist
```

### Push website files to CDN
```bash
hop cdn push --key your-api-key --zone amazingctosite --from ./public
```

### Check SSL configuration for CDN
```bash
hop cdn check --key your-api-key --zone amazingctosite
```

### List DNS records for pull zone
```bash
hop dns list --key your-api-key --zone amazingctosite
```

### Check DNS records for pull zone
```bash
hop dns check --key your-api-key --zone amazingctosite
```

### Debug any command (add --debug before command)
```bash
hop --debug check --key your-api-key --zone amazingctosite
hop --debug dns list --key your-api-key --zone amazingctosite
hop --debug dns check --key your-api-key --zone amazingctosite  
hop --debug rules check --key your-api-key --zone amazingctosite
hop --debug cdn push --key your-api-key --zone amazingctosite --from ./dist
hop --debug cdn check --key your-api-key --zone amazingctosite
```

## Building

```bash
make
```