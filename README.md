# Hop

**This app is not an official one.**

A Go command-line tool to manage 302 redirects in [Bunny CDN](https://bunny.net) pull zones.

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
hop add -key YOUR_API_KEY -zone PULL_ZONE_NAME -from SOURCE_URL -to DESTINATION_URL [-desc DESCRIPTION]
```

### List existing redirects
```bash
hop list -key YOUR_API_KEY -zone PULL_ZONE_NAME
```

## Commands

### `add` - Add a new 302 redirect

**Required Parameters:**
- `-key`: Your Bunny CDN API key
- `-zone`: The Pull Zone name (e.g., "amazingctosite") - will automatically lookup the ID
- `-from`: Source URL path to redirect from (e.g., "/old-page")
- `-to`: Destination URL to redirect to (e.g., "https://example.com/new-page")

**Optional Parameters:**
- `-desc`: Custom description for the redirect rule (auto-generated if not provided)

### `list` - List existing 302 redirects

**Required Parameters:**
- `-key`: Your Bunny CDN API key
- `-zone`: The Pull Zone name (e.g., "amazingctosite") - will automatically lookup the ID

## Examples

### Add a redirect for a specific page
```bash
hop add -key your-api-key -zone amazingctosite -from "/old-page" -to "https://amazingcto.com/new-page"
```

### Add a redirect with wildcard pattern
```bash
hop add -key your-api-key -zone amazingctosite -from "/blog/*" -to "https://amazingcto.com/articles/$1"
```

### Add a redirect to external domain
```bash
hop add -key your-api-key -zone amazingctosite -from "/external" -to "https://external-site.com/"
```

### Add a redirect with custom description
```bash
hop add -key your-api-key -zone amazingctosite -from "/contact" -to "/contact-us" -desc "Redirect old contact page"
```

### List all existing redirects
```bash
hop list -key your-api-key -zone amazingctosite
```

## Building

```bash
make
```