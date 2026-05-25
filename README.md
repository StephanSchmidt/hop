# Hop

**This app is not an official one.**

A Go command-line tool to use [Bunny CDN](https://bunny.net) for static sites 

* Manage 302 redirects
* Upload files to CDN storage
* Minify HTML/CSS/JS and optimize images to WebP
* List DNS A and CNAME records for pull zones
* View pull zone usage statistics
* Check and fix security headers (HSTS, X-Frame-Options, etc.)

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

### Minify and Optimize
```bash
# Minify HTML/CSS/JS and convert images to WebP
hop minify SOURCE_DIR TARGET_DIR

# Force reprocessing of all files (ignore cache)
hop minify SOURCE_DIR TARGET_DIR --force

# Exclude specific paths
hop minify SOURCE_DIR TARGET_DIR --exclude "drafts/**" --exclude "private/**"
```

### DNS Records Management
```bash
# List DNS A and CNAME records for pull zone
hop dns list --key YOUR_API_KEY --zone PULL_ZONE_NAME

# Check DNS records exist for pull zone hostnames
hop dns check --key YOUR_API_KEY --zone PULL_ZONE_NAME
```

### Statistics and Analytics
```bash
# View pull zone usage statistics (default: last 7 days)
hop stats show --key YOUR_API_KEY --zone PULL_ZONE_NAME

# View hourly breakdown for last 3 days
hop stats show --key YOUR_API_KEY --zone PULL_ZONE_NAME --days 3 --hourly

# View detailed statistics with charts and geographic distribution
hop stats show --key YOUR_API_KEY --zone PULL_ZONE_NAME --detailed
```

### Security Headers
```bash
# Check which security headers are configured
hop security check --key YOUR_API_KEY --zone PULL_ZONE_NAME

# Add missing security headers as edge rules
hop security fix --key YOUR_API_KEY --zone PULL_ZONE_NAME
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

### `minify` - Minify HTML/CSS/JS and optimize images

**Required Arguments:**
- `<source>`: Source directory containing files to process
- `<target>`: Target directory for output (will be cleaned on each run)

**Optional Parameters:**
- `--cache`: Cache directory for WebP conversions (default: `.minify-cache`)
- `--force`: Force reprocessing of all files, ignoring cache
- `--exclude`: Glob patterns to exclude (default: `newsletter/**`). Can be specified multiple times.

**What it does:**
- Minifies HTML, CSS, JavaScript, SVG, and XML files
- Converts PNG/JPG images to WebP format with quality optimization
- Generates responsive image variants with srcset
- Converts TTF fonts to WOFF2 format
- Uses persistent cache to skip unchanged images (based on content hash)
- Parallel processing for faster builds

**Notes:**
- The target directory is completely replaced on each run
- Image processing is cached in `.minify-cache/` by default (add to .gitignore)
- Uses content hashing to detect changes - only reprocesses modified files
- Ideal for static site generators: run before `cdn push`

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

### `stats show` - Show usage statistics for a pull zone

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will show statistics for this pull zone

**Optional Parameters:**
- `--days`: Number of days to retrieve statistics for (1-30, default: 7)
- `--hourly`: Show hourly breakdown instead of daily aggregation
- `--detailed`: Show detailed charts and geographic distribution data

**Notes:**
- Shows total bandwidth used, requests served, and cache hit rate
- By default shows summary statistics for the last 7 days
- With `--detailed` flag, displays bandwidth/requests charts over time and geographic traffic distribution
- With `--hourly` flag, breaks down data by hour instead of by day
- Shows top 5 countries by bandwidth usage in summary view
- All bandwidth values are displayed in human-readable format (B, KB, MB, GB, TB)
- Request counts are formatted with comma separators for readability

### `security check` - Check security headers configuration

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will check edge rules for security headers

**What it checks:**
- Strict-Transport-Security (HSTS) - Forces HTTPS connections
- X-Frame-Options - Prevents clickjacking
- X-Content-Type-Options - Prevents MIME sniffing
- Referrer-Policy - Controls referrer information
- X-XSS-Protection - Disables legacy XSS filter
- Permissions-Policy - Restricts browser features
- Cross-Origin-Opener-Policy - Isolates browsing context
- Cross-Origin-Embedder-Policy - Blocks cross-origin resources
- Cross-Origin-Resource-Policy - Limits resource loading

**Notes:**
- Critical headers (HSTS, X-Frame-Options, X-Content-Type-Options) show as ERROR if missing
- Other headers show as WARN if missing
- Exits with status code 1 if any critical headers are missing
- Does NOT check dangerous headers like HPKP (certificate pinning) or CSP (site-specific)

### `security fix` - Add missing security headers as edge rules

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will add edge rules for missing security headers

**What it does:**
- Creates edge rules for each missing security header
- Uses recommended values from OWASP guidelines
- Skips headers that are already configured
- Applies headers to all URLs (using `*` pattern)

**Notes:**
- Only adds safe headers - excludes HPKP (can brick sites), CSP (site-specific), and Expect-CT (deprecated)
- Does not modify existing header configurations
- Each header is added as a separate edge rule for easy management

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

### View pull zone statistics
```bash
hop stats show --key your-api-key --zone amazingctosite
```

### View statistics for last 14 days with hourly breakdown
```bash
hop stats show --key your-api-key --zone amazingctosite --days 14 --hourly
```

### View detailed statistics with charts and geographic data
```bash
hop stats show --key your-api-key --zone amazingctosite --detailed
```

### Minify and optimize a static site
```bash
hop minify ./site ./dist
```

### Force full rebuild (ignore cache)
```bash
hop minify ./site ./dist --force
```

### Exclude specific directories from minification
```bash
hop minify ./site ./dist --exclude "drafts/**" --exclude "assets/raw/**"
```

### Full static site deployment workflow
```bash
hop minify ./site ./dist && hop cdn push --key your-api-key --zone amazingctosite --from ./dist
```

### Check security headers
```bash
hop security check --key your-api-key --zone amazingctosite
```

### Fix missing security headers
```bash
hop security fix --key your-api-key --zone amazingctosite
```

### Debug any command (add --debug before command)
```bash
hop --debug check --key your-api-key --zone amazingctosite
hop --debug dns list --key your-api-key --zone amazingctosite
hop --debug dns check --key your-api-key --zone amazingctosite
hop --debug rules check --key your-api-key --zone amazingctosite
hop --debug cdn push --key your-api-key --zone amazingctosite --from ./dist
hop --debug cdn check --key your-api-key --zone amazingctosite
hop --debug stats show --key your-api-key --zone amazingctosite
hop --debug security check --key your-api-key --zone amazingctosite
hop --debug security fix --key your-api-key --zone amazingctosite
hop --debug minify ./site ./dist
```

## Building

```bash
make
```