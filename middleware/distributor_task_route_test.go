package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelRequestTaskCreateRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/task/create", strings.NewReader(`{"model":"doubao-seedance-2.0"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	req, shouldSelectChannel, err := getModelRequest(c)

	require.NoError(t, err)
	require.NotNil(t, req)
	assert.True(t, shouldSelectChannel)
	assert.Equal(t, "doubao-seedance-2.0", req.Model)
	assert.Equal(t, relayconstant.RelayModeVideoSubmit, c.GetInt("relay_mode"))
}

func TestGetModelRequestTaskGetRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/task/get/task_test", nil)

	req, shouldSelectChannel, err := getModelRequest(c)

	require.NoError(t, err)
	require.NotNil(t, req)
	assert.False(t, shouldSelectChannel)
	assert.Empty(t, req.Model)
	assert.Equal(t, relayconstant.RelayModeVideoFetchByID, c.GetInt("relay_mode"))
}

func TestGetModelRequestTaskCancelRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/task/cancel/task_test", nil)

	req, shouldSelectChannel, err := getModelRequest(c)

	require.NoError(t, err)
	require.NotNil(t, req)
	assert.False(t, shouldSelectChannel)
	assert.Empty(t, req.Model)
	assert.Equal(t, relayconstant.RelayModeVideoFetchByID, c.GetInt("relay_mode"))
}
