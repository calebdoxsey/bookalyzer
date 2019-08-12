package bookalyzer

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetFilePath(t *testing.T) {
	assert.Equal(t, "http---www.google.com", GetFilePath("http://www.google.com"))
}
