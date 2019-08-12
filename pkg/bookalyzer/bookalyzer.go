package bookalyzer

import "regexp"

var fileNameRE = regexp.MustCompile(`[^a-z0-9A-Z.\-_~]`)

// GetFilePath gets a filepath for the file.
func GetFilePath(url string) string {
	return fileNameRE.ReplaceAllLiteralString(url, "-")
}
