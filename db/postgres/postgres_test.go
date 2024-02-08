package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeStringToken(t *testing.T) {
	assert := assert.New(t)
	token := makeStringToken()
	t.Log("token:", token)
	assert.True(len(token) >= (AUTH_TOKEN_LENGTH * 4 / 3))
}
