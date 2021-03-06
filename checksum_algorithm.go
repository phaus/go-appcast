package appcast

// ChecksumAlgorithm holds different available checksum algorithms.
type ChecksumAlgorithm int

const (
	// SHA256 represents a SHA256 checksum
	SHA256 ChecksumAlgorithm = iota

	// SHA256HomebrewCask represents a SHA256 checksum used in Homebrew-Cask
	SHA256HomebrewCask

	// MD5 represents an MD5 checksum
	MD5
)

var checksumAlgorithmNames = [...]string{
	"SHA256",
	"SHA256 (Homebrew-Cask checkpoint)",
	"MD5",
}

// String returns a string representation of the ChecksumAlgorithm.
func (a ChecksumAlgorithm) String() string {
	return checksumAlgorithmNames[a]
}
