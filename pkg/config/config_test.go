package config_test

import (
	"context"
	"testing"

	"autoshop/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnv(t *testing.T) {
	t.Setenv("AUTOSHOP_TESTE", "valor")
	assert.Equal(t, "valor", config.Env("AUTOSHOP_TESTE", "padrao"))
	assert.Equal(t, "padrao", config.Env("AUTOSHOP_INEXISTENTE", "padrao"))
}

func TestLoadAWSConfig(t *testing.T) {
	t.Setenv("AWS_REGION", "sa-east-1")
	cfg, err := config.LoadAWSConfig(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "sa-east-1", cfg.Region)
}
