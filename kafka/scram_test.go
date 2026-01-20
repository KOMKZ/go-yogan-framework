package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSHA256_HashGenerator(t *testing.T) {
	hash := SHA256()
	assert.NotNil(t, hash)

	// Verify as SHA256
	hash.Write([]byte("test"))
	sum := hash.Sum(nil)
	assert.Len(t, sum, 32) // SHA256 output is 32 bytes
}

func TestSHA512_HashGenerator(t *testing.T) {
	hash := SHA512()
	assert.NotNil(t, hash)

	// Validate as SHA512
	hash.Write([]byte("test"))
	sum := hash.Sum(nil)
	assert.Len(t, sum, 64) // SHA512 output is 64 bytes
}

func TestXDGSCRAMClient_Begin(t *testing.T) {
	client := &XDGSCRAMClient{
		HashGeneratorFcn: SHA256,
	}

	err := client.Begin("username", "password", "")
	assert.NoError(t, err)
	assert.NotNil(t, client.Client)
	assert.NotNil(t, client.ClientConversation)
}

func TestXDGSCRAMClient_Begin_SHA512(t *testing.T) {
	client := &XDGSCRAMClient{
		HashGeneratorFcn: SHA512,
	}

	err := client.Begin("username", "password", "")
	assert.NoError(t, err)
	assert.NotNil(t, client.Client)
}

func TestXDGSCRAMClient_Done(t *testing.T) {
	client := &XDGSCRAMClient{
		HashGeneratorFcn: SHA256,
	}

	err := client.Begin("username", "password", "")
	assert.NoError(t, err)

	// The conversation has not yet been completed after starting
	assert.False(t, client.Done())
}

func TestXDGSCRAMClient_Step(t *testing.T) {
	client := &XDGSCRAMClient{
		HashGeneratorFcn: SHA256,
	}

	err := client.Begin("username", "password", "")
	assert.NoError(t, err)

	// First Step Generate client's first message
	response, err := client.Step("")
	assert.NoError(t, err)
	assert.NotEmpty(t, response)
}

