package main

import (
	"testing"

	"github.com/MboyaDan/dakota-college/backend/internal/config"
)

func TestCheckAllowed(t *testing.T) {
	tests := []struct {
		env     config.Env
		command string
		confirm bool
		wantErr bool
	}{
		{config.EnvDevelopment, "reset", false, false},
		{config.EnvStaging, "up", false, false},
		{config.EnvProduction, "status", false, false},
		{config.EnvProduction, "up", false, true},
		{config.EnvProduction, "down", false, true},
		{config.EnvProduction, "up", true, false},
		{config.EnvProduction, "down", true, false},
		{config.EnvProduction, "reset", true, true},
	}
	for _, tt := range tests {
		err := checkAllowed(tt.env, tt.command, tt.confirm)
		if (err != nil) != tt.wantErr {
			t.Errorf("checkAllowed(%s, %s, confirm=%v) = %v, wantErr %v", tt.env, tt.command, tt.confirm, err, tt.wantErr)
		}
	}
}
