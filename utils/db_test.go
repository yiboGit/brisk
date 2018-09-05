package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var db2 = GetDB(false)

// func TestDbQuery(t *testing.T) {
// 	var result AppIdToken
// 	db.Select(&result, "SELECT appid as Appid, refresh_token as RefreshToken FROM n_wechat_open limit 1")
// 	assert.Equal(t, AppIdToken{Appid:"wx7116d2e8f61c5c3e", RefreshToken:"refreshtoken@@@MK-YigRru_DAUjSMk67nzYZBjTrbGKjbDXcDIKbKSdc"}, result)
// }

func TestQuerySingle(t *testing.T) {
	var result string
	error := db2.QuerySingle(&result, "SELECT appid as RefreshToken FROM n_wechat_open_refresh where appid = ? ", "wxe5e1ebb94f05c1ae")
	if error != nil {
		panic(error)
	}
	assert.Equal(t, "", result)
}
