# DSP (Disconnected Sync Protocol)

A secure protocol for synchronizing data across disconnected or intermittently connected devices — Git-like, but for everything. DSP provides robust offline-first synchronization with strong security guarantees.

## Features

### Core Functionality
- Track directory state through cryptographic hashes
- Generate and apply diffs efficiently
- Work completely offline
- Sync any kind of file or folder
- Secure bundle transport with TLS encryption

### Security Features
- Password-based and user-based authentication
- One-time token system for secure downloads
- TLS encryption for all transfers
- Certificate-based host verification
- Multiple recipient encryption support
- Key exchange protocol for trusted hosts
- Bundle integrity verification

### Bundle Transfer
- Secure server for bundle distribution
- Multiple download support with token tracking
- Download limits and expiration
- Progress tracking and status monitoring
- Automatic server shutdown after completion

## Getting Started

### Prerequisites
- Go 1.21 or later
- Git (for version control)

### Installation

#### Automatic Install (Recommended)
```bash
# Download and run the installer script
curl -sSL https://raw.githubusercontent.com/Mattddixo/dsp/main/install.sh | bash
```

#### Quick Install
```bash
# Install the latest version
go install github.com/Mattddixo/dsp/cmd/dsp@latest

# Make sure your Go bin directory is in your PATH
# For Linux/macOS:
export PATH=$PATH:$(go env GOPATH)/bin

# For Windows (Git Bash):
export PATH=$PATH:/c/Users/$USER/go/bin
```

#### From Source
1. Clone this repository:
   ```bash
   git clone https://github.com/Mattddixo/dsp.git
   cd dsp
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Build and install:
   ```bash
   go install ./cmd/dsp
   ```

4. Make sure your Go bin directory is in your PATH (see Quick Install above)

### Verifying Installation
```bash
# Check if dsp is installed
which dsp

# Test the command
dsp --version
```

## Usage

### Basic Operations

```bash
# Initialize a new DSP repository
dsp init

# Take a snapshot of current state
dsp snapshot

# Create a bundle of changes
dsp bundle

# Apply a bundle
dsp apply bundle.dspbundle
```

### Secure Export and Import

```bash
# Export a bundle with password protection
dsp export -p "your-password" -f bundle.zip bundle.json

# Export with user authentication
dsp export -u "user1,user2" -f bundle.zip bundle.json

# Export with download limit
dsp export -p "your-password" -n 5 -f bundle.zip bundle.json

# Import a bundle
dsp import -p "your-password" export-info.json
```

### Security Options

- Password Authentication: Encrypts bundles with password + one-time tokens
- User Authentication: Simple user-based access control
- Certificate Management: Automatic certificate generation and verification
- Key Exchange: Secure key exchange for trusted hosts
- Download Limits: Control number of allowed downloads
- Token Expiration: Automatic token expiry for security

## Architecture

### Components
- Bundle Management: Handles bundle creation, verification, and application
- Export Server: Secure HTTP server for bundle distribution
- Import Client: Secure client for bundle retrieval
- Crypto Manager: Handles encryption, signing, and key management
- Host Manager: Manages trusted hosts and certificates

### Security Model
- All transfers use TLS encryption
- Bundles are encrypted for multiple recipients
- One-time tokens prevent replay attacks
- Certificate verification ensures server authenticity
- Key exchange establishes trusted relationships

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see LICENSE file for details
