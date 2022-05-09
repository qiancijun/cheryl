package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadConfig(t *testing.T) {
	config, err := ReadConfig("config.yaml")
	assert.NoError(t, err)
	port := config.Port
	assert.Equal(t, 9119, port)
}