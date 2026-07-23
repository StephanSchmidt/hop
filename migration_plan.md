# Hop CLI - Go to Rust Migration Plan

## Project Overview

**Current:** Go CLI tool for managing Bunny CDN pull zones
**Target:** Rust CLI tool with identical functionality and CLI interface

## CLI Interface (Must Match Exactly)

```
hop [--debug] <command>

Commands:
  check                    Run all checks (rules, DNS, SSL) for a pull zone
    -k, --key <KEY>        Bunny CDN API key (required)
    -z, --zone <ZONE>      Pull Zone name (required)
    --skip-health          Skip HTTP health checks

  rules                    Manage redirect rules
    add                    Add a new 302 redirect
      -k, --key <KEY>
      -z, --zone <ZONE>
      --from <URL>         Source URL path (required)
      --to <URL>           Destination URL (required)
      --desc <DESC>        Edge rule description

    list                   List all existing 302 redirects
      -k, --key <KEY>
      -z, --zone <ZONE>

    check                  Check redirect rules for potential issues
      -k, --key <KEY>
      -z, --zone <ZONE>
      --skip-health

    slash-add              Add trailing slash redirects for extensionless URLs
      -k, --key <KEY>
      -z, --zone <ZONE>
      --host <HOST>        Destination hostname (required)

    slash-remove           Remove trailing slash redirect rules
      -k, --key <KEY>
      -z, --zone <ZONE>

  cdn                      Manage CDN content
    push                   Push files from local directory to CDN storage
      -k, --key <KEY>
      -z, --zone <ZONE>
      --from <DIR>         Local directory path (required)

    check                  Check SSL configuration for all pull zone hostnames
      -k, --key <KEY>
      -z, --zone <ZONE>

  dns                      Manage DNS records
    list                   List DNS A and CNAME records for a pull zone
      -k, --key <KEY>
      -z, --zone <ZONE>

    check                  Check DNS records exist for pull zone hostnames
      -k, --key <KEY>
      -z, --zone <ZONE>

  stats                    View pull zone statistics
    show                   Show usage statistics for a pull zone
      -k, --key <KEY>
      -z, --zone <ZONE>
      --days <N>           Number of days (1-30, default: 7)
      --hourly             Show hourly breakdown
      --detailed           Show detailed charts and geographic distribution

  minify <source> <target> Minify HTML/CSS/JS and optimize images to WebP
    --cache <DIR>          Cache directory (default: .minify-cache)
    --force                Force reprocessing of all files
    --exclude <PATTERN>    Glob patterns to exclude (default: newsletter/**)

  security                 Check security headers configuration
    check                  Check security headers are configured as edge rules
      -k, --key <KEY>
      -z, --zone <ZONE>

    fix                    Add missing security headers as edge rules
      -k, --key <KEY>
      -z, --zone <ZONE>
```

## Library Mappings (From rinku)

| Go Library | Rust Equivalent | Notes |
|------------|-----------------|-------|
| github.com/spf13/cobra | clap | CLI framework - already used in existing skeleton |
| github.com/kolesa-team/go-webp | webp (aspect-analytics) | WebP encoding |
| github.com/tdewolff/minify/v2 | minify-html, minify-js, css-minify | Separate crates for each |
| github.com/tdewolff/font | woff2 or DROP | TTF to WOFF2 conversion - evaluate if pure Rust solution exists |
| net/http | reqwest | HTTP client |
| encoding/json | serde, serde_json | JSON serialization |
| crypto/sha256 | sha2 | SHA256 hashing |
| image/* | image | Image decoding/processing |
| golang.org/x/image/draw | image (resize feature) | Image resizing |
| regexp | regex | Regular expressions |
| sync | tokio (async) or rayon (parallel) | Concurrency |
| path/filepath | std::path | Path operations |
| context | tokio timeout/cancellation | Context and timeouts |

## Required Rust Dependencies (Cargo.toml)

```toml
[dependencies]
clap = { version = "4", features = ["derive"] }
reqwest = { version = "0.12", features = ["json", "blocking"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
tokio = { version = "1", features = ["full"] }
sha2 = "0.10"
hex = "0.4"
image = "0.25"
webp = "0.3"
minify-html = "0.15"
regex = "1"
walkdir = "2"
chrono = { version = "0.4", features = ["serde"] }
thiserror = "1"
anyhow = "1"

[dev-dependencies]
tokio-test = "0.4"
```

## Module Structure

```
src/
  main.rs           # Entry point, CLI dispatch
  cli/
    mod.rs          # CLI definitions (clap structs)
    handlers.rs     # Command handlers
  api/
    mod.rs          # Bunny API client
    types.rs        # API data types (PullZone, EdgeRule, etc.)
  cdn/
    mod.rs          # CDN push functionality
    upload.rs       # File upload logic
  dns/
    mod.rs          # DNS operations
  rules/
    mod.rs          # Edge rules management
    check.rs        # Rule validation
    slash.rs        # Trailing slash rules
  security/
    mod.rs          # Security headers check/fix
  stats/
    mod.rs          # Statistics display
  minify/
    mod.rs          # Minification pipeline
    html.rs         # HTML processing with srcset rewriting
    images.rs       # Image to WebP conversion
    fonts.rs        # TTF to WOFF2 conversion (may be dropped)
  util/
    mod.rs          # Common utilities
```

## Implementation Phases

### Phase 1: Core Infrastructure
1. Set up Cargo.toml with dependencies
2. Implement api/ module with Bunny API client
3. Implement api/types.rs with all data structures

### Phase 2: Read-Only Commands
1. `rules list` - List edge rules
2. `dns list` - List DNS records
3. `dns check` - Check DNS records
4. `cdn check` - Check SSL configuration
5. `stats show` - Show statistics

### Phase 3: Write Commands
1. `rules add` - Add edge rule
2. `rules check` - Check rules (includes HTTP health checks)
3. `rules slash-add` - Add trailing slash rules
4. `rules slash-remove` - Remove slash rules
5. `security check` - Check security headers
6. `security fix` - Fix security headers

### Phase 4: CDN Push
1. File listing and checksum calculation
2. Remote file comparison
3. Parallel upload with progress

### Phase 5: Minify Command
1. File walking with exclusions
2. HTML/CSS/JS minification
3. Image to WebP conversion with responsive srcset
4. HTML img tag rewriting
5. CSS font reference rewriting
6. TTF to WOFF2 conversion (evaluate or drop)

### Phase 6: Global Check Command
1. Combine rules, DNS, SSL checks

## Test Translation

| Go Test File | Tests to Translate |
|--------------|-------------------|
| bunny_test.go | BunnyTime parsing, formatBoolStatus, formatSSLCertificateStatus, strictUnmarshal |
| dns_test.go | formatDNSRecordType, isTargetRecordType, normalizeHostname, createHostnameMap, filterMatchingDNSRecords |
| edgerule_test.go | isValidDomain, isSuspiciousURL, normalizeURL, extractSourceURL, buildRedirectMap, slash rule tests |
| push_test.go | shouldSkipUpload |
| security_test.go | analyzeSecurityHeaders, action type constants, header recommendations |

## Functionality to Potentially Drop

1. **TTF to WOFF2 conversion** - The `tdewolff/font` library has no direct Rust equivalent from rinku. Options:
   - Use `woff2` crate (evaluate)
   - Drop this feature
   - Use external tool

   **Decision:** Evaluate woff2 crate first; drop if not viable.

## Output Format Requirements

- NO emojis in output (per CLAUDE.md)
- Use text prefixes: OK, ERROR, WARN, MISSING, SKIP
- Match existing output format exactly

## Verification Checklist

- [ ] All CLI commands match Go version exactly
- [ ] All flags and options match
- [ ] Output format matches (no emojis)
- [ ] All unit tests translated and passing
- [ ] HTTP API calls produce identical results
- [ ] File operations (push, minify) work correctly
- [ ] Error handling matches Go behavior
- [ ] Exit codes match

## Confidence Assessment

**High Confidence:**
- CLI structure (clap is mature, skeleton already exists)
- HTTP API calls (reqwest is well-established)
- JSON handling (serde is the standard)
- Edge rules, DNS, security modules (pure logic)
- Statistics display (pure formatting)

**Medium Confidence:**
- CDN push with parallel uploads (tokio concurrency patterns differ from Go)
- Image processing (image crate is good, but WebP encoding quality needs testing)
- HTML minification with srcset rewriting (minify-html may have different behavior)

**Low Confidence:**
- TTF to WOFF2 conversion (may need to drop)
- Exact output format matching (needs careful testing)

## Next Steps

1. Verify the existing skeleton in src/ is compatible with this plan
2. Create full Cargo.toml with all dependencies
3. Start with Phase 1: Core Infrastructure
4. Implement and test incrementally
