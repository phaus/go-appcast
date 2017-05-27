package appcast

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/jarcoal/httpmock.v1"
)

var testdataPath = "./testdata/"

// getWorkingDir returns a current working directory path. If it's not available
// prints an error to os.Stdout and exits with error status 1.
func getWorkingDir() string {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	return pwd
}

// getTestdata returns a file content as a byte array from provided testdata
// filename. If file not found, prints an error to os.Stdout and exits with exit
// status 1.
func getTestdata(filename string) []byte {
	path := filepath.Join(getWorkingDir(), testdataPath, filename)
	content, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(fmt.Errorf(err.Error()))
		os.Exit(1)
	}

	return content
}

// ReadLine reads a provided line number from io.Reader and returns it alongside
// with an error. Error should be "nil", if the line has been retrieved
// successfully.
func readLine(r io.Reader, lineNum int) (line string, err error) {
	var lastLine int

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		lastLine++
		if lastLine == lineNum {
			return sc.Text(), nil
		}
	}

	return "", fmt.Errorf("There is no line \"%d\" in specified io.Reader", lineNum)
}

// getLineFromString returns a specified line from the passed string content and
// an error. Error should be "nil", if the line has been retrieved successfully.
func getLineFromString(lineNum int, content string) (line string, err error) {
	r := bytes.NewReader([]byte(content))

	return readLine(r, lineNum)
}

func TestNew(t *testing.T) {
	a := New()
	assert.IsType(t, BaseAppcast{}, *a)
	assert.Equal(t, Unknown, a.Provider)
}

func TestLoadFromURL(t *testing.T) {
	// mock the request
	content := string(getTestdata("sparkle_default.xml"))
	httpmock.Activate()
	httpmock.RegisterResponder("GET", "https://example.com/appcast.xml", httpmock.NewStringResponder(200, content))
	defer httpmock.DeactivateAndReset()

	// test (successful)
	a := New()
	err := a.LoadFromURL("https://example.com/appcast.xml")
	assert.Nil(t, err)
	assert.NotEmpty(t, a.Content)
	assert.Equal(t, SparkleRSSFeed, a.Provider)
	assert.Empty(t, a.Checksum.Result)

	// test "Invalid URL" error
	a = New()
	err = a.LoadFromURL("http://192.168.0.%31/")
	assert.Error(t, err)
	assert.Equal(t, "parse http://192.168.0.%31/: invalid URL escape \"%31\"", err.Error())
	assert.Equal(t, Unknown, a.Provider)
	assert.Empty(t, a.Checksum.Result)

	// test "Invalid request" error
	a = New()
	err = a.LoadFromURL("invalid")
	assert.Error(t, err)
	assert.Equal(t, "Get invalid: no responder found", err.Error())
	assert.Equal(t, Unknown, a.Provider)
	assert.Empty(t, a.Checksum.Result)
}

func TestGenerateChecksum(t *testing.T) {
	// preparations
	a := New()
	a.Content = "test"

	// before
	assert.Equal(t, Sha256, a.Checksum.Algorithm)
	assert.Empty(t, a.Checksum.Result)

	// test
	result := a.GenerateChecksum(Md5)
	assert.Equal(t, "098f6bcd4621d373cade4e832627b4f6", result)
	assert.Equal(t, "098f6bcd4621d373cade4e832627b4f6", a.Checksum.Result)
	assert.Equal(t, Md5, a.Checksum.Algorithm)
}

func TestGetChecksum(t *testing.T) {
	// preparations
	a := New()
	a.Content = "test"
	a.GenerateChecksum(Sha256)

	// test
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", a.GetChecksum())
}

func TestUncommentUnknown(t *testing.T) {
	// preparations
	a := New()

	// test
	err := a.Uncomment()
	assert.Error(t, err)
	assert.Equal(t, "Uncommenting is not available for \"Unknown\" provider", err.Error())
}

func TestUncommentSparkleRSSFeed(t *testing.T) {
	// preparations
	a := New()
	regexCommentStart := regexp.MustCompile(`<!--([[:space:]]*)?<`)
	regexCommentEnd := regexp.MustCompile(`>([[:space:]]*)?-->`)

	// test
	a.Content = string(getTestdata("sparkle_with_comments.xml"))
	a.Provider = SparkleRSSFeed
	err := a.Uncomment()
	assert.Nil(t, err)

	for _, commentLine := range []int{13, 20} {
		line, _ := getLineFromString(commentLine, a.Content)
		check := (regexCommentStart.MatchString(line) && regexCommentEnd.MatchString(line))
		assert.False(t, check)
	}
}

func TestUncommentSourceForgeRSSFeed(t *testing.T) {
	// preparations
	a := New()

	// test
	a.Content = string(getTestdata("sourceforge_default.xml"))
	a.Provider = SourceForgeRSSFeed
	err := a.Uncomment()
	assert.Error(t, err)
	assert.Equal(t, "Uncommenting is not available for \"SourceForge RSS Feed\" provider", err.Error())
}

func TestUncommentGitHubAtomFeed(t *testing.T) {
	// preparations
	a := New()

	// test
	a.Content = string(getTestdata("github_default.xml"))
	a.Provider = GitHubAtomFeed
	err := a.Uncomment()
	assert.Error(t, err)
	assert.Equal(t, "Uncommenting is not available for \"GitHub Atom Feed\" provider", err.Error())
}

func TestExtractReleasesUnknown(t *testing.T) {
	// preparations
	a := New()

	// provider "Unknown"
	err := a.ExtractReleases()
	assert.Error(t, err)
	assert.Equal(t, "Releases can't be extracted from \"Unknown\" provider", err.Error())
}

func TestExtractReleasesSparkleRSSFeed(t *testing.T) {
	testCases := map[string]map[string]interface{}{
		"sparkle_attributes_as_elements.xml": {
			"checksum": "8c42d7835109ff61fe85bba66a44689773e73e0d773feba699bceecefaf09359",
			"releases": 4,
		},
		"sparkle_default_asc.xml": {
			"checksum": "9f94a728eab952284b47cc52acfbbb64de71f3d38e5b643d1f3523ef84495d9f",
			"releases": 4,
		},
		"sparkle_default.xml": {
			"checksum": "83c1fd76a250dd50334db793a0db5da7575fc83d292c7c58fd9d31d5bcef6566",
			"releases": 4,
		},
		"sparkle_incorrect_namespace.xml": {
			"checksum": "2e66ef346c49a8472bf8bf26e6e778c5b4d494723223c84c35d9f272a7792430",
			"releases": 4,
		},
		"sparkle_invalid_pubdate.xml": {
			"checksum": "e0273ccbce5a6fb6a5fe31b5edffb8173d88afa308566cf9b4373f3fed909705",
			"releases": 4,
		},
		// "sparkle_multiple_enclosure.xml": {
		// 	"checksum": "48fc8531b253c5d3ed83abfe040edeeafb327d103acbbacf12c2288769dc80b9",
		// 	"releases": 4,
		// },
		"sparkle_no_releases.xml": {
			"checksum": "befd99d96be280ca7226c58ef1400309905ad20d2723e69e829cf050e802afcf",
			"releases": 0,
		},
		"sparkle_only_version.xml": {
			"checksum": "5c3e7cf62383d4c0e10e5ec0f7afd1a5e328137101e8b6bade050812e4e7451f",
			"releases": 4,
		},
		"sparkle_single.xml": {
			"checksum": "ac649bebe55f84d85767072e3a1122778a04e03f56b78226bd57ab50ce9f9306",
			"releases": 1,
		},
		"sparkle_without_namespaces.xml": {
			"checksum": "ee2d28f74e7d557bd7259c0f24a261658a9f27a710308a5c539ab761dae487c1",
			"releases": 4,
		},
	}

	errorTestCases := map[string]string{
		"sparkle_invalid_version.xml": "Malformed version: invalid",
		"sparkle_with_comments.xml":   "Version is required, but it's not specified in release #1",
	}

	// preparations for mocking the request
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// test (successful)
	for filename, data := range testCases {
		// mock the request
		content := string(getTestdata(filename))
		httpmock.RegisterResponder("GET", "https://example.com/appcast.xml", httpmock.NewStringResponder(200, content))

		// preparations
		a := New()
		assert.Equal(t, Unknown, a.Provider)
		assert.Empty(t, a.Content)
		assert.Empty(t, a.Checksum.Source)
		assert.Empty(t, a.Checksum.Result)
		assert.Len(t, a.Releases, 0)

		// load from URL
		a.LoadFromURL("https://example.com/appcast.xml")
		assert.Equal(t, SparkleRSSFeed, a.Provider)
		assert.NotEmpty(t, a.Content)
		assert.NotEmpty(t, a.Checksum.Source)
		assert.Empty(t, a.Checksum.Result)
		assert.Len(t, a.Releases, 0)

		// generate checksum
		a.GenerateChecksum(Sha256)
		assert.Equal(t, SparkleRSSFeed, a.Provider)
		assert.Equal(t, data["checksum"].(string), a.GetChecksum())

		// releases
		err := a.ExtractReleases()
		assert.Nil(t, err)
		assert.Len(t, a.Releases, data["releases"].(int), fmt.Sprintf("%s: number of releases doesn't match", filename))
	}

	// test (error)
	for filename, errorMsg := range errorTestCases {
		// mock the request
		content := string(getTestdata(filename))
		httpmock.RegisterResponder("GET", "https://example.com/appcast.xml", httpmock.NewStringResponder(200, content))

		// preparations
		a := New()
		a.LoadFromURL("https://example.com/appcast.xml")

		// test
		err := a.ExtractReleases()
		assert.Error(t, err)
		assert.Equal(t, errorMsg, err.Error())
	}
}

func TestExtractReleasesSourceForgeRSSFeed(t *testing.T) {
	testCases := map[string]map[string]interface{}{
		"sourceforge_default.xml": {
			"checksum": "c15a5e4755b424b20e3e7138c36045893aec70f9569acd5946796199c6f79596",
			"releases": 4,
		},
		"sourceforge_empty.xml": {
			"checksum": "12bbf7be638d5cf251c320aacd68c90acef450e3a9a22cc6cbfa29ffa4ee7f6a",
			"releases": 0,
		},
		"sourceforge_single.xml": {
			"checksum": "5f3df25c0979faae5b5abef266f5929f4ac6aeb4df74e054461f93e0dbc51183",
			"releases": 1,
		},
	}

	errorTestCases := map[string]string{
		"sourceforge_invalid_version.xml": "Version is required, but it's not specified in release #2",
	}

	// preparations for mocking the request
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// test (successful)
	for filename, data := range testCases {
		// mock the request
		content := string(getTestdata(filename))
		httpmock.RegisterResponder("GET", "https://example.com/appcast.xml", httpmock.NewStringResponder(200, content))

		// preparations
		a := New()
		assert.Equal(t, Unknown, a.Provider)
		assert.Empty(t, a.Content)
		assert.Empty(t, a.Checksum.Source)
		assert.Empty(t, a.Checksum.Result)
		assert.Len(t, a.Releases, 0)

		// load from URL
		a.LoadFromURL("https://example.com/appcast.xml")
		assert.Equal(t, SourceForgeRSSFeed, a.Provider)
		assert.NotEmpty(t, a.Content)
		assert.NotEmpty(t, a.Checksum.Source)
		assert.Empty(t, a.Checksum.Result)
		assert.Len(t, a.Releases, 0)

		// generate checksum
		a.GenerateChecksum(Sha256)
		assert.Equal(t, SourceForgeRSSFeed, a.Provider)
		assert.Equal(t, data["checksum"].(string), a.GetChecksum())

		// releases
		err := a.ExtractReleases()
		assert.Nil(t, err)
		assert.Len(t, a.Releases, data["releases"].(int), fmt.Sprintf("%s: number of releases doesn't match", filename))
	}

	// test (error)
	for filename, errorMsg := range errorTestCases {
		// mock the request
		content := string(getTestdata(filename))
		httpmock.RegisterResponder("GET", "https://example.com/appcast.xml", httpmock.NewStringResponder(200, content))

		// preparations
		a := New()
		a.LoadFromURL("https://example.com/appcast.xml")

		// test
		err := a.ExtractReleases()
		assert.Error(t, err)
		assert.Equal(t, errorMsg, err.Error())
	}
}

func TestSortReleasesByVersions(t *testing.T) {
	testCases := []string{
		"sparkle_attributes_as_elements.xml",
		"sparkle_default_asc.xml",
		"sparkle_default.xml",
		"sparkle_incorrect_namespace.xml",
		// "sparkle_multiple_enclosure.xml",
		"sparkle_without_namespaces.xml",
	}

	// preparations for mocking the request
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	for _, filename := range testCases {
		// mock the request
		content := string(getTestdata(filename))
		httpmock.RegisterResponder("GET", "https://example.com/appcast.xml", httpmock.NewStringResponder(200, content))

		// preparations
		a := New()
		a.LoadFromURL("https://example.com/appcast.xml")
		err := a.ExtractReleases()
		assert.Nil(t, err)

		// test (ASC)
		a.SortReleasesByVersions(ASC)
		assert.Equal(t, "1.0.0", a.Releases[0].Version.String())

		// test (DESC)
		a.SortReleasesByVersions(DESC)
		assert.Equal(t, "2.0.0", a.Releases[0].Version.String())
	}
}

func TestExtractSemanticVersions(t *testing.T) {
	testCases := map[string][]string{
		// single
		"Version 1":           nil,
		"Version 1.0":         nil,
		"Version 1.0.2":       {"1.0.2"},
		"Version 1.0.2-alpha": {"1.0.2-alpha"},
		"Version 1.0.2-beta":  {"1.0.2-beta"},
		"Version 1.0.2-dev":   {"1.0.2-dev"},
		"Version 1.0.2-rc1":   {"1.0.2-rc1"},

		// multiples
		"First is v1.0.1, second is v1.0.2, third is v1.0.3": {"1.0.1", "1.0.2", "1.0.3"},
	}

	// test
	for data, versions := range testCases {
		actual, err := ExtractSemanticVersions(data)
		if versions == nil {
			assert.Error(t, err)
			assert.Equal(t, "No semantic versions found", err.Error())
		} else {
			assert.Nil(t, err)
			assert.Equal(t, versions, actual)
		}
	}
}
