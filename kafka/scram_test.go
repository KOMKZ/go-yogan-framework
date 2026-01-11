package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSHA256_HashGenerator(t *testing.T) {
	hash := SHA256()
	assert.NotNil(t, hash)

	// 验证是 SHA256
	hash.Write([]byte("test"))
	sum := hash.Sum(nil)
	assert.Len(t, sum, 32) // SHA256 输出 32 字节
}

func TestSHA512_HashGenerator(t *testing.T) {
	hash := SHA512()
	assert.NotNil(t, hash)

	// 验证是 SHA512
	hash.Write([]byte("test"))
	sum := hash.Sum(nil)
	assert.Len(t, sum, 64) // SHA512 输出 64 字节
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

	// 在开始后，对话尚未完成
	assert.False(t, client.Done())
}

func TestXDGSCRAMClient_Step(t *testing.T) {
	client := &XDGSCRAMClient{
		HashGeneratorFcn: SHA256,
	}

	err := client.Begin("username", "password", "")
	assert.NoError(t, err)

	// 第一次 Step 生成客户端第一条消息
	response, err := client.Step("")
	assert.NoError(t, err)
	assert.NotEmpty(t, response)
}

