# iWork Converter with Box Integration

A comprehensive tool for converting Apple iWork files (Pages, Numbers, Keynote) to HTML and text formats, with integrated Box cloud storage support for automated batch processing.

## Features

- **Local File Conversion**: Convert individual iWork files to HTML or text
- **Box Cloud Integration**: Automatically discover and process iWork files from Box folders
- **Multiple Format Support**: Pages (.pages), Numbers (.numbers), Keynote (.keynote/.key)
- **OCR Capabilities**: Extract text from embedded images using Tesseract
- **Batch Processing**: Process entire Box folders with detailed reporting
- **Legacy Support**: Works with Pages'08, Pages'09, and iOS '.pages-tef' formats

## Quick Start

### Build the Enhanced Converter

```bash
# Clone the repository
git clone https://github.com/orcastor/iwork-converter.git
cd iwork-converter

# Build the enhanced converter with Box support
go build -o iwork-converter main.go
```

## Usage

### Single File Conversion (Original Functionality)

```bash
# Convert to HTML (default)
./iwork-converter document.pages document.html
./iwork-converter spreadsheet.numbers spreadsheet.html
./iwork-converter presentation.keynote presentation.html

# Convert to text
./iwork-converter document.pages document.txt
./iwork-converter presentation.key presentation.txt
```

### Box Cloud Processing (New Feature)

#### Basic Box Usage

```bash
# Process all iWork files in Box root folder
./iwork-converter -box -token="your_box_access_token"

# Process specific Box folder
./iwork-converter -box -token="your_token" -folder="folder_id"

# Convert to HTML format
./iwork-converter -box -token="your_token" -format="html"

# Custom output directory
./iwork-converter -box -token="your_token" -output="./my_extracts"
```

#### Box Configuration Options

| Flag | Description | Default |
|------|-------------|---------|
| `-box` | Enable Box processing mode | false |
| `-token` | Box API access token | (required) |
| `-folder` | Box folder ID | "0" (root) |
| `-format` | Output format: "txt" or "html" | "txt" |
| `-output` | Output directory | "./extracted" |
| `-temp` | Temporary directory | "./temp" |

#### Environment Variables (Alternative)

```bash
export BOX_ACCESS_TOKEN="your_access_token"
export BOX_FOLDER_ID="folder_id"
export OUTPUT_DIR="./output"
export TEMP_DIR="./temp"

# Then run without flags
./iwork-converter -box
```

## Box API Setup

1. Go to [Box Developer Console](https://developer.box.com/)
2. Create a new "Custom App"
3. Choose "Standard OAuth 2.0" authentication
4. Generate a Developer Token for testing
5. For production, implement full OAuth 2.0 flow

## Output Examples

### Text Output with Metadata

```
# Extracted from: MyDocument.pages
# File ID: 123456789
# Size: 1048576 bytes
# Modified: 2025-06-27T10:30:00Z
# Extracted: 2025-06-27T14:30:22Z
# Extension: .pages

--------------------------------------------------

[Extracted document text content...]
```

### Processing Report

```json
{
  "total_files": 5,
  "successful": 4,
  "failed": 1,
  "errors": ["Failed to convert corrupted.pages: invalid format"],
  "processed_files": [
    {
      "original_file": "document.pages",
      "output_path": "./extracted/document_extracted.txt",
      "file_size": 2048,
      "process_time": "1.2s"
    }
  ],
  "start_time": "2025-06-27T14:30:00Z",
  "end_time": "2025-06-27T14:35:22Z",
  "duration": "5m22s"
}
```

## Advanced Features

### OCR Support for Embedded Images

The converter supports OCR extraction from embedded images using Tesseract:

```go
func ConvertString(in string, ocr func(io.Reader) (string, error)) (string, error)
```

#### Example OCR Implementation

```go
import "github.com/danlock/gogosseract"

var tess *gogosseract.Tesseract

func InitTesseractOCR(model string) error {
    trainingDataFile, err := os.Open(model)
    if err != nil {
        return err
    }

    ctx := context.TODO()
    cfg := gogosseract.Config{
        Language:     "eng",
        TrainingData: trainingDataFile,
    }
    cfg.Stderr = io.Discard
    cfg.Stdout = io.Discard
    
    tess, err = gogosseract.New(ctx, cfg)
    if err != nil {
        return err
    }
    return nil
}

func OCR(reader io.Reader) (string, error) {
    if tess == nil {
        return "", fmt.Errorf("tesseract not initialized")
    }

    ctx := context.TODO()
    err := tess.LoadImage(ctx, reader, gogosseract.LoadImageOptions{})
    if err != nil {
        return "", err
    }

    text, err := tess.GetText(ctx, nil)
    if err != nil {
        return "", err
    }
    return text, nil
}
```

## Supported File Formats

### Current iWork Formats
- **Pages**: `.pages` documents
- **Numbers**: `.numbers` spreadsheets  
- **Keynote**: `.keynote` and `.key` presentations

### Legacy Formats
- **Pages'08**: Bundle format directories
- **Pages'09**: Zip-based format
- **iOS Pages**: `.pages-tef` bundles for iCloud

### Format Compatibility Notes

- Works best with iWork files from 2013-2019
- Some newer Keynote files may show "Unknown type" warnings (normal behavior)
- Files using unsupported compression may fail with "snap header" errors

## Technical Architecture

### Core Components

1. **File Format Parsing**: Based on [Sean Patrick O'Brien's iWorkFileFormat](https://github.com/obriensp/iWorkFileFormat)
2. **Protobuf Decoding**: Handles snappy-compressed protobuf records in `.iwa` files
3. **Box API Integration**: RESTful API client for cloud file access
4. **Batch Processing**: Concurrent file processing with error handling

### Dependencies

- Go 1.19+
- Standard library only (no external dependencies for Box integration)
- Optional: [gogosseract](https://github.com/danlock/gogosseract) for OCR

## Legacy XSLT Support

For Pages'08 and Pages'09 files, use the included XSLT transformer:

```bash
# Apply to Pages'08 bundle
xsltproc pages08tohtml.xsl /path/to/document.pages/index.xml.gz

# Apply to Pages'09 zip file  
xsltproc pages08tohtml.xsl /path/to/extracted/index.xml
```

## Troubleshooting

### Common Issues

1. **Box Authentication Errors (401)**
   - Verify access token is valid and not expired
   - Check Box app permissions and scopes

2. **File Conversion Errors**
   - "Unknown type" warnings are normal for newer files
   - "snap header" errors indicate unsupported compression
   - Try with different file versions or formats

3. **Permission Errors**
   - Ensure read/write access to temp and output directories
   - Verify Box folder access permissions

### Getting Help

- Check processing reports for detailed error information
- Enable verbose logging for debugging
- Test with simple documents first
- Verify file format compatibility

## Contributing

This project builds upon the excellent work of [Sean Patrick O'Brien](https://github.com/obriensp/iWorkFileFormat) for iWork file format reverse engineering. The Box integration extends the original converter to support cloud-based batch processing workflows.

## License

[Include your license information here]
