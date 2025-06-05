# Create main project directories
$directories = @(
    "cmd/dsp",
    "internal/snapshot",
    "internal/bundle",
    "internal/crypto",
    "internal/storage",
    "pkg/protocol",
    "pkg/utils",
    "config"
)

# Create all directories
foreach ($dir in $directories) {
    New-Item -ItemType Directory -Path $dir -Force
    Write-Host "Created directory: $dir"
}

# Create empty Go files
$files = @(
    "cmd/dsp/main.go",
    "internal/snapshot/snapshot.go",
    "internal/snapshot/state.go",
    "internal/bundle/bundle.go",
    "internal/bundle/diff.go",
    "internal/bundle/apply.go",
    "internal/crypto/encryption.go",
    "internal/crypto/signing.go",
    "internal/storage/metadata.go",
    "internal/storage/files.go",
    "pkg/protocol/types.go",
    "pkg/protocol/constants.go",
    "pkg/utils/hashing.go",
    "pkg/utils/compression.go",
    "config/config.go",
    "config/defaults.go"
)

# Create all files
foreach ($file in $files) {
    New-Item -ItemType File -Path $file -Force
    Write-Host "Created file: $file"
}

# Create .env.example
$envExample = @"
# DSP Configuration
DSP_DATA_DIR=.dsp
DSP_HASH_ALGORITHM=blake3
DSP_COMPRESSION_LEVEL=6
DSP_ENCRYPTION_ENABLED=false
DSP_SIGNING_ENABLED=false

# Optional: Path to GPG key for signing
# DSP_SIGNING_KEY_PATH=~/.gnupg/private.key

# Optional: Path to age key for encryption
# DSP_ENCRYPTION_KEY_PATH=~/.config/dsp/age.key
"@

Set-Content -Path ".env.example" -Value $envExample
Write-Host "Created .env.example"

# Create README.md
$readme = @"
# DSP (Disconnected Sync Protocol)

A protocol to synchronize folders and data across disconnected or intermittently connected devices — Git-like, but for everything.

## Features

- Track directory state through hashes
- Generate and apply diffs
- Work offline
- Sync any kind of file or folder
- Secure bundle transport

## Getting Started

1. Install Go 1.21 or later
2. Clone this repository
3. Copy .env.example to .env and configure as needed
4. Run \`go mod tidy\` to install dependencies
5. Build with \`go build ./cmd/dsp\`

## Usage

\`\`\`bash
# Initialize a new DSP repository
dsp init

# Take a snapshot of current state
dsp snapshot

# Create a bundle of changes
dsp bundle

# Apply a bundle
dsp apply bundle.dspbundle
\`\`\`

## License

MIT
"@

Set-Content -Path "README.md" -Value $readme
Write-Host "Created README.md"

Write-Host "`nProject structure setup complete! Next steps:"
Write-Host "1. Install Go if you haven't already"
Write-Host "2. Run 'go mod init github.com/yourusername/dsp'"
Write-Host "3. Copy .env.example to .env and configure as needed"
Write-Host "4. Start implementing the core functionality" 