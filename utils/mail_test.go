package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendMail(t *testing.T) {
	err := SendMail([]string{"hb@epeijing.cn"}, "subject", "body")
	assert.Equal(t, nil, err)
}
