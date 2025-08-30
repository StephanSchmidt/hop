# Hop

**This app is not an official one.**

A Go command-line tool to use [Bunny CDN](https://bunny.net) for static sites 

* Manage 302 redirects
* Upload files to CDN storage

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

### Add a new redirect
```bash
hop rules add --key YOUR_API_KEY --zone PULL_ZONE_NAME --from SOURCE_URL --to DESTINATION_URL [--desc DESCRIPTION]
```

### List existing redirects
```bash
hop rules list --key YOUR_API_KEY --zone PULL_ZONE_NAME
```

### Push files to CDN storage
```bash
hop cdn push --key YOUR_API_KEY --zone PULL_ZONE_NAME --from LOCAL_DIRECTORY
```

## Commands

### `rules add` - Add a new 302 redirect

**Required Parameters:**
- `--key`: Your Bunny CDN API key
- `--zone`: The Pull Zone name (e.g., "amazingctosite") - will automatically lookup the ID
- `--from`: Source URL path to redirect from (e.g., "/old-page")
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

## Examples

### Add a redirect for a specific page
```bash
hop rules add --key your-api-key --zone amazingctosite --from "/old-page" --to "https://amazingcto.com/new-page"
```

### Add a redirect with wildcard pattern
```bash
hop rules add --key your-api-key --zone amazingctosite --from "/blog/*" --to "https://amazingcto.com/articles/$1"
```

### Add a redirect to external domain
```bash
hop rules add --key your-api-key --zone amazingctosite --from "/external" --to "https://external-site.com/"
```

### Add a redirect with custom description
```bash
hop rules add --key your-api-key --zone amazingctosite --from "/contact" --to "/contact-us" --desc "Redirect old contact page"
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

## Building

```bash
make
```