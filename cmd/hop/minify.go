package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"github.com/tdewolff/font"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/json"
	"github.com/tdewolff/minify/v2/svg"
	"github.com/tdewolff/minify/v2/xml"
	"golang.org/x/image/draw"
)

// MinifyConfig holds configuration for the minify command
type MinifyConfig struct {
	Source   string
	Target   string
	CacheDir string // persistent cache for WebP conversions
	Force    bool
	Exclude  []string
}

// ImageInfo holds information about a processed image
type ImageInfo struct {
	OriginalPath string
	Hash         string
	Width        int
	Height       int
	Sizes        []ImageSize
}

// ImageSize represents one size variant of an image
type ImageSize struct {
	Width    int
	Filename string
	Path     string
}

// Minifier handles the minification process
type Minifier struct {
	config           MinifyConfig
	minify           *minify.M
	imageMap         map[string]*ImageInfo // maps original path to image info
	imageMapMu       sync.Mutex
	imageWorkerCount int // CPU-heavy: image resize + WebP encoding
	fileWorkerCount  int // lighter: minification + I/O
}

// NewMinifier creates a new Minifier instance
func NewMinifier(config MinifyConfig) *Minifier {
	m := minify.New()
	m.Add("text/html", &html.Minifier{KeepDocumentTags: true})
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("application/javascript", js.Minify)
	m.AddFunc("text/javascript", js.Minify)
	m.AddFunc("application/json", json.Minify)
	m.AddFunc("image/svg+xml", svg.Minify)
	m.AddFunc("text/xml", xml.Minify)
	m.AddFunc("application/xml", xml.Minify)

	numCPU := runtime.NumCPU()

	return &Minifier{
		config:           config,
		minify:           m,
		imageMap:         make(map[string]*ImageInfo),
		imageWorkerCount: numCPU,            // CPU-bound: match core count
		fileWorkerCount:  numCPU + numCPU/2, // IO-bound: can exceed cores (1.5x)
	}
}

// Run executes the minification process
func (m *Minifier) Run() error {
	// Clean target directory
	if err := os.RemoveAll(m.config.Target); err != nil {
		return fmt.Errorf("failed to clean target directory: %w", err)
	}

	// First pass: collect all files and process images
	var files []string
	var imageFiles []string

	err := filepath.Walk(m.config.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check exclusions
		relPath, _ := filepath.Rel(m.config.Source, path)
		for _, exclude := range m.config.Exclude {
			if matched, _ := filepath.Match(exclude, relPath); matched {
				return nil
			}
			// Also check if path starts with exclude pattern (for directory patterns like newsletter/**)
			if strings.HasPrefix(relPath, strings.TrimSuffix(exclude, "**")) {
				return nil
			}
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			imageFiles = append(imageFiles, path)
		} else {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk source directory: %w", err)
	}

	fmt.Printf("Found %d files and %d images to process\n", len(files), len(imageFiles))
	fmt.Printf("Using %d image workers (CPU-bound), %d file workers (IO-bound)\n", m.imageWorkerCount, m.fileWorkerCount)

	// Process images first (to build imageMap for HTML rewriting)
	fmt.Println("Processing images...")
	if err := m.processImages(imageFiles); err != nil {
		return fmt.Errorf("failed to process images: %w", err)
	}

	// Process other files
	fmt.Println("Processing files...")
	if err := m.processFiles(files); err != nil {
		return fmt.Errorf("failed to process files: %w", err)
	}

	fmt.Printf("Done! Output written to %s\n", m.config.Target)
	return nil
}

// processImages processes all image files
func (m *Minifier) processImages(files []string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(files))
	semaphore := make(chan struct{}, m.imageWorkerCount)

	for _, file := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := m.processImage(path); err != nil {
				errChan <- fmt.Errorf("%s: %w", path, err)
			}
		}(file)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("image processing errors:\n%s", strings.Join(errs, "\n"))
	}

	return nil
}

// processImage processes a single image file
func (m *Minifier) processImage(srcPath string) error {
	relPath, _ := filepath.Rel(m.config.Source, srcPath)

	// Read source file
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Compute hash
	hash := computeHash(srcData)

	// Decode image
	img, _, err := image.Decode(bytes.NewReader(srcData))
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	// Generate size variants
	sizes := generateSizeVariants(origWidth)

	// Base name without extension
	ext := filepath.Ext(relPath)
	baseName := strings.TrimSuffix(relPath, ext)
	baseDir := filepath.Dir(relPath)

	info := &ImageInfo{
		OriginalPath: relPath,
		Hash:         hash,
		Width:        origWidth,
		Height:       origHeight,
		Sizes:        []ImageSize{},
	}

	// Process each size
	for _, width := range sizes {
		var filename string
		if width == origWidth {
			filename = fmt.Sprintf("%s.%s.webp", filepath.Base(baseName), hash)
		} else {
			filename = fmt.Sprintf("%s@%dw.%s.webp", filepath.Base(baseName), width, hash)
		}

		cachePath := filepath.Join(m.config.CacheDir, baseDir, filename)
		targetPath := filepath.Join(m.config.Target, baseDir, filename)

		// Check if already exists in cache
		if !m.config.Force {
			if _, err := os.Stat(cachePath); err == nil {
				if err := copyFile(cachePath, targetPath); err != nil {
					return fmt.Errorf("failed to copy from cache %s: %w", filename, err)
				}
				info.Sizes = append(info.Sizes, ImageSize{
					Width:    width,
					Filename: filename,
					Path:     filepath.Join(baseDir, filename),
				})
				continue
			}
		}

		// Resize image if needed
		var resizedImg image.Image
		if width == origWidth {
			resizedImg = img
		} else {
			resizedImg = resizeImage(img, width)
		}

		// Write to cache
		if err := m.writeWebP(cachePath, resizedImg); err != nil {
			return fmt.Errorf("failed to write WebP to cache %s: %w", filename, err)
		}

		// Copy from cache to target
		if err := copyFile(cachePath, targetPath); err != nil {
			return fmt.Errorf("failed to copy to target %s: %w", filename, err)
		}

		info.Sizes = append(info.Sizes, ImageSize{
			Width:    width,
			Filename: filename,
			Path:     filepath.Join(baseDir, filename),
		})
	}

	// Sort sizes descending
	sort.Slice(info.Sizes, func(i, j int) bool {
		return info.Sizes[i].Width > info.Sizes[j].Width
	})

	// Copy original PNG/JPG as fallback for meta tags (og:image, etc.)
	// These don't get rewritten like <img> tags do
	originalTargetPath := filepath.Join(m.config.Target, relPath)
	if err := copyFile(srcPath, originalTargetPath); err != nil {
		return fmt.Errorf("failed to copy original image %s: %w", relPath, err)
	}

	// Store in map
	m.imageMapMu.Lock()
	m.imageMap[relPath] = info
	m.imageMapMu.Unlock()

	fmt.Printf("  [IMG] %s → %d sizes + original\n", relPath, len(info.Sizes))
	return nil
}

// writeWebP writes an image as WebP
func (m *Minifier) writeWebP(path string, img image.Image) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 85)
	if err != nil {
		return err
	}

	return webp.Encode(f, img, options)
}

// processFiles processes non-image files
func (m *Minifier) processFiles(files []string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(files))
	semaphore := make(chan struct{}, m.fileWorkerCount)

	for _, file := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := m.processFile(path); err != nil {
				errChan <- fmt.Errorf("%s: %w", path, err)
			}
		}(file)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("file processing errors:\n%s", strings.Join(errs, "\n"))
	}

	return nil
}

// processFile processes a single file
func (m *Minifier) processFile(srcPath string) error {
	relPath, _ := filepath.Rel(m.config.Source, srcPath)
	targetPath := filepath.Join(m.config.Target, relPath)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	// Read source file
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(srcPath))
	var mimeType string
	var shouldRewriteImages bool

	switch ext {
	case ".html", ".htm":
		mimeType = "text/html"
		shouldRewriteImages = true
	case ".css":
		mimeType = "text/css"
	case ".js":
		mimeType = "application/javascript"
	case ".json":
		mimeType = "application/json"
	case ".svg":
		mimeType = "image/svg+xml"
	case ".xml":
		mimeType = "text/xml"
	case ".webp":
		// Copy WebP as-is
		return copyFile(srcPath, targetPath)
	case ".ttf":
		// Convert TTF to WOFF2
		return m.convertFontToWOFF2(srcPath, targetPath, relPath)
	default:
		// Copy other files as-is
		return copyFile(srcPath, targetPath)
	}

	content := string(srcData)

	// Rewrite image references in HTML and inline critical CSS
	if shouldRewriteImages {
		content = m.rewriteImageTags(content, filepath.Dir(relPath))
		content = injectResourceHints(content)
		content = m.inlineCriticalCSS(content)
	}

	// Rewrite font references in CSS (.ttf → .woff2, add font-display: swap)
	if mimeType == "text/css" {
		content = rewriteFontReferences(content)
	}

	// Minify
	minified, err := m.minify.String(mimeType, content)
	if err != nil {
		// If minification fails, use original content
		fmt.Printf("  [WARN] Minification failed for %s: %v\n", relPath, err)
		minified = content
	}

	// Write output
	if err := os.WriteFile(targetPath, []byte(minified), 0644); err != nil {
		return err
	}

	fmt.Printf("  [OK] %s\n", relPath)
	return nil
}

// rewriteImageTags rewrites <img> tags to use WebP srcset
func (m *Minifier) rewriteImageTags(content string, htmlDir string) string {
	// Match <img> tags - handles both minified (no space) and regular HTML
	imgRe := regexp.MustCompile(`<img\s*([^>]*?)>`)

	return imgRe.ReplaceAllStringFunc(content, func(match string) string {
		return m.rewriteImgTag(match, htmlDir)
	})
}

// rewriteImgTag rewrites a single <img> tag
func (m *Minifier) rewriteImgTag(imgTag string, htmlDir string) string {
	// Extract src attribute - handles both quoted and unquoted values
	// Quoted: src="value" or src='value'
	// Unquoted: src=value (ends at space or >)
	srcRe := regexp.MustCompile(`src=(?:["']([^"']+)["']|([^\s>]+))`)
	srcMatch := srcRe.FindStringSubmatch(imgTag)
	if srcMatch == nil {
		return imgTag
	}

	// Get the captured value (either group 1 for quoted or group 2 for unquoted)
	srcValue := srcMatch[1]
	if srcValue == "" {
		srcValue = srcMatch[2]
	}

	// Skip external URLs and data URIs
	if strings.HasPrefix(srcValue, "http://") || strings.HasPrefix(srcValue, "https://") || strings.HasPrefix(srcValue, "data:") {
		return imgTag
	}

	// Resolve path relative to HTML file
	var imagePath string
	if strings.HasPrefix(srcValue, "/") {
		// Absolute path from root
		imagePath = strings.TrimPrefix(srcValue, "/")
	} else {
		// Relative path
		imagePath = filepath.Join(htmlDir, srcValue)
	}
	imagePath = filepath.Clean(imagePath)

	// Check if we have this image in our map
	m.imageMapMu.Lock()
	info, ok := m.imageMap[imagePath]
	m.imageMapMu.Unlock()

	if !ok {
		// Not a processed image, return as-is
		return imgTag
	}

	// Build WebP srcset
	var srcsetParts []string
	for _, size := range info.Sizes {
		var webpPath string
		if strings.HasPrefix(srcValue, "/") {
			webpPath = "/" + strings.ReplaceAll(size.Path, "\\", "/")
		} else {
			// Make relative to HTML file
			relPath, _ := filepath.Rel(htmlDir, size.Path)
			webpPath = "./" + strings.ReplaceAll(relPath, "\\", "/")
		}
		srcsetParts = append(srcsetParts, fmt.Sprintf("%s %dw", webpPath, size.Width))
	}
	srcset := strings.Join(srcsetParts, ", ")

	// Get smallest size for fallback src
	smallestSize := info.Sizes[len(info.Sizes)-1]
	var fallbackSrc string
	if strings.HasPrefix(srcValue, "/") {
		fallbackSrc = "/" + strings.ReplaceAll(smallestSize.Path, "\\", "/")
	} else {
		relPath, _ := filepath.Rel(htmlDir, smallestSize.Path)
		fallbackSrc = "./" + strings.ReplaceAll(relPath, "\\", "/")
	}

	// Build new <img> tag with srcset
	newImg := imgTag

	// Replace src with fallback
	srcReplaceRe := regexp.MustCompile(`src=(?:["'][^"']+["']|[^\s>]+)`)
	newImg = srcReplaceRe.ReplaceAllString(newImg, fmt.Sprintf(`src="%s"`, fallbackSrc))

	// Add or replace srcset
	srcsetRe := regexp.MustCompile(`\s*srcset=["'][^"']*["']`)
	if srcsetRe.MatchString(newImg) {
		newImg = srcsetRe.ReplaceAllString(newImg, "")
	}

	// Add srcset and sizes before the closing >
	newImg = strings.TrimSuffix(newImg, ">")
	newImg = strings.TrimSuffix(newImg, " ")
	newImg = fmt.Sprintf(`%s srcset="%s" sizes="100vw">`, newImg, srcset)

	// Add loading="lazy" if not present
	if !strings.Contains(newImg, "loading=") {
		newImg = strings.Replace(newImg, "<img ", `<img loading="lazy" `, 1)
	}

	// Add decoding="async" if not present
	if !strings.Contains(newImg, "decoding=") {
		newImg = strings.Replace(newImg, "<img ", `<img decoding="async" `, 1)
	}

	// Add width and height only if NEITHER was present in original
	// AND no CSS sizing is used (to avoid conflicting with style-based sizing)
	widthRe := regexp.MustCompile(`width=["']?(\d+)["']?`)
	heightRe := regexp.MustCompile(`height=["']?(\d+)["']?`)

	hasWidth := widthRe.MatchString(imgTag)
	hasHeight := heightRe.MatchString(imgTag)
	hasCSSSize := strings.Contains(imgTag, "style=") &&
		(strings.Contains(imgTag, "width:") || strings.Contains(imgTag, "height:"))

	if !hasWidth && !hasHeight && !hasCSSSize {
		newImg = strings.Replace(newImg, "<img ", fmt.Sprintf(`<img width="%d" height="%d" `, info.Width, info.Height), 1)
	}

	return newImg
}

// generateSizeVariants generates the size variants to create
func generateSizeVariants(origWidth int) []int {
	// Standard widths, similar to jampack
	standardWidths := []int{3872, 3572, 3272, 2972, 2672, 2372, 2072, 1772, 1472, 1172, 872}

	var sizes []int
	for _, w := range standardWidths {
		if w <= origWidth {
			sizes = append(sizes, w)
		}
	}

	// Always include original if not in list
	if len(sizes) == 0 || sizes[0] != origWidth {
		sizes = append([]int{origWidth}, sizes...)
	}

	return sizes
}

// resizeImage resizes an image to the given width, maintaining aspect ratio
func resizeImage(img image.Image, width int) image.Image {
	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	// Calculate new height maintaining aspect ratio
	height := (origHeight * width) / origWidth

	// Create new image
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// Use high-quality resampling
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	return dst
}

// computeHash computes a short hash of the given data
func computeHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])[:8]
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// convertFontToWOFF2 converts a TTF font to WOFF2 format
func (m *Minifier) convertFontToWOFF2(srcPath, targetPath, relPath string) error {
	// Read TTF file
	ttfData, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read TTF: %w", err)
	}

	// Parse TTF to SFNT
	sfnt, err := font.ParseSFNT(ttfData, 0)
	if err != nil {
		return fmt.Errorf("failed to parse TTF: %w", err)
	}

	// Convert to WOFF2
	woff2Data, err := sfnt.WriteWOFF2()
	if err != nil {
		return fmt.Errorf("failed to convert to WOFF2: %w", err)
	}

	// Change extension from .ttf to .woff2
	woff2Path := strings.TrimSuffix(targetPath, ".ttf") + ".woff2"

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(woff2Path), 0755); err != nil {
		return err
	}

	// Write WOFF2 file
	if err := os.WriteFile(woff2Path, woff2Data, 0644); err != nil {
		return err
	}

	// Calculate size reduction
	origSize := len(ttfData)
	newSize := len(woff2Data)
	reduction := 100 - (newSize * 100 / origSize)

	fmt.Printf("  [FONT] %s → .woff2 (%d%% smaller)\n", relPath, reduction)
	return nil
}

// injectResourceHints adds preconnect hints for external domains
func injectResourceHints(htmlContent string) string {
	// Skip if already has preconnect hints
	if strings.Contains(htmlContent, `rel="preconnect"`) || strings.Contains(htmlContent, `rel=preconnect`) {
		return htmlContent
	}

	hints := `<link rel="preconnect" href="https://scripts.simpleanalyticscdn.com" crossorigin>
<link rel="preconnect" href="https://cal.com" crossorigin>
<link rel="dns-prefetch" href="https://scripts.simpleanalyticscdn.com">
<link rel="dns-prefetch" href="https://cal.com">`

	// Insert after <head> tag
	headRe := regexp.MustCompile(`(<head[^>]*>)`)
	return headRe.ReplaceAllString(htmlContent, "$1\n"+hints)
}

// inlineCriticalCSS inlines small CSS files directly into HTML
// Only inlines CSS files under the size threshold (10KB) to avoid bloating HTML
// Larger CSS files are left as external links to benefit from caching
func (m *Minifier) inlineCriticalCSS(htmlContent string) string {
	const maxInlineSize = 10 * 1024 // 10KB threshold - conservative to avoid breaking things

	// Match <link rel="stylesheet" href="..."> or <link rel=stylesheet href=...>
	// Handles both quoted and unquoted attributes
	linkRe := regexp.MustCompile(`<link\s+[^>]*rel=["']?stylesheet["']?[^>]*>`)

	var inlinedCSS []string
	firstLinkReplaced := false

	matches := linkRe.FindAllString(htmlContent, -1)
	for _, link := range matches {
		// Extract href - handles quoted and unquoted
		hrefRe := regexp.MustCompile(`href=["']?([^"'\s>]+)["']?`)
		hrefMatch := hrefRe.FindStringSubmatch(link)
		if hrefMatch == nil {
			continue
		}
		href := hrefMatch[1]

		// Convert URL to local path
		cssPath := m.urlToLocalPath(href)
		if cssPath == "" {
			// External or unresolvable CSS - keep the link
			continue
		}

		// Read the CSS file
		cssContent, err := os.ReadFile(cssPath)
		if err != nil {
			// Can't read file - keep the link
			continue
		}

		// Check size threshold
		if len(cssContent) > maxInlineSize {
			// Too large to inline - keep as external link
			continue
		}

		// Minify the CSS before inlining
		minifiedCSS, err := m.minify.String("text/css", string(cssContent))
		if err != nil {
			minifiedCSS = string(cssContent)
		}

		// Apply font reference rewrites
		minifiedCSS = rewriteFontReferences(minifiedCSS)

		inlinedCSS = append(inlinedCSS, minifiedCSS)

		// Remove this link from HTML
		// Replace first inlined link with the style tag, remove others
		if !firstLinkReplaced && len(inlinedCSS) > 0 {
			// Mark position for later replacement
			htmlContent = strings.Replace(htmlContent, link, "<!--INLINE_CSS_PLACEHOLDER-->", 1)
			firstLinkReplaced = true
		} else {
			htmlContent = strings.Replace(htmlContent, link, "", 1)
		}
	}

	// Replace placeholder with combined inline styles
	if len(inlinedCSS) > 0 {
		combinedCSS := strings.Join(inlinedCSS, "")
		styleTag := fmt.Sprintf("<style>%s</style>", combinedCSS)
		htmlContent = strings.Replace(htmlContent, "<!--INLINE_CSS_PLACEHOLDER-->", styleTag, 1)
	}

	return htmlContent
}

// urlToLocalPath converts a CSS URL to a local file path
// Returns empty string if the URL can't be resolved to a local file
func (m *Minifier) urlToLocalPath(href string) string {
	// Handle absolute URLs with our domain
	// e.g., https://www.amazingcto.com/css/main.min.xxx.css -> css/main.min.xxx.css
	if strings.HasPrefix(href, "https://www.amazingcto.com/") {
		href = strings.TrimPrefix(href, "https://www.amazingcto.com/")
	} else if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		// External CSS - can't inline
		return ""
	}

	// Handle root-relative paths
	href = strings.TrimPrefix(href, "/")

	// Construct full path from source directory
	localPath := filepath.Join(m.config.Source, href)

	// Verify the file exists
	if _, err := os.Stat(localPath); err != nil {
		return ""
	}

	return localPath
}

// rewriteFontReferences updates CSS to use WOFF2 and adds font-display: swap
func rewriteFontReferences(cssContent string) string {
	// Replace .ttf with .woff2 in font URLs
	// Matches: url('/fonts/name.ttf') or url("/fonts/name.ttf") or url(/fonts/name.ttf)
	ttfRe := regexp.MustCompile(`url\((['"]?)([^'"()]+)\.ttf(['"]?)\)`)
	cssContent = ttfRe.ReplaceAllString(cssContent, `url($1$2.woff2$3)`)

	// Replace format('truetype') with format('woff2')
	cssContent = strings.ReplaceAll(cssContent, `format('truetype')`, `format('woff2')`)
	cssContent = strings.ReplaceAll(cssContent, `format("truetype")`, `format("woff2")`)

	// Add font-display: swap to @font-face blocks that don't have it
	// Match @font-face { ... } blocks
	fontFaceRe := regexp.MustCompile(`(@font-face\s*\{)([^}]*)(})`)
	cssContent = fontFaceRe.ReplaceAllStringFunc(cssContent, func(match string) string {
		if strings.Contains(match, "font-display") {
			return match // Already has font-display
		}
		// Insert font-display: swap after the opening brace
		return fontFaceRe.ReplaceAllString(match, `$1$2font-display:swap;$3`)
	})

	return cssContent
}

// minifyCommand is the entry point for the minify command
func minifyCommand(source, target, cacheDir string, force bool, exclude []string) error {
	config := MinifyConfig{
		Source:   source,
		Target:   target,
		CacheDir: cacheDir,
		Force:    force,
		Exclude:  exclude,
	}

	m := NewMinifier(config)
	return m.Run()
}
