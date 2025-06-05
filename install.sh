#!/bin/bash

# Exit on any error
set -e

# Function to print error messages and exit
error() {
    echo "Error: $1" >&2
    exit 1
}

# Function to detect OS
detect_os() {
    case "$(uname -s)" in
        "Darwin")
            echo "macos"
            ;;
        "Linux")
            echo "linux"
            ;;
        "MINGW"*|"MSYS"*|"CYGWIN"*)
            echo "windows"
            ;;
        *)
            error "Unsupported operating system: $(uname -s)"
            ;;
    esac
}

# Function to detect shell
detect_shell() {
    local shell_type
    shell_type=$(basename "$SHELL")
    
    case "$shell_type" in
        "zsh")
            if [[ "$(detect_os)" == "macos" ]]; then
                echo "$HOME/.zshrc"
            else
                echo "$HOME/.zshrc"
            fi
            ;;
        "bash")
            if [[ "$(detect_os)" == "windows" ]]; then
                # Check for Git Bash specific files
                if [[ -f "$HOME/.bash_profile" ]]; then
                    echo "$HOME/.bash_profile"
                else
                    echo "$HOME/.bashrc"
                fi
            else
                if [[ -f "$HOME/.bash_profile" ]]; then
                    echo "$HOME/.bash_profile"
                elif [[ -f "$HOME/.bashrc" ]]; then
                    echo "$HOME/.bashrc"
                else
                    echo "$HOME/.bash_profile"
                fi
            fi
            ;;
        *)
            error "Unsupported shell: $shell_type"
            ;;
    esac
}

# Function to check if Go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        error "Go is not installed. Please install Go 1.21 or later first."
    fi
    
    # Check Go version
    local go_version
    go_version=$(go version | awk '{print $3}' | sed 's/go//')
    local required_version="1.21"
    
    if [[ "$(printf '%s\n' "$required_version" "$go_version" | sort -V | head -n1)" != "$required_version" ]]; then
        error "Go version $go_version is too old. Please install Go 1.21 or later."
    fi
}

# Function to check if directory is writable
check_writable() {
    if [[ ! -w "$1" ]]; then
        error "Cannot write to $1. Please check permissions."
    fi
}

# Function to add to PATH if not already present
add_to_path() {
    local profile_file="$1"
    local go_bin="$2"
    
    # Create profile file if it doesn't exist
    if [[ ! -f "$profile_file" ]]; then
        touch "$profile_file" || error "Cannot create $profile_file"
    fi
    
    # Check if PATH already includes Go bin
    if ! grep -q "export PATH=.*$go_bin" "$profile_file"; then
        echo "" >> "$profile_file"
        echo "# Added by DSP installer $(date '+%Y-%m-%d %H:%M:%S')" >> "$profile_file"
        echo "export PATH=\$PATH:$go_bin" >> "$profile_file"
        echo "PATH updated in $profile_file"
    else
        echo "Go bin directory already in PATH in $profile_file"
    fi
}

# Main installation process
main() {
    echo "DSP Installer"
    echo "============="
    
    # Check Go installation
    echo "Checking Go installation..."
    check_go
    
    # Detect OS and shell
    echo "Detecting environment..."
    local os_type
    os_type=$(detect_os)
    echo "OS: $os_type"
    
    local profile_file
    profile_file=$(detect_shell)
    echo "Shell profile: $profile_file"
    
    # Check if profile file is writable
    check_writable "$(dirname "$profile_file")"
    
    # Install DSP
    echo "Installing DSP..."
    go install github.com/Mattddixo/dsp/cmd/dsp@latest || error "Failed to install DSP"
    
    # Get Go bin directory
    local go_bin
    go_bin=$(go env GOPATH)/bin
    if [[ ! -d "$go_bin" ]]; then
        error "Go bin directory not found: $go_bin"
    fi
    
    # Add to PATH
    add_to_path "$profile_file" "$go_bin"
    
    # Add to current session
    export PATH=$PATH:$go_bin
    
    # Verify installation
    if command -v dsp &> /dev/null; then
        echo "DSP installed successfully!"
        echo "Try running: dsp --version"
    else
        echo "Installation complete, but 'dsp' command not found in current session."
        echo "Please restart your terminal or run:"
        echo "  source $profile_file"
    fi
}

# Run main function
main 