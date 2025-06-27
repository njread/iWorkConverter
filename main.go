package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/orcastor/iwork-converter/iwork2html"
	"github.com/orcastor/iwork-converter/iwork2text"
)

// BoxConfig holds Box API configuration
type BoxConfig struct {
	AccessToken string
	BaseURL     string
	FolderID    string
}

// FileInfo represents a Box file
type FileInfo struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	Extension  string    `json:"extension"`
	ModifiedAt time.Time `json:"modified_at"`
}

// ProcessingSummary tracks processing results
type ProcessingSummary struct {
	TotalFiles     int             `json:"total_files"`
	Successful     int             `json:"successful"`
	Failed         int             `json:"failed"`
	Errors         []string        `json:"errors"`
	ProcessedFiles []ProcessedFile `json:"processed_files"`
	StartTime      time.Time       `json:"start_time"`
	EndTime        time.Time       `json:"end_time"`
	Duration       time.Duration   `json:"duration"`
}

// ProcessedFile represents a successfully processed file
type ProcessedFile struct {
	OriginalFile string `json:"original_file"`
	OutputPath   string `json:"output_path"`
	FileSize     int64  `json:"file_size"`
	ProcessTime  string `json:"process_time"`
}

// BoxClient handles Box API interactions
type BoxClient struct {
	config     BoxConfig
	httpClient *http.Client
}

// NewBoxClient creates a new Box API client
func NewBoxClient(accessToken, folderID string) *BoxClient {
	return &BoxClient{
		config: BoxConfig{
			AccessToken: accessToken,
			BaseURL:     "https://api.box.com/2.0",
			FolderID:    folderID,
		},
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetIWorkFiles retrieves iWork files from specified Box folder
func (bc *BoxClient) GetIWorkFiles() ([]FileInfo, error) {
	url := fmt.Sprintf("%s/folders/%s/items", bc.config.BaseURL, bc.config.FolderID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+bc.config.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := bc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Box API error: %s", resp.Status)
	}

	var result struct {
		Entries []struct {
			ID         string    `json:"id"`
			Name       string    `json:"name"`
			Type       string    `json:"type"`
			Size       int64     `json:"size"`
			ModifiedAt time.Time `json:"modified_at"`
		} `json:"entries"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var iworkFiles []FileInfo
	supportedExtensions := map[string]bool{
		".pages":   true,
		".numbers": true,
		".keynote": true,
		".key":     true, // Keynote files can have .key extension
		".nmbrs":   true, // Numbers files can have .nmbrs extension (rare)
	}

	for _, entry := range result.Entries {
		if entry.Type == "file" {
			ext := strings.ToLower(filepath.Ext(entry.Name))
			if supportedExtensions[ext] {
				iworkFiles = append(iworkFiles, FileInfo{
					ID:         entry.ID,
					Name:       entry.Name,
					Size:       entry.Size,
					Extension:  ext,
					ModifiedAt: entry.ModifiedAt,
				})
			}
		}
	}

	log.Printf("Found %d iWork files in Box folder %s", len(iworkFiles), bc.config.FolderID)
	return iworkFiles, nil
}

// DownloadFile downloads a file from Box to local temp directory
func (bc *BoxClient) DownloadFile(fileID, filename string, tempDir string) (string, error) {
	url := fmt.Sprintf("%s/files/%s/content", bc.config.BaseURL, fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+bc.config.AccessToken)

	resp, err := bc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create temp file path
	tempPath := filepath.Join(tempDir, filename)

	// Create the file
	file, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	// Copy response body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("Downloaded %s to %s", filename, tempPath)
	return tempPath, nil
}

// IWorkProcessor handles the text extraction process
type IWorkProcessor struct {
	boxClient *BoxClient
	tempDir   string
	outputDir string
	format    string // "txt" or "html"
}

// NewIWorkProcessor creates a new processor instance
func NewIWorkProcessor(boxClient *BoxClient, tempDir, outputDir, format string) *IWorkProcessor {
	return &IWorkProcessor{
		boxClient: boxClient,
		tempDir:   tempDir,
		outputDir: outputDir,
		format:    format,
	}
}

// ConvertIWorkFile converts an iWork file using the existing converters
func (p *IWorkProcessor) ConvertIWorkFile(inputPath, outputPath string) error {
	switch p.format {
	case "txt":
		return iwork2text.Convert(inputPath, outputPath)
	default:
		return iwork2html.Convert(inputPath, outputPath)
	}
}

// SaveFileWithMetadata saves the converted file with metadata header (for text files)
func (p *IWorkProcessor) SaveFileWithMetadata(originalPath, convertedPath string, metadata FileInfo) (string, error) {
	if p.format != "txt" {
		// For HTML files, just return the converted path as-is
		return convertedPath, nil
	}

	// For text files, add metadata header
	convertedContent, err := os.ReadFile(convertedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read converted file: %w", err)
	}

	// Create output content with metadata header
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("# Extracted from: %s\n", metadata.Name))
	buffer.WriteString(fmt.Sprintf("# File ID: %s\n", metadata.ID))
	buffer.WriteString(fmt.Sprintf("# Size: %d bytes\n", metadata.Size))
	buffer.WriteString(fmt.Sprintf("# Modified: %s\n", metadata.ModifiedAt.Format(time.RFC3339)))
	buffer.WriteString(fmt.Sprintf("# Extracted: %s\n", time.Now().Format(time.RFC3339)))
	buffer.WriteString(fmt.Sprintf("# Extension: %s\n", metadata.Extension))
	buffer.WriteString("\n")
	buffer.WriteString(strings.Repeat("-", 50))
	buffer.WriteString("\n\n")
	buffer.Write(convertedContent)

	// Write enhanced content back to file
	err = os.WriteFile(convertedPath, buffer.Bytes(), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write enhanced file: %w", err)
	}

	log.Printf("Enhanced %s with metadata", convertedPath)
	return convertedPath, nil
}

// ProcessFolder processes all iWork files in the specified Box folder
func (p *IWorkProcessor) ProcessFolder() (*ProcessingSummary, error) {
	summary := &ProcessingSummary{
		StartTime:      time.Now(),
		Errors:         []string{},
		ProcessedFiles: []ProcessedFile{},
	}

	// Get iWork files from Box
	iworkFiles, err := p.boxClient.GetIWorkFiles()
	if err != nil {
		return summary, fmt.Errorf("failed to get iWork files: %w", err)
	}

	summary.TotalFiles = len(iworkFiles)

	if len(iworkFiles) == 0 {
		log.Println("No iWork files found to process")
		return summary, nil
	}

	// Process each file
	for _, fileInfo := range iworkFiles {
		log.Printf("Processing %s...", fileInfo.Name)
		processStart := time.Now()

		// Download file
		localPath, err := p.boxClient.DownloadFile(fileInfo.ID, fileInfo.Name, p.tempDir)
		if err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("Failed to download %s: %v", fileInfo.Name, err))
			continue
		}

		// Determine output file path
		baseName := strings.TrimSuffix(fileInfo.Name, filepath.Ext(fileInfo.Name))
		var outputPath string
		if p.format == "txt" {
			outputPath = filepath.Join(p.outputDir, fmt.Sprintf("%s_extracted.txt", baseName))
		} else {
			outputPath = filepath.Join(p.outputDir, fmt.Sprintf("%s_converted.html", baseName))
		}

		// Convert file using existing converters
		err = p.ConvertIWorkFile(localPath, outputPath)
		if err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("Failed to convert %s: %v", fileInfo.Name, err))
			// Clean up downloaded file
			os.Remove(localPath)
			continue
		}

		// Add metadata (for text files)
		finalPath, err := p.SaveFileWithMetadata(localPath, outputPath, fileInfo)
		if err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("Failed to save enhanced file for %s: %v", fileInfo.Name, err))
		} else {
			summary.Successful++
			summary.ProcessedFiles = append(summary.ProcessedFiles, ProcessedFile{
				OriginalFile: fileInfo.Name,
				OutputPath:   finalPath,
				FileSize:     fileInfo.Size,
				ProcessTime:  time.Since(processStart).String(),
			})
		}

		// Clean up downloaded file
		os.Remove(localPath)
	}

	summary.EndTime = time.Now()
	summary.Duration = summary.EndTime.Sub(summary.StartTime)

	return summary, nil
}

// GenerateReport creates a JSON report of the processing results
func (p *IWorkProcessor) GenerateReport(summary *ProcessingSummary) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	reportPath := filepath.Join(p.outputDir, fmt.Sprintf("processing_report_%s.json", timestamp))

	reportData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal report: %w", err)
	}

	err = os.WriteFile(reportPath, reportData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write report: %w", err)
	}

	log.Printf("Generated processing report: %s", reportPath)
	return reportPath, nil
}

// setupDirectories creates necessary directories
func setupDirectories(tempDir, outputDir string) error {
	dirs := []string{tempDir, outputDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// showUsage displays usage information
func showUsage() {
	fmt.Printf(`iwork-converter - Converts iWork files to html/json/txt

Usage:
    # Convert single file
    %s infile.pages outfile.html
    %s infile.pages outfile.txt

    # Process Box folder
    %s -box -token=BOX_TOKEN [options]

Box Options:
    -box                 Enable Box mode
    -token=TOKEN         Box API access token (required for Box mode)
    -folder=ID           Box folder ID (default: "0" for root)
    -format=FORMAT       Output format: "txt" or "html" (default: "txt")
    -output=DIR          Output directory (default: "./extracted")
    -temp=DIR            Temp directory (default: "./temp")

Environment Variables (alternative to flags):
    BOX_ACCESS_TOKEN     Box API access token
    BOX_FOLDER_ID        Box folder ID
    OUTPUT_DIR           Output directory
    TEMP_DIR             Temp directory

Examples:
    # Convert single file
    %s document.pages document.html
    %s document.pages document.txt

    # Process entire Box folder
    %s -box -token=your_token
    %s -box -token=your_token -folder=123456 -format=html

`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func main() {
	// Define flags for Box mode
	var (
		boxMode   = flag.Bool("box", false, "Enable Box processing mode")
		boxToken  = flag.String("token", "", "Box API access token")
		folderID  = flag.String("folder", "", "Box folder ID (default: root folder)")
		format    = flag.String("format", "txt", "Output format: txt or html")
		outputDir = flag.String("output", "", "Output directory")
		tempDir   = flag.String("temp", "", "Temporary directory")
		help      = flag.Bool("help", false, "Show help")
	)

	flag.Parse()

	// Show help if requested
	if *help {
		showUsage()
		return
	}

	// Box mode processing
	if *boxMode {
		// Get configuration from flags or environment
		token := *boxToken
		if token == "" {
			token = os.Getenv("BOX_ACCESS_TOKEN")
		}
		if token == "" {
			fmt.Println("Error: Box access token required. Use -token flag or BOX_ACCESS_TOKEN environment variable.")
			showUsage()
			return
		}

		folder := *folderID
		if folder == "" {
			folder = os.Getenv("BOX_FOLDER_ID")
		}
		if folder == "" {
			folder = "0" // Default to root folder
		}

		output := *outputDir
		if output == "" {
			output = os.Getenv("OUTPUT_DIR")
		}
		if output == "" {
			output = "./extracted"
		}

		temp := *tempDir
		if temp == "" {
			temp = os.Getenv("TEMP_DIR")
		}
		if temp == "" {
			temp = "./temp"
		}

		// Validate format
		if *format != "txt" && *format != "html" {
			fmt.Printf("Error: Invalid format '%s'. Must be 'txt' or 'html'.\n", *format)
			return
		}

		// Setup directories
		if err := setupDirectories(temp, output); err != nil {
			log.Fatalf("Failed to setup directories: %v", err)
		}

		// Initialize components
		boxClient := NewBoxClient(token, folder)
		processor := NewIWorkProcessor(boxClient, temp, output, *format)

		// Process folder
		fmt.Printf("Starting Box iWork processing...\n")
		fmt.Printf("Folder ID: %s\n", folder)
		fmt.Printf("Output format: %s\n", *format)
		fmt.Printf("Output directory: %s\n", output)

		summary, err := processor.ProcessFolder()
		if err != nil {
			log.Fatalf("Processing failed: %v", err)
		}

		// Generate report
		reportPath, err := processor.GenerateReport(summary)
		if err != nil {
			log.Printf("Failed to generate report: %v", err)
		}

		// Display summary
		fmt.Printf("\n%s\n", strings.Repeat("=", 50))
		fmt.Printf("PROCESSING SUMMARY\n")
		fmt.Printf("%s\n", strings.Repeat("=", 50))
		fmt.Printf("Total files found: %d\n", summary.TotalFiles)
		fmt.Printf("Successfully processed: %d\n", summary.Successful)
		fmt.Printf("Failed: %d\n", summary.Failed)
		fmt.Printf("Processing time: %s\n", summary.Duration)

		if len(summary.Errors) > 0 {
			fmt.Printf("\nErrors:\n")
			for _, err := range summary.Errors {
				fmt.Printf("  - %s\n", err)
			}
		}

		if reportPath != "" {
			fmt.Printf("\nDetailed report: %s\n", reportPath)
		}
		fmt.Printf("Output files: %s\n", output)

		return
	}

	// Original single file conversion mode
	if len(os.Args) < 3 {
		showUsage()
		return
	}

	// Use existing conversion logic
	switch {
	case strings.HasSuffix(os.Args[2], ".txt"):
		if err := iwork2text.Convert(os.Args[1], os.Args[2]); err != nil {
			panic(err)
		}
	default:
		if err := iwork2html.Convert(os.Args[1], os.Args[2]); err != nil {
			panic(err)
		}
	}
}
