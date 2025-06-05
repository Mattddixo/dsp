package config

// DefaultConfigYAML is the embedded default configuration
const DefaultConfigYAML = `# DSP Configuration
# This is the default configuration that will be used for new repositories

# Directory where DSP stores its metadata
dsp_dir: .dsp

# Directory where DSP stores its data
data_dir: .dsp/data

# Hash algorithm to use for file hashing
# Supported algorithms: blake3, sha256, sha512
hash_algorithm: blake3

# Compression level for bundles (1-9)
# 1 = fastest, 9 = best compression
compression_level: 6

# Enable signing for bundles
signing_enabled: false

# Path to GPG private key for signing (required if signing_enabled is true)
# signing_key_path: ~/.gnupg/private.key

# Path to age encryption key (required if encryption_enabled is true)
# encryption_key_path: ~/.config/dsp/age.key
`
