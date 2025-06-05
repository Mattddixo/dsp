package config

// Default configuration values
const (
	// DefaultDataDir is the default directory for DSP metadata
	// Using a simple string literal for the constant
	DefaultDataDir = ".dsp"

	// DefaultHashAlgorithm is the default algorithm for file hashing
	DefaultHashAlgorithm = "blake3"

	// DefaultCompressionLevel is the default compression level (1-9)
	DefaultCompressionLevel = 6

	// DefaultSigningEnabled determines if signing is enabled by default
	DefaultSigningEnabled = false
)

// ValidHashAlgorithms contains the list of supported hash algorithms
var ValidHashAlgorithms = []string{
	"blake3",
	"sha256",
	"sha512",
}

// ValidCompressionLevels defines the valid range for compression levels
const (
	MinCompressionLevel = 1
	MaxCompressionLevel = 9
)
